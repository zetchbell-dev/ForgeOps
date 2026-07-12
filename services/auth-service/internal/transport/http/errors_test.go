package http_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enterprise-cicd-platform/auth-service/internal/domain"
	authhttp "github.com/enterprise-cicd-platform/auth-service/internal/transport/http"
)

// mapViaWriteError exercises the unexported mapDomainError indirectly
// through the package's public surface (WriteError), since this is an
// external test package (http_test) and has no access to unexported
// identifiers. Asserting on the written response is also the more useful
// test anyway — it's what a handler actually produces.
func mapViaWriteError(t *testing.T, err error) (status int, body map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)

	authhttp.WriteError(rec, req, err)

	var decoded map[string]any
	if decodeErr := decodeJSONBody(rec.Body.Bytes(), &decoded); decodeErr != nil {
		t.Fatalf("decoding response body: %v", decodeErr)
	}
	return rec.Code, decoded
}

func TestWriteError_MapsEachDomainSentinel(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"invalid credentials", domain.ErrInvalidCredentials, http.StatusUnauthorized, "INVALID_CREDENTIALS"},
		{"account locked", domain.ErrAccountLocked, http.StatusForbidden, "ACCOUNT_LOCKED"},
		{"account disabled", domain.ErrAccountDisabled, http.StatusForbidden, "ACCOUNT_DISABLED"},
		{"email already exists", domain.ErrEmailAlreadyExists, http.StatusConflict, "ACCOUNT_ALREADY_EXISTS"},
		{"token expired", domain.ErrTokenExpired, http.StatusUnauthorized, "TOKEN_EXPIRED"},
		{"token revoked", domain.ErrTokenRevoked, http.StatusUnauthorized, "TOKEN_REVOKED"},
		{"token not found", domain.ErrTokenNotFound, http.StatusUnauthorized, "TOKEN_NOT_FOUND"},
		{"rate limited", domain.ErrRateLimited, http.StatusTooManyRequests, "RATE_LIMITED"},
		{"internal", domain.ErrInternal, http.StatusInternalServerError, "INTERNAL_ERROR"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := mapViaWriteError(t, tc.err)
			if status != tc.wantStatus {
				t.Errorf("status = %d, want %d", status, tc.wantStatus)
			}
			errBody, ok := body["error"].(map[string]any)
			if !ok {
				t.Fatalf("error field missing or wrong shape: %#v", body["error"])
			}
			if errBody["code"] != tc.wantCode {
				t.Errorf("code = %v, want %v", errBody["code"], tc.wantCode)
			}
			if body["data"] != nil {
				t.Errorf("data = %v, want nil", body["data"])
			}
		})
	}
}

func TestWriteError_WrappedSentinelStillMaps(t *testing.T) {
	wrapped := fmt.Errorf("looking up credential: %w", domain.ErrInvalidCredentials)

	status, body := mapViaWriteError(t, wrapped)
	if status != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", status, http.StatusUnauthorized)
	}
	errBody := body["error"].(map[string]any)
	if errBody["code"] != "INVALID_CREDENTIALS" {
		t.Errorf("code = %v, want INVALID_CREDENTIALS", errBody["code"])
	}
}

func TestWriteError_UnknownErrorFallsBackToInternal(t *testing.T) {
	status, body := mapViaWriteError(t, errors.New("some unmapped infra failure"))
	if status != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", status, http.StatusInternalServerError)
	}
	errBody := body["error"].(map[string]any)
	if errBody["code"] != "INTERNAL_ERROR" {
		t.Errorf("code = %v, want INTERNAL_ERROR", errBody["code"])
	}
	// The raw error text must never reach the client — that's the whole
	// point of the generic fallback (M2 §6).
	if msg, _ := errBody["message"].(string); msg == "some unmapped infra failure" {
		t.Error("raw internal error text leaked into the client-facing message")
	}
}
