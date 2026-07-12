// Package postgres implements domain.CredentialRepository and
// domain.RefreshTokenRepository against the `auth` Postgres schema (M2 §5).
// Postgres is the durable source of truth for both tables — a Redis cache
// (internal/infrastructure/redis) is rebuilt from what's read here, never
// the reverse (M2 §5, M2 §9's refresh-token-replay mitigation).
//
// Every exported method returns either a domain sentinel error (already
// declared in internal/domain/errors.go — ErrEmailAlreadyExists,
// ErrInvalidCredentials, ErrTokenNotFound) for a condition this package
// knows how to identify, or the underlying pgx error unwrapped for
// anything else. Wrapping the unwrapped case into domain.ErrInternal is
// the usecase layer's job (M2 §6), not this package's — every usecase
// file already does that wrapping at its own call site, and duplicating
// it here would double-wrap the error message.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool constructs a pgx connection pool from a DSN. The DSN itself
// comes from an environment variable at the composition root
// (cmd/server/main.go) per M2 §7 (twelve-factor config) — never
// hardcoded or read from a file baked into the image.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing postgres dsn: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating postgres connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging postgres: %w", err)
	}

	return pool, nil
}
