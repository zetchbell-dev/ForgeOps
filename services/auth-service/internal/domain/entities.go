// Package domain holds the entities and port interfaces for Auth Service.
// Per M2 §3 (Clean Architecture), this package has zero external
// dependencies — no database driver, no HTTP framework, no JWT library
// imports here. That's what makes usecase (which depends only on this
// package) unit-testable with fakes and no real infrastructure.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// CredentialStatus is the account status recorded alongside a credential.
// Fixed set of values, not a free-text string, so callers can exhaustively
// switch on it (M2 §4's fixed-error-code convention applies the same
// reasoning to status here).
type CredentialStatus string

const (
	CredentialStatusActive   CredentialStatus = "active"
	CredentialStatusLocked   CredentialStatus = "locked"
	CredentialStatusDisabled CredentialStatus = "disabled"
)

// Credential is Auth Service's only durable record of "how to verify this
// user." Per M2 §2, it deliberately holds nothing else — no email, no
// display name. That data belongs to User Service and is joined only by
// UserID.
type Credential struct {
	UserID uuid.UUID

	// LoginIdentifier is the value a client authenticates with (email or
	// username) before they have a UserID to look anything up by. M2 §5's
	// data model table lists only user_id/password_hash/status and is
	// explicit that email/name belong to User Service, not this table —
	// but that leaves no way for /v1/auth/login to resolve a submitted
	// identifier to a user_id at all. This field is the minimum needed to
	// close that gap: it is an auth lookup key only (never returned to
	// other services, never used for anything but "which credential row is
	// this login attempt for"), not a reintroduction of profile data.
	LoginIdentifier string

	PasswordHash string
	Status       CredentialStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// CanAuthenticate reports whether this credential's status permits a login
// attempt to proceed to password verification at all. A locked or disabled
// account fails here before bcrypt is ever invoked.
func (c Credential) CanAuthenticate() bool {
	return c.Status == CredentialStatusActive
}

// RefreshToken is the durable (Postgres) record of an issued refresh token.
// Per M2 §5, Postgres is the source of truth; Redis (internal/infrastructure/redis)
// is a cache rebuilt from this record, never the reverse.
type RefreshToken struct {
	TokenID   uuid.UUID
	UserID    uuid.UUID
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// IsValid reports whether this refresh token can still be exchanged for a
// new access token: not expired, and not revoked. Checked against the
// Postgres-sourced value on every refresh (M2 §9 risk: "refresh token
// replay after logout" — this is the check that closes that gap), not just
// whatever Redis happens to be caching.
func (t RefreshToken) IsValid(now time.Time) bool {
	if t.RevokedAt != nil {
		return false
	}
	return now.Before(t.ExpiresAt)
}

// AccessTokenClaims is the minimal set of claims Auth Service issues in a
// signed JWT access token. Kept minimal deliberately: this token is
// verified by every other service in the platform (M5's token-verify SLO
// exists because of that), so its shape is a cross-service contract, not
// an internal implementation detail to grow casually.
type AccessTokenClaims struct {
	UserID    uuid.UUID
	IssuedAt  time.Time
	ExpiresAt time.Time
}
