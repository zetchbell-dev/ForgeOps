package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	authhttp "github.com/enterprise-cicd-platform/auth-service/internal/transport/http"
)

func TestHealthCheck_StatusOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	authhttp.HealthCheck(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHealthCheck_ContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	authhttp.HealthCheck(rec, req)

	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json; charset=utf-8")
	}
}

func TestHealthCheck_ResponseBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	authhttp.HealthCheck(rec, req)

	body := decodeEnvelope(t, rec)
	if body["error"] != nil {
		t.Errorf("error field = %v, want nil", body["error"])
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data field missing or wrong shape: %#v", body["data"])
	}
	if data["status"] != "ok" {
		t.Errorf("data.status = %v, want %q", data["status"], "ok")
	}
}

func TestHealthCheck_ReadyzSameBehaviorAsHealthz(t *testing.T) {
	// /readyz is routed to the same HealthCheck handler as /healthz
	// (router.go) — this locks in that the handler's own output doesn't
	// vary by path, since HealthCheck itself never inspects r.URL.
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	authhttp.HealthCheck(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var decoded struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decoding response body: %v", err)
	}
	if decoded.Data.Status != "ok" {
		t.Errorf("data.status = %q, want %q", decoded.Data.Status, "ok")
	}
	if decoded.Error != nil {
		t.Errorf("error field = %v, want nil", decoded.Error)
	}
}
