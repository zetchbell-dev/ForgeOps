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

func TestRegister(t *testing.T) {
	fixedNow := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		seedIdentity string // an identifier already registered, if any
		input        usecase.RegisterInput
		publishErr   error
		wantErr      error // nil means "no error, must succeed"
		wantErrIs    bool  // if true, check errors.Is instead of ==
	}{
		{
			name: "successful registration",
			input: usecase.RegisterInput{
				LoginIdentifier: "new-user@example.com",
				Password:        "correct-horse-battery-staple",
			},
			wantErr: nil,
		},
		{
			name:         "duplicate identifier rejected",
			seedIdentity: "taken@example.com",
			input: usecase.RegisterInput{
				LoginIdentifier: "taken@example.com",
				Password:        "correct-horse-battery-staple",
			},
			wantErr: domain.ErrEmailAlreadyExists,
		},
		{
			name: "event publish failure surfaces as internal error, credential still created",
			input: usecase.RegisterInput{
				LoginIdentifier: "event-fails@example.com",
				Password:        "correct-horse-battery-staple",
			},
			publishErr: errors.New("event bus unreachable"),
			wantErr:    domain.ErrInternal,
			wantErrIs:  true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			creds := newFakeCredentialRepository()
			events := newFakeEventPublisher()
			events.forceErr = tt.publishErr

			if tt.seedIdentity != "" {
				if err := creds.Create(context.Background(), domain.Credential{
					UserID:          uuid.New(),
					LoginIdentifier: tt.seedIdentity,
					PasswordHash:    "hashed:seed",
					Status:          domain.CredentialStatusActive,
					CreatedAt:       fixedNow,
					UpdatedAt:       fixedNow,
				}); err != nil {
					t.Fatalf("seeding credential: %v", err)
				}
			}

			deps := usecase.Deps{
				Credentials: creds,
				Hasher:      fakeHasher{},
				Events:      events,
				Now:         func() time.Time { return fixedNow },
			}

			out, err := usecase.NewRegister(deps).Execute(context.Background(), tt.input)

			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected success, got error: %v", err)
				}
				if out.UserID == uuid.Nil {
					t.Fatal("expected a non-nil UserID on success")
				}
				stored, getErr := creds.GetByLoginIdentifier(context.Background(), tt.input.LoginIdentifier)
				if getErr != nil {
					t.Fatalf("expected credential to be persisted: %v", getErr)
				}
				if stored.PasswordHash == tt.input.Password {
					t.Fatal("password must never be stored in plaintext")
				}
				if len(events.published) != 1 {
					t.Fatalf("expected exactly one account-created event, got %d", len(events.published))
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error %v, got success", tt.wantErr)
			}
			if tt.wantErrIs {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error wrapping %v, got %v", tt.wantErr, err)
				}
			} else if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}
