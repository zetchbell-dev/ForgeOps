package httpmw_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/enterprise-cicd-platform/auth-service/internal/observability/httpmw"
	"github.com/enterprise-cicd-platform/auth-service/internal/observability/metrics"
)

// noRequestID stands in for transport/http.RequestIDFromContext without
// importing that package (see middleware.go's package doc comment for
// why this package can't import it back).
func noRequestID(context.Context) string { return "" }

func TestMiddleware_RecordsMetricsByRoutePattern(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)

	logBuf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logBuf, nil))

	r := chi.NewRouter()
	r.Use(httpmw.New(m, logger, noRequestID))
	r.Get("/v1/auth/verify", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/verify", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("response status = %d, want %d", rw.Code, http.StatusOK)
	}

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gathering metrics: %v", err)
	}

	var found bool
	for _, mf := range families {
		if mf.GetName() != "http_requests_total" {
			continue
		}
		for _, metric := range mf.GetMetric() {
			labels := map[string]string{}
			for _, l := range metric.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}
			// The label must be the chi route PATTERN ("/v1/auth/verify"),
			// which happens to equal the literal request path here since
			// the route has no path params — routePattern's cardinality
			// protection is exercised more directly by the unmatched-route
			// test below.
			if labels["method"] == "GET" && labels["path"] == "/v1/auth/verify" && labels["status"] == "200" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected a http_requests_total series labeled method=GET path=/v1/auth/verify status=200")
	}

	if !strings.Contains(logBuf.String(), `"http_request"`) {
		t.Errorf("expected an access-log line, got: %s", logBuf.String())
	}
}

func TestMiddleware_UnmatchedRouteDoesNotLeakRawPathAsLabel(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)

	r := chi.NewRouter()
	r.Use(httpmw.New(m, nil, noRequestID))
	r.Get("/v1/auth/verify", func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest(http.MethodGet, "/this/path/does/not/exist/12345", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gathering metrics: %v", err)
	}

	for _, mf := range families {
		if mf.GetName() != "http_requests_total" {
			continue
		}
		for _, metric := range mf.GetMetric() {
			for _, l := range metric.GetLabel() {
				if l.GetName() == "path" && l.GetValue() == "/this/path/does/not/exist/12345" {
					t.Fatalf("raw unmatched path leaked into the path label; expected the bounded \"unmatched\" fallback")
				}
			}
		}
	}
}

func TestMiddleware_SkipsAccessLogForMetricsEndpoint(t *testing.T) {
	logBuf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logBuf, nil))

	reg := prometheus.NewRegistry()
	m := metrics.New(reg)

	r := chi.NewRouter()
	r.Use(httpmw.New(m, logger, noRequestID))
	r.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if logBuf.Len() != 0 {
		t.Errorf("expected no access-log line for /metrics, got: %s", logBuf.String())
	}
}

func TestMiddleware_NilMetricsAndLoggerDoNotPanic(t *testing.T) {
	r := chi.NewRouter()
	r.Use(httpmw.New(nil, nil, nil))
	r.Get("/v1/auth/verify", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/verify", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("response status = %d, want %d", rw.Code, http.StatusOK)
	}
}
