package ratings

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// MatchOutcome indicates the result of a match from the perspective of Player A (Player 1).
type MatchOutcome int

const (
	Player1Win MatchOutcome = iota
	Draw
	Player2Win
)

// RatingEngine defines a stateless calculator for rating changes.
type RatingEngine interface {
	Calculate(playerA, playerB PlayerRating, outcome MatchOutcome) (RatingUpdate, RatingUpdate)
}

// RatingCoordinator is the interface used by Battle to apply rating updates.
type RatingCoordinator interface {
	ApplyRatingUpdatesTx(ctx context.Context, tx pgx.Tx, result MatchResult) (RatingUpdate, RatingUpdate, error)
}

// RatingRepository defines persistence operations for the ratings module.
type RatingRepository interface {
	InsertRatingHistory(ctx context.Context, tx pgx.Tx, rh RatingHistory) error
}
