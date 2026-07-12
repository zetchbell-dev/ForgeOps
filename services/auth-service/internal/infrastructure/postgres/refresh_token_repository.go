package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/enterprise-cicd-platform/auth-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RefreshTokenRepository implements domain.RefreshTokenRepository against
// the `auth.refresh_tokens` table (M2 §5) — the durable source of truth;
// internal/infrastructure/redis caches what's read here, never the reverse.
type RefreshTokenRepository struct {
	pool *pgxpool.Pool
}

func NewRefreshTokenRepository(pool *pgxpool.Pool) *RefreshTokenRepository {
	return &RefreshTokenRepository{pool: pool}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, token domain.RefreshToken) error {
	const query = `
		INSERT INTO auth.refresh_tokens (token_id, user_id, expires_at, revoked_at)
		VALUES ($1, $2, $3, $4)
	`
	_, err := r.pool.Exec(ctx, query, token.TokenID, token.UserID, token.ExpiresAt, token.RevokedAt)
	return err
}

func (r *RefreshTokenRepository) GetByTokenID(ctx context.Context, tokenID uuid.UUID) (domain.RefreshToken, error) {
	const query = `
		SELECT token_id, user_id, expires_at, revoked_at
		FROM auth.refresh_tokens
		WHERE token_id = $1
	`
	var token domain.RefreshToken
	err := r.pool.QueryRow(ctx, query, tokenID).Scan(&token.TokenID, &token.UserID, &token.ExpiresAt, &token.RevokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RefreshToken{}, domain.ErrTokenNotFound
		}
		return domain.RefreshToken{}, err
	}
	return token, nil
}

// Revoke sets revoked_at unconditionally on the matching row — it does not
// first check whether the token is already revoked (an idempotent second
// logout call for the same token is not itself an error condition per M2
// §4; only a token_id that matches no row at all is ErrTokenNotFound).
func (r *RefreshTokenRepository) Revoke(ctx context.Context, tokenID uuid.UUID, revokedAt time.Time) error {
	const query = `
		UPDATE auth.refresh_tokens
		SET revoked_at = $2
		WHERE token_id = $1
	`
	tag, err := r.pool.Exec(ctx, query, tokenID, revokedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrTokenNotFound
	}
	return nil
}
