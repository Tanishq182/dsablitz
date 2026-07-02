package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

type User struct {
	ID           string
	Email        string
	PasswordHash *string
	Handle       string
	DisplayName  string
	AvatarURL    *string
	Status       string
	LastLoginAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Session struct {
	ID               string
	UserID           string
	RefreshTokenHash string
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type CreateUserParams struct {
	Email        string
	PasswordHash string
	Handle       string
	DisplayName  string
}

type CreateSessionParams struct {
	UserID           string
	RefreshTokenHash string
	UserAgent        string
	IPAddress        string
	ExpiresAt        time.Time
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateUser(ctx context.Context, params CreateUserParams) (User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return User{}, fmt.Errorf("begin create user: %w", err)
	}
	defer tx.Rollback(ctx)

	user, err := scanUser(tx.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, handle, display_name)
		VALUES ($1, $2, $3, $4)
		RETURNING id::TEXT, email::TEXT, password_hash, handle::TEXT, display_name, avatar_url, status, last_login_at, created_at, updated_at
	`, params.Email, params.PasswordHash, params.Handle, params.DisplayName))
	if err != nil {
		if isUniqueViolation(err, "users_email_key") {
			return User{}, ErrEmailTaken
		}
		if isUniqueViolation(err, "users_handle_key") {
			return User{}, ErrHandleTaken
		}
		return User{}, fmt.Errorf("insert user: %w", err)
	}

	if _, err := tx.Exec(ctx, `INSERT INTO user_stats (user_id) VALUES ($1)`, user.ID); err != nil {
		return User{}, fmt.Errorf("insert user stats: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit create user: %w", err)
	}

	return user, nil
}

func (r *Repository) FindUserByEmail(ctx context.Context, email string) (User, error) {
	user, err := scanUser(r.db.QueryRow(ctx, `
		SELECT id::TEXT, email::TEXT, password_hash, handle::TEXT, display_name, avatar_url, status, last_login_at, created_at, updated_at
		FROM users
		WHERE email = $1
	`, email))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, fmt.Errorf("find user by email: %w", err)
	}

	return user, nil
}

func (r *Repository) FindUserByID(ctx context.Context, userID string) (User, error) {
	user, err := scanUser(r.db.QueryRow(ctx, `
		SELECT id::TEXT, email::TEXT, password_hash, handle::TEXT, display_name, avatar_url, status, last_login_at, created_at, updated_at
		FROM users
		WHERE id = $1
	`, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, fmt.Errorf("find user by id: %w", err)
	}

	return user, nil
}

func (r *Repository) UpdateLastLogin(ctx context.Context, userID string) error {
	if _, err := r.db.Exec(ctx, `UPDATE users SET last_login_at = NOW() WHERE id = $1`, userID); err != nil {
		return fmt.Errorf("update last login: %w", err)
	}

	return nil
}

func (r *Repository) CreateSession(ctx context.Context, params CreateSessionParams) (Session, error) {
	session, err := scanSession(r.db.QueryRow(ctx, `
		INSERT INTO auth_sessions (user_id, refresh_token_hash, user_agent, ip_address, expires_at)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, '')::INET, $5)
		RETURNING id::TEXT, user_id::TEXT, refresh_token_hash, expires_at, revoked_at, created_at, updated_at
	`, params.UserID, params.RefreshTokenHash, params.UserAgent, params.IPAddress, params.ExpiresAt))
	if err != nil {
		return Session{}, fmt.Errorf("insert auth session: %w", err)
	}

	return session, nil
}

func (r *Repository) FindActiveSessionByHash(ctx context.Context, refreshTokenHash string) (Session, error) {
	session, err := scanSession(r.db.QueryRow(ctx, `
		SELECT id::TEXT, user_id::TEXT, refresh_token_hash, expires_at, revoked_at, created_at, updated_at
		FROM auth_sessions
		WHERE refresh_token_hash = $1
			AND revoked_at IS NULL
			AND expires_at > NOW()
	`, refreshTokenHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrUnauthorized
	}
	if err != nil {
		return Session{}, fmt.Errorf("find active session: %w", err)
	}

	return session, nil
}

func (r *Repository) RotateSession(ctx context.Context, oldRefreshTokenHash string, params CreateSessionParams) (Session, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Session{}, fmt.Errorf("begin rotate session: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE auth_sessions
		SET revoked_at = NOW()
		WHERE refresh_token_hash = $1
			AND revoked_at IS NULL
			AND expires_at > NOW()
	`, oldRefreshTokenHash)
	if err != nil {
		return Session{}, fmt.Errorf("revoke old session: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return Session{}, ErrUnauthorized
	}

	session, err := scanSession(tx.QueryRow(ctx, `
		INSERT INTO auth_sessions (user_id, refresh_token_hash, user_agent, ip_address, expires_at)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, '')::INET, $5)
		RETURNING id::TEXT, user_id::TEXT, refresh_token_hash, expires_at, revoked_at, created_at, updated_at
	`, params.UserID, params.RefreshTokenHash, params.UserAgent, params.IPAddress, params.ExpiresAt))
	if err != nil {
		return Session{}, fmt.Errorf("insert rotated session: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Session{}, fmt.Errorf("commit rotate session: %w", err)
	}

	return session, nil
}

func (r *Repository) RevokeSessionByHash(ctx context.Context, refreshTokenHash string) error {
	if refreshTokenHash == "" {
		return nil
	}

	if _, err := r.db.Exec(ctx, `
		UPDATE auth_sessions
		SET revoked_at = NOW()
		WHERE refresh_token_hash = $1
			AND revoked_at IS NULL
	`, refreshTokenHash); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}

	return nil
}

func scanUser(row pgx.Row) (User, error) {
	var user User
	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Handle,
		&user.DisplayName,
		&user.AvatarURL,
		&user.Status,
		&user.LastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	return user, err
}

func scanSession(row pgx.Row) (Session, error) {
	var session Session
	err := row.Scan(
		&session.ID,
		&session.UserID,
		&session.RefreshTokenHash,
		&session.ExpiresAt,
		&session.RevokedAt,
		&session.CreatedAt,
		&session.UpdatedAt,
	)
	return session, err
}

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}
