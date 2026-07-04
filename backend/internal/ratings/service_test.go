package ratings

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type mockRatingRepository struct {
	history []RatingHistory
	failIns bool
}

func newMockRatingRepository() *mockRatingRepository {
	return &mockRatingRepository{
		history: []RatingHistory{},
	}
}

func (m *mockRatingRepository) InsertRatingHistory(ctx context.Context, tx pgx.Tx, rh RatingHistory) error {
	if m.failIns {
		return errors.New("database error insert")
	}
	m.history = append(m.history, rh)
	return nil
}

func TestService_ApplyRatingUpdatesTx(t *testing.T) {
	p1ID := uuid.New()
	p2ID := uuid.New()
	battleID := uuid.New()

	t.Run("successful win rating update", func(t *testing.T) {
		repo := newMockRatingRepository()
		engine := NewEloEngine(32)
		service := NewService(repo, engine)

		result := MatchResult{
			BattleID: battleID,
			PlayerA: PlayerRating{
				UserID: p1ID,
				Rating: Rating{Value: 1000},
			},
			PlayerB: PlayerRating{
				UserID: p2ID,
				Rating: Rating{Value: 1000},
			},
			Outcome: Player1Win,
		}

		up1, up2, err := service.ApplyRatingUpdatesTx(context.Background(), nil, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if up1.After.Value != 1016 {
			t.Errorf("expected player 1 rating to be 1016, got %d", up1.After.Value)
		}
		if up2.After.Value != 984 {
			t.Errorf("expected player 2 rating to be 984, got %d", up2.After.Value)
		}

		if len(repo.history) != 2 {
			t.Fatalf("expected 2 rating history entries, got %d", len(repo.history))
		}

		// check history details
		h1 := repo.history[0]
		if h1.UserID != p1ID || h1.RatingBefore != 1000 || h1.RatingAfter != 1016 || h1.Delta != 16 {
			t.Errorf("invalid history entry 1: %+v", h1)
		}

		h2 := repo.history[1]
		if h2.UserID != p2ID || h2.RatingBefore != 1000 || h2.RatingAfter != 984 || h2.Delta != -16 {
			t.Errorf("invalid history entry 2: %+v", h2)
		}
	})

	t.Run("invalid outcome returns error", func(t *testing.T) {
		repo := newMockRatingRepository()
		engine := NewEloEngine(32)
		service := NewService(repo, engine)

		result := MatchResult{
			BattleID: battleID,
			PlayerA: PlayerRating{
				UserID: p1ID,
				Rating: Rating{Value: 1000},
			},
			PlayerB: PlayerRating{
				UserID: p2ID,
				Rating: Rating{Value: 1000},
			},
			Outcome: MatchOutcome(999),
		}

		_, _, err := service.ApplyRatingUpdatesTx(context.Background(), nil, result)
		if !errors.Is(err, ErrInvalidOutcome) {
			t.Errorf("expected ErrInvalidOutcome, got %v", err)
		}
	})

	t.Run("repository error on insert history", func(t *testing.T) {
		repo := newMockRatingRepository()
		repo.failIns = true
		engine := NewEloEngine(32)
		service := NewService(repo, engine)

		result := MatchResult{
			BattleID: battleID,
			PlayerA: PlayerRating{
				UserID: p1ID,
				Rating: Rating{Value: 1000},
			},
			PlayerB: PlayerRating{
				UserID: p2ID,
				Rating: Rating{Value: 1000},
			},
			Outcome: Player1Win,
		}

		_, _, err := service.ApplyRatingUpdatesTx(context.Background(), nil, result)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
