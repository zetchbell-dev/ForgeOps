package http_test

import (
	"context"
	"time"

	"github.com/enterprise-cicd-platform/auth-service/internal/domain"
	"github.com/google/uuid"
)

// The fakes below are deliberately minimal in-memory implementations of
// each domain port — enough to drive Register/Login/Verify end to end in
// handlers_test.go, not a general-purpose test double library. They are
// package-local to this _test.go file and never imported by production
// code.

type fakeCredentialRepo struct {
	byIdentifier map[string]domain.Credential
	byUserID     map[uuid.UUID]domain.Credential
}

func newFakeCredentialRepo() *fakeCredentialRepo {
	return &fakeCredentialRepo{
		byIdentifier: map[string]domain.Credential{},
		byUserID:     map[uuid.UUID]domain.Credential{},
	}
}

func (f *fakeCredentialRepo) Create(ctx context.Context, cred domain.Credential) error {
	if _, exists := f.byIdentifier[cred.LoginIdentifier]; exists {
		return domain.ErrEmailAlreadyExists
	}
	f.byIdentifier[cred.LoginIdentifier] = cred
	f.byUserID[cred.UserID] = cred
	return nil
}

func (f *fakeCredentialRepo) GetByLoginIdentifier(ctx context.Context, identifier string) (domain.Credential, error) {
	cred, ok := f.byIdentifier[identifier]
	if !ok {
		return domain.Credential{}, domain.ErrInvalidCredentials
	}
	return cred, nil
}

func (f *fakeCredentialRepo) GetByUserID(ctx context.Context, userID uuid.UUID) (domain.Credential, error) {
	cred, ok := f.byUserID[userID]
	if !ok {
		return domain.Credential{}, domain.ErrInvalidCredentials
	}
	return cred, nil
}

type fakeRefreshTokenRepo struct {
	tokens map[uuid.UUID]domain.RefreshToken
}

func newFakeRefreshTokenRepo() *fakeRefreshTokenRepo {
	return &fakeRefreshTokenRepo{tokens: map[uuid.UUID]domain.RefreshToken{}}
}

func (f *fakeRefreshTokenRepo) Create(ctx context.Context, token domain.RefreshToken) error {
	f.tokens[token.TokenID] = token
	return nil
}

func (f *fakeRefreshTokenRepo) GetByTokenID(ctx context.Context, tokenID uuid.UUID) (domain.RefreshToken, error) {
	token, ok := f.tokens[tokenID]
	if !ok {
		return domain.RefreshToken{}, domain.ErrTokenNotFound
	}
	return token, nil
}

func (f *fakeRefreshTokenRepo) Revoke(ctx context.Context, tokenID uuid.UUID, revokedAt time.Time) error {
	token, ok := f.tokens[tokenID]
	if !ok {
		return domain.ErrTokenNotFound
	}
	token.RevokedAt = &revokedAt
	f.tokens[tokenID] = token
	return nil
}

type fakeRefreshCache struct {
	entries map[uuid.UUID]domain.RefreshToken
}

func newFakeRefreshCache() *fakeRefreshCache {
	return &fakeRefreshCache{entries: map[uuid.UUID]domain.RefreshToken{}}
}

func (f *fakeRefreshCache) Get(ctx context.Context, tokenID uuid.UUID) (domain.RefreshToken, bool, error) {
	token, ok := f.entries[tokenID]
	return token, ok, nil
}

func (f *fakeRefreshCache) Set(ctx context.Context, token domain.RefreshToken, ttl time.Duration) error {
	f.entries[token.TokenID] = token
	return nil
}

func (f *fakeRefreshCache) Delete(ctx context.Context, tokenID uuid.UUID) error {
	delete(f.entries, tokenID)
	return nil
}

// fakeRateLimiter always allows — rate-limit behavior itself is already
// covered by internal/infrastructure/redis's integration tests; these
// handler tests only need Login to be callable.
type fakeRateLimiter struct{}

func (fakeRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	return true, nil
}

// fakeTokenIssuer encodes the user ID directly into the "token" string
// rather than doing anything cryptographic — real JWT behavior is already
// covered by internal/infrastructure/jwt's own tests; these tests only
// need a verifiable round trip.
type fakeTokenIssuer struct{}

const fakeTokenPrefix = "fake-access-token-"

func (fakeTokenIssuer) IssueAccessToken(ctx context.Context, userID uuid.UUID) (string, domain.AccessTokenClaims, error) {
	now := time.Now()
	claims := domain.AccessTokenClaims{UserID: userID, IssuedAt: now, ExpiresAt: now.Add(time.Hour)}
	return fakeTokenPrefix + userID.String(), claims, nil
}

func (fakeTokenIssuer) VerifyAccessToken(ctx context.Context, token string) (domain.AccessTokenClaims, error) {
	if len(token) <= len(fakeTokenPrefix) || token[:len(fakeTokenPrefix)] != fakeTokenPrefix {
		return domain.AccessTokenClaims{}, domain.ErrInvalidCredentials
	}
	id, err := uuid.Parse(token[len(fakeTokenPrefix):])
	if err != nil {
		return domain.AccessTokenClaims{}, domain.ErrInvalidCredentials
	}
	now := time.Now()
	return domain.AccessTokenClaims{UserID: id, IssuedAt: now, ExpiresAt: now.Add(time.Hour)}, nil
}

func (fakeTokenIssuer) NewRefreshTokenID() uuid.UUID {
	return uuid.New()
}

// fakeHasher is not a real hash — real bcrypt behavior is already covered
// by internal/infrastructure/bcrypt's own tests.
type fakeHasher struct{}

func (fakeHasher) Hash(ctx context.Context, plaintext string) (string, error) {
	return "hashed:" + plaintext, nil
}

func (fakeHasher) Verify(ctx context.Context, plaintext, hash string) (bool, error) {
	return hash == "hashed:"+plaintext, nil
}

type fakeEventPublisher struct{}

func (fakeEventPublisher) PublishAccountCreated(ctx context.Context, userID uuid.UUID) error {
	return nil
}
