package http

import (
	"net/http"

	"github.com/enterprise-cicd-platform/auth-service/internal/domain"
	"github.com/enterprise-cicd-platform/auth-service/internal/usecase"
)

// logoutRequest is POST /v1/auth/logout's body (M2 §4/§5): the
// refresh_token_id being revoked.
type logoutRequest struct {
	RefreshTokenID string `json:"refresh_token_id"`
}

type logoutResponseData struct {
	Revoked bool `json:"revoked"`
}

// Logout handles POST /v1/auth/logout.
//
// ASSUMPTION FLAG: M2 §4 lists this endpoint's auth requirement as
// "Access token" (unlike Refresh above), so this handler gates on a valid
// Bearer access token via the Verify use case before ever calling
// usecase.Logout. What §4/§5 does NOT specify is whether the access
// token's UserID must match the RefreshToken.UserID being revoked — i.e.
// whether a caller can revoke a refresh token that isn't theirs as long as
// they present any valid access token. usecase.Logout (see logout.go)
// takes only a RefreshTokenID and never checks ownership, and adding that
// check here would require an extra RefreshTokenRepository read this
// handler doesn't otherwise need. Implemented as written — auth-gated but
// not ownership-checked — and flagged rather than silently resolved,
// since closing this gap changes the use case's contract, not just this
// handler.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := bearerToken(r)
	if !ok {
		WriteError(w, r, domain.ErrInvalidCredentials)
		return
	}
	if _, err := h.verify.Execute(r.Context(), usecase.VerifyTokenInput{AccessToken: accessToken}); err != nil {
		WriteError(w, r, err)
		return
	}

	var req logoutRequest
	if err := decodeJSON(r, &req); err != nil {
		writeValidationError(w, r, "request body must be valid JSON")
		return
	}
	if req.RefreshTokenID == "" {
		writeValidationError(w, r, "refresh_token_id is required")
		return
	}

	if err := h.logout.Execute(r.Context(), usecase.LogoutInput{RefreshTokenID: req.RefreshTokenID}); err != nil {
		WriteError(w, r, err)
		return
	}

	if h.metrics != nil {
		// Mirrors login_handler.go's IncActiveRefreshTokens on the
		// revoke side. Same known limitation applies: a token that
		// expires naturally (never explicitly logged out) is never
		// decremented — see auth_active_refresh_tokens' Help text.
		h.metrics.DecActiveRefreshTokens()
	}

	WriteData(w, http.StatusOK, logoutResponseData{Revoked: true})
}
