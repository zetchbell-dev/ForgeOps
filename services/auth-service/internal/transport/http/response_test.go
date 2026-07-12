package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enterprise-cicd-platform/auth-service/internal/domain"
	authhttp "github.com/enterprise-cicd-platform/auth-service/internal/transport/http"
)

// decodeJSONBody is a small shared helper for this package's tests —
// used by both response_test.go and errors_test.go.
func decodeJSONBody(body []byte, v any) error {
	return json.Unmarshal(body, v)
}

func TestWriteData_EnvelopeShape(t *testing.T) {
	rec := httptest.NewRecorder()

	authhttp.WriteData(rec, http.StatusOK, map[string]string{"user_id": "abc-123"})

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/json; charset=utf-8", ct)
	}

	var decoded map[string]any
	if err := decodeJSONBody(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decoding response body: %v", err)
	}
	if decoded["error"] != nil {
		t.Errorf("error = %v, want nil", decoded["error"])
	}
	data, ok := decoded["data"].(map[string]any)
	if !ok {
		t.Fatalf("data field missing or wrong shape: %#v", decoded["data"])
	}
	if data["user_id"] != "abc-123" {
		t.Errorf("data.user_id = %v, want abc-123", data["user_id"])
	}
}

func TestWriteError_IncludesRequestIDFromContext(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
	ctx := authhttp.WithRequestID(req.Context(), "req-42")
	req = req.WithContext(ctx)

	authhttp.WriteError(rec, req, domain.ErrInvalidCredentials)

	var decoded map[string]any
	if err := decodeJSONBody(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decoding response body: %v", err)
	}
	errBody := decoded["error"].(map[string]any)
	if errBody["request_id"] != "req-42" {
		t.Errorf("request_id = %v, want req-42", errBody["request_id"])
	}
}

func TestWriteError_EmptyRequestIDWhenNoneSet(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)

	authhttp.WriteError(rec, req, domain.ErrInvalidCredentials)

	var decoded map[string]any
	if err := decodeJSONBody(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decoding response body: %v", err)
	}
	errBody := decoded["error"].(map[string]any)
	if errBody["request_id"] != "" {
		t.Errorf("request_id = %v, want empty string", errBody["request_id"])
	}
}

func TestRequestIDFromContext_EmptyWhenUnset(t *testing.T) {
	if got := authhttp.RequestIDFromContext(httptest.NewRequest(http.MethodGet, "/", nil).Context()); got != "" {
		t.Errorf("RequestIDFromContext = %q, want empty string", got)
	}
}
