package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	authhttp "github.com/enterprise-cicd-platform/auth-service/internal/transport/http"
)

func TestRequestIDMiddleware_GeneratesIDWhenAbsent(t *testing.T) {
	var sawContextID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawContextID = authhttp.RequestIDFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	authhttp.RequestIDMiddleware(next).ServeHTTP(rec, req)

	headerID := rec.Header().Get("X-Request-Id")
	if headerID == "" {
		t.Fatal("expected X-Request-Id response header to be set")
	}
	if sawContextID == "" {
		t.Fatal("expected request ID to be present on the handler's request context")
	}
	if headerID != sawContextID {
		t.Errorf("response header X-Request-Id = %q, want it to match context value %q", headerID, sawContextID)
	}
}

func TestRequestIDMiddleware_PropagatesExistingID(t *testing.T) {
	const incomingID = "upstream-request-id-123"

	var sawContextID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawContextID = authhttp.RequestIDFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-Id", incomingID)
	rec := httptest.NewRecorder()

	authhttp.RequestIDMiddleware(next).ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-Id"); got != incomingID {
		t.Errorf("response header X-Request-Id = %q, want existing incoming ID %q to be echoed back unchanged", got, incomingID)
	}
	if sawContextID != incomingID {
		t.Errorf("context request ID = %q, want existing incoming ID %q", sawContextID, incomingID)
	}
}

func TestRequestIDMiddleware_GeneratesDistinctIDsAcrossRequests(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	handler := authhttp.RequestIDMiddleware(next)

	req1 := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	req2 := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	id1 := rec1.Header().Get("X-Request-Id")
	id2 := rec2.Header().Get("X-Request-Id")
	if id1 == "" || id2 == "" {
		t.Fatal("expected both requests to receive a generated X-Request-Id")
	}
	if id1 == id2 {
		t.Errorf("expected distinct generated request IDs across separate requests, both were %q", id1)
	}
}

func TestRequestIDMiddleware_CallsNextHandler(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	authhttp.RequestIDMiddleware(next).ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected RequestIDMiddleware to invoke the next handler")
	}
	if rec.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d (from wrapped handler)", rec.Code, http.StatusTeapot)
	}
}

func TestRequestIDFromContext_EmptyWhenUnset(t *testing.T) {
	// Direct context.go coverage: a context that never went through
	// WithRequestID/RequestIDMiddleware must yield "", not panic, per
	// RequestIDFromContext's doc comment.
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	if got := authhttp.RequestIDFromContext(req.Context()); got != "" {
		t.Errorf("RequestIDFromContext on a context with no request ID set = %q, want empty string", got)
	}
}

func TestWithRequestID_RoundTrip(t *testing.T) {
	base := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	ctx := authhttp.WithRequestID(base.Context(), "abc-123")
	if got := authhttp.RequestIDFromContext(ctx); got != "abc-123" {
		t.Errorf("RequestIDFromContext after WithRequestID = %q, want %q", got, "abc-123")
	}
}
