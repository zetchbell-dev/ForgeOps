package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// NewRouter builds the full chi.Router for Auth Service: the five M2 §4
// endpoints, plus /healthz and /readyz for Kubernetes probes (M2 scopes
// its own Dockerfile/Helm chart out to M3-M5, but the service still has to
// expose something a liveness/readiness probe can hit once those
// milestones wire it up — this is that endpoint, not the probe config
// itself).
//
// chimiddleware.Recoverer is chi's own panic recovery — kept as-is rather
// than reimplemented, since M2 doesn't specify custom panic-response
// shape and a 500/INTERNAL_ERROR envelope from a recovered panic is no
// different from any other unmapped error reaching WriteError.
func NewRouter(h *Handlers) http.Handler {
	r := chi.NewRouter()

	r.Use(RequestIDMiddleware)
	r.Use(chimiddleware.Recoverer)

	r.Get("/healthz", HealthCheck)
	r.Get("/readyz", HealthCheck)

	r.Route("/v1/auth", func(r chi.Router) {
		r.Post("/register", h.Register)
		r.Post("/login", h.Login)
		r.Post("/refresh", h.Refresh)
		r.Post("/logout", h.Logout)
		r.Get("/verify", h.Verify)
	})

	return r
}
