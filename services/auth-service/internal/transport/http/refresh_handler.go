package http

import (
	"net/http"

	"github.com/enterprise-cicd-platform/auth-service/internal/usecase"
)

// refreshRequest is POST /v1/auth/refresh's body (M2 §4).
type refreshRequest struct {
	RefreshTokenID string `json:"refresh_token_id"`
}

type refreshResponseData struct {
	AccessToken string `json:"access_token"`
}

// Refresh handles POST /v1/auth/refresh.
//
// ASSUMPTION FLAG: M2 §4's API contract table lists this endpoint's "Auth
// required" column as "Refresh token" — distinct from Verify/Logout's
// "Access token". Taken literally, the refresh_token_id in the request
// body IS the credential for this endpoint; there is no documented
// Authorization-header/Bearer requirement here at all, unlike Logout
// below. This handler therefore does not check for a Bearer access token
// and never calls the Verify use case — usecase.Refresh's own validation
// (does the token ID parse, exist, and remain unexpired/unrevoked) is the
// entire auth check for this endpoint. If that reading of §4 is wrong,
// it's a documentation gap the same class as §11.1/§11.2/§11.3 and should
// go through this project's change-control process, not be silently
// guessed at differently between endpoints.
func (h *Handlers) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := decodeJSON(r, &req); err != nil {
		writeValidationError(w, r, "request body must be valid JSON")
		return
	}
	if req.RefreshTokenID == "" {
		writeValidationError(w, r, "refresh_token_id is required")
		return
	}

	out, err := h.refresh.Execute(r.Context(), usecase.RefreshInput{RefreshTokenID: req.RefreshTokenID})
	if err != nil {
		WriteError(w, r, err)
		return
	}

	WriteData(w, http.StatusOK, refreshResponseData{AccessToken: out.AccessToken})
}
