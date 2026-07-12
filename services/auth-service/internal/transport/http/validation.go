package http

import (
	"encoding/json"
	"net/http"
)

// decodeJSON decodes r's body into v. It exists as a single call site so
// every handler treats a malformed body identically (VALIDATION_FAILED,
// not INTERNAL_ERROR — a client sent bad JSON, the server didn't fail).
func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	// DisallowUnknownFields is deliberately NOT set: M2 §4 doesn't specify
	// strict field rejection, and adding it here would make every request
	// DTO break the moment a new optional field is added to the wire
	// contract — a stricter decode policy is a real option but belongs in
	// an M2 amendment (per this project's change-control rule), not a
	// silent default chosen at the handler layer.
	return dec.Decode(v)
}

// writeValidationError writes the fixed VALIDATION_FAILED envelope (M2 §4:
// fixed error codes, not free text) for a request that decoded but failed
// a required-field check, or that didn't decode at all. Deliberately not
// added to errors.go's domainErrorMapping — that table maps use-case
// domain errors, and a validation failure is a transport-layer decision
// made before any use case ever runs.
func writeValidationError(w http.ResponseWriter, r *http.Request, message string) {
	WriteJSON(w, http.StatusBadRequest, Envelope{
		Data: nil,
		Error: &ErrorBody{
			Code:      ErrorCodeValidationFailed,
			Message:   message,
			RequestID: RequestIDFromContext(r.Context()),
		},
	})
}
