package http

import (
	"net"
	"net/http"
	"strings"
)

// bearerToken extracts the access token from the Authorization header
// (M2 §4: Verify and Logout both require "Access token" auth). Returns
// ok=false for a missing header, a non-Bearer scheme, or an empty token —
// all three are the same "no credential presented" case to the caller, and
// every caller of this function maps that case to domain.ErrInvalidCredentials
// (§6's fixed-error-code convention: no separate "malformed header" code
// exists on the wire contract).
func bearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(h, prefix))
	if token == "" {
		return "", false
	}
	return token, true
}

// clientIP resolves the caller's source address for Login's per-IP rate
// limit key (M2 §4/§9).
//
// ASSUMPTION FLAG: M1's architecture places an AWS ALB (and, per M0, a
// Gateway service) in front of every service, so the TCP-layer RemoteAddr
// seen by this process is the load balancer's, not the client's — X-
// Forwarded-For is the only source of the real client IP. No milestone doc
// specifies which proxy hops are trusted to set that header truthfully.
// This implementation takes the first entry of X-Forwarded-For when
// present (the standard convention for "original client"), which is safe
// ONLY if something upstream (ALB, Gateway) already strips/overwrites any
// client-supplied X-Forwarded-For before this service ever sees the
// request — otherwise a client can forge their own rate-limit key and
// bypass §4/§9's per-IP limiting entirely. That stripping guarantee is an
// AWS/Gateway configuration concern outside this file, not something this
// handler can enforce. Falling back to RemoteAddr when the header is
// absent is safe either way, since RemoteAddr always reflects the actual
// TCP peer.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first, _, ok := strings.Cut(xff, ","); ok {
			if ip := strings.TrimSpace(first); ip != "" {
				return ip
			}
		}
		if ip := strings.TrimSpace(xff); ip != "" {
			return ip
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
