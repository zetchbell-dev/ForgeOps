package http

import (
	obsmetrics "github.com/enterprise-cicd-platform/auth-service/internal/observability/metrics"
	"github.com/enterprise-cicd-platform/auth-service/internal/usecase"
)

// Handlers holds the five use cases this package adapts to HTTP (M2 §3/§4).
// It depends only on usecase — never on a concrete infrastructure type —
// consistent with context.go/errors.go/response.go's foundation already
// established in this package: transport wires use cases to chi, nothing
// more.
//
// metrics is the one exception (M5 Phase 1): login_handler.go,
// logout_handler.go, and verify_handler.go record auth-specific
// observability signals (login result, active refresh token count,
// verify latency) that only the handler layer has enough context to
// produce — the use cases themselves return plain domain values/errors,
// not metric labels. It's set via SetMetrics rather than added as a
// NewHandlers parameter so the existing constructor signature (already
// called positionally from cmd/server/main.go and
// handlers_test.go) doesn't have to change. A nil metrics is valid and
// every call site below nil-checks before recording, matching
// internal/observability/httpmw.New's same nil-safe degradation.
type Handlers struct {
	register *usecase.Register
	login    *usecase.Login
	refresh  *usecase.Refresh
	logout   *usecase.Logout
	verify   *usecase.VerifyToken
	metrics  *obsmetrics.Metrics
}

// SetMetrics wires the observability metrics collector into Handlers.
// Called once from cmd/server/main.go after both NewHandlers and
// observability/metrics.New; left unset (nil), handlers simply skip
// recording — see the Handlers doc comment above.
func (h *Handlers) SetMetrics(m *obsmetrics.Metrics) {
	h.metrics = m
}

// NewHandlers wires the five use cases into a Handlers. Order matches the
// API contract's listing order in M2 §4 (register, login, refresh,
// logout, verify), which is also the order the composition root
// (cmd/server/main.go) constructs them in.
func NewHandlers(register *usecase.Register, login *usecase.Login, refresh *usecase.Refresh, logout *usecase.Logout, verify *usecase.VerifyToken) *Handlers {
	return &Handlers{
		register: register,
		login:    login,
		refresh:  refresh,
		logout:   logout,
		verify:   verify,
	}
}
