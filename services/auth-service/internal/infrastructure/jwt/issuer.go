// Package jwt implements domain.TokenIssuer (M2 §3/§5): signing and
// verifying access tokens, and generating opaque refresh token IDs.
//
// Access tokens are self-contained signed JWTs (HS256) — the whole point
// of the design per M2 §2 is that every other service in the platform can
// verify one in-process, with no call back to Auth Service, which is what
// M5's token-verify SLO is written against. Refresh token IDs are
// deliberately NOT JWTs (see NewRefreshTokenID) — a refresh token must be
// revocable server-side by looking up its ID in Postgres/Redis, and a
// self-contained signed token can't be revoked before its own expiry.
package jwt

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/enterprise-cicd-platform/auth-service/internal/domain"
	libjwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Config holds the signing parameters wired from environment variables at
// the composition root (cmd/server/main.go) per M2 §7 — the key itself is
// a secret and must never be a source-controlled constant.
type Config struct {
	// SigningKey is the HMAC secret used for both signing and
	// verification. M2 leaves algorithm choice at the design-doc level;
	// HS256 is used here because Auth Service is the only signer AND the
	// only verifier's trust root (other services verify against a key
	// distributed out-of-band, not against Auth Service's public key —
	// there is no asymmetric keypair in scope for this milestone).
	SigningKey []byte

	// AccessTokenTTL is how long an issued access token remains valid.
	// Kept short deliberately (M2 §9): since an access token can't be
	// revoked before expiry, TTL length is the only lever bounding how
	// long a compromised one stays usable.
	AccessTokenTTL time.Duration

	// Issuer is the JWT "iss" claim, set to a fixed value identifying
	// this Auth Service deployment (e.g. "forgeops-auth-service").
	Issuer string
}

// TokenIssuer implements domain.TokenIssuer.
type TokenIssuer struct {
	cfg Config
}

// NewTokenIssuer validates cfg and returns a TokenIssuer. Validation
// happens here, once, at composition-root wiring time — not on every
// IssueAccessToken/VerifyAccessToken call — so a misconfigured signing
// key fails Auth Service's startup instead of its first request.
func NewTokenIssuer(cfg Config) (*TokenIssuer, error) {
	if len(cfg.SigningKey) == 0 {
		return nil, errors.New("jwt: signing key must not be empty")
	}
	if cfg.AccessTokenTTL <= 0 {
		return nil, errors.New("jwt: access token ttl must be positive")
	}
	if cfg.Issuer == "" {
		return nil, errors.New("jwt: issuer must not be empty")
	}
	return &TokenIssuer{cfg: cfg}, nil
}

// accessTokenClaims is the on-the-wire JWT claim set. Subject carries the
// user ID (as a string — JWT has no native UUID type); IssuedAt/ExpiresAt
// map directly onto domain.AccessTokenClaims on verify.
type accessTokenClaims struct {
	libjwt.RegisteredClaims
}

// IssueAccessToken implements domain.TokenIssuer.
func (i *TokenIssuer) IssueAccessToken(ctx context.Context, userID uuid.UUID) (string, domain.AccessTokenClaims, error) {
	now := time.Now()
	expiresAt := now.Add(i.cfg.AccessTokenTTL)

	claims := accessTokenClaims{
		RegisteredClaims: libjwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    i.cfg.Issuer,
			IssuedAt:  libjwt.NewNumericDate(now),
			ExpiresAt: libjwt.NewNumericDate(expiresAt),
		},
	}

	token := libjwt.NewWithClaims(libjwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(i.cfg.SigningKey)
	if err != nil {
		return "", domain.AccessTokenClaims{}, fmt.Errorf("signing access token: %w", err)
	}

	return signed, domain.AccessTokenClaims{
		UserID:    userID,
		IssuedAt:  now,
		ExpiresAt: expiresAt,
	}, nil
}

// VerifyAccessToken implements domain.TokenIssuer. Per the port's doc
// comment, it returns only domain.ErrTokenExpired or
// domain.ErrInvalidCredentials — never a raw JWT library error, so a
// caller (VerifyToken use case, HTTP middleware) never needs to know this
// package's underlying library.
func (i *TokenIssuer) VerifyAccessToken(ctx context.Context, tokenString string) (domain.AccessTokenClaims, error) {
	var claims accessTokenClaims

	token, err := libjwt.ParseWithClaims(tokenString, &claims, func(t *libjwt.Token) (any, error) {
		// Reject anything not HMAC before ever touching the key, so a
		// token crafted with alg:"none" or a different family can't
		// coerce this verifier into skipping the signature check.
		if _, ok := t.Method.(*libjwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return i.cfg.SigningKey, nil
	}, libjwt.WithIssuer(i.cfg.Issuer))

	if err != nil {
		if errors.Is(err, libjwt.ErrTokenExpired) {
			return domain.AccessTokenClaims{}, domain.ErrTokenExpired
		}
		// Malformed, bad signature, wrong issuer, wrong alg, not-yet-valid
		// (clock skew) — all collapse to the same sentinel per the port's
		// doc comment; the caller has no legitimate use for the
		// distinction and a raw parse error can leak details about why a
		// token was rejected.
		return domain.AccessTokenClaims{}, domain.ErrInvalidCredentials
	}
	if !token.Valid {
		return domain.AccessTokenClaims{}, domain.ErrInvalidCredentials
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return domain.AccessTokenClaims{}, domain.ErrInvalidCredentials
	}
	if claims.IssuedAt == nil || claims.ExpiresAt == nil {
		return domain.AccessTokenClaims{}, domain.ErrInvalidCredentials
	}

	return domain.AccessTokenClaims{
		UserID:    userID,
		IssuedAt:  claims.IssuedAt.Time,
		ExpiresAt: claims.ExpiresAt.Time,
	}, nil
}

// NewRefreshTokenID implements domain.TokenIssuer. Opaque (a random UUID),
// not a signed token — see the package doc comment for why a refresh
// token can't be self-contained the way an access token is.
func (i *TokenIssuer) NewRefreshTokenID() uuid.UUID {
	return uuid.New()
}
