package http

import (
	"net/http"
	"time"

	"github.com/enterprise-cicd-platform/auth-service/internal/domain"
	"github.com/enterprise-cicd-platform/auth-service/internal/usecase"
)

type verifyResponseData struct {
	UserID    string `json:"user_id"`
	IssuedAt  string `json:"issued_at"`
	ExpiresAt string `json:"expires_at"`
}

// Verify handles GET /v1/auth/verify (M2 §4: "used internally by
// Gateway"). A missing or malformed Authorization header is
// indistinguishable, from the caller's side, from a token that failed
// verification — both are domain.ErrInvalidCredentials, matching §6's
// same-error convention for credential failures.
func (h *Handlers) Verify(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerToken(r)
	if !ok {
		WriteError(w, r, domain.ErrInvalidCredentials)
		return
	}

	out, err := h.verify.Execute(r.Context(), usecase.VerifyTokenInput{AccessToken: token})
	if err != nil {
		WriteError(w, r, err)
		return
	}

	WriteData(w, http.StatusOK, verifyResponseData{
		UserID:    out.Claims.UserID.String(),
		IssuedAt:  out.Claims.IssuedAt.Format(time.RFC3339),
		ExpiresAt: out.Claims.ExpiresAt.Format(time.RFC3339),
	})
}
