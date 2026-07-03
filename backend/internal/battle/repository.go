package battle

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("record not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// WithTransaction runs a function inside a pgx transaction.
func (r *Repository) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// InsertBattle inserts a battle metadata row.
func (r *Repository) InsertBattle(ctx context.Context, tx pgx.Tx, b Battle) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO battles (id, room_id, status, battle_seed, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`, b.ID, b.RoomID, b.Status, b.BattleSeed)
	if err != nil {
		return fmt.Errorf("insert battle: %w", err)
	}
	return nil
}

// InsertBattlePlayers inserts all player slots.
func (r *Repository) InsertBattlePlayers(ctx context.Context, tx pgx.Tx, players []BattlePlayer) error {
	for _, p := range players {
		_, err := tx.Exec(ctx, `
			INSERT INTO battle_players (id, battle_id, user_id, seat_number, rating_before, score, correct_count, incorrect_count, current_question_index, current_question_attempts, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
		`, p.ID, p.BattleID, p.UserID, p.SeatNumber, p.RatingBefore, p.Score, p.CorrectCount, p.IncorrectCount, p.CurrentQuestionIndex, p.CurrentQuestionAttempts)
		if err != nil {
			return fmt.Errorf("insert battle player '%s': %w", p.UserID, err)
		}
	}
	return nil
}

// InsertBattleSequence writes the pre-generated sequence.
func (r *Repository) InsertBattleSequence(ctx context.Context, tx pgx.Tx, battleID uuid.UUID, sequence []uuid.UUID) error {
	for idx, qID := range sequence {
		_, err := tx.Exec(ctx, `
			INSERT INTO battle_question_sequence (battle_id, sequence_index, question_id)
			VALUES ($1, $2, $3)
		`, battleID, idx, qID)
		if err != nil {
			return fmt.Errorf("insert sequence at index %d: %w", idx, err)
		}
	}
	return nil
}

// GetBattle retrieves a battle by ID.
func (r *Repository) GetBattle(ctx context.Context, id uuid.UUID) (Battle, error) {
	var b Battle
	err := r.db.QueryRow(ctx, `
		SELECT id, room_id, status, battle_seed, created_at, updated_at
		FROM battles
		WHERE id = $1
	`, id).Scan(&b.ID, &b.RoomID, &b.Status, &b.BattleSeed, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Battle{}, ErrNotFound
		}
		return Battle{}, fmt.Errorf("get battle: %w", err)
	}
	return b, nil
}

// GetBattlePlayerForUpdate locking a single player row using FOR UPDATE.
func (r *Repository) GetBattlePlayerForUpdate(ctx context.Context, tx pgx.Tx, battleID, userID uuid.UUID) (BattlePlayer, error) {
	row := tx.QueryRow(ctx, `
		SELECT id, battle_id, user_id, seat_number, rating_before, rating_after, score, correct_count, incorrect_count, current_question_index, current_question_attempts, created_at, updated_at
		FROM battle_players
		WHERE battle_id = $1 AND user_id = $2
		FOR UPDATE
	`, battleID, userID)
	return scanBattlePlayer(row)
}

// GetBattlePlayer retrieves player details without locking.
func (r *Repository) GetBattlePlayer(ctx context.Context, battleID, userID uuid.UUID) (BattlePlayer, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, battle_id, user_id, seat_number, rating_before, rating_after, score, correct_count, incorrect_count, current_question_index, current_question_attempts, created_at, updated_at
		FROM battle_players
		WHERE battle_id = $1 AND user_id = $2
	`, battleID, userID)
	return scanBattlePlayer(row)
}


// UpdateBattlePlayer updates player stats inside a transaction.
func (r *Repository) UpdateBattlePlayer(ctx context.Context, tx pgx.Tx, p BattlePlayer) error {
	res, err := tx.Exec(ctx, `
		UPDATE battle_players
		SET score = $3,
		    correct_count = $4,
		    incorrect_count = $5,
		    current_question_index = $6,
		    current_question_attempts = $7,
		    updated_at = NOW()
		WHERE battle_id = $1 AND user_id = $2
	`, p.BattleID, p.UserID, p.Score, p.CorrectCount, p.IncorrectCount, p.CurrentQuestionIndex, p.CurrentQuestionAttempts)
	if err != nil {
		return fmt.Errorf("update battle player counters: %w", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("no player row updated for battle %s user %s", p.BattleID, p.UserID)
	}
	return nil
}

// InsertSubmission inserts a submission log inside a transaction.
func (r *Repository) InsertSubmission(ctx context.Context, tx pgx.Tx, bID, uID, qID uuid.UUID, isCorrect bool, responseTimeMs int) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO submissions (id, battle_id, user_id, question_id, is_correct, response_time_ms, created_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, NOW())
	`, bID, uID, qID, isCorrect, responseTimeMs)
	if err != nil {
		return fmt.Errorf("insert submission: %w", err)
	}
	return nil
}

// GetQuestionIDAtSequenceIndex reads the question mapping.
func (r *Repository) GetQuestionIDAtSequenceIndex(ctx context.Context, battleID uuid.UUID, index int) (uuid.UUID, error) {
	var qID uuid.UUID
	err := r.db.QueryRow(ctx, `
		SELECT question_id
		FROM battle_question_sequence
		WHERE battle_id = $1 AND sequence_index = $2
	`, battleID, index).Scan(&qID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrNotFound
		}
		return uuid.Nil, fmt.Errorf("get sequence item: %w", err)
	}
	return qID, nil
}

// CompleteBattle marks battle status to completed.
func (r *Repository) CompleteBattle(ctx context.Context, battleID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE battles
		SET status = 'completed',
		    updated_at = NOW()
		WHERE id = $1
	`, battleID)
	if err != nil {
		return fmt.Errorf("complete battle status: %w", err)
	}
	return nil
}

func scanBattlePlayer(row pgx.Row) (BattlePlayer, error) {
	var p BattlePlayer
	err := row.Scan(
		&p.ID, &p.BattleID, &p.UserID, &p.SeatNumber, &p.RatingBefore,
		&p.RatingAfter, &p.Score, &p.CorrectCount, &p.IncorrectCount,
		&p.CurrentQuestionIndex, &p.CurrentQuestionAttempts, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BattlePlayer{}, ErrNotFound
		}
		return BattlePlayer{}, err
	}
	return p, nil
}
