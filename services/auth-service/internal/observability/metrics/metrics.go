// Package metrics defines and registers every custom Prometheus
// collector Auth Service emits, per M5 §3's metrics table:
//
//   - http_requests_total / http_request_duration_seconds — the two
//     metrics M4's canary analysis actually queries (M5 §3's "canary-critical
//     subset"), so their label sets are chosen to stay exactly what that
//     doc specifies (method, path, status / method, path) — nothing extra
//     that could complicate the canary/stable split Argo Rollouts injects
//     at the Prometheus scrape-target level (see Phase 2's ServiceMonitor
//     relabeling, not this package).
//   - auth_login_attempts_total, auth_active_refresh_tokens,
//     auth_token_verify_duration_seconds — the three Auth-Service-specific
//     application metrics from the same table.
//   - the Go runtime collector, process collector, and an
//     auth_service_build_info gauge — M5 Phase 1's "Go runtime metrics"
//     and "Build/version metrics" bullets.
//
// Every collector is registered against a caller-supplied
// *prometheus.Registry, never prometheus.DefaultRegisterer — so each test
// (and, if it's ever needed, each independent caller) gets its own
// isolated registry instead of racing to register the same metric name
// twice against a shared global.
package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"

	"github.com/enterprise-cicd-platform/auth-service/internal/observability/version"
)

// Login result labels. M5 §3 names four canonical values (Success,
// InvalidCredentials, Locked, RateLimited). This package adds two more —
// Disabled and Error — because the codebase's domain layer
// (internal/domain/errors.go) already returns a distinct
// ErrAccountDisabled separate from ErrInvalidCredentials, and an
// unexpected/internal failure is a genuinely different outcome from any
// of the four. Collapsing either into an existing label would hide a
// real distinction rather than just omit an unused one — see
// login_handler.go's ASSUMPTION FLAG comment at the call site, which
// follows this codebase's existing convention (refresh_handler.go,
// logout_handler.go) for flagging a doc-vs-implementation gap instead of
// silently resolving it.
const (
	LoginResultSuccess            = "success"
	LoginResultInvalidCredentials = "invalid_credentials"
	LoginResultLocked             = "locked"
	LoginResultRateLimited        = "rate_limited"
	LoginResultDisabled           = "disabled"
	LoginResultError              = "error"
)

// Metrics holds every custom collector this package registers. Handlers
// and middleware record against the methods below; nothing outside this
// package touches a prometheus.Collector directly.
type Metrics struct {
	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
	loginAttemptsTotal  *prometheus.CounterVec
	activeRefreshTokens prometheus.Gauge
	tokenVerifyDuration prometheus.Histogram
}

// New constructs Metrics and registers every collector — the five custom
// ones above, plus the Go runtime collector, the process collector, and
// the build-info gauge — onto reg. It panics (via MustRegister) only on a
// duplicate registration against the same registry, which would be a bug
// in this package's own collector names, not a runtime condition a
// caller could meaningfully recover from — main.go calls this once, at
// startup, before the server starts accepting traffic.
func New(reg *prometheus.Registry) *Metrics {
	m := &Metrics{
		httpRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests processed, labeled by method, route pattern, and status code.",
		}, []string{"method", "path", "status"}),

		httpRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds, labeled by method and route pattern.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),

		loginAttemptsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "auth_login_attempts_total",
			Help: "Total POST /v1/auth/login attempts, labeled by result (success/invalid_credentials/locked/rate_limited/disabled/error).",
		}, []string{"result"}),

		activeRefreshTokens: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "auth_active_refresh_tokens",
			Help: "Approximate count of active refresh tokens issued by this process since it started. " +
				"KNOWN LIMITATION: an in-process counter (incremented on login, decremented on logout), " +
				"not reconciled against Postgres — resets to 0 on restart and does not decrement when a " +
				"token expires naturally without an explicit logout. Sum across replicas in PromQL; treat " +
				"as a directional/capacity-planning signal, not an exact count. A periodic Postgres " +
				"reconciliation job is the documented follow-up, not implemented in this phase.",
		}),

		tokenVerifyDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "auth_token_verify_duration_seconds",
			Help: "Latency of the GET /v1/auth/verify endpoint specifically (M5 §3): tracked separately " +
				"from http_request_duration_seconds because this endpoint is called on nearly every " +
				"authenticated request platform-wide (M5 §4's p99 < 50ms SLO).",
			Buckets: []float64{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 1},
		}),
	}

	reg.MustRegister(
		m.httpRequestsTotal,
		m.httpRequestDuration,
		m.loginAttemptsTotal,
		m.activeRefreshTokens,
		m.tokenVerifyDuration,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "auth_service_build_info",
			Help: "Always 1. version/commit/go_version are carried as labels — the standard Prometheus " +
				"build-info-metric pattern (join against this in Grafana rather than parsing a log line).",
			ConstLabels: prometheus.Labels{
				"version":    version.Version,
				"commit":     version.Commit,
				"go_version": version.GoVersion,
			},
		}, func() float64 { return 1 }),
	)

	return m
}

// ObserveHTTPRequest records one completed request against
// http_requests_total and http_request_duration_seconds (M5 §3).
func (m *Metrics) ObserveHTTPRequest(method, path string, status int, duration time.Duration) {
	statusLabel := strconv.Itoa(status)
	m.httpRequestsTotal.WithLabelValues(method, path, statusLabel).Inc()
	m.httpRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
}

// ObserveLoginAttempt records one login outcome against
// auth_login_attempts_total. result should be one of the LoginResult*
// constants above.
func (m *Metrics) ObserveLoginAttempt(result string) {
	m.loginAttemptsTotal.WithLabelValues(result).Inc()
}

// IncActiveRefreshTokens records a newly issued refresh token (called
// from login_handler.go on a successful login). See
// auth_active_refresh_tokens' Help text above for this gauge's known
// limitations.
func (m *Metrics) IncActiveRefreshTokens() { m.activeRefreshTokens.Inc() }

// DecActiveRefreshTokens records a revoked refresh token (called from
// logout_handler.go on a successful logout).
func (m *Metrics) DecActiveRefreshTokens() { m.activeRefreshTokens.Dec() }

// ObserveTokenVerifyDuration records one GET /v1/auth/verify call's
// latency against auth_token_verify_duration_seconds.
func (m *Metrics) ObserveTokenVerifyDuration(duration time.Duration) {
	m.tokenVerifyDuration.Observe(duration.Seconds())
}
