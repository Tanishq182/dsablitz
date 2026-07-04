package ratings

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// InsertRatingHistory inserts a new rating history record.
func (r *Repository) InsertRatingHistory(ctx context.Context, tx pgx.Tx, rh RatingHistory) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO rating_history (id, user_id, battle_id, rating_before, rating_after, delta, reason, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
	`, rh.ID, rh.UserID, rh.BattleID, rh.RatingBefore, rh.RatingAfter, rh.Delta, rh.Reason)
	if err != nil {
		return fmt.Errorf("insert rating history: %w", err)
	}
	return nil
}
