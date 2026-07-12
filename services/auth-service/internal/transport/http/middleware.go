package http

import (
	"net/http"

	"github.com/google/uuid"
)

// requestIDHeader is the header this service both reads (to respect an
// upstream-assigned ID, e.g. from Gateway) and writes (so a caller that
// didn't set one can still correlate its logs to this service's own).
const requestIDHeader = "X-Request-Id"

// RequestIDMiddleware assigns every inbound request a request ID via
// WithRequestID (context.go) and echoes it back as a response header.
// Every error response's request_id field (response.go's ErrorBody) comes
// from this — M2 §6 requires every error response to carry one.
//
// Deliberately not chi/v5/middleware's built-in RequestID: that
// implementation uses its own context key, which context.go's
// RequestIDFromContext (already written, already tested by
// response_test.go) would never see. One request-ID mechanism, defined
// once in this package, is what keeps WithRequestID/RequestIDFromContext
// the actual source of truth end to end.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(requestIDHeader)
		if requestID == "" {
			requestID = uuid.NewString()
		}
		w.Header().Set(requestIDHeader, requestID)
		next.ServeHTTP(w, r.WithContext(WithRequestID(r.Context(), requestID)))
	})
}
