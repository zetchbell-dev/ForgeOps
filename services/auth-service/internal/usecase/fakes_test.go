package usecase_test

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/enterprise-cicd-platform/auth-service/internal/domain"
	"github.com/google/uuid"
)

// fakeCredentialRepository is an in-memory domain.CredentialRepository.
// Table-driven tests in this package construct one per test case rather
// than sharing state across cases.
type fakeCredentialRepository struct {
	mu         sync.Mutex
	byUserID   map[uuid.UUID]domain.Credential
	byIdentity map[string]uuid.UUID

	// forceErr, if set, is returned by every method — used to simulate an
	// infra failure (M2 §6: infra errors wrap into ErrInternal upstream).
	forceErr error
}

func newFakeCredentialRepository() *fakeCredentialRepository {
	return &fakeCredentialRepository{
		byUserID:   make(map[uuid.UUID]domain.Credential),
		byIdentity: make(map[string]uuid.UUID),
	}
}

func (f *fakeCredentialRepository) Create(_ context.Context, cred domain.Credential) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.forceErr != nil {
		return f.forceErr
	}
	if _, exists := f.byIdentity[cred.LoginIdentifier]; exists {
		return domain.ErrEmailAlreadyExists
	}
	f.byUserID[cred.UserID] = cred
	f.byIdentity[cred.LoginIdentifier] = cred.UserID
	return nil
}

func (f *fakeCredentialRepository) GetByLoginIdentifier(_ context.Context, identifier string) (domain.Credential, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.forceErr != nil {
		return domain.Credential{}, f.forceErr
	}
	id, ok := f.byIdentity[identifier]
	if !ok {
		return domain.Credential{}, domain.ErrInvalidCredentials
	}
	return f.byUserID[id], nil
}

func (f *fakeCredentialRepository) GetByUserID(_ context.Context, userID uuid.UUID) (domain.Credential, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.forceErr != nil {
		return domain.Credential{}, f.forceErr
	}
	cred, ok := f.byUserID[userID]
	if !ok {
		return domain.Credential{}, domain.ErrInvalidCredentials
	}
	return cred, nil
}

// fakeRefreshTokenRepository is an in-memory domain.RefreshTokenRepository
// (the Postgres source-of-truth port).
type fakeRefreshTokenRepository struct {
	mu     sync.Mutex
	tokens map[uuid.UUID]domain.RefreshToken

	// forceGetErr / forceRevokeErr, if set, are returned by GetByTokenID /
	// Revoke respectively regardless of what's in tokens — used to
	// simulate a Postgres failure that is NOT "row doesn't exist" (that
	// case is already covered by the ordinary not-found path above).
	forceGetErr    error
	forceRevokeErr error

	// forceCreateErr, if set, is returned by Create — used to simulate a
	// Postgres write failure when Login issues a new refresh token.
	forceCreateErr error
}

func newFakeRefreshTokenRepository() *fakeRefreshTokenRepository {
	return &fakeRefreshTokenRepository{tokens: make(map[uuid.UUID]domain.RefreshToken)}
}

func (f *fakeRefreshTokenRepository) Create(_ context.Context, token domain.RefreshToken) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.forceCreateErr != nil {
		return f.forceCreateErr
	}
	f.tokens[token.TokenID] = token
	return nil
}

func (f *fakeRefreshTokenRepository) GetByTokenID(_ context.Context, tokenID uuid.UUID) (domain.RefreshToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.forceGetErr != nil {
		return domain.RefreshToken{}, f.forceGetErr
	}
	t, ok := f.tokens[tokenID]
	if !ok {
		return domain.RefreshToken{}, domain.ErrTokenNotFound
	}
	return t, nil
}

func (f *fakeRefreshTokenRepository) Revoke(_ context.Context, tokenID uuid.UUID, revokedAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.forceRevokeErr != nil {
		return f.forceRevokeErr
	}
	t, ok := f.tokens[tokenID]
	if !ok {
		return domain.ErrTokenNotFound
	}
	t.RevokedAt = &revokedAt
	f.tokens[tokenID] = t
	return nil
}

// fakeRefreshTokenCache is an in-memory domain.RefreshTokenCache (the Redis
// hot-cache port). Deliberately allowed to "miss" independently of the
// repository above, so tests can exercise the cache-miss-falls-back-to-repo
// path (M2 §5).
type fakeRefreshTokenCache struct {
	mu    sync.Mutex
	cache map[uuid.UUID]domain.RefreshToken

	// forceSetErr / forceDeleteErr, if set, are returned by Set / Delete
	// respectively — used to simulate a Redis failure (Refresh treats a
	// Set failure as fatal per its cache-warming comment; Logout treats a
	// Delete failure as non-fatal to the already-durable revoke but still
	// surfaces it as an internal error to the caller).
	forceSetErr    error
	forceDeleteErr error
}

func newFakeRefreshTokenCache() *fakeRefreshTokenCache {
	return &fakeRefreshTokenCache{cache: make(map[uuid.UUID]domain.RefreshToken)}
}

func (f *fakeRefreshTokenCache) Get(_ context.Context, tokenID uuid.UUID) (domain.RefreshToken, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.cache[tokenID]
	return t, ok, nil
}

