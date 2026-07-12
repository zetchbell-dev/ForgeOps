package usecase

import (
	"context"
	"fmt"

	"github.com/enterprise-cicd-platform/auth-service/internal/domain"
)

type VerifyTokenInput struct {
	AccessToken string
}

type VerifyTokenOutput struct {
	Claims domain.AccessTokenClaims
}

// VerifyToken validates a caller-presented access token (M2 §3/§4). This is
// the use case every other service in the platform calls (directly or via
// a shared client) on every authenticated request, so it deliberately does
// nothing beyond signature/expiry validation — no Postgres or Redis call,
// no per-request credential-status check. Adding either here would turn a
// per-request, in-process JWT verification into a per-request database
// round trip for every service in the platform, which is exactly the
// cross-service cost M5's token-verify SLO (see the AccessTokenClaims
// comment in internal/domain/entities.go) is written against. Revoking a
// *access* token before its natural (short) expiry is intentionally out of
// scope for this milestone: M2 only specifies revocation for refresh
// tokens (§4, §9), and the access token's own short TTL is the mechanism
// that bounds how long a compromised one stays usable.
type VerifyToken struct {
	deps Deps
}

func NewVerifyToken(deps Deps) *VerifyToken {
	return &VerifyToken{deps: deps}
}

func (uc *VerifyToken) Execute(ctx context.Context, in VerifyTokenInput) (VerifyTokenOutput, error) {
	claims, err := uc.deps.Tokens.VerifyAccessToken(ctx, in.AccessToken)
	if err != nil {
		// TokenIssuer.VerifyAccessToken's port contract (internal/domain/ports.go)
		// already restricts this to domain.ErrTokenExpired or
		// domain.ErrInvalidCredentials (bad signature) — never a raw JWT
		// library error — so both pass through unwrapped, matching how
		// Login/Register pass through the domain errors their
		// dependencies are contractually restricted to. Anything else
		// would be a port-contract violation by the infrastructure
		// implementation, not an expected outcome of this use case, so it
		// is wrapped as internal rather than silently passed through.
		if err == domain.ErrTokenExpired || err == domain.ErrInvalidCredentials {
			return VerifyTokenOutput{}, err
		}
		return VerifyTokenOutput{}, fmt.Errorf("%w: verifying access token: %v", domain.ErrInternal, err)
	}

	return VerifyTokenOutput{Claims: claims}, nil
}
