package httpmw

import "net/http"

// statusRecorder wraps a http.ResponseWriter to capture the status code
// actually written, so middleware.go can label
// http_requests_total/http_request_duration_seconds and the access-log
// line with the real response status instead of assuming 200.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// newStatusRecorder defaults status to 200 — net/http's own behavior
// when a handler writes a body without ever calling WriteHeader
// explicitly (the common case for a plain 200 OK).
func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{ResponseWriter: w, status: http.StatusOK}
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
