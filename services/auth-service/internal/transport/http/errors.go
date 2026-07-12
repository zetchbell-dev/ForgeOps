package http

import (
	"errors"
	"net/http"

	"github.com/enterprise-cicd-platform/auth-service/internal/domain"
)

// ErrorCode is a fixed, closed set of wire values (M2 §4: "Fixed error
// code values (not free-text) so Gateway and Frontend can branch on them
// without string matching"). Defined as a named string type, not a bare
// const block of untyped strings, so a handler constructing an error body
// by hand (rather than through mapDomainError) is caught by the compiler
// if it passes something that isn't one of these.
type ErrorCode string

const (
	ErrorCodeInvalidCredentials  ErrorCode = "INVALID_CREDENTIALS"
	ErrorCodeAccountLocked       ErrorCode = "ACCOUNT_LOCKED"
	ErrorCodeAccountDisabled     ErrorCode = "ACCOUNT_DISABLED"
	ErrorCodeAccountAlreadyExist ErrorCode = "ACCOUNT_ALREADY_EXISTS"
	ErrorCodeTokenExpired        ErrorCode = "TOKEN_EXPIRED"
	ErrorCodeTokenRevoked        ErrorCode = "TOKEN_REVOKED"
	ErrorCodeTokenNotFound       ErrorCode = "TOKEN_NOT_FOUND"
	ErrorCodeRateLimited         ErrorCode = "RATE_LIMITED"
	ErrorCodeValidationFailed    ErrorCode = "VALIDATION_FAILED"
	ErrorCodeInternal            ErrorCode = "INTERNAL_ERROR"
)

// domainErrorMapping is the one place M2 §6's error-mapping rule lives:
// "the transport layer maps domain errors to HTTP status + error code, so
// a Postgres error message never leaks to a client response." Every
// use-case-returned error a handler can see is listed here explicitly —
// deliberately not a switch with a default that guesses, so adding a new
// domain error without adding it here is a silent fallthrough to
// INTERNAL_ERROR (safe) rather than an accidental information leak.
var domainErrorMapping = []struct {
	err    error
	status int
	code   ErrorCode
}{
	{domain.ErrInvalidCredentials, http.StatusUnauthorized, ErrorCodeInvalidCredentials},
	{domain.ErrAccountLocked, http.StatusForbidden, ErrorCodeAccountLocked},
	{domain.ErrAccountDisabled, http.StatusForbidden, ErrorCodeAccountDisabled},
	{domain.ErrEmailAlreadyExists, http.StatusConflict, ErrorCodeAccountAlreadyExist},
	{domain.ErrTokenExpired, http.StatusUnauthorized, ErrorCodeTokenExpired},
	{domain.ErrTokenRevoked, http.StatusUnauthorized, ErrorCodeTokenRevoked},
	{domain.ErrTokenNotFound, http.StatusUnauthorized, ErrorCodeTokenNotFound},
	{domain.ErrRateLimited, http.StatusTooManyRequests, ErrorCodeRateLimited},
}

// mapDomainError resolves err to (status, code, safe client-facing
// message). Matching is by errors.Is, so a use case that wraps a sentinel
// (fmt.Errorf("...: %w", domain.ErrInvalidCredentials)) still maps
// correctly — use cases are allowed to add context to an error on its way
// out as long as the sentinel is preserved.
//
// Anything not in domainErrorMapping — including domain.ErrInternal and
// any error a use case returns that isn't one of the typed domain
// sentinels at all (a bug, or an infra error that escaped the use-case
// layer's own wrapping) — maps to the generic 500/INTERNAL_ERROR case.
// The raw err is deliberately not interpolated into the client-facing
// message here; a caller that needs it for logs has the original err,
// not just this function's return values.
func mapDomainError(err error) (status int, code ErrorCode, message string) {
	for _, m := range domainErrorMapping {
		if errors.Is(err, m.err) {
			return m.status, m.code, m.err.Error()
		}
	}
	return http.StatusInternalServerError, ErrorCodeInternal, "an internal error occurred"
}
