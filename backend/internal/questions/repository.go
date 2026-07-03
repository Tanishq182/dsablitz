package questions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("question not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// FindQuestionByID retrieves a question by its unique UUID.
func (r *Repository) FindQuestionByID(ctx context.Context, id uuid.UUID) (Question, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, question_type, difficulty, title, prompt, options, correct_answer, explanation, time_limit_sec, tags, source, created_by, is_active, created_at, updated_at
		FROM questions
		WHERE id = $1
	`, id)
	return scanQuestion(row)
}

// FindActiveQuestionsByFilters queries active questions with optional difficulty and tag intersection.
func (r *Repository) FindActiveQuestionsByFilters(ctx context.Context, difficulty int, tags []string) ([]Question, error) {
	query := `
		SELECT id, question_type, difficulty, title, prompt, options, correct_answer, explanation, time_limit_sec, tags, source, created_by, is_active, created_at, updated_at
		FROM questions
		WHERE is_active = true
	`
	args := []interface{}{}
	argIdx := 1

	if difficulty > 0 {
		query += fmt.Sprintf(" AND difficulty = $%d", argIdx)
		args = append(args, difficulty)
		argIdx++
	}

	if len(tags) > 0 {
		query += fmt.Sprintf(" AND tags && $%d", argIdx)
		args = append(args, tags)
		argIdx++
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query active questions: %w", err)
	}
	defer rows.Close()

	var questions []Question
	for rows.Next() {
		q, err := scanQuestion(rows)
		if err != nil {
			return nil, err
		}
		questions = append(questions, q)
	}

	return questions, nil
}

// InsertOrUpdateQuestion inserts a question into the DB (used by seeding scripts).
func (r *Repository) InsertOrUpdateQuestion(ctx context.Context, q Question) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	optionsJSON, err := json.Marshal(q.Options)
	if err != nil {
		return fmt.Errorf("marshal options: %w", err)
	}

	// Idempotent seeding upsert on primary key (id) conflict
	var questionID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO questions (id, question_type, difficulty, title, prompt, options, correct_answer, explanation, time_limit_sec, tags, source, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, true)
		ON CONFLICT (id) DO UPDATE
		SET question_type = EXCLUDED.question_type,
		    difficulty = EXCLUDED.difficulty,
		    title = EXCLUDED.title,
		    prompt = EXCLUDED.prompt,
		    options = EXCLUDED.options,
		    correct_answer = EXCLUDED.correct_answer,
		    explanation = EXCLUDED.explanation,
		    time_limit_sec = EXCLUDED.time_limit_sec,
		    tags = EXCLUDED.tags,
		    source = EXCLUDED.source,
		    updated_at = NOW()
		RETURNING id
	`, q.ID, q.QuestionType, q.Difficulty, q.Title, q.Prompt, optionsJSON, q.CorrectAnswer, q.Explanation, q.TimeLimitSec, q.Tags, q.Source).Scan(&questionID)
	if err != nil {
		return fmt.Errorf("upsert question: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO question_stats (question_id)
		VALUES ($1)
		ON CONFLICT (question_id) DO NOTHING
	`, questionID)
	if err != nil {
		return fmt.Errorf("initialize question stats: %w", err)
	}

	return tx.Commit(ctx)
}

func scanQuestion(row pgx.Row) (Question, error) {
	var q Question
	var optionsBytes []byte
	var tags []string
	var createdBy *uuid.UUID

	err := row.Scan(
		&q.ID, &q.QuestionType, &q.Difficulty, &q.Title, &q.Prompt,
		&optionsBytes, &q.CorrectAnswer, &q.Explanation, &q.TimeLimitSec,
		&tags, &q.Source, &createdBy, &q.IsActive, &q.CreatedAt, &q.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Question{}, ErrNotFound
		}
		return Question{}, err
	}

	q.Tags = tags
	q.CreatedBy = createdBy

	if len(optionsBytes) > 0 {
		var opts []string
		if err := json.Unmarshal(optionsBytes, &opts); err == nil {
			q.Options = opts
		}
	}

	return q, nil
}
