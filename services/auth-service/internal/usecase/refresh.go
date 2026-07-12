package usecase

import (
	"context"
	"fmt"

	"github.com/enterprise-cicd-platform/auth-service/internal/domain"
	"github.com/google/uuid"
)

type RefreshInput struct {
	RefreshTokenID string
}

type RefreshOutput struct {
	AccessToken string
}

// Refresh exchanges a valid refresh token for a new access token (M2 §4).
// It does NOT rotate the refresh token itself — M2 §4 as currently written
// only specifies issuing a new access token. See this response's note on
// the refresh-token-rotation gap: rotation would change this endpoint's
// response contract and needs an M2 amendment first, per this project's
// change-control process, not a silent addition here.
type Refresh struct {
	deps Deps
	cfg  Config
}

func NewRefresh(deps Deps, cfg Config) *Refresh {
	return &Refresh{deps: deps, cfg: cfg}
}

func (uc *Refresh) Execute(ctx context.Context, in RefreshInput) (RefreshOutput, error) {
	tokenID, err := uuid.Parse(in.RefreshTokenID)
	if err != nil {
		// A malformed token ID is indistinguishable, from the caller's
		// side, from one that never existed.
		return RefreshOutput{}, domain.ErrTokenNotFound
	}

	// Revocation is checked against Postgres — the source of truth — on
	// EVERY refresh, not only on a cache miss (M2 §9: "refresh token replay
	// after logout" is explicitly closed this way, not by a cache-miss
	// fallback alone). A cache hit would otherwise let a token that was
	// revoked after being cached keep validating for the rest of its
	// cached TTL if the cache-invalidation step in Logout ever failed —
	// exactly the replay M2 §9 calls out.
	token, err := uc.deps.RefreshTokens.GetByTokenID(ctx, tokenID)
	if err != nil {
		if err == domain.ErrTokenNotFound {
			return RefreshOutput{}, domain.ErrTokenNotFound
		}
		return RefreshOutput{}, fmt.Errorf("%w: reading refresh token repository: %v", domain.ErrInternal, err)
	}

	// The cache is still populated/refreshed here — it just isn't trusted
	// for the revocation decision above. Keeping it warm is what lets
	// other, lower-stakes reads of this token (not implemented by any use
	// case yet) avoid a Postgres round trip; a failure to warm it doesn't
	// affect this call's correctness, so it's surfaced but non-fatal to
	// the refresh itself... except this project's error-handling
	// convention (M2 §6) doesn't have a "non-fatal, log and continue"
	// path, so it's surfaced as the same internal error as everything
	// else rather than silently swallowed.
	if err := uc.deps.RefreshCache.Set(ctx, token, uc.cfg.RefreshTokenTTL); err != nil {
		return RefreshOutput{}, fmt.Errorf("%w: refreshing token cache: %v", domain.ErrInternal, err)
	}

	now := uc.deps.Now()
	if token.RevokedAt != nil {
		return RefreshOutput{}, domain.ErrTokenRevoked
	}
	if !token.IsValid(now) {
		return RefreshOutput{}, domain.ErrTokenExpired
	}

	accessToken, _, err := uc.deps.Tokens.IssueAccessToken(ctx, token.UserID)
	if err != nil {
		return RefreshOutput{}, fmt.Errorf("%w: issuing access token: %v", domain.ErrInternal, err)
	}

	return RefreshOutput{AccessToken: accessToken}, nil
}
