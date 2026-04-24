package models

import (
	"time"

	"gorm.io/gorm"
)

// Mistake is one chemistry problem the student got wrong (or asked help on).
// Persistent record for a personal 错题本; spaced-repetition scheduling +
// AI-generated variants attach to it.
type Mistake struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	UserID     uint           `gorm:"not null;index" json:"user_id"`
	ImageURL   string         `gorm:"size:512" json:"image_url"` // OSS URL of the photo
	OCRText    string         `gorm:"type:text" json:"ocr_text"` // structured text the LLM extracted
	Subject    string         `gorm:"size:32;default:'chemistry'" json:"subject"`
	// Concept tag — e.g. "redox-balance" / "ion-equation" / "organic-substitution".
	// Free-text from the LLM; we don't enforce a fixed taxonomy yet.
	Concept    string         `gorm:"size:64;index" json:"concept"`
	// AI's tier-1 hint, tier-2 partial-solution, tier-3 full solution. Stored
	// once at create time so re-reads are cheap and the model stays consistent.
	HintLevel1 string         `gorm:"type:text" json:"hint_level_1"`
	HintLevel2 string         `gorm:"type:text" json:"hint_level_2"`
	HintLevel3 string         `gorm:"type:text" json:"hint_level_3"` // full solution
	// Spaced-repetition next-review timestamp. NULL means "don't schedule".
	NextReviewAt *time.Time   `gorm:"index" json:"next_review_at,omitempty"`
	ReviewCount  int          `gorm:"default:0" json:"review_count"`
	// LastResult: "correct" / "wrong" / "skipped" — drives next interval.
	LastResult string         `gorm:"size:16" json:"last_result"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Mistake) TableName() string { return "mistakes" }

// Variant is a fresh problem the AI generated targeting the same concept as
// a Mistake — used during spaced-repetition reviews to test true mastery
// (rather than re-asking the exact same question, which the student may have
// memorised by rote).
type Variant struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	MistakeID  uint           `gorm:"not null;index" json:"mistake_id"`
	UserID     uint           `gorm:"not null;index" json:"user_id"`
	Question   string         `gorm:"type:text;not null" json:"question"`
	Answer     string         `gorm:"type:text" json:"answer"` // canonical answer for grading
	// User's submission + grading verdict for this variant.
	UserAnswer string         `gorm:"type:text" json:"user_answer"`
	Result     string         `gorm:"size:16" json:"result"` // correct / wrong / skipped
	CreatedAt  time.Time      `json:"created_at"`
	GradedAt   *time.Time     `json:"graded_at,omitempty"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Variant) TableName() string { return "variants" }
