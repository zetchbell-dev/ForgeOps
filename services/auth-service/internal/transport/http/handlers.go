package http

import "github.com/enterprise-cicd-platform/auth-service/internal/usecase"

// Handlers holds the five use cases this package adapts to HTTP (M2 §3/§4).
// It depends only on usecase — never on a concrete infrastructure type —
// consistent with context.go/errors.go/response.go's foundation already
// established in this package: transport wires use cases to chi, nothing
// more.
type Handlers struct {
	register *usecase.Register
	login    *usecase.Login
	refresh  *usecase.Refresh
	logout   *usecase.Logout
	verify   *usecase.VerifyToken
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
