package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackwangfeng/study-agent/backend/internal/services"
	"go.uber.org/zap"
)

type ChemHandler struct {
	service *services.ChemService
	logger  *zap.Logger
}

func NewChemHandler(s *services.ChemService, logger *zap.Logger) *ChemHandler {
	return &ChemHandler{service: s, logger: logger}
}

// SolveRequest accepts EITHER an inline base64 image data URL OR a hosted URL.
// For MVP we accept both — frontend will lean on data URL (camera capture
// straight to the form, no separate upload endpoint round-trip).
type SolveRequest struct {
	UserID    uint   `json:"user_id"` // optional — anonymous solves allowed during dev
	ImageData string `json:"image_data,omitempty"` // data:image/jpeg;base64,...
	ImageURL  string `json:"image_url,omitempty"`  // public URL alternative
	Question  string `json:"question,omitempty"`   // optional follow-up text
}

func (h *ChemHandler) Solve(c *gin.Context) {
	var req SolveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	imgRef := req.ImageData
	if imgRef == "" {
		imgRef = req.ImageURL
	}
	if imgRef == "" && req.Question == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "需要图片或文字问题"})
		return
	}

	answer, err := h.service.SolveProblem(c.Request.Context(), imgRef, req.Question)
	if err != nil {
		h.logger.Error("chem solve failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"answer": answer})
}

// LogMistake records a problem the student got wrong (or asked help on)
// into their 错题本. The frontend calls this AFTER the student says "I got
// it wrong / I want to remember this" — not automatically on every solve.
func (h *ChemHandler) LogMistake(c *gin.Context) {
	var req struct {
		UserID       uint   `json:"user_id" binding:"required"`
		ImageURL     string `json:"image_url"`
		OCRText      string `json:"ocr_text"`
		Concept      string `json:"concept"`
		FullSolution string `json:"full_solution"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	m, err := h.service.LogMistake(req.UserID, req.ImageURL, req.OCRText, req.Concept, req.FullSolution)
	if err != nil {
		h.logger.Error("log mistake failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusCreated, m)
}

func (h *ChemHandler) ListMistakes(c *gin.Context) {
	uidStr := c.Query("user_id")
	if uidStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 user_id"})
		return
	}
	uid, err := strconv.ParseUint(uidStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id 无效"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	ms, err := h.service.ListMistakes(uint(uid), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"mistakes": ms, "count": len(ms)})
}

func (h *ChemHandler) DueReview(c *gin.Context) {
	uidStr := c.Query("user_id")
	uid, err := strconv.ParseUint(uidStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id 无效"})
		return
	}
	ms, err := h.service.DueForReview(uint(uid))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"mistakes": ms, "count": len(ms)})
}
