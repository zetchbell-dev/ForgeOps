// Package redis implements domain.RefreshTokenCache and domain.RateLimiter
// against Redis (M2 §5). Both ports are explicitly caches/counters, never
// a source of truth: RefreshTokenCache is rebuilt from Postgres
// (internal/infrastructure/postgres) on a miss, and RateLimiter's counters
// are allowed to reset if Redis is flushed — that only loosens rate
// limiting temporarily, it never grants access a token wouldn't otherwise
// have (M2 §9).
package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// ClientConfig holds the connection parameters read from environment
// variables at the composition root (cmd/server/main.go) per M2 §7
// (twelve-factor config) — never hardcoded here.
type ClientConfig struct {
	Addr     string
	Password string
	DB       int
}

// NewClient constructs a go-redis client and verifies connectivity with a
// PING before returning it, the same fail-fast shape as
// postgres.NewPool — a broken Redis dependency should surface at startup,
// not on the first request that happens to need the cache.
func NewClient(ctx context.Context, cfg ClientConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("pinging redis: %w", err)
	}

	return client, nil
}
