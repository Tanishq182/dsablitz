package battle

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// BattleStatus represents the state of a battle.
type BattleStatus string

const (
	StatusPending   BattleStatus = "pending"
	StatusActive    BattleStatus = "active"
	StatusCompleted BattleStatus = "completed"
)

// MaxQuestionStreamSize defines the maximum number of pre-generated questions in a stream.
const MaxQuestionStreamSize = 200

// Battle represents the battle entity stored in the database.
type Battle struct {
	ID         uuid.UUID    `db:"id"`
	RoomID     uuid.UUID    `db:"room_id"`
	Status     BattleStatus `db:"status"`
	BattleSeed int64        `db:"battle_seed"`
	CreatedAt  time.Time    `db:"created_at"`
	UpdatedAt  time.Time    `db:"updated_at"`
}

// Validate checks core structural invariants for a Battle.
func (b *Battle) Validate() error {
	if b.ID == uuid.Nil {
		return errors.New("battle ID cannot be nil")
	}
	if b.RoomID == uuid.Nil {
		return errors.New("room ID cannot be nil")
	}
	switch b.Status {
	case StatusPending, StatusActive, StatusCompleted:
		// Valid status
	default:
		return fmt.Errorf("unsupported battle status: %s", b.Status)
	}
	if b.BattleSeed == 0 {
		return errors.New("battle seed cannot be zero")
	}
	return nil
}

// BattlePlayer represents a participant inside a battle.
type BattlePlayer struct {
	ID                      uuid.UUID  `db:"id"`
	BattleID                uuid.UUID  `db:"battle_id"`
	UserID                  uuid.UUID  `db:"user_id"`
	SeatNumber              int16      `db:"seat_number"`
	RatingBefore            int        `db:"rating_before"`
	RatingAfter             *int       `db:"rating_after"`
	Score                   int        `db:"score"`
	CorrectCount            int        `db:"correct_count"`
	IncorrectCount          int        `db:"incorrect_count"`
	CurrentQuestionIndex    int        `db:"current_question_index"`
	CurrentQuestionAttempts int        `db:"current_question_attempts"`
	CreatedAt               time.Time  `db:"created_at"`
	UpdatedAt               time.Time  `db:"updated_at"`
}

// Validate checks core structural invariants for a BattlePlayer.
func (bp *BattlePlayer) Validate() error {
	if bp.ID == uuid.Nil {
		return errors.New("player ID cannot be nil")
	}
	if bp.BattleID == uuid.Nil {
		return errors.New("battle ID cannot be nil")
	}
	if bp.UserID == uuid.Nil {
		return errors.New("user ID cannot be nil")
	}
	if bp.SeatNumber < 1 {
		return fmt.Errorf("seat number must be positive, got %d", bp.SeatNumber)
	}
	if bp.RatingBefore <= 0 {
		return fmt.Errorf("rating must be positive, got %d", bp.RatingBefore)
	}
	if bp.CurrentQuestionIndex < 0 {
		return fmt.Errorf("current question index cannot be negative, got %d", bp.CurrentQuestionIndex)
	}
	if bp.CurrentQuestionAttempts < 0 {
		return fmt.Errorf("current question attempts cannot be negative, got %d", bp.CurrentQuestionAttempts)
	}
	return nil
}

// SubmissionResult represents the output of evaluating a user submission.
type SubmissionResult struct {
	IsCorrect            bool      `json:"is_correct"`
	AttemptsMade         int       `json:"attempts_made"`
	CurrentQuestionIndex int       `json:"current_question_index"`
	Score                int       `json:"score"`
	QuestionID           uuid.UUID `json:"question_id"`
}
