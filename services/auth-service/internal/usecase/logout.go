package usecase

import (
	"context"
	"fmt"

	"github.com/enterprise-cicd-platform/auth-service/internal/domain"
	"github.com/google/uuid"
)

type LogoutInput struct {
	RefreshTokenID string
}

// Logout revokes a refresh token (M2 §4). Revocation is written to
// Postgres — the source of truth (M2 §5) — before the cache entry is
// dropped, so a crash between the two steps leaves the safe state (revoked
// in Postgres, stale-but-soon-checked-again in cache), never the unsafe one.
type Logout struct {
	deps Deps
}

func NewLogout(deps Deps) *Logout {
	return &Logout{deps: deps}
}

func (uc *Logout) Execute(ctx context.Context, in LogoutInput) error {
	tokenID, err := uuid.Parse(in.RefreshTokenID)
	if err != nil {
		return domain.ErrTokenNotFound
	}

	if err := uc.deps.RefreshTokens.Revoke(ctx, tokenID, uc.deps.Now()); err != nil {
		if err == domain.ErrTokenNotFound {
			return domain.ErrTokenNotFound
		}
		return fmt.Errorf("%w: revoking refresh token: %v", domain.ErrInternal, err)
	}

	// Best-effort cache invalidation after the durable revoke succeeds. If
	// this fails, Refresh's own revocation check (against the Postgres
	// value on a cache miss, or against what the cache holds on a hit)
	// still catches a revoked token — see refresh.go — so a failure here
	// is logged as internal but does not undo the logout.
	if err := uc.deps.RefreshCache.Delete(ctx, tokenID); err != nil {
		return fmt.Errorf("%w: invalidating cached refresh token: %v", domain.ErrInternal, err)
	}

	return nil
}
