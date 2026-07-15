package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// This file is deliberately package http (white-box), not http_test, so
// it can call the unexported bearerToken and clientIP directly (auth.go)
// — neither is reachable from outside the package, and there is no
// exported wrapper around either to test through instead.

func TestBearerToken_ValidHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/verify", nil)
	req.Header.Set("Authorization", "Bearer abc123.def456.ghi789")

	token, ok := bearerToken(req)
	if !ok {
		t.Fatal("expected ok=true for a well-formed Bearer header")
	}
	if token != "abc123.def456.ghi789" {
		t.Errorf("token = %q, want %q", token, "abc123.def456.ghi789")
	}
}

func TestBearerToken_MissingHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/verify", nil)

	token, ok := bearerToken(req)
	if ok {
		t.Fatalf("expected ok=false when Authorization header is absent, got token %q", token)
	}
}

func TestBearerToken_EmptyHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/verify", nil)
	req.Header.Set("Authorization", "")

	if _, ok := bearerToken(req); ok {
		t.Fatal("expected ok=false for an empty Authorization header")
	}
}

func TestBearerToken_NonBearerScheme(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/verify", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")

	if _, ok := bearerToken(req); ok {
		t.Fatal("expected ok=false for a non-Bearer auth scheme")
	}
}

func TestBearerToken_BearerPrefixWithEmptyToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/verify", nil)
	req.Header.Set("Authorization", "Bearer ")

	if _, ok := bearerToken(req); ok {
		t.Fatal("expected ok=false when the token portion after 'Bearer ' is empty")
	}
}

func TestBearerToken_BearerPrefixWithOnlyWhitespace(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/verify", nil)
	req.Header.Set("Authorization", "Bearer    ")

	if _, ok := bearerToken(req); ok {
		t.Fatal("expected ok=false when the token portion is only whitespace")
	}
}

func TestBearerToken_TrimsSurroundingWhitespace(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/verify", nil)
	req.Header.Set("Authorization", "Bearer   my-token   ")

	token, ok := bearerToken(req)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if token != "my-token" {
		t.Errorf("token = %q, want %q (surrounding whitespace trimmed)", token, "my-token")
	}
}

func TestBearerToken_CaseSensitiveScheme(t *testing.T) {
	// strings.HasPrefix is case-sensitive, and bearerToken uses it
	// as-is (auth.go) — "bearer" (lowercase) is therefore rejected, even
	// though some other implementations treat the scheme name as
	// case-insensitive per RFC 7235's ABNF. This test locks in the
	// current, stricter behavior rather than assuming the lenient one.
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/verify", nil)
	req.Header.Set("Authorization", "bearer my-token")

	if _, ok := bearerToken(req); ok {
		t.Fatal("expected ok=false for lowercase 'bearer' scheme (case-sensitive prefix match)")
	}
}

func TestClientIP_FromXForwardedForSingleValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/login", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	req.RemoteAddr = "10.0.0.1:54321"

	if got := clientIP(req); got != "203.0.113.7" {
		t.Errorf("clientIP = %q, want %q", got, "203.0.113.7")
	}
}

func TestClientIP_FromXForwardedForTakesFirstOfMultiple(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/login", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 198.51.100.2, 192.0.2.1")
	req.RemoteAddr = "10.0.0.1:54321"

	if got := clientIP(req); got != "203.0.113.7" {
		t.Errorf("clientIP = %q, want first entry %q", got, "203.0.113.7")
	}
}

func TestClientIP_XForwardedForWithLeadingWhitespace(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/login", nil)
	req.Header.Set("X-Forwarded-For", "  203.0.113.7  , 198.51.100.2")
	req.RemoteAddr = "10.0.0.1:54321"

	if got := clientIP(req); got != "203.0.113.7" {
		t.Errorf("clientIP = %q, want trimmed first entry %q", got, "203.0.113.7")
	}
}

func TestClientIP_FallsBackToRemoteAddrWhenHeaderAbsent(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/login", nil)
	req.RemoteAddr = "192.0.2.55:12345"

	if got := clientIP(req); got != "192.0.2.55" {
		t.Errorf("clientIP = %q, want host portion of RemoteAddr %q", got, "192.0.2.55")
	}
}

func TestClientIP_FallsBackToRemoteAddrWhenHeaderEmpty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/login", nil)
	req.Header.Set("X-Forwarded-For", "")
	req.RemoteAddr = "192.0.2.55:12345"

	if got := clientIP(req); got != "192.0.2.55" {
		t.Errorf("clientIP = %q, want host portion of RemoteAddr %q", got, "192.0.2.55")
	}
}

func TestClientIP_RemoteAddrWithoutPortReturnedAsIs(t *testing.T) {
	// net.SplitHostPort errors when there's no ":port" — clientIP's
	// fallback (auth.go) returns r.RemoteAddr verbatim in that case
	// rather than failing.
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/login", nil)
	req.RemoteAddr = "not-a-host-port"

	if got := clientIP(req); got != "not-a-host-port" {
		t.Errorf("clientIP = %q, want RemoteAddr returned verbatim %q", got, "not-a-host-port")
	}
}
