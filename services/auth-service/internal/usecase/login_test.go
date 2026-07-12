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

func TestLogin(t *testing.T) {
	fixedNow := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	cfg := usecase.DefaultConfig()

	const (
		identifier = "person@example.com"
		password   = "correct-horse-battery-staple"
	)

	newSeededCreds := func(status domain.CredentialStatus) *fakeCredentialRepository {
		creds := newFakeCredentialRepository()
		_ = creds.Create(context.Background(), domain.Credential{
			UserID:          uuid.New(),
			LoginIdentifier: identifier,
			PasswordHash:    "hashed:" + password,
			Status:          status,
			CreatedAt:       fixedNow,
			UpdatedAt:       fixedNow,
		})
		return creds
	}

	tests := []struct {
		name string

		creds         *fakeCredentialRepository
		input         usecase.LoginInput
		hasherErr     error
		limiter       *fakeRateLimiter // nil means a fresh, unlimited one
		repoCreateErr error
		cacheSetErr   error

		wantErr error
	}{
		{
			name:  "successful login issues access and refresh tokens",
			creds: newSeededCreds(domain.CredentialStatusActive),
			input: usecase.LoginInput{IP: "203.0.113.1", LoginIdentifier: identifier, Password: password},
		},
		{
			name:    "unknown identifier returns invalid credentials, not a distinct not-found error",
			creds:   newFakeCredentialRepository(),
			input:   usecase.LoginInput{IP: "203.0.113.1", LoginIdentifier: "nobody@example.com", Password: password},
			wantErr: domain.ErrInvalidCredentials,
		},
		{
			name:    "wrong password returns invalid credentials",
			creds:   newSeededCreds(domain.CredentialStatusActive),
			input:   usecase.LoginInput{IP: "203.0.113.1", LoginIdentifier: identifier, Password: "wrong-password"},
			wantErr: domain.ErrInvalidCredentials,
		},
		{
			name:    "locked account is rejected before password verification",
			creds:   newSeededCreds(domain.CredentialStatusLocked),
			input:   usecase.LoginInput{IP: "203.0.113.1", LoginIdentifier: identifier, Password: "irrelevant-would-fail-anyway"},
			wantErr: domain.ErrAccountLocked,
		},
		{
			name:    "disabled account is rejected before password verification",
			creds:   newSeededCreds(domain.CredentialStatusDisabled),
			input:   usecase.LoginInput{IP: "203.0.113.1", LoginIdentifier: identifier, Password: "irrelevant-would-fail-anyway"},
			wantErr: domain.ErrAccountDisabled,
		},
		{
			name:  "per-IP rate limit exhausted blocks the attempt",
			creds: newSeededCreds(domain.CredentialStatusActive),
			input: usecase.LoginInput{IP: "203.0.113.1", LoginIdentifier: identifier, Password: password},
			limiter: func() *fakeRateLimiter {
				l := newFakeRateLimiter()
				l.denyKeyPrefix = "ratelimit:login:ip:"
				return l
			}(),
			wantErr: domain.ErrRateLimited,
		},
		{
			name:  "per-account rate limit exhausted blocks the attempt even with fresh IP",
			creds: newSeededCreds(domain.CredentialStatusActive),
			input: usecase.LoginInput{IP: "203.0.113.1", LoginIdentifier: identifier, Password: password},
			limiter: func() *fakeRateLimiter {
				l := newFakeRateLimiter()
				l.denyKeyPrefix = "ratelimit:login:user:"
				return l
			}(),
			wantErr: domain.ErrRateLimited,
		},
		{
			name:  "rate limiter infra failure surfaces as internal error",
			creds: newSeededCreds(domain.CredentialStatusActive),
			input: usecase.LoginInput{IP: "203.0.113.1", LoginIdentifier: identifier, Password: password},
			limiter: func() *fakeRateLimiter {
				l := newFakeRateLimiter()
				l.forceErr = errors.New("redis unavailable")
				return l
			}(),
			wantErr: domain.ErrInternal,
		},
		{
			name:      "hasher infra failure on a known account surfaces as internal error",
			creds:     newSeededCreds(domain.CredentialStatusActive),
			input:     usecase.LoginInput{IP: "203.0.113.1", LoginIdentifier: identifier, Password: password},
			hasherErr: errors.New("bcrypt library panic recovered"),
			wantErr:   domain.ErrInternal,
		},
		{
			name:          "refresh token persistence failure surfaces as internal error",
			creds:         newSeededCreds(domain.CredentialStatusActive),
			input:         usecase.LoginInput{IP: "203.0.113.1", LoginIdentifier: identifier, Password: password},
			repoCreateErr: errors.New("connection reset"),
			wantErr:       domain.ErrInternal,
		},
		{
			name:        "refresh token cache failure surfaces as internal error",
			creds:       newSeededCreds(domain.CredentialStatusActive),
			input:       usecase.LoginInput{IP: "203.0.113.1", LoginIdentifier: identifier, Password: password},
			cacheSetErr: errors.New("redis unavailable"),
			wantErr:     domain.ErrInternal,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			refreshRepo := newFakeRefreshTokenRepository()
			refreshCache := newFakeRefreshTokenCache()
			tokens := newFakeTokenIssuer()
			hasher := fakeHasher{forceVerifyErr: tt.hasherErr}

			limiter := tt.limiter
			if limiter == nil {
				limiter = newFakeRateLimiter()
			}

			refreshRepo.forceCreateErr = tt.repoCreateErr
			refreshCache.forceSetErr = tt.cacheSetErr

			deps := usecase.Deps{
				Credentials:   tt.creds,
				RefreshTokens: refreshRepo,
				RefreshCache:  refreshCache,
				RateLimiter:   limiter,
				Tokens:        tokens,
				Hasher:        hasher,
				Now:           func() time.Time { return fixedNow },
			}

			out, err := usecase.NewLogin(deps, cfg).Execute(context.Background(), tt.input)

			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected success, got error: %v", err)
				}
				if out.AccessToken == "" {
					t.Fatal("expected a non-empty access token on success")
				}
				if out.RefreshTokenID == "" {
					t.Fatal("expected a non-empty refresh token id on success")
				}
				refreshTokenUUID, parseErr := uuid.Parse(out.RefreshTokenID)
				if parseErr != nil {
					t.Fatalf("expected refresh token id to be a valid uuid: %v", parseErr)
				}
				stored, getErr := refreshRepo.GetByTokenID(context.Background(), refreshTokenUUID)
				if getErr != nil {
					t.Fatalf("expected refresh token to be persisted: %v", getErr)
				}
				if stored.UserID != out.UserID {
					t.Fatalf("expected persisted refresh token to belong to user %s, got %s", out.UserID, stored.UserID)
				}
				if _, hit, _ := refreshCache.Get(context.Background(), refreshTokenUUID); !hit {
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
