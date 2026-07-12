package http

import (
	"net/http"

	"github.com/enterprise-cicd-platform/auth-service/internal/usecase"
)

// loginRequest is POST /v1/auth/login's body (M2 §4).
type loginRequest struct {
	LoginIdentifier string `json:"login_identifier"`
	Password        string `json:"password"`
}

type loginResponseData struct {
	UserID         string `json:"user_id"`
	AccessToken    string `json:"access_token"`
	RefreshTokenID string `json:"refresh_token_id"`
}

// Login handles POST /v1/auth/login. Rate limiting (per-IP and
// per-account, M2 §4/§9) happens inside usecase.Login, not here — this
// handler's only job is decoding the request and resolving the caller's IP
// for that use case to key on.
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeValidationError(w, r, "request body must be valid JSON")
		return
	}
	if req.LoginIdentifier == "" || req.Password == "" {
		writeValidationError(w, r, "login_identifier and password are required")
		return
	}

	out, err := h.login.Execute(r.Context(), usecase.LoginInput{
		IP:              clientIP(r),
		LoginIdentifier: req.LoginIdentifier,
		Password:        req.Password,
	})
	if err != nil {
		WriteError(w, r, err)
		return
	}

	WriteData(w, http.StatusOK, loginResponseData{
		UserID:         out.UserID.String(),
		AccessToken:    out.AccessToken,
		RefreshTokenID: out.RefreshTokenID,
	})
}
