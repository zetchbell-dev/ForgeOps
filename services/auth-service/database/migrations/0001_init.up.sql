-- Schema and tables for Auth Service (M2 §5), reverse-engineered exactly
-- from the columns internal/infrastructure/postgres's queries already read and write:
--   credential_repository.go:  user_id, login_identifier, password_hash,
--                               status, created_at, updated_at
--   refresh_token_repository.go: token_id, user_id, expires_at, revoked_at
--
-- `status` is stored as TEXT with no CHECK constraint: the repository
-- layer treats it as an opaque string (domain.CredentialStatus(status) on
-- read, string(cred.Status) on write) and this migration has no visibility
-- into which values domain.CredentialStatus actually defines, so it does
-- not guess at an enum/check list.

CREATE SCHEMA IF NOT EXISTS auth;

CREATE TABLE auth.credentials (
    user_id           UUID PRIMARY KEY,
    login_identifier  TEXT NOT NULL UNIQUE,
    password_hash     TEXT NOT NULL,
    status            TEXT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL,
    updated_at        TIMESTAMPTZ NOT NULL
);

CREATE TABLE auth.refresh_tokens (
    token_id    UUID PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES auth.credentials (user_id),
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ NULL
);

-- Not read by any query in the repository package today, but every
-- Refresh/Logout call looks up or revokes by token_id (already the
-- primary key) — this index only helps a not-yet-implemented
-- "list a user's active sessions" style query, added because it's a
-- standard, low-risk index for this access pattern, not because any
-- current code needs it.
CREATE INDEX idx_refresh_tokens_user_id ON auth.refresh_tokens (user_id);
