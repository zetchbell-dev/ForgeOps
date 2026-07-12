package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/enterprise-cicd-platform/auth-service/internal/domain"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// refreshTokenKeyPrefix namespaces cached refresh tokens from rate-limit
// keys and any future key families in the same Redis instance/DB.
const refreshTokenKeyPrefix = "refresh_token:"

// RefreshTokenCache implements domain.RefreshTokenCache. Per M2 §5 and the
// port's own doc comment, this is a cache rebuilt from
// infrastructure/postgres — a miss here is not "token invalid," it's
// "ask Postgres and repopulate," and that fallback is the Refresh use
// case's job, not this package's.
type RefreshTokenCache struct {
	client *redis.Client
}

// NewRefreshTokenCache constructs a RefreshTokenCache over an already-
// connected client (see NewClient) — connection lifecycle is the
// composition root's responsibility, not this type's.
func NewRefreshTokenCache(client *redis.Client) *RefreshTokenCache {
	return &RefreshTokenCache{client: client}
}

func refreshTokenKey(tokenID uuid.UUID) string {
	return refreshTokenKeyPrefix + tokenID.String()
}

// Get looks up a cached refresh token. A missing key returns
// (zero-value, false, nil) — not an error — because a cache miss is an
// expected, routine outcome (TTL expiry, cold cache after a Redis
// restart, key never cached) that the caller falls back to Postgres for,
// per the port's contract.
func (c *RefreshTokenCache) Get(ctx context.Context, tokenID uuid.UUID) (domain.RefreshToken, bool, error) {
	vals, err := c.client.HGetAll(ctx, refreshTokenKey(tokenID)).Result()
	if err != nil {
		return domain.RefreshToken{}, false, fmt.Errorf("getting cached refresh token: %w", err)
	}
	if len(vals) == 0 {
		return domain.RefreshToken{}, false, nil
	}

	userID, err := uuid.Parse(vals["user_id"])
	if err != nil {
		return domain.RefreshToken{}, false, fmt.Errorf("parsing cached user_id: %w", err)
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, vals["expires_at"])
	if err != nil {
		return domain.RefreshToken{}, false, fmt.Errorf("parsing cached expires_at: %w", err)
	}

	token := domain.RefreshToken{
		TokenID:   tokenID,
		UserID:    userID,
		ExpiresAt: expiresAt,
	}

	// revoked_at is only present in the hash when the token has been
	// revoked (see Set/Revoke-through-Set below) — its absence, not an
	// empty string, is what means "not revoked." vals[...] on a missing
	// field returns "" for both cases in go-redis, so check presence via
	// the second map form explicitly.
	if raw, ok := vals["revoked_at"]; ok && raw != "" {
		revokedAt, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return domain.RefreshToken{}, false, fmt.Errorf("parsing cached revoked_at: %w", err)
		}
		token.RevokedAt = &revokedAt
	}

	return token, true, nil
}

// Set caches a refresh token with the given TTL. ttl is a cache-expiry
// concern chosen by the caller (typically the remaining time until the
// token's own ExpiresAt) — this method does not derive it from the token
// itself, since a Refresh use case revoking a token wants to shrink the
// cache TTL immediately rather than wait out the original expiry.
func (c *RefreshTokenCache) Set(ctx context.Context, token domain.RefreshToken, ttl time.Duration) error {
	if ttl <= 0 {
		return c.Delete(ctx, token.TokenID)
	}

	key := refreshTokenKey(token.TokenID)
	fields := map[string]any{
		"user_id":    token.UserID.String(),
		"expires_at": token.ExpiresAt.Format(time.RFC3339Nano),
	}
	if token.RevokedAt != nil {
		fields["revoked_at"] = token.RevokedAt.Format(time.RFC3339Nano)
	}

	pipe := c.client.TxPipeline()
	pipe.Del(ctx, key) // clears a stale revoked_at field from a prior Set before HSet re-adds only current fields
	pipe.HSet(ctx, key, fields)
	pipe.Expire(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("caching refresh token: %w", err)
	}
	return nil
}

// Delete evicts a cached refresh token. Called on logout/revoke so a
// revoked token can't be served from a stale cache entry until its TTL
// naturally expires (M2 §9's refresh-token-replay mitigation).
func (c *RefreshTokenCache) Delete(ctx context.Context, tokenID uuid.UUID) error {
	if err := c.client.Del(ctx, refreshTokenKey(tokenID)).Err(); err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("deleting cached refresh token: %w", err)
	}
	return nil
}
