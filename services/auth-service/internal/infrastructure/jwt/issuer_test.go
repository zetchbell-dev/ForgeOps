package jwt_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/enterprise-cicd-platform/auth-service/internal/domain"
	authjwt "github.com/enterprise-cicd-platform/auth-service/internal/infrastructure/jwt"
	libjwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func testConfig() authjwt.Config {
	return authjwt.Config{
		SigningKey:     []byte("test-signing-key-do-not-use-in-prod"),
		AccessTokenTTL: time.Hour,
		Issuer:         "forgeops-auth-service-test",
	}
}

func TestNewTokenIssuer_RejectsInvalidConfig(t *testing.T) {
	base := testConfig()

	cases := []struct {
		name string
		cfg  authjwt.Config
	}{
		{"empty signing key", authjwt.Config{SigningKey: nil, AccessTokenTTL: base.AccessTokenTTL, Issuer: base.Issuer}},
		{"zero ttl", authjwt.Config{SigningKey: base.SigningKey, AccessTokenTTL: 0, Issuer: base.Issuer}},
		{"negative ttl", authjwt.Config{SigningKey: base.SigningKey, AccessTokenTTL: -time.Second, Issuer: base.Issuer}},
		{"empty issuer", authjwt.Config{SigningKey: base.SigningKey, AccessTokenTTL: base.AccessTokenTTL, Issuer: ""}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := authjwt.NewTokenIssuer(tc.cfg); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestIssueAndVerify_RoundTrip(t *testing.T) {
	issuer, err := authjwt.NewTokenIssuer(testConfig())
	if err != nil {
		t.Fatalf("NewTokenIssuer: %v", err)
	}

	userID := uuid.New()
	ctx := context.Background()

	token, issued, err := issuer.IssueAccessToken(ctx, userID)
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	if token == "" {
		t.Fatal("IssueAccessToken returned empty token string")
	}
	if issued.UserID != userID {
		t.Errorf("issued.UserID = %v, want %v", issued.UserID, userID)
	}

	verified, err := issuer.VerifyAccessToken(ctx, token)
	if err != nil {
		t.Fatalf("VerifyAccessToken: %v", err)
	}
	if verified.UserID != userID {
		t.Errorf("verified.UserID = %v, want %v", verified.UserID, userID)
	}
	// JWT NumericDate is second-precision, so compare truncated to the
	// second rather than requiring exact equality with the pre-encode
	// time.Time.
	if !verified.ExpiresAt.Truncate(time.Second).Equal(issued.ExpiresAt.Truncate(time.Second)) {
		t.Errorf("verified.ExpiresAt = %v, want %v", verified.ExpiresAt, issued.ExpiresAt)
	}
	if !verified.IssuedAt.Truncate(time.Second).Equal(issued.IssuedAt.Truncate(time.Second)) {
		t.Errorf("verified.IssuedAt = %v, want %v", verified.IssuedAt, issued.IssuedAt)
	}
}

func TestVerifyAccessToken_WrongSigningKey(t *testing.T) {
	issuer, err := authjwt.NewTokenIssuer(testConfig())
	if err != nil {
		t.Fatalf("NewTokenIssuer: %v", err)
	}
	token, _, err := issuer.IssueAccessToken(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	other := testConfig()
	other.SigningKey = []byte("a-completely-different-signing-key")
	otherIssuer, err := authjwt.NewTokenIssuer(other)
	if err != nil {
		t.Fatalf("NewTokenIssuer (other): %v", err)
	}

	_, err = otherIssuer.VerifyAccessToken(context.Background(), token)
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("VerifyAccessToken error = %v, want domain.ErrInvalidCredentials", err)
	}
}

func TestVerifyAccessToken_WrongIssuer(t *testing.T) {
	cfgA := testConfig()
	cfgA.Issuer = "issuer-a"
	issuerA, err := authjwt.NewTokenIssuer(cfgA)
	if err != nil {
		t.Fatalf("NewTokenIssuer (a): %v", err)
	}
	token, _, err := issuerA.IssueAccessToken(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	cfgB := testConfig()
	cfgB.Issuer = "issuer-b"
	issuerB, err := authjwt.NewTokenIssuer(cfgB)
	if err != nil {
		t.Fatalf("NewTokenIssuer (b): %v", err)
	}

	_, err = issuerB.VerifyAccessToken(context.Background(), token)
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("VerifyAccessToken error = %v, want domain.ErrInvalidCredentials", err)
	}
}

func TestVerifyAccessToken_Malformed(t *testing.T) {
	issuer, err := authjwt.NewTokenIssuer(testConfig())
	if err != nil {
		t.Fatalf("NewTokenIssuer: %v", err)
	}

	_, err = issuer.VerifyAccessToken(context.Background(), "not-a-jwt-at-all")
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("VerifyAccessToken error = %v, want domain.ErrInvalidCredentials", err)
	}
}

func TestVerifyAccessToken_Expired(t *testing.T) {
	cfg := testConfig()
	issuer, err := authjwt.NewTokenIssuer(cfg)
	if err != nil {
		t.Fatalf("NewTokenIssuer: %v", err)
	}

	// Craft an already-expired token directly with the library rather
	// than through IssueAccessToken (whose Config rejects a non-positive
	// TTL), signed with the same key so only the expiry check is under
	// test, not the signature check.
	claims := libjwt.RegisteredClaims{
		Subject:   uuid.New().String(),
		Issuer:    cfg.Issuer,
		IssuedAt:  libjwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		ExpiresAt: libjwt.NewNumericDate(time.Now().Add(-time.Hour)),
	}
	raw := libjwt.NewWithClaims(libjwt.SigningMethodHS256, claims)
	expiredToken, err := raw.SignedString(cfg.SigningKey)
	if err != nil {
		t.Fatalf("signing expired test token: %v", err)
	}

	_, err = issuer.VerifyAccessToken(context.Background(), expiredToken)
	if !errors.Is(err, domain.ErrTokenExpired) {
		t.Errorf("VerifyAccessToken error = %v, want domain.ErrTokenExpired", err)
	}
}

func TestVerifyAccessToken_RejectsAlgNone(t *testing.T) {
	issuer, err := authjwt.NewTokenIssuer(testConfig())
	if err != nil {
		t.Fatalf("NewTokenIssuer: %v", err)
	}

	claims := libjwt.RegisteredClaims{
		Subject:   uuid.New().String(),
		Issuer:    testConfig().Issuer,
		IssuedAt:  libjwt.NewNumericDate(time.Now()),
		ExpiresAt: libjwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	raw := libjwt.NewWithClaims(libjwt.SigningMethodNone, claims)
	noneToken, err := raw.SignedString(libjwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("signing alg:none test token: %v", err)
	}

	_, err = issuer.VerifyAccessToken(context.Background(), noneToken)
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("VerifyAccessToken error = %v, want domain.ErrInvalidCredentials", err)
	}
}

func TestNewRefreshTokenID_ReturnsUniqueValues(t *testing.T) {
	issuer, err := authjwt.NewTokenIssuer(testConfig())
	if err != nil {
		t.Fatalf("NewTokenIssuer: %v", err)
	}

	a := issuer.NewRefreshTokenID()
	b := issuer.NewRefreshTokenID()
	if a == b {
		t.Fatal("NewRefreshTokenID returned the same value twice")
	}
	if a == uuid.Nil || b == uuid.Nil {
		t.Fatal("NewRefreshTokenID returned a nil UUID")
	}
}
