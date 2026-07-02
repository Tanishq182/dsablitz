package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Manager struct {
	client *redis.Client
}

func Connect(ctx context.Context, redisURL string) (*Manager, error) {
	if redisURL == "" {
		return nil, errors.New("REDIS_URL is required")
	}

	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	client := redis.NewClient(options)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &Manager{client: client}, nil
}

func (m *Manager) Client() *redis.Client {
	return m.client
}

func (m *Manager) Ping(ctx context.Context) error {
	if m == nil || m.client == nil {
		return errors.New("redis client is not initialized")
	}

	return m.client.Ping(ctx).Err()
}

func (m *Manager) Close() error {
	if m == nil || m.client == nil {
		return nil
	}

	return m.client.Close()
}
