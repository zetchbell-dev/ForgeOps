// Package bcrypt implements domain.PasswordHasher (M2 §3/§9) against
// golang.org/x/crypto/bcrypt. The cost factor is this package's constant
// to own, not usecase's — M2 §9 ties it to the M5 p99 login-latency SLO,
// which is an infrastructure/ops tradeoff (higher cost = stronger hash,
// slower login, more CPU per request), not an application-logic decision.
package bcrypt

import (
	"context"
	"errors"
	"fmt"

	xbcrypt "golang.org/x/crypto/bcrypt"
)

// DefaultCost is the starting cost factor (M2 §9's "starting points, not
// tuned values" caveat — see usecase.DefaultConfig's identical framing —
// applies here too, pending real p99 latency data from M5's observability
// stack). 12 is one above bcrypt's own default (10) and was chosen as a
// deliberately conservative starting point given Auth Service sits on the
// critical path of every login.
const DefaultCost = 12

// Hasher implements domain.PasswordHasher.
type Hasher struct {
	cost int
}

// NewHasher validates cost against bcrypt's supported range and returns a
// Hasher. Validated once at composition-root wiring time, not per call,
// so a misconfigured cost fails Auth Service's startup rather than its
// first register/login request.
func NewHasher(cost int) (*Hasher, error) {
	if cost < xbcrypt.MinCost || cost > xbcrypt.MaxCost {
		return nil, fmt.Errorf("bcrypt: cost %d out of range [%d, %d]", cost, xbcrypt.MinCost, xbcrypt.MaxCost)
	}
	return &Hasher{cost: cost}, nil
}

// Hash implements domain.PasswordHasher.
func (h *Hasher) Hash(ctx context.Context, plaintext string) (string, error) {
	hash, err := xbcrypt.GenerateFromPassword([]byte(plaintext), h.cost)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}
	return string(hash), nil
}

// Verify implements domain.PasswordHasher. bcrypt.CompareHashAndPassword
// itself always re-derives the hash from plaintext and does a full
// comparison before returning — there is no early-exit path inside it —
// which is what satisfies the port's constant-time requirement (M2 §6):
// the only way a caller could break that guarantee is short-circuiting
// *before* calling Verify, which is exactly what login.go's
// dummyPasswordHash exists to prevent for the "identifier not found"
// case.
//
// A password mismatch is reported as (false, nil), not an error — it's an
// expected outcome, not a failure. A malformed/corrupt hash argument (not
// a real bcrypt hash at all) returns a non-nil error, since that
// indicates a data problem in what was stored, not a real verification
// outcome.
func (h *Hasher) Verify(ctx context.Context, plaintext, hash string) (bool, error) {
	err := xbcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext))
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, xbcrypt.ErrMismatchedHashAndPassword):
		return false, nil
	default:
		return false, fmt.Errorf("verifying password: %w", err)
	}
}
