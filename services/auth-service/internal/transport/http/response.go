package http

import (
	"encoding/json"
	"net/http"
)

// Envelope is M2 §4's fixed response shape for every endpoint:
//
//	{ "data": { ... }, "error": null }
//	{ "data": null, "error": { "code": "...", "message": "..." } }
//
// Exactly one of Data/Error is non-nil on any given response — enforced
// by construction (WriteData/WriteError each set only one), not by a
// runtime check, since both helpers are the only way this package ever
// produces an Envelope.
type Envelope struct {
	Data  any        `json:"data"`
	Error *ErrorBody `json:"error"`
}

// ErrorBody is the envelope's "error" field per M2 §4/§6. RequestID
// correlates this response to structured logs/traces (M2 §6, ties to M5
// observability) — it's an empty string, not an omitted field, when no
// request ID was set on the context, so clients can rely on the field's
// presence in the JSON shape even if its value is sometimes blank.
type ErrorBody struct {
	Code      ErrorCode `json:"code"`
	Message   string    `json:"message"`
	RequestID string    `json:"request_id"`
}

// WriteJSON encodes v as the response body with the given status code and
// the standard JSON content type. It is the lowest-level writer in this
// package — WriteData and WriteError are the ones handlers (a later
// implementation chunk) actually call; WriteJSON exists as their shared
// primitive and for any future response shape that isn't Envelope (e.g. a
// health-check endpoint with its own body shape).
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// The status line and headers are already written by this point, so a
	// failed Encode (e.g. the client disconnected mid-write) has nothing
	// left for this function to correct — there is no second response to
	// send. Connection-level failures here are an access-log/observability
	// concern (M5), not a transport-layer error-handling one.
	_ = json.NewEncoder(w).Encode(v)
}

// WriteData writes a successful Envelope: {"data": data, "error": null}.
func WriteData(w http.ResponseWriter, status int, data any) {
	WriteJSON(w, status, Envelope{Data: data, Error: nil})
}

// WriteError maps err through mapDomainError and writes the corresponding
// error Envelope, attaching the request ID from r's context if one was
// set (see WithRequestID). This is the single call every handler (a later
// chunk) makes on a use-case error — no handler constructs an ErrorBody
// or chooses an HTTP status by hand, which is what keeps M2 §6's mapping
// rule enforced in one place instead of re-implemented per endpoint.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := mapDomainError(err)
	WriteJSON(w, status, Envelope{
		Data: nil,
		Error: &ErrorBody{
			Code:      code,
			Message:   message,
			RequestID: RequestIDFromContext(r.Context()),
		},
	})
}
