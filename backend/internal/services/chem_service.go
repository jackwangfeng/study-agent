package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackwangfeng/study-agent/backend/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ChemService talks to a DashScope (Aliyun Qwen) OpenAI-compatible
// chat-completions endpoint. We deliberately use the OpenAI-compatible API
// path (NOT DashScope's native protocol) so swapping in DeepSeek / Doubao /
// any other OpenAI-compatible provider later is one env var change.
type ChemService struct {
	db        *gorm.DB
	logger    *zap.Logger
	apiKey    string
	baseURL   string // e.g. https://dashscope.aliyuncs.com/compatible-mode/v1
	model     string // e.g. qwen-vl-max-2024-11-19
	allowMock bool
}

func NewChemService(db *gorm.DB, logger *zap.Logger, apiKey, baseURL, model string, allowMock bool) *ChemService {
	if baseURL == "" {
		baseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	}
	if model == "" {
		model = "qwen-vl-max" // multimodal default; switch to qwen2.5-vl-72b-instruct if account has it.
	}
	return &ChemService{db: db, logger: logger, apiKey: apiKey, baseURL: baseURL, model: model, allowMock: allowMock}
}

// chemSystemPrompt is the tutor persona. Pinned anchors:
//   - Chinese 高二 (senior high year 2) curriculum
//   - 人教版 default — when the student says otherwise we adapt at runtime.
//   - Three-tier hint ladder: NEVER skip to full solution unless explicitly asked.
//   - Chemistry equations rendered with `mhchem` LaTeX (`\ce{H2O}`) for the
//     frontend KaTeX renderer.
const chemSystemPrompt = `你是高二学生的化学辅导助教，目标是帮他真正掌握知识，不是替他做题。

教材版本：默认人教版（必修 1+2、选修 4 化学反应原理、选修 5 有机化学基础），如果学生提到其他版本（鲁科 / 苏教 / 沪科），按对应版本回答。

## 回答原则

1. **不直接给完整答案**。除非学生明确说"我已经放弃，给我答案"，否则按"提示阶梯"逐级推进：
   - **第一级提示**：只给一个方向上的关键词或问题（"先想想这是什么类型的反应？"），不超过 30 字。
   - **第二级提示**：给关键步骤或公式，不写出最后结果。
   - **第三级**：完整解答 + 步骤拆解 + 易错点标注。
2. **化学方程式用 LaTeX mhchem 语法**：例如 \( \ce{2Na + 2H2O -> 2NaOH + H2 ^} \)，离子方程式用 \( \ce{Na+ + ...} \)。前端会用 KaTeX + mhchem 渲染。
3. **指出他错在哪一步**，不只说"你错了"。
4. **总结这道题考的具体知识点**，粒度细到"必修 1 第三章氧化还原中的电子守恒配平"，不要笼统说"化学反应"。
5. 用简洁的中文，避免 emoji 和"加油哦"之类的话。`

type chemRequest struct {
	Model    string        `json:"model"`
	Messages []chemMessage `json:"messages"`
}

type chemMessage struct {
	Role    string         `json:"role"`
	Content []chemContent `json:"content,omitempty"`
	// Some models accept a plain string content; for assistant messages we
	// fall back to that to avoid forcing every response into a parts array.
	StringContent string `json:"-"`
}

type chemContent struct {
	Type     string         `json:"type"`
	Text     string         `json:"text,omitempty"`
	ImageURL *chemImageURL `json:"image_url,omitempty"`
}

type chemImageURL struct {
	URL string `json:"url"`
}

type chemResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// SolveProblem sends an image + optional text question to the LLM and gets
// back the tutor reply (which itself contains the three hint tiers).
func (s *ChemService) SolveProblem(ctx context.Context, imageDataURL, userText string) (string, error) {
	if s.apiKey == "" {
		if s.allowMock {
			return mockChemReply(userText), nil
		}
		return "", errors.New("DashScope API key not set")
	}

	user := []chemContent{}
	if imageDataURL != "" {
		user = append(user, chemContent{Type: "image_url", ImageURL: &chemImageURL{URL: imageDataURL}})
	}
	if userText != "" {
		user = append(user, chemContent{Type: "text", Text: userText})
	} else {
		user = append(user, chemContent{Type: "text", Text: "请按提示阶梯帮我分析这道化学题。"})
	}

	body := chemRequest{
		Model: s.model,
		Messages: []chemMessage{
			{Role: "system", Content: []chemContent{{Type: "text", Text: chemSystemPrompt}}},
			{Role: "user", Content: user},
		},
	}
	buf, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		s.logger.Error("DashScope non-200",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(respBody)))
		return "", fmt.Errorf("LLM upstream error %d", resp.StatusCode)
	}

	var parsed chemResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("LLM returned no choices")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

// LogMistake records the problem as a mistake in the user's 错题本 and
// schedules the first spaced-repetition review for 3 days out (Anki-ish
// initial interval; subsequent intervals are computed when the variant is
// graded).
func (s *ChemService) LogMistake(userID uint, imageURL, ocr, concept, fullSolution string) (*models.Mistake, error) {
	next := time.Now().Add(72 * time.Hour)
	m := &models.Mistake{
		UserID:       userID,
		ImageURL:     imageURL,
		OCRText:      ocr,
		Concept:      concept,
		HintLevel3:   fullSolution,
		NextReviewAt: &next,
	}
	if err := s.db.Create(m).Error; err != nil {
		return nil, err
	}
	return m, nil
}

// ListMistakes returns the user's recent mistakes, newest first.
func (s *ChemService) ListMistakes(userID uint, limit int) ([]models.Mistake, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var ms []models.Mistake
	err := s.db.Where("user_id = ?", userID).
		Order("created_at DESC").Limit(limit).Find(&ms).Error
	return ms, err
}

// DueForReview returns mistakes with NextReviewAt <= now — used by the daily
// "today's review" feed.
func (s *ChemService) DueForReview(userID uint) ([]models.Mistake, error) {
	var ms []models.Mistake
	err := s.db.Where("user_id = ? AND next_review_at IS NOT NULL AND next_review_at <= ?",
		userID, time.Now()).
		Order("next_review_at ASC").Find(&ms).Error
	return ms, err
}

func mockChemReply(text string) string {
	return "（mock 模式 — 没配 DashScope API key）你的问题：" + text + "\n\n第一级提示：先判断这是什么类型的反应（氧化还原？酸碱？复分解？）。"
}
