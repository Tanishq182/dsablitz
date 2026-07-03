package battle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"dsablitz/backend/internal/questions"
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
		INSERT INTO battles (id, room_id, status, duration_seconds, question_count, winner_user_id, started_at, ended_at, battle_seed, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
	`, b.ID, b.RoomID, b.Status, b.DurationSeconds, b.QuestionCount, b.WinnerUserID, b.StartedAt, b.EndedAt, b.BattleSeed)
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
		SELECT id, room_id, status, duration_seconds, question_count, winner_user_id, started_at, ended_at, battle_seed, created_at, updated_at
		FROM battles
		WHERE id = $1
	`, id).Scan(&b.ID, &b.RoomID, &b.Status, &b.DurationSeconds, &b.QuestionCount, &b.WinnerUserID, &b.StartedAt, &b.EndedAt, &b.BattleSeed, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Battle{}, ErrNotFound
		}
		return Battle{}, fmt.Errorf("get battle: %w", err)
	}
	return b, nil
}

// GetBattleTx retrieves a battle by ID inside a transaction.
func (r *Repository) GetBattleTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Battle, error) {
	var b Battle
	err := tx.QueryRow(ctx, `
		SELECT id, room_id, status, duration_seconds, question_count, winner_user_id, started_at, ended_at, battle_seed, created_at, updated_at
		FROM battles
		WHERE id = $1
	`, id).Scan(&b.ID, &b.RoomID, &b.Status, &b.DurationSeconds, &b.QuestionCount, &b.WinnerUserID, &b.StartedAt, &b.EndedAt, &b.BattleSeed, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Battle{}, ErrNotFound
		}
		return Battle{}, fmt.Errorf("get battle tx: %w", err)
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
func (r *Repository) InsertSubmission(ctx context.Context, tx pgx.Tx, bID, uID, qID uuid.UUID, answer questions.SubmissionAnswer, isCorrect bool, scoreAwarded int, responseTimeMs int) error {
	rawJSON, err := json.Marshal(answer)
	if err != nil {
		return fmt.Errorf("marshal raw answer: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO submissions (id, battle_id, user_id, question_id, raw_answer, is_correct, score_awarded, response_time_ms, submitted_at, created_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
	`, bID, uID, qID, rawJSON, isCorrect, scoreAwarded, responseTimeMs)
	if err != nil {
		return fmt.Errorf("insert submission: %w", err)
	}
	return nil
}

// GetSubmissionsForQuestion gets all submissions by a user for a specific question in a battle.
func (r *Repository) GetSubmissionsForQuestion(ctx context.Context, tx pgx.Tx, battleID, userID, questionID uuid.UUID) ([]questions.SubmissionAnswer, error) {
	rows, err := tx.Query(ctx, `
		SELECT raw_answer FROM submissions
		WHERE battle_id = $1 AND user_id = $2 AND question_id = $3
		ORDER BY submitted_at ASC
	`, battleID, userID, questionID)
	if err != nil {
		return nil, fmt.Errorf("query submissions for question: %w", err)
	}
	defer rows.Close()

	var subs []questions.SubmissionAnswer
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan submission raw_answer: %w", err)
		}
		var ans questions.SubmissionAnswer
		if err := json.Unmarshal(raw, &ans); err != nil {
			return nil, fmt.Errorf("unmarshal submission raw_answer: %w", err)
		}
		subs = append(subs, ans)
	}
	return subs, nil
}

// GetLastSubmission gets the most recent submission timestamp by a user in a battle.
func (r *Repository) GetLastSubmission(ctx context.Context, tx pgx.Tx, battleID, userID uuid.UUID) (time.Time, error) {
	var submittedAt time.Time
	err := tx.QueryRow(ctx, `
		SELECT submitted_at FROM submissions
		WHERE battle_id = $1 AND user_id = $2
		ORDER BY submitted_at DESC
		LIMIT 1
	`, battleID, userID).Scan(&submittedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, nil // No previous submission
		}
		return time.Time{}, fmt.Errorf("query last submission: %w", err)
	}
	return submittedAt, nil
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
func (r *Repository) CompleteBattle(ctx context.Context, tx pgx.Tx, battleID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		UPDATE battles
		SET status = $2,
		    updated_at = NOW()
		WHERE id = $1
	`, battleID, StatusCompleted)
	if err != nil {
		return fmt.Errorf("complete battle status: %w", err)
	}
	return nil
}

// GetPlayerQuestionState retrieves the player's progression state and mapped question ID in a single query.
func (r *Repository) GetPlayerQuestionState(ctx context.Context, battleID, userID uuid.UUID) (PlayerQuestionState, error) {
	var state PlayerQuestionState
	err := r.db.QueryRow(ctx, `
		SELECT b.status, b.ended_at, bp.current_question_index, b.question_count, COALESCE(q.question_id, '00000000-0000-0000-0000-000000000000'::uuid)
		FROM battle_players bp
		JOIN battles b ON bp.battle_id = b.id
		LEFT JOIN battle_question_sequence q ON b.id = q.battle_id AND bp.current_question_index = q.sequence_index
		WHERE bp.battle_id = $1 AND bp.user_id = $2
	`, battleID, userID).Scan(&state.BattleStatus, &state.EndedAt, &state.CurrentQuestionIndex, &state.QuestionCount, &state.QuestionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PlayerQuestionState{}, ErrNotFound
		}
		return PlayerQuestionState{}, fmt.Errorf("get player question state: %w", err)
	}
	return state, nil
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

// GetBattlePlayersTx retrieves all players inside a battle under transaction locks.
func (r *Repository) GetBattlePlayersTx(ctx context.Context, tx pgx.Tx, battleID uuid.UUID) ([]BattlePlayer, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, battle_id, user_id, seat_number, rating_before, rating_after, score, correct_count, incorrect_count, current_question_index, current_question_attempts, created_at, updated_at
		FROM battle_players
		WHERE battle_id = $1
		ORDER BY user_id ASC
		FOR UPDATE
	`, battleID)
	if err != nil {
		return nil, fmt.Errorf("query battle players tx: %w", err)
	}
	defer rows.Close()

	var players []BattlePlayer
	for rows.Next() {
		p, err := scanBattlePlayer(rows)
		if err != nil {
			return nil, fmt.Errorf("scan battle player: %w", err)
		}
		players = append(players, p)
	}
	return players, nil
}

// UpdateBattlePlayerResult persists the player's outcome.
func (r *Repository) UpdateBattlePlayerResult(ctx context.Context, tx pgx.Tx, battleID, userID uuid.UUID, result string) error {
	_, err := tx.Exec(ctx, `
		UPDATE battle_players
		SET result = $3,
		    updated_at = NOW()
		WHERE battle_id = $1 AND user_id = $2
	`, battleID, userID, result)
	if err != nil {
		return fmt.Errorf("update battle player result: %w", err)
	}
	return nil
}

// UpdateRoomStatusDirect updates rooms table status directly.
func (r *Repository) UpdateRoomStatusDirect(ctx context.Context, tx pgx.Tx, roomID uuid.UUID, status string) error {
	_, err := tx.Exec(ctx, `
		UPDATE rooms
		SET status = $2,
		    updated_at = NOW()
		WHERE id = $1
	`, roomID, status)
	if err != nil {
		return fmt.Errorf("update room status direct: %w", err)
	}
	return nil
}

// CompleteBattleWithResultTx sets the final status, winner, and ended time for the battle.
func (r *Repository) CompleteBattleWithResultTx(ctx context.Context, tx pgx.Tx, battleID uuid.UUID, winnerUserID *uuid.UUID, endedAt time.Time) error {
	_, err := tx.Exec(ctx, `
		UPDATE battles
		SET status = $2,
		    winner_user_id = $3,
		    ended_at = $4,
		    updated_at = NOW()
		WHERE id = $1
	`, battleID, StatusCompleted, winnerUserID, endedAt)
	if err != nil {
		return fmt.Errorf("complete battle with result tx: %w", err)
	}
	return nil
}
