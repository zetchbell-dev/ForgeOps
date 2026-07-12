package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/enterprise-cicd-platform/auth-service/internal/domain"
	"github.com/enterprise-cicd-platform/auth-service/internal/usecase"
	"github.com/google/uuid"
)

func TestVerifyToken(t *testing.T) {
	tests := []struct {
		name string

		// issueForUserID, if non-nil, is used to have the fake issuer
		// mint a real token before Execute runs, so the "valid token"
		// case exercises an actual issued token rather than a hardcoded
		// string.
		issueForUserID *uuid.UUID
		tokenOverride  string
		verifyErr      error

		wantErr    error
		wantUserID uuid.UUID
	}{
		{
			name:           "valid token returns its claims",
			issueForUserID: uuidPtr(uuid.New()),
		},
		{
			name:          "unrecognized token is rejected as invalid",
			tokenOverride: "not-a-token-this-issuer-ever-signed",
			wantErr:       domain.ErrInvalidCredentials,
		},
		{
			name:          "expired token is rejected",
			tokenOverride: "irrelevant-forceVerifyErr-takes-over",
			verifyErr:     domain.ErrTokenExpired,
			wantErr:       domain.ErrTokenExpired,
		},
		{
			name:          "invalid signature is rejected",
			tokenOverride: "irrelevant-forceVerifyErr-takes-over",
			verifyErr:     domain.ErrInvalidCredentials,
			wantErr:       domain.ErrInvalidCredentials,
		},
		{
			name:          "unexpected issuer error surfaces as internal error",
			tokenOverride: "irrelevant-forceVerifyErr-takes-over",
			verifyErr:     errors.New("jwt library panic recovered"),
			wantErr:       domain.ErrInternal,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tokens := newFakeTokenIssuer()

			token := tt.tokenOverride
			var wantUserID uuid.UUID
			if tt.issueForUserID != nil {
				wantUserID = *tt.issueForUserID
				issued, _, err := tokens.IssueAccessToken(context.Background(), wantUserID)
				if err != nil {
					t.Fatalf("seeding issued token: %v", err)
				}
				token = issued
			}

			tokens.forceVerifyErr = tt.verifyErr

			deps := usecase.Deps{Tokens: tokens}

			out, err := usecase.NewVerifyToken(deps).Execute(context.Background(), usecase.VerifyTokenInput{AccessToken: token})

			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected success, got error: %v", err)
				}
				if out.Claims.UserID != wantUserID {
					t.Fatalf("expected claims for user %s, got %s", wantUserID, out.Claims.UserID)
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

func uuidPtr(id uuid.UUID) *uuid.UUID { return &id }
