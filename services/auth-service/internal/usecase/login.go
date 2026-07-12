package usecase

import (
	"context"
	"fmt"

	"github.com/enterprise-cicd-platform/auth-service/internal/domain"
	"github.com/google/uuid"
)

// dummyPasswordHash is verified against on a login attempt for an
// identifier that doesn't exist, so that a not-found lookup still pays the
// same bcrypt cost as a real one before returning ErrInvalidCredentials
// (M2 §6's same-error rule for "user not found" vs. "password mismatch" is
// only a timing guarantee if both paths call Hasher.Verify — a short
// circuit here would let a client distinguish the two by response latency
// even though the returned error is identical).
const dummyPasswordHash = "$2a$10$CwTycUXWue0Thq9StjUM0uJ8ZKQ0uv4Q9c9Xr9Xr9Xr9Xr9Xr9Xru"

type LoginInput struct {
	// IP is the caller's source address, used as one of the two rate
	// limit keys (M2 §4/§9: per-IP AND per-account, since either alone
	// leaves a gap — a single compromised account gets rate-limited by
	// account, credential stuffing across many accounts from one source
	// gets rate-limited by IP).
	IP              string
	LoginIdentifier string
	Password        string
}

type LoginOutput struct {
	UserID         uuid.UUID
	AccessToken    string
	RefreshTokenID string
}

// Login authenticates a credential and issues a new access/refresh token
// pair (M2 §4). Rate limiting runs before any credential lookup or bcrypt
// call, so an attacker already over threshold never gets to spend the
// server's bcrypt budget at all.
type Login struct {
	deps Deps
	cfg  Config
}

func NewLogin(deps Deps, cfg Config) *Login {
	return &Login{deps: deps, cfg: cfg}
}

func (uc *Login) Execute(ctx context.Context, in LoginInput) (LoginOutput, error) {
	ipAllowed, err := uc.deps.RateLimiter.Allow(ctx, rateLimitKeyIP(in.IP), uc.cfg.LoginRateLimitPerIP, uc.cfg.LoginRateLimitWindow)
	if err != nil {
		return LoginOutput{}, fmt.Errorf("%w: checking per-IP rate limit: %v", domain.ErrInternal, err)
	}
	if !ipAllowed {
		return LoginOutput{}, domain.ErrRateLimited
	}

	acctAllowed, err := uc.deps.RateLimiter.Allow(ctx, rateLimitKeyAccount(in.LoginIdentifier), uc.cfg.LoginRateLimitPerAccount, uc.cfg.LoginRateLimitWindow)
	if err != nil {
		return LoginOutput{}, fmt.Errorf("%w: checking per-account rate limit: %v", domain.ErrInternal, err)
	}
	if !acctAllowed {
		return LoginOutput{}, domain.ErrRateLimited
	}

	cred, err := uc.deps.Credentials.GetByLoginIdentifier(ctx, in.LoginIdentifier)
	if err != nil {
		if err == domain.ErrInvalidCredentials {
			// No such identifier. Still pay the bcrypt cost against a
			// dummy hash — see dummyPasswordHash — before returning the
			// same error a wrong password would produce.
			if _, verifyErr := uc.deps.Hasher.Verify(ctx, in.Password, dummyPasswordHash); verifyErr != nil {
				return LoginOutput{}, fmt.Errorf("%w: verifying password: %v", domain.ErrInternal, verifyErr)
			}
			return LoginOutput{}, domain.ErrInvalidCredentials
		}
		return LoginOutput{}, fmt.Errorf("%w: reading credential repository: %v", domain.ErrInternal, err)
	}

	// Status is checked before bcrypt runs (domain.Credential.CanAuthenticate's
	// own doc comment: "before bcrypt is ever invoked"). Unlike the
	// not-found case above, a locked/disabled account is a genuine,
	// distinct error per M2 §6 — these accounts are known to exist and
	// the client is expected to see why login is refused (e.g. to show a
	// "contact support" message), so no timing-parity dummy call is
	// needed here.
	if !cred.CanAuthenticate() {
		switch cred.Status {
		case domain.CredentialStatusLocked:
			return LoginOutput{}, domain.ErrAccountLocked
		case domain.CredentialStatusDisabled:
			return LoginOutput{}, domain.ErrAccountDisabled
		default:
			return LoginOutput{}, domain.ErrInvalidCredentials
		}
	}

	ok, err := uc.deps.Hasher.Verify(ctx, in.Password, cred.PasswordHash)
	if err != nil {
		return LoginOutput{}, fmt.Errorf("%w: verifying password: %v", domain.ErrInternal, err)
	}
	if !ok {
		return LoginOutput{}, domain.ErrInvalidCredentials
	}

	accessToken, _, err := uc.deps.Tokens.IssueAccessToken(ctx, cred.UserID)
	if err != nil {
		return LoginOutput{}, fmt.Errorf("%w: issuing access token: %v", domain.ErrInternal, err)
	}

	refreshTokenID := uc.deps.Tokens.NewRefreshTokenID()
	now := uc.deps.Now()
	refreshToken := domain.RefreshToken{
		TokenID:   refreshTokenID,
		UserID:    cred.UserID,
		ExpiresAt: now.Add(uc.cfg.RefreshTokenTTL),
	}

	// Postgres — the source of truth (M2 §5) — is written before the
	// cache. A crash between the two steps leaves a refresh token that is
	// valid but not yet cached, which Refresh's cache-miss-falls-back-to-
	// repository path (see refresh.go) already handles correctly; the
	// reverse ordering could leave a cached token with no durable record
	// backing it.
	if err := uc.deps.RefreshTokens.Create(ctx, refreshToken); err != nil {
		return LoginOutput{}, fmt.Errorf("%w: creating refresh token: %v", domain.ErrInternal, err)
	}

	if err := uc.deps.RefreshCache.Set(ctx, refreshToken, uc.cfg.RefreshTokenTTL); err != nil {
		return LoginOutput{}, fmt.Errorf("%w: caching refresh token: %v", domain.ErrInternal, err)
	}

	return LoginOutput{
		UserID:         cred.UserID,
		AccessToken:    accessToken,
		RefreshTokenID: refreshTokenID.String(),
	}, nil
}
