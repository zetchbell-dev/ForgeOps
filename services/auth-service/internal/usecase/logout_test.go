package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/enterprise-cicd-platform/auth-service/internal/domain"
	"github.com/enterprise-cicd-platform/auth-service/internal/usecase"
	"github.com/google/uuid"
)

func TestLogout(t *testing.T) {
	fixedNow := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	userID := uuid.New()

	// newSeededRepo creates a repository containing exactly one
	// unrevoked refresh token, seeded under a freshly generated ID, and
	// returns both so the test case can use the ID as its input.
	newSeededRepo := func() (*fakeRefreshTokenRepository, uuid.UUID) {
		tokenID := uuid.New()
		repo := newFakeRefreshTokenRepository()
		if err := repo.Create(context.Background(), domain.RefreshToken{
			TokenID:   tokenID,
			UserID:    userID,
			ExpiresAt: fixedNow.Add(24 * time.Hour),
		}); err != nil {
			t.Fatalf("seeding refresh token: %v", err)
		}
		return repo, tokenID
	}

	tests := []struct {
		name string

		repo  func() (*fakeRefreshTokenRepository, string) // returns repo and the token id to request
		cache func() *fakeRefreshTokenCache

		wantErr error
	}{
		{
			name: "successful logout revokes token and clears cache",
			repo: func() (*fakeRefreshTokenRepository, string) {
				repo, tokenID := newSeededRepo()
				return repo, tokenID.String()
			},
			cache: newFakeRefreshTokenCache,
		},
		{
			name: "invalid (malformed) refresh token id is rejected as not found",
			repo: func() (*fakeRefreshTokenRepository, string) {
				return newFakeRefreshTokenRepository(), "not-a-valid-uuid"
			},
			cache:   newFakeRefreshTokenCache,
			wantErr: domain.ErrTokenNotFound,
		},
		{
			name: "unknown token id is rejected as not found",
			repo: func() (*fakeRefreshTokenRepository, string) {
				return newFakeRefreshTokenRepository(), uuid.New().String()
			},
			cache:   newFakeRefreshTokenCache,
			wantErr: domain.ErrTokenNotFound,
		},
		{
			name: "repository revoke failure surfaces as internal error",
			repo: func() (*fakeRefreshTokenRepository, string) {
				repo, tokenID := newSeededRepo()
				repo.forceRevokeErr = errors.New("connection reset")
				return repo, tokenID.String()
			},
			cache:   newFakeRefreshTokenCache,
			wantErr: domain.ErrInternal,
		},
		{
			name: "cache deletion failure surfaces as internal error",
			repo: func() (*fakeRefreshTokenRepository, string) {
				repo, tokenID := newSeededRepo()
				return repo, tokenID.String()
			},
			cache: func() *fakeRefreshTokenCache {
				c := newFakeRefreshTokenCache()
				c.forceDeleteErr = errors.New("redis unavailable")
				return c
			},
			wantErr: domain.ErrInternal,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			repo, tokenID := tt.repo()
			cache := tt.cache()

			deps := usecase.Deps{
				RefreshTokens: repo,
				RefreshCache:  cache,
				Now:           func() time.Time { return fixedNow },
			}

			err := usecase.NewLogout(deps).Execute(context.Background(), usecase.LogoutInput{RefreshTokenID: tokenID})

			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected success, got error: %v", err)
				}

				// Repository update: the token must be durably marked
				// revoked in Postgres (the source of truth) …
				parsed := uuid.MustParse(tokenID)
				stored, getErr := repo.GetByTokenID(context.Background(), parsed)
				if getErr != nil {
					t.Fatalf("expected token still present in repo after logout, got error: %v", getErr)
				}
				if stored.RevokedAt == nil {
					t.Fatal("expected RevokedAt to be set after logout")
				}
				if !stored.RevokedAt.Equal(fixedNow) {
					t.Errorf("RevokedAt = %v, want %v", stored.RevokedAt, fixedNow)
				}

				// … and cache deletion: the hot cache entry must be
				// dropped so a subsequent Refresh can't serve a stale
				// unrevoked copy.
				if _, hit, _ := cache.Get(context.Background(), parsed); hit {
					t.Error("expected refresh token to be removed from cache after logout")
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error %v, got success", tt.wantErr)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error wrapping %v, got %v", tt.wantErr, err)
			}
		})
	}
}

// TestLogout_Idempotent verifies that logging out an already-revoked
// refresh token succeeds rather than erroring — Logout.Execute has no
// "already revoked" branch of its own (see logout.go), and neither a
// Postgres UPDATE of an already-revoked row nor a Redis DELETE of an
// already-missing key is itself a failure condition.
func TestLogout_Idempotent(t *testing.T) {
	fixedNow := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	userID := uuid.New()
	tokenID := uuid.New()

	repo := newFakeRefreshTokenRepository()
	if err := repo.Create(context.Background(), domain.RefreshToken{
		TokenID:   tokenID,
		UserID:    userID,
		ExpiresAt: fixedNow.Add(24 * time.Hour),
	}); err != nil {
		t.Fatalf("seeding refresh token: %v", err)
	}

	cache := newFakeRefreshTokenCache()
	if err := cache.Set(context.Background(), domain.RefreshToken{
		TokenID:   tokenID,
		UserID:    userID,
		ExpiresAt: fixedNow.Add(24 * time.Hour),
	}, time.Hour); err != nil {
		t.Fatalf("seeding cache: %v", err)
	}

	deps := usecase.Deps{
		RefreshTokens: repo,
		RefreshCache:  cache,
		Now:           func() time.Time { return fixedNow },
	}
	uc := usecase.NewLogout(deps)

	if err := uc.Execute(context.Background(), usecase.LogoutInput{RefreshTokenID: tokenID.String()}); err != nil {
		t.Fatalf("first logout: expected success, got error: %v", err)
	}

	if err := uc.Execute(context.Background(), usecase.LogoutInput{RefreshTokenID: tokenID.String()}); err != nil {
		t.Fatalf("second logout: expected idempotent success, got error: %v", err)
	}

	stored, err := repo.GetByTokenID(context.Background(), tokenID)
	if err != nil {
		t.Fatalf("expected token still present in repo, got error: %v", err)
	}
	if stored.RevokedAt == nil {
		t.Error("expected RevokedAt to remain set after second logout")
	}

	if _, hit, _ := cache.Get(context.Background(), tokenID); hit {
		t.Error("expected cache entry to remain absent after second logout")
	}
}
