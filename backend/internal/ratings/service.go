package ratings

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Service struct {
	repo   RatingRepository
	engine RatingEngine
}

func NewService(repo RatingRepository, engine RatingEngine) *Service {
	return &Service{
		repo:   repo,
		engine: engine,
	}
}

// ApplyRatingUpdatesTx calculates rating updates, logs them to history, and returns the results.
// Implements the RatingCoordinator interface.
func (s *Service) ApplyRatingUpdatesTx(ctx context.Context, tx pgx.Tx, result MatchResult) (RatingUpdate, RatingUpdate, error) {
	// Validate outcome value
	if result.Outcome != Player1Win && result.Outcome != Player2Win && result.Outcome != Draw {
		return RatingUpdate{}, RatingUpdate{}, ErrInvalidOutcome
	}

	// Calculate rating updates using the stateless engine
	updateA, updateB := s.engine.Calculate(result.PlayerA, result.PlayerB, result.Outcome)

	// Persist the updates to rating_history.
	// Write Player A's history
	rhA := RatingHistory{
		ID:           uuid.New(),
		UserID:       result.PlayerA.UserID,
		BattleID:     result.BattleID,
		RatingBefore: updateA.Before.Value,
		RatingAfter:  updateA.After.Value,
		Delta:        updateA.Delta,
		Reason:       "battle_result",
	}
	err := s.repo.InsertRatingHistory(ctx, tx, rhA)
	if err != nil {
		return RatingUpdate{}, RatingUpdate{}, fmt.Errorf("record rating history for player A: %w", err)
	}

	// Write Player B's history
	rhB := RatingHistory{
		ID:           uuid.New(),
		UserID:       result.PlayerB.UserID,
		BattleID:     result.BattleID,
		RatingBefore: updateB.Before.Value,
		RatingAfter:  updateB.After.Value,
		Delta:        updateB.Delta,
		Reason:       "battle_result",
	}
	err = s.repo.InsertRatingHistory(ctx, tx, rhB)
	if err != nil {
		return RatingUpdate{}, RatingUpdate{}, fmt.Errorf("record rating history for player B: %w", err)
	}

	return updateA, updateB, nil
}
