package questions

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// QuestionType represents the kind of question.
type QuestionType string

const (
	TypeMCQ                  QuestionType = "mcq"
	TypeComplexityPrediction QuestionType = "complexity_prediction"
	TypeNumericAnswer        QuestionType = "numeric_answer"
	TypeAlgorithmOrdering    QuestionType = "algorithm_ordering"
)

// Question represents the full question entity stored in the database.
type Question struct {
	ID            uuid.UUID    `json:"id" db:"id"`
	QuestionType  QuestionType `json:"question_type" db:"question_type"`
	Difficulty    int          `json:"difficulty" db:"difficulty"`
	Title         string       `json:"title" db:"title"`
	Prompt        string       `json:"prompt" db:"prompt"`
	Options       []string     `json:"options" db:"options"` // JSONB array of strings
	CorrectAnswer string       `json:"correct_answer" db:"correct_answer"`
	Explanation   string       `json:"explanation,omitempty" db:"explanation"`
	TimeLimitSec  int          `json:"time_limit_sec" db:"time_limit_sec"`
	Tags          []string     `json:"tags" db:"tags"` // TEXT[] Array of tags
	Source        string       `json:"source,omitempty" db:"source"`
	CreatedBy     *uuid.UUID   `json:"created_by,omitempty" db:"created_by"`
	IsActive      bool         `json:"is_active" db:"is_active"`
	CreatedAt     time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at" db:"updated_at"`
}

// SanitizedQuestionResponse represents the client-facing question data.
// It completely omits correct_answer and explanation for anti-cheating security.
type SanitizedQuestionResponse struct {
	ID           uuid.UUID    `json:"id"`
	QuestionType QuestionType `json:"question_type"`
	Difficulty   int          `json:"difficulty"`
	Title         string       `json:"title"`
	Prompt       string       `json:"prompt"`
	Options      []string     `json:"options,omitempty"`
	TimeLimitSec int          `json:"time_limit_sec"`
	Tags         []string     `json:"tags"`
}

// Validate checks core domain invariants of the Question.
func (q *Question) Validate() error {
	if q.ID == uuid.Nil {
		return errors.New("question ID cannot be nil")
	}

	switch q.QuestionType {
	case TypeMCQ, TypeComplexityPrediction, TypeNumericAnswer, TypeAlgorithmOrdering:
		// Valid types
	default:
		return fmt.Errorf("unsupported question type: %s", q.QuestionType)
	}

	if q.Difficulty < 1 || q.Difficulty > 5 {
		return fmt.Errorf("difficulty must be between 1 and 5, got %d", q.Difficulty)
	}

	if q.Title == "" {
		return errors.New("title cannot be empty")
	}

	if q.Prompt == "" {
		return errors.New("prompt cannot be empty")
	}

	if q.TimeLimitSec < 10 || q.TimeLimitSec > 120 {
		return fmt.Errorf("time limit must be between 10 and 120 seconds, got %d", q.TimeLimitSec)
	}

	return nil
}
