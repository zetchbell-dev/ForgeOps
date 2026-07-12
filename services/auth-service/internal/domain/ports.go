package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// CredentialRepository is the Postgres-backed port for credential storage
// (M2 §3/§5). Implemented by internal/infrastructure/postgres. Returns
// domain errors (ErrInvalidCredentials, ErrEmailAlreadyExists, ErrInternal),
// never a raw driver error — that translation happens at the
// implementation boundary, not in usecase.
type CredentialRepository interface {
	// Create inserts a new credential row. Returns ErrEmailAlreadyExists if
	// LoginIdentifier is already taken.
	Create(ctx context.Context, cred Credential) error

	// GetByLoginIdentifier looks up a credential by the value a client
	// authenticates with. Returns ErrInvalidCredentials (not a distinct
	// "not found" error — M2 §6's same-error, constant-time rule for
	// credential lookups) if no row matches.
	GetByLoginIdentifier(ctx context.Context, identifier string) (Credential, error)

	// GetByUserID looks up a credential by its primary key, used by
	// VerifyToken and Refresh where a UserID is already known from a valid
	// token.
	GetByUserID(ctx context.Context, userID uuid.UUID) (Credential, error)
}

// RefreshTokenRepository is the Postgres-backed port for refresh token
// records — the source of truth per M2 §5 (Redis is a cache rebuilt from
// this, never the reverse).
type RefreshTokenRepository interface {
	Create(ctx context.Context, token RefreshToken) error
	GetByTokenID(ctx context.Context, tokenID uuid.UUID) (RefreshToken, error)
	Revoke(ctx context.Context, tokenID uuid.UUID, revokedAt time.Time) error
}

// RefreshTokenCache is the Redis-backed port caching active refresh tokens
// for fast lookup (M2 §5). A cache miss falls back to RefreshTokenRepository
// and repopulates this cache — this port is never the source of truth and
// a Refresh/VerifyToken use case must not treat a cache miss as "token
// invalid."
type RefreshTokenCache interface {
	Get(ctx context.Context, tokenID uuid.UUID) (RefreshToken, bool, error)
	Set(ctx context.Context, token RefreshToken, ttl time.Duration) error
	Delete(ctx context.Context, tokenID uuid.UUID) error
}

// RateLimiter is the Redis-backed sliding-window port for login rate
// limiting (M2 §4): checked per-IP AND per-account, since either alone
// leaves a gap (M2 §9).
type RateLimiter interface {
	// Allow increments the named window's counter and reports whether the
	// action is still within threshold. key is fully qualified by the
	// caller (e.g. "ratelimit:login:ip:<ip>" or
	// "ratelimit:login:user:<user_id>", per M2 §5).
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

// TokenIssuer is the port for signing and verifying access tokens (JWT) and
// generating refresh token IDs. Implemented by internal/infrastructure/jwt.
type TokenIssuer interface {
	// IssueAccessToken signs a short-lived JWT for userID. TTL is an
	// infrastructure-layer concern (signing key, algorithm), not a usecase
	// decision.
	IssueAccessToken(ctx context.Context, userID uuid.UUID) (token string, claims AccessTokenClaims, err error)

	// VerifyAccessToken validates signature and expiry, returning the
	// claims. Returns ErrTokenExpired or ErrInvalidCredentials (invalid
	// signature) — never a raw JWT library error.
	VerifyAccessToken(ctx context.Context, token string) (AccessTokenClaims, error)

	// NewRefreshTokenID generates a new opaque refresh token identifier.
	// Opaque, not a signed JWT (M2 §2) — a refresh token must be
	// revocable server-side, which a self-contained JWT is not.
	NewRefreshTokenID() uuid.UUID
}

// EventPublisher is the port for the event Register emits after creating a
// credential. M2 §2 states profile creation "is a separate call to User
// Service triggered by an event," and M2 §4's register endpoint is
// "credential only" — but M2's port list (UserRepository, TokenIssuer,
// PasswordHasher) never named the thing that emits that event. This port
// closes that gap explicitly rather than having Register silently call
// User Service directly (which would violate the service boundary M2 §2
// draws) or silently do nothing (which would leave accounts with no
// profile, ever). The concrete implementation (SNS, an outbox table, etc.)
// is an infrastructure decision not yet made in any milestone doc.
type EventPublisher interface {
	PublishAccountCreated(ctx context.Context, userID uuid.UUID) error
}

// PasswordHasher is the port for credential hashing/verification.
// Implemented by internal/infrastructure/bcrypt. The bcrypt cost factor
// itself is an infrastructure-layer constant (M2 §9), benchmarked against
// the M5 p99 login-latency SLO — usecase never sees or chooses a cost
// factor.
type PasswordHasher interface {
	Hash(ctx context.Context, plaintext string) (hash string, err error)

	// Verify reports whether plaintext matches hash. Must run in constant
	// time regardless of match/mismatch (M2 §6) — callers must not add a
	// short-circuit before calling this.
	Verify(ctx context.Context, plaintext, hash string) (bool, error)
}
