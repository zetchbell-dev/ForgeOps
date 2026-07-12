// Package usecase implements Auth Service's application logic: Login,
// Refresh, Logout, Register, VerifyToken (M2 §3). Every use case depends
// only on internal/domain's port interfaces — never on a concrete
// Postgres/Redis/JWT/bcrypt type. That's what lets every use case in this
// package be tested with fakes and no real infrastructure (see the
// *_test.go files, which use the fakes in fakes_test.go).
package usecase

import (
	"time"

	"github.com/enterprise-cicd-platform/auth-service/internal/domain"
)

// Deps is the set of port dependencies every use case in this package
// draws from. Wired to real infrastructure implementations at the
// composition root (cmd/server/main.go) — never constructed here.
type Deps struct {
	Credentials   domain.CredentialRepository
	RefreshTokens domain.RefreshTokenRepository
	RefreshCache  domain.RefreshTokenCache
	RateLimiter   domain.RateLimiter
	Tokens        domain.TokenIssuer
	Hasher        domain.PasswordHasher
	Events        domain.EventPublisher

	// Now returns the current time. A field, not a direct time.Now() call,
	// so tests can control expiry/rate-limit-window behavior
	// deterministically without sleeping.
	Now func() time.Time
}

// Config holds the policy values M2 leaves unspecified at the design-doc
// level (rate limit thresholds, refresh token lifetime) but which must be
// concrete constants somewhere in the code, not magic numbers scattered
// across use cases. Wired from environment variables at the composition
// root per M2 §7 (twelve-factor config).
type Config struct {
	// RefreshTokenTTL is how long an issued refresh token remains valid.
	RefreshTokenTTL time.Duration

	// LoginRateLimitPerIP and LoginRateLimitPerAccount are the sliding
	// window thresholds for M2 §4's dual rate limiting (per-IP AND
	// per-account — either alone leaves the gap described in M2 §9).
	LoginRateLimitPerIP      int
	LoginRateLimitPerAccount int
	LoginRateLimitWindow     time.Duration
}

// DefaultConfig returns conservative defaults. These are starting points,
// not tuned values — M5 §4 makes the same point about SLOs being
// provisional until real traffic data exists, and the same caveat applies
// to these thresholds.
func DefaultConfig() Config {
	return Config{
		RefreshTokenTTL:          30 * 24 * time.Hour,
		LoginRateLimitPerIP:      20,
		LoginRateLimitPerAccount: 5,
		LoginRateLimitWindow:     5 * time.Minute,
	}
}

func rateLimitKeyIP(ip string) string {
	return "ratelimit:login:ip:" + ip
}

func rateLimitKeyAccount(identifier string) string {
	return "ratelimit:login:user:" + identifier
}
