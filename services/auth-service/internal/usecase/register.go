package usecase

import (
	"context"
	"fmt"

	"github.com/enterprise-cicd-platform/auth-service/internal/domain"
	"github.com/google/uuid"
)

// RegisterInput is what /v1/auth/register (M2 §4) passes in. Credential
// only — no name, no profile fields (M2 §2).
type RegisterInput struct {
	LoginIdentifier string
	Password        string
}

type RegisterOutput struct {
	UserID uuid.UUID
}

// Register creates a new credential and emits an account-created event for
// User Service to build the profile from (M2 §2/§4). It does not itself
// log the user in — a separate Login call is required, consistent with
// register/login being distinct endpoints in the API contract.
type Register struct {
	deps Deps
}

func NewRegister(deps Deps) *Register {
	return &Register{deps: deps}
}

func (uc *Register) Execute(ctx context.Context, in RegisterInput) (RegisterOutput, error) {
	hash, err := uc.deps.Hasher.Hash(ctx, in.Password)
	if err != nil {
		return RegisterOutput{}, fmt.Errorf("%w: hashing password: %v", domain.ErrInternal, err)
	}

	userID := uuid.New()
	now := uc.deps.Now()

	cred := domain.Credential{
		UserID:          userID,
		LoginIdentifier: in.LoginIdentifier,
		PasswordHash:    hash,
		Status:          domain.CredentialStatusActive,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := uc.deps.Credentials.Create(ctx, cred); err != nil {
		// ErrEmailAlreadyExists passes through unwrapped — it's already a
		// domain error the transport layer knows how to map (M2 §6).
		// Anything else is wrapped as internal.
		if err == domain.ErrEmailAlreadyExists {
			return RegisterOutput{}, err
		}
		return RegisterOutput{}, fmt.Errorf("%w: creating credential: %v", domain.ErrInternal, err)
	}

	// Event publish failure after a successful credential write is a known,
	// accepted gap for this milestone: retried delivery / an outbox pattern
	// is an infrastructure decision not yet specified in any milestone doc
	// (see the EventPublisher port comment in internal/domain/ports.go).
	// Register still succeeds — an account that exists but is slow to grow
	// a profile is recoverable; a account that silently failed to create is
	// not.
	if err := uc.deps.Events.PublishAccountCreated(ctx, userID); err != nil {
		return RegisterOutput{UserID: userID}, fmt.Errorf("%w: publishing account-created event: %v", domain.ErrInternal, err)
	}

	return RegisterOutput{UserID: userID}, nil
}
