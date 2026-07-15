package domain_test

import (
	"testing"
	"time"

	"github.com/enterprise-cicd-platform/auth-service/internal/domain"
)

func TestCredential_CanAuthenticate(t *testing.T) {
	tests := []struct {
		name   string
		status domain.CredentialStatus
		want   bool
	}{
		{"active status can authenticate", domain.CredentialStatusActive, true},
		{"locked status cannot authenticate", domain.CredentialStatusLocked, false},
		{"disabled status cannot authenticate", domain.CredentialStatusDisabled, false},
		{"unknown/zero-value status cannot authenticate", domain.CredentialStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred := domain.Credential{Status: tt.status}
			if got := cred.CanAuthenticate(); got != tt.want {
				t.Errorf("Credential{Status: %q}.CanAuthenticate() = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestRefreshToken_IsValid(t *testing.T) {
	now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	revokedTime := now.Add(-time.Minute)

	tests := []struct {
		name  string
		token domain.RefreshToken
		now   time.Time
		want  bool
	}{
		{
			name:  "unexpired unrevoked token is valid",
			token: domain.RefreshToken{ExpiresAt: now.Add(time.Hour), RevokedAt: nil},
			now:   now,
			want:  true,
		},
		{
			name:  "revoked token is invalid even if not yet expired",
			token: domain.RefreshToken{ExpiresAt: now.Add(time.Hour), RevokedAt: &revokedTime},
			now:   now,
			want:  false,
		},
		{
			name:  "expired token is invalid",
			token: domain.RefreshToken{ExpiresAt: now.Add(-time.Second), RevokedAt: nil},
			now:   now,
			want:  false,
		},
		{
			name:  "token expiring exactly now is invalid (not strictly before expiry)",
			token: domain.RefreshToken{ExpiresAt: now, RevokedAt: nil},
			now:   now,
			want:  false,
		},
		{
			name:  "revoked and expired token is invalid",
			token: domain.RefreshToken{ExpiresAt: now.Add(-time.Hour), RevokedAt: &revokedTime},
			now:   now,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.token.IsValid(tt.now); got != tt.want {
				t.Errorf("IsValid(%v) = %v, want %v", tt.now, got, tt.want)
			}
		})
	}
}
