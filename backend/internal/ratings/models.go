package ratings

import (
	"time"

	"github.com/google/uuid"
)

// Rating represents a player's skill rating.
type Rating struct {
	Value int
}

// PlayerRating holds a user ID and their rating.
type PlayerRating struct {
	UserID uuid.UUID
	Rating Rating
}

// MatchResult encapsulates the inputs of a resolved match.
type MatchResult struct {
	BattleID uuid.UUID
	PlayerA  PlayerRating
	PlayerB  PlayerRating
	Outcome  MatchOutcome
}

// RatingUpdate encapsulates a player's rating progression.
type RatingUpdate struct {
	Before Rating
	After  Rating
	Delta  int
}

// RatingHistory represents a record in the rating_history table.
type RatingHistory struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	BattleID     uuid.UUID
	RatingBefore int
	RatingAfter  int
	Delta        int
	Reason       string
	CreatedAt    time.Time
}
