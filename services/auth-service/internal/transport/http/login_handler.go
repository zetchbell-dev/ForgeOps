package http

import (
	"errors"
	"net/http"

	"github.com/enterprise-cicd-platform/auth-service/internal/domain"
	obsmetrics "github.com/enterprise-cicd-platform/auth-service/internal/observability/metrics"
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
		if h.metrics != nil {
			h.metrics.ObserveLoginAttempt(loginResultForError(err))
		}
		WriteError(w, r, err)
		return
	}

	if h.metrics != nil {
		h.metrics.ObserveLoginAttempt(obsmetrics.LoginResultSuccess)
		// A successful login always issues exactly one new refresh
		// token (see usecase.Login.Execute) — see
		// auth_active_refresh_tokens' Help text (metrics.go) for this
		// gauge's known limitations (in-process, not reconciled
		// against Postgres).
		h.metrics.IncActiveRefreshTokens()
	}

	WriteData(w, http.StatusOK, loginResponseData{
		UserID:         out.UserID.String(),
		AccessToken:    out.AccessToken,
		RefreshTokenID: out.RefreshTokenID,
	})
}

// loginResultForError maps a usecase.Login.Execute error to one of
// metrics.LoginResult* (M5 §3's auth_login_attempts_total labels).
// Matching is by errors.Is against the same domain sentinels
// errors.go's mapDomainError already uses for the HTTP response, so the
// two mappings can't silently drift apart for the errors they share.
// Anything not in domainErrorMapping (domain.ErrInternal, or an error
// that reaches here unwrapped) becomes LoginResultError — a genuinely
// different outcome from a client-facing 4xx, per metrics.go's package
// doc comment.
func loginResultForError(err error) string {
	switch {
	case errors.Is(err, domain.ErrInvalidCredentials):
		return obsmetrics.LoginResultInvalidCredentials
	case errors.Is(err, domain.ErrAccountLocked):
		return obsmetrics.LoginResultLocked
	case errors.Is(err, domain.ErrAccountDisabled):
		return obsmetrics.LoginResultDisabled
	case errors.Is(err, domain.ErrRateLimited):
		return obsmetrics.LoginResultRateLimited
	default:
		return obsmetrics.LoginResultError
	}
}