func (f *fakeRefreshTokenCache) Set(_ context.Context, token domain.RefreshToken, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.forceSetErr != nil {
		return f.forceSetErr
	}
	f.cache[token.TokenID] = token
	return nil
}

func (f *fakeRefreshTokenCache) Delete(_ context.Context, tokenID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.forceDeleteErr != nil {
		return f.forceDeleteErr
	}
	delete(f.cache, tokenID)
	return nil
}

// fakeRateLimiter is an in-memory domain.RateLimiter. Counts calls per key
// with no real time-window decay — tests that need window-expiry behavior
// set the threshold directly rather than relying on wall-clock time.
type fakeRateLimiter struct {
	mu     sync.Mutex
	counts map[string]int

	// forceErr, if set, is returned by Allow when key has forceErrPrefix
	// (or always, if forceErrPrefix is empty) — used to simulate a Redis
	// failure during rate-limit checking, scoped to just the per-IP or
	// per-account check so tests can exercise each independently.
	forceErr       error
	forceErrPrefix string

	// denyKeyPrefix, if set, makes Allow report "not allowed" for any key
	// with this prefix on its very first call, without needing to tune
	// limit/window — used to simulate an already-exhausted rate limit
	// scoped to just the per-IP or per-account check.
	denyKeyPrefix string
}

func newFakeRateLimiter() *fakeRateLimiter {
	return &fakeRateLimiter{counts: make(map[string]int)}
}

func (f *fakeRateLimiter) Allow(_ context.Context, key string, limit int, _ time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.forceErr != nil && (f.forceErrPrefix == "" || strings.HasPrefix(key, f.forceErrPrefix)) {
		return false, f.forceErr
	}
	if f.denyKeyPrefix != "" && strings.HasPrefix(key, f.denyKeyPrefix) {
		return false, nil
	}
	f.counts[key]++
	return f.counts[key] <= limit, nil
}

// fakeTokenIssuer is an in-memory domain.TokenIssuer. Access tokens are
// just their claims' UserID as a string — this fake never needs to satisfy
// a real JWT parser, only the domain.TokenIssuer contract.
type fakeTokenIssuer struct {
	mu     sync.Mutex
	tokens map[string]domain.AccessTokenClaims

	// forceVerifyErr, if set, is returned by VerifyAccessToken — used to
	// simulate an expired/invalid token.
	forceVerifyErr error

	// forceIssueErr, if set, is returned by IssueAccessToken — used to
	// simulate a signing-key/infrastructure failure at issuance time.
	forceIssueErr error
}

func newFakeTokenIssuer() *fakeTokenIssuer {
	return &fakeTokenIssuer{tokens: make(map[string]domain.AccessTokenClaims)}
}

func (f *fakeTokenIssuer) IssueAccessToken(_ context.Context, userID uuid.UUID) (string, domain.AccessTokenClaims, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.forceIssueErr != nil {
		return "", domain.AccessTokenClaims{}, f.forceIssueErr
	}
	claims := domain.AccessTokenClaims{
		UserID:    userID,
		IssuedAt:  time.Now(),
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
	token := "fake-access-token-" + userID.String()
	f.tokens[token] = claims
	return token, claims, nil
}

func (f *fakeTokenIssuer) VerifyAccessToken(_ context.Context, token string) (domain.AccessTokenClaims, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.forceVerifyErr != nil {
		return domain.AccessTokenClaims{}, f.forceVerifyErr
	}
	claims, ok := f.tokens[token]
	if !ok {
		return domain.AccessTokenClaims{}, domain.ErrInvalidCredentials
	}
	return claims, nil
}

func (f *fakeTokenIssuer) NewRefreshTokenID() uuid.UUID {
	return uuid.New()
}

// fakeHasher is an in-memory domain.PasswordHasher. Not bcrypt — a fake
// standing in for the port contract, since usecase must not care which
// concrete hashing algorithm is behind the interface.
type fakeHasher struct {
	// forceVerifyErr, if set, is returned by Verify — used to simulate a
	// bcrypt-layer infrastructure failure distinct from an ordinary
	// mismatch (which returns false, nil, not an error).
	forceVerifyErr error
}

func (f fakeHasher) Hash(_ context.Context, plaintext string) (string, error) {
	return "hashed:" + plaintext, nil
}

func (f fakeHasher) Verify(_ context.Context, plaintext, hash string) (bool, error) {
	if f.forceVerifyErr != nil {
		return false, f.forceVerifyErr
	}
	return "hashed:"+plaintext == hash, nil
}

// fakeEventPublisher is an in-memory domain.EventPublisher recording every
// published event for assertions.
type fakeEventPublisher struct {
	mu        sync.Mutex
	published []uuid.UUID
	forceErr  error
}

func newFakeEventPublisher() *fakeEventPublisher {
	return &fakeEventPublisher{}
}

func (f *fakeEventPublisher) PublishAccountCreated(_ context.Context, userID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.forceErr != nil {
		return f.forceErr
	}
	f.published = append(f.published, userID)
	return nil
}
