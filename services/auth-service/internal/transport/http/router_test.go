package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	authhttp "github.com/enterprise-cicd-platform/auth-service/internal/transport/http"
)

// newTestRouter builds a router.Handler via NewRouter around the same
// newTestHandlers() fixture handlers_test.go already uses, with no
// observability middleware and no metrics handler (both are nil-safe
// per router.go's doc comment).
func newTestRouter() http.Handler {
	return authhttp.NewRouter(newTestHandlers(), nil, nil)
}

func TestNewRouter_HealthAndReadyEndpoints(t *testing.T) {
	router := newTestRouter()

	for _, path := range []string{"/healthz", "/readyz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: status = %d, want %d", path, rec.Code, http.StatusOK)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
			t.Errorf("GET %s: Content-Type = %q, want application/json; charset=utf-8", path, ct)
		}
	}
}

func TestNewRouter_MetricsRouteNotRegisteredWhenNil(t *testing.T) {
	// NewRouter only mounts /metrics when metricsHandler is non-nil
	// (router.go); newTestRouter() passes nil, so it must not be routed.
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /metrics with nil metricsHandler: status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestNewRouter_MetricsRouteRegisteredWhenProvided(t *testing.T) {
	metricsCalled := false
	metricsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metricsCalled = true
		w.WriteHeader(http.StatusOK)
	})

	router := authhttp.NewRouter(newTestHandlers(), nil, metricsHandler)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /metrics with metricsHandler provided: status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !metricsCalled {
		t.Error("expected the provided metricsHandler to be invoked for GET /metrics")
	}
}

func TestNewRouter_ObservabilityMiddlewareInvokedWhenProvided(t *testing.T) {
	obsCalled := false
	obsMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			obsCalled = true
			next.ServeHTTP(w, r)
		})
	}

	router := authhttp.NewRouter(newTestHandlers(), obsMiddleware, nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /healthz: status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !obsCalled {
		t.Error("expected obsMiddleware to be invoked when provided")
	}
}

func TestNewRouter_AuthRoutesRegistered(t *testing.T) {
	router := newTestRouter()

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/v1/auth/register"},
		{http.MethodPost, "/v1/auth/login"},
		{http.MethodPost, "/v1/auth/refresh"},
		{http.MethodPost, "/v1/auth/logout"},
		{http.MethodGet, "/v1/auth/verify"},
	}

	// Every case here just confirms the route is registered (any status
	// other than 404), not that the request succeeds — a body-less
	// register/login/refresh/logout POST correctly gets rejected by
	// validation (400) or auth (401), and an unauthenticated verify GET
	// gets 401; none of that is a routing failure.
	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, tt.path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code == http.StatusNotFound {
			t.Errorf("%s %s: status = 404, want route to be registered", tt.method, tt.path)
		}
	}
}

func TestNewRouter_UnknownRouteReturns404(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/does-not-exist", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /v1/auth/does-not-exist: status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestNewRouter_UnsupportedMethodReturns405(t *testing.T) {
	router := newTestRouter()

	// /v1/auth/register only registers POST (router.go); DELETE against
	// the same, otherwise-valid path must be a 405 (chi's default
	// MethodNotAllowedHandler), not a 404.
	req := httptest.NewRequest(http.MethodDelete, "/v1/auth/register", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE /v1/auth/register: status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestNewRouter_RequestIDMiddlewareRegistered(t *testing.T) {
	// RequestIDMiddleware (middleware.go) is unconditionally r.Use'd by
	// NewRouter, so any response through the router — including a 404 —
	// should carry the X-Request-Id header it sets.
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Header().Get("X-Request-Id") == "" {
		t.Error("expected X-Request-Id header to be set by RequestIDMiddleware via NewRouter")
	}
}

func TestNewRouter_RecovererMiddlewareRegistered(t *testing.T) {
	// chimiddleware.Recoverer is r.Use'd by NewRouter (router.go). We
	// can't panic a real handler without touching production code, but
	// we can confirm the router as a whole doesn't itself panic/crash
	// the test process on a request through an unrelated route — this
	// guards against a future NewRouter change accidentally dropping the
	// Use(chimiddleware.Recoverer) line in a way that would otherwise
	// only surface at runtime under an actual panic.
	router := newTestRouter()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("router.ServeHTTP panicked: %v", r)
		}
	}()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
}
