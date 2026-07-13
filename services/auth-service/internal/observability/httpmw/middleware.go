// Package httpmw is the single combined HTTP middleware for the parts of
// M5's three pillars that touch every inbound request: it records
// http_requests_total/http_request_duration_seconds (M5 §3), starts (or
// continues, via W3C traceparent propagation) a trace span for the
// request (M5 §7), and emits one structured access-log line per request
// carrying request_id and trace_id (M5 §6). All three live in one
// http.Handler, rather than three separate middlewares, so the response
// is wrapped for status-code capture exactly once, and so the log line
// naturally has the trace ID the span just produced already at hand.
//
// This package deliberately does not import
// internal/transport/http (the package that mounts it, via
// cmd/server/main.go): that package's own RequestIDMiddleware sets the
// request ID on the context using an unexported key, readable only
// through its exported RequestIDFromContext func — which this package
// receives as a plain func(context.Context) string parameter instead of
// importing directly, since transport/http will need to import httpmw's
// constructed middleware value (not this package itself) when wiring the
// router, and importing back the other way would cycle.
package httpmw

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/enterprise-cicd-platform/auth-service/internal/observability/metrics"
)

const tracerName = "github.com/enterprise-cicd-platform/auth-service"

// skipAccessLog holds paths this middleware still records metrics/traces
// for, but doesn't emit an access-log line for — Prometheus scraping
// /metrics every ~15s would otherwise dominate the log stream with
// content that carries no incident-investigation value (M5 §6's whole
// point is signal an on-call engineer can use).
var skipAccessLog = map[string]bool{
	"/metrics": true,
}

// New builds the combined middleware. m may be nil (metrics not
// recorded) and logger may be nil (no access log emitted) — both are
// nil-checked so this middleware degrades gracefully rather than
// panicking if main.go is ever run without one wired up, matching this
// codebase's general preference for explicit, safe degradation over a
// required-non-nil contract enforced only by convention.
func New(m *metrics.Metrics, logger *slog.Logger, requestIDFromContext func(context.Context) string) func(http.Handler) http.Handler {
	tracer := otel.Tracer(tracerName)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
			ctx, span := tracer.Start(ctx, r.Method+" "+r.URL.Path,
				oteltrace.WithSpanKind(oteltrace.SpanKindServer),
				oteltrace.WithAttributes(
					attribute.String("http.method", r.Method),
					attribute.String("http.target", r.URL.Path),
				),
			)
			defer span.End()

			rec := newStatusRecorder(w)
			next.ServeHTTP(rec, r.WithContext(ctx))

			duration := time.Since(start)
			path := routePattern(r)
			status := rec.status

			span.SetAttributes(attribute.Int("http.status_code", status))

			if m != nil {
				m.ObserveHTTPRequest(r.Method, path, status, duration)
			}

			if logger != nil && !skipAccessLog[r.URL.Path] {
				attrs := []any{
					"method", r.Method,
					"path", path,
					"status", status,
					"duration_ms", duration.Milliseconds(),
					"remote_addr", r.RemoteAddr,
				}
				if requestIDFromContext != nil {
					if id := requestIDFromContext(ctx); id != "" {
						attrs = append(attrs, "request_id", id)
					}
				}
				if tid := span.SpanContext().TraceID(); tid.IsValid() {
					attrs = append(attrs, "trace_id", tid.String())
				}
				logger.InfoContext(ctx, "http_request", attrs...)
			}
		})
	}
}

// routePattern returns chi's matched route pattern (e.g.
// "/v1/auth/login"), not the raw request path — using the raw path as a
// metric label would let an attacker or a buggy client inflate
// http_requests_total's cardinality with one series per distinct
// unmatched URL. Reading chi.RouteContext AFTER next.ServeHTTP has
// returned is deliberate: chi populates the route pattern into the
// *chi.Context it already stored on the request's context as it walks
// its routing tree while handling the request, so the pattern is only
// fully known once the inner handler has actually run — not before.
func routePattern(r *http.Request) string {
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		if p := rctx.RoutePattern(); p != "" {
			return p
		}
	}
	return "unmatched"
}
