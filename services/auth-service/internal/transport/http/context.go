// Package http implements Auth Service's transport layer (M2 §3/§4):
// mapping use-case calls to chi HTTP handlers, and domain errors to the
// fixed response envelope defined in M2 §4. This file and errors.go/
// response.go are the transport-wide foundation everything else in this
// package (handlers, middleware, routes, DTOs — each its own
// implementation chunk) is built on, so they land first.
package http

import "context"

// requestIDContextKey is an unexported type so this package's context key
// can never collide with a key set by another package using the same
// underlying string — the standard Go anti-collision idiom for
// context.WithValue keys.
type requestIDContextKey struct{}

// WithRequestID returns a copy of ctx carrying requestID. Set once per
// inbound request — by RequestID middleware, an implementation chunk that
// comes after this one — and read by every handler/error path downstream
// via RequestIDFromContext.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

// RequestIDFromContext returns the request ID set by WithRequestID, or ""
// if none was set. Callers must treat "" as a valid (if degraded) case —
// e.g. a request that reached error handling before the ID middleware ran
// — rather than treating a missing ID as itself an error condition; M2
// §6 requires every error response to *carry* a request ID field, not
// that the field is always non-empty.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDContextKey{}).(string)
	return id
}
