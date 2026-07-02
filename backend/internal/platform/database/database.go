package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Manager struct {
	pool *pgxpool.Pool
}

func Connect(ctx context.Context, databaseURL string) (*Manager, error) {
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &Manager{pool: pool}, nil
}

func (m *Manager) Pool() *pgxpool.Pool {
	return m.pool
}

func (m *Manager) Ping(ctx context.Context) error {
	if m == nil || m.pool == nil {
		return errors.New("postgres pool is not initialized")
	}

	return m.pool.Ping(ctx)
}

func (m *Manager) Close() {
	if m == nil || m.pool == nil {
		return
	}

	m.pool.Close()
}
