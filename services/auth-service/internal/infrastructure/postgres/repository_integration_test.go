//go:build integration

// Package postgres_test holds M2 §8's "infrastructure/postgres: Integration
// tests: testcontainers-go spinning a real Postgres" requirement. These
// tests prove CredentialRepository/RefreshTokenRepository actually satisfy
// their port contracts against a real database — the fakes in
// internal/usecase/fakes_test.go prove the *use cases* are correct, but
// they can't prove this package's SQL is.
//
// Build-tagged `integration` deliberately: it needs a Docker daemon, which
// `go test ./...` in a plain dev/CI unit-test step does not have. M3's CI
// pipeline runs this tag in its own stage where Docker-in-Docker (or an
// equivalent runner) is available — see M3-ci-pipeline.md's integration
// stage, which this test is written to satisfy, not this milestone's
// ordinary `go test ./...` step.
package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/enterprise-cicd-platform/auth-service/internal/domain"
	"github.com/enterprise-cicd-platform/auth-service/internal/infrastructure/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// schemaDDL is the minimal `auth` schema (M2 §5) needed for these tests.
// This is intentionally NOT the same file as the golang-migrate migrations
// that will land in services/auth-service/migrations/ later in the build
// order (the master implementation doc places "Database migrations" after
// HTTP transport, deliberately after this package) — once those migrations
// exist, this test should apply them instead of redefining the schema
// inline, so the two never drift apart silently.
const schemaDDL = `
CREATE SCHEMA IF NOT EXISTS auth;

CREATE TABLE auth.credentials (
	user_id           UUID PRIMARY KEY,
	login_identifier  TEXT UNIQUE NOT NULL,
	password_hash     TEXT NOT NULL,
	status            TEXT NOT NULL,
	created_at        TIMESTAMPTZ NOT NULL,
	updated_at        TIMESTAMPTZ NOT NULL
);

CREATE TABLE auth.refresh_tokens (
	token_id    UUID PRIMARY KEY,
	user_id     UUID NOT NULL,
	expires_at  TIMESTAMPTZ NOT NULL,
	revoked_at  TIMESTAMPTZ
);
`

// newTestPool starts a throwaway Postgres container, applies schemaDDL, and
// returns a pool pointed at it. t.Cleanup tears the container down —
// callers don't need their own defer.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("authservice_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
		tcpostgres.WithWaitStrategy(wait.ForListeningPort("5432/tcp")),
	)
	if err != nil {
		t.Fatalf("starting postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("terminating postgres container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("getting connection string: %v", err)
	}

	pool, err := postgres.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting to test postgres: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, schemaDDL); err != nil {
		t.Fatalf("applying schema: %v", err)
	}

	return pool
}

func TestCredentialRepository_CreateAndLookup(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewCredentialRepository(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	cred := domain.Credential{
		UserID:          uuid.New(),
		LoginIdentifier: "person@example.com",
		PasswordHash:    "hashed-value",
		Status:          domain.CredentialStatusActive,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := repo.Create(ctx, cred); err != nil {
		t.Fatalf("Create: %v", err)
	}

	byIdentifier, err := repo.GetByLoginIdentifier(ctx, cred.LoginIdentifier)
	if err != nil {
		t.Fatalf("GetByLoginIdentifier: %v", err)
	}
	if byIdentifier.UserID != cred.UserID {
		t.Fatalf("expected user id %s, got %s", cred.UserID, byIdentifier.UserID)
	}

	byUserID, err := repo.GetByUserID(ctx, cred.UserID)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if byUserID.LoginIdentifier != cred.LoginIdentifier {
		t.Fatalf("expected identifier %s, got %s", cred.LoginIdentifier, byUserID.LoginIdentifier)
	}
}

func TestCredentialRepository_DuplicateIdentifierRejected(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewCredentialRepository(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	cred := domain.Credential{
		UserID:          uuid.New(),
		LoginIdentifier: "taken@example.com",
		PasswordHash:    "hashed-value",
		Status:          domain.CredentialStatusActive,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := repo.Create(ctx, cred); err != nil {
		t.Fatalf("seeding first credential: %v", err)
	}

	second := cred
	second.UserID = uuid.New()
	err := repo.Create(ctx, second)
	if err != domain.ErrEmailAlreadyExists {
		t.Fatalf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func TestCredentialRepository_UnknownIdentifierReturnsInvalidCredentials(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewCredentialRepository(pool)

	_, err := repo.GetByLoginIdentifier(context.Background(), "nobody@example.com")
	if err != domain.ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestRefreshTokenRepository_CreateGetRevoke(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewRefreshTokenRepository(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	token := domain.RefreshToken{
		TokenID:   uuid.New(),
		UserID:    uuid.New(),
		ExpiresAt: now.Add(24 * time.Hour),
	}
	if err := repo.Create(ctx, token); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByTokenID(ctx, token.TokenID)
	if err != nil {
		t.Fatalf("GetByTokenID: %v", err)
	}
	if got.RevokedAt != nil {
		t.Fatal("expected a freshly created token to be unrevoked")
	}

	revokedAt := now.Add(time.Minute)
	if err := repo.Revoke(ctx, token.TokenID, revokedAt); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	got, err = repo.GetByTokenID(ctx, token.TokenID)
	if err != nil {
		t.Fatalf("GetByTokenID after revoke: %v", err)
	}
	if got.RevokedAt == nil {
		t.Fatal("expected token to be revoked")
	}
}

func TestRefreshTokenRepository_UnknownTokenNotFound(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewRefreshTokenRepository(pool)
	ctx := context.Background()

	if _, err := repo.GetByTokenID(ctx, uuid.New()); err != domain.ErrTokenNotFound {
		t.Fatalf("expected ErrTokenNotFound, got %v", err)
	}

	if err := repo.Revoke(ctx, uuid.New(), time.Now()); err != domain.ErrTokenNotFound {
		t.Fatalf("expected ErrTokenNotFound on revoking an unknown token, got %v", err)
	}
}
