//go:build integration

// Package redis_test holds M2 §8's infrastructure-layer integration test
// requirement, mirroring internal/infrastructure/postgres's
// repository_integration_test.go: a real dependency spun up via
// testcontainers-go, not a fake, since the fakes in
// internal/usecase/fakes_test.go already prove the use cases are correct
// against the port contracts — what's unproven until this file is the
// Redis-specific behavior underneath those contracts (HGETALL field
// presence semantics, sorted-set eviction, key TTLs).
//
// Build-tagged `integration` for the same reason as the postgres test:
// needs a Docker daemon, run in CI's integration stage (M3-ci-pipeline.md),
// not in an ordinary `go test ./...`.
package redis_test

import (
	"context"
	"testing"
	"time"

	"github.com/enterprise-cicd-platform/auth-service/internal/domain"
	authredis "github.com/enterprise-cicd-platform/auth-service/internal/infrastructure/redis"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

// newTestClient starts a throwaway Redis container and returns a
// connected client. t.Cleanup tears the container down.
func newTestClient(t *testing.T) *goredis.Client {
	t.Helper()
	ctx := context.Background()

	container, err := tcredis.Run(ctx, "redis:7.2-alpine")
	if err != nil {
		t.Fatalf("starting redis container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("terminating redis container: %v", err)
		}
	})

	addr, err := container.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("getting redis endpoint: %v", err)
	}

	client, err := authredis.NewClient(ctx, authredis.ClientConfig{Addr: addr})
	if err != nil {
		t.Fatalf("connecting to redis: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	return client
}

func TestRefreshTokenCache_SetGetRoundTrip(t *testing.T) {
	ctx := context.Background()
	cache := authredis.NewRefreshTokenCache(newTestClient(t))

	token := domain.RefreshToken{
		TokenID:   uuid.New(),
		UserID:    uuid.New(),
		ExpiresAt: time.Now().Add(time.Hour).Truncate(time.Millisecond),
	}

	if err := cache.Set(ctx, token, time.Hour); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, found, err := cache.Get(ctx, token.TokenID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("expected cache hit, got miss")
	}
	if got.UserID != token.UserID {
		t.Errorf("UserID = %v, want %v", got.UserID, token.UserID)
	}
	if !got.ExpiresAt.Equal(token.ExpiresAt) {
		t.Errorf("ExpiresAt = %v, want %v", got.ExpiresAt, token.ExpiresAt)
	}
	if got.RevokedAt != nil {
		t.Errorf("RevokedAt = %v, want nil", got.RevokedAt)
	}
}

func TestRefreshTokenCache_Miss(t *testing.T) {
	ctx := context.Background()
	cache := authredis.NewRefreshTokenCache(newTestClient(t))

	_, found, err := cache.Get(ctx, uuid.New())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if found {
		t.Fatal("expected cache miss for a token that was never set")
	}
}

func TestRefreshTokenCache_RevokedRoundTrip(t *testing.T) {
	ctx := context.Background()
	cache := authredis.NewRefreshTokenCache(newTestClient(t))

	revokedAt := time.Now().Truncate(time.Millisecond)
	token := domain.RefreshToken{
		TokenID:   uuid.New(),
		UserID:    uuid.New(),
		ExpiresAt: time.Now().Add(time.Hour).Truncate(time.Millisecond),
		RevokedAt: &revokedAt,
	}

	if err := cache.Set(ctx, token, time.Hour); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, found, err := cache.Get(ctx, token.TokenID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("expected cache hit, got miss")
	}
	if got.RevokedAt == nil {
		t.Fatal("RevokedAt = nil, want non-nil")
	}
	if !got.RevokedAt.Equal(revokedAt) {
		t.Errorf("RevokedAt = %v, want %v", *got.RevokedAt, revokedAt)
	}
	if got.IsValid(time.Now()) {
		t.Error("IsValid() = true for a revoked token, want false")
	}
}

func TestRefreshTokenCache_Delete(t *testing.T) {
	ctx := context.Background()
	cache := authredis.NewRefreshTokenCache(newTestClient(t))

	token := domain.RefreshToken{
		TokenID:   uuid.New(),
		UserID:    uuid.New(),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := cache.Set(ctx, token, time.Hour); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := cache.Delete(ctx, token.TokenID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, found, err := cache.Get(ctx, token.TokenID)
	if err != nil {
		t.Fatalf("Get after Delete: %v", err)
	}
	if found {
		t.Fatal("expected cache miss after Delete")
	}
}

func TestRateLimiter_AllowsWithinLimit(t *testing.T) {
	ctx := context.Background()
	limiter := authredis.NewRateLimiter(newTestClient(t))
	key := "ratelimit:test:" + uuid.New().String()

	for i := 0; i < 3; i++ {
		ok, err := limiter.Allow(ctx, key, 3, time.Minute)
		if err != nil {
			t.Fatalf("Allow (call %d): %v", i, err)
		}
		if !ok {
			t.Fatalf("Allow (call %d) = false, want true (within limit)", i)
		}
	}
}

func TestRateLimiter_RejectsOverLimit(t *testing.T) {
	ctx := context.Background()
	limiter := authredis.NewRateLimiter(newTestClient(t))
	key := "ratelimit:test:" + uuid.New().String()

	for i := 0; i < 2; i++ {
		if _, err := limiter.Allow(ctx, key, 2, time.Minute); err != nil {
			t.Fatalf("Allow (call %d): %v", i, err)
		}
	}

	ok, err := limiter.Allow(ctx, key, 2, time.Minute)
	if err != nil {
		t.Fatalf("Allow (call 3): %v", err)
	}
	if ok {
		t.Fatal("Allow (call 3) = true, want false (over limit)")
	}
}

func TestRateLimiter_WindowSlidesOldEntriesOut(t *testing.T) {
	ctx := context.Background()
	limiter := authredis.NewRateLimiter(newTestClient(t))
	key := "ratelimit:test:" + uuid.New().String()

	// Fill the limit in a very short window, then wait it out; the next
	// call should succeed because the old entries have aged past the
	// window, not because the key was reset.
	window := 300 * time.Millisecond
	for i := 0; i < 2; i++ {
		if _, err := limiter.Allow(ctx, key, 2, window); err != nil {
			t.Fatalf("Allow (call %d): %v", i, err)
		}
	}

	time.Sleep(window + 200*time.Millisecond)

	ok, err := limiter.Allow(ctx, key, 2, window)
	if err != nil {
		t.Fatalf("Allow after window slide: %v", err)
	}
	if !ok {
		t.Fatal("Allow after window slide = false, want true (old entries should have expired out)")
	}
}
