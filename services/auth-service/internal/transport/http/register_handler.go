package http

import (
	"net/http"

	"github.com/enterprise-cicd-platform/auth-service/internal/usecase"
)

// registerRequest is POST /v1/auth/register's body (M2 §4): credential
// only, no profile fields — see usecase.RegisterInput's own doc comment
// for why.
type registerRequest struct {
	LoginIdentifier string `json:"login_identifier"`
	Password        string `json:"password"`
}

type registerResponseData struct {
	UserID string `json:"user_id"`
}

// Register handles POST /v1/auth/register.
func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeValidationError(w, r, "request body must be valid JSON")
		return
	}
	if req.LoginIdentifier == "" || req.Password == "" {
		writeValidationError(w, r, "login_identifier and password are required")
		return
	}

	out, err := h.register.Execute(r.Context(), usecase.RegisterInput{
		LoginIdentifier: req.LoginIdentifier,
		Password:        req.Password,
	})
	if err != nil {
		WriteError(w, r, err)
		return
	}

	WriteData(w, http.StatusCreated, registerResponseData{UserID: out.UserID.String()})
}
