package bcrypt_test

import (
	"context"
	"testing"

	authbcrypt "github.com/enterprise-cicd-platform/auth-service/internal/infrastructure/bcrypt"
	xbcrypt "golang.org/x/crypto/bcrypt"
)

// testCost keeps tests fast — cost 12 (DefaultCost) is deliberately slow
// by design, which is the wrong tradeoff for a test suite run on every
// commit. Correctness at cost 12 is guaranteed by the same bcrypt
// algorithm at a lower cost; only the CPU-time constant changes.
const testCost = xbcrypt.MinCost

func TestNewHasher_RejectsOutOfRangeCost(t *testing.T) {
	if _, err := authbcrypt.NewHasher(xbcrypt.MinCost - 1); err == nil {
		t.Error("expected error for cost below MinCost, got nil")
	}
	if _, err := authbcrypt.NewHasher(xbcrypt.MaxCost + 1); err == nil {
		t.Error("expected error for cost above MaxCost, got nil")
	}
}

func TestHashAndVerify_RoundTrip(t *testing.T) {
	h, err := authbcrypt.NewHasher(testCost)
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}
	ctx := context.Background()

	hash, err := h.Hash(ctx, "correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if hash == "" {
		t.Fatal("Hash returned empty string")
	}
	if hash == "correct-horse-battery-staple" {
		t.Fatal("Hash returned the plaintext unchanged")
	}

	ok, err := h.Verify(ctx, "correct-horse-battery-staple", hash)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !ok {
		t.Error("Verify = false for the correct password, want true")
	}
}

func TestVerify_WrongPassword(t *testing.T) {
	h, err := authbcrypt.NewHasher(testCost)
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}
	ctx := context.Background()

	hash, err := h.Hash(ctx, "correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	ok, err := h.Verify(ctx, "wrong-password", hash)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if ok {
		t.Error("Verify = true for the wrong password, want false")
	}
}

func TestVerify_MalformedHash(t *testing.T) {
	h, err := authbcrypt.NewHasher(testCost)
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}

	_, err = h.Verify(context.Background(), "any-password", "not-a-real-bcrypt-hash")
	if err == nil {
		t.Fatal("expected error for a malformed hash, got nil")
	}
}

func TestHash_SameInputDifferentOutput(t *testing.T) {
	// bcrypt salts each hash independently, so two Hash calls on the same
	// plaintext must never produce identical output — otherwise two users
	// with the same password would have identical stored hashes, leaking
	// that fact to anything with database read access.
	h, err := authbcrypt.NewHasher(testCost)
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}
	ctx := context.Background()

	a, err := h.Hash(ctx, "same-password")
	if err != nil {
		t.Fatalf("Hash (a): %v", err)
	}
	b, err := h.Hash(ctx, "same-password")
	if err != nil {
		t.Fatalf("Hash (b): %v", err)
	}
	if a == b {
		t.Error("two Hash calls on the same plaintext produced identical output")
	}
}
