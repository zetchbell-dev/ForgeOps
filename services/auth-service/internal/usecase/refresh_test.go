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

func TestRefresh(t *testing.T) {
	fixedNow := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	cfg := usecase.DefaultConfig()

	userID := uuid.New()

	// newSeededRepo creates a repository containing exactly one refresh
	// token, seeded under a freshly generated ID, and returns both so the
	// test case can use the ID as its input.
	newSeededRepo := func(revokedAt *time.Time, expiresAt time.Time) (*fakeRefreshTokenRepository, uuid.UUID) {
		tokenID := uuid.New()
		repo := newFakeRefreshTokenRepository()
		if err := repo.Create(context.Background(), domain.RefreshToken{
			TokenID:   tokenID,
			UserID:    userID,
			ExpiresAt: expiresAt,
			RevokedAt: revokedAt,
		}); err != nil {
			t.Fatalf("seeding refresh token: %v", err)
		}
		return repo, tokenID
	}

	tests := []struct {
		name string

		repo func() (*fakeRefreshTokenRepository, string) // returns repo and the token id to request

		cacheSetErr error

		issueErr error

		wantErr error
	}{
		{
			name: "successful refresh issues a new access token",
			repo: func() (*fakeRefreshTokenRepository, string) {
				repo, tokenID := newSeededRepo(nil, fixedNow.Add(24*time.Hour))
				return repo, tokenID.String()
			},
		},
		{
			name: "malformed token id is rejected as not found",
			repo: func() (*fakeRefreshTokenRepository, string) {
				return newFakeRefreshTokenRepository(), "not-a-valid-uuid"
			},
			wantErr: domain.ErrTokenNotFound,
		},
		{
			name: "unknown token id is rejected as not found",
			repo: func() (*fakeRefreshTokenRepository, string) {
				return newFakeRefreshTokenRepository(), uuid.New().String()
			},
			wantErr: domain.ErrTokenNotFound,
		},
		{
			name: "revoked token is rejected",
			repo: func() (*fakeRefreshTokenRepository, string) {
				repo, tokenID := newSeededRepo(&fixedNow, fixedNow.Add(24*time.Hour))
				return repo, tokenID.String()
			},
			wantErr: domain.ErrTokenRevoked,
		},
		{
			name: "expired token is rejected",
			repo: func() (*fakeRefreshTokenRepository, string) {
				repo, tokenID := newSeededRepo(nil, fixedNow.Add(-time.Second))
				return repo, tokenID.String()
			},
			wantErr: domain.ErrTokenExpired,
		},
		{
			name: "repository failure surfaces as internal error",
			repo: func() (*fakeRefreshTokenRepository, string) {
				repo := newFakeRefreshTokenRepository()
				repo.forceGetErr = errors.New("connection reset")
				return repo, uuid.New().String()
			},
			wantErr: domain.ErrInternal,
		},
		{
			name: "cache warming failure surfaces as internal error",
			repo: func() (*fakeRefreshTokenRepository, string) {
				repo, tokenID := newSeededRepo(nil, fixedNow.Add(24*time.Hour))
				return repo, tokenID.String()
			},
			cacheSetErr: errors.New("redis unavailable"),
			wantErr:     domain.ErrInternal,
		},
		{
			name: "access token issuance failure surfaces as internal error",
			repo: func() (*fakeRefreshTokenRepository, string) {
				repo, tokenID := newSeededRepo(nil, fixedNow.Add(24*time.Hour))
				return repo, tokenID.String()
			},
			issueErr: errors.New("signing key unavailable"),
			wantErr:  domain.ErrInternal,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			repo, tokenID := tt.repo()

			refreshCache := newFakeRefreshTokenCache()
			refreshCache.forceSetErr = tt.cacheSetErr

			tokens := newFakeTokenIssuer()
			tokens.forceIssueErr = tt.issueErr

			deps := usecase.Deps{
				RefreshTokens: repo,
				RefreshCache:  refreshCache,
				Tokens:        tokens,
				Now:           func() time.Time { return fixedNow },
			}

			out, err := usecase.NewRefresh(deps, cfg).Execute(context.Background(), usecase.RefreshInput{RefreshTokenID: tokenID})

			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected success, got error: %v", err)
				}
				if out.AccessToken == "" {
					t.Fatal("expected a non-empty access token on success")
				}
				if _, hit, _ := refreshCache.Get(context.Background(), uuid.MustParse(tokenID)); !hit {
					t.Fatal("expected refresh token to be cached")
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
