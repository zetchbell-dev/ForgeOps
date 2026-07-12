package http

import "net/http"

// healthResponseData backs both /healthz (liveness) and /readyz
// (readiness). This milestone (M2) has no dependency-ping wired in yet —
// see NewRouter's doc comment — so both currently report the same
// "process is up" signal; a real readiness check (Postgres/Redis ping)
// is infrastructure wiring that belongs at the composition root
// (cmd/server/main.go), not this package.
type healthResponseData struct {
	Status string `json:"status"`
}

// HealthCheck handles GET /healthz and GET /readyz.
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	WriteData(w, http.StatusOK, healthResponseData{Status: "ok"})
}
