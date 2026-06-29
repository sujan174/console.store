// Package auth implements consolestore's delegated authentication against
// Swiggy's OAuth 2.1 authorization server: PKCE, Dynamic Client Registration,
// the authorization-code exchange, and a cross-device pending-authorize manager.
// It never stores tokens itself — binding goes through an injected AccountStore.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// GenerateVerifier returns a fresh PKCE code verifier (48 random bytes,
// base64url-encoded → 64 chars, within RFC 7636's 43-128 range).
func GenerateVerifier() string { return randB64(48) }

// Challenge returns the S256 code challenge for a verifier.
func Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// RandState returns a fresh CSRF state token (24 random bytes).
func RandState() string { return randB64(24) }

func randB64(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("auth: crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
