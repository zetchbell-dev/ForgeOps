package domain

import "errors"

// Typed domain errors (M2 §6). Use cases return these, never raw
// database/Redis/JWT errors — the transport layer (internal/transport/http)
// maps each of these to an HTTP status and a fixed error code (M2 §4), so a
// driver error message never reaches a client response.
//
// ErrInvalidCredentials is deliberately returned for BOTH "user not found"
// and "password mismatch" (M2 §6): distinguishing them would let an
// attacker enumerate valid usernames. Callers must not branch differently
// on which underlying condition produced it.
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountLocked      = errors.New("account locked")
	ErrAccountDisabled    = errors.New("account disabled")
	ErrEmailAlreadyExists = errors.New("account already exists")
	ErrTokenExpired       = errors.New("token expired")
	ErrTokenRevoked       = errors.New("token revoked")
	ErrTokenNotFound      = errors.New("token not found")
	ErrRateLimited        = errors.New("rate limited")

	// ErrInternal wraps any unexpected infrastructure failure (DB
	// unreachable, Redis timeout, etc.) that has already been logged with
	// request-ID context at the point it crossed into the usecase layer.
	// The transport layer maps this to a generic INTERNAL_ERROR response —
	// never a stack trace or driver error — regardless of what's wrapped.
	ErrInternal = errors.New("internal error")
)
