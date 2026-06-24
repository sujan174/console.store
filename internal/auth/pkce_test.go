package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestChallengeMatchesS256(t *testing.T) {
	v := GenerateVerifier()
	if len(v) < 43 || len(v) > 128 {
		t.Fatalf("verifier length %d out of RFC7636 range", len(v))
	}
	got := Challenge(v)
	sum := sha256.Sum256([]byte(v))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("challenge = %q, want %q", got, want)
	}
}

func TestVerifiersAndStatesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		v := GenerateVerifier()
		if seen[v] {
			t.Fatal("duplicate verifier generated")
		}
		seen[v] = true
		if RandState() == RandState() {
			t.Fatal("RandState returned identical values")
		}
	}
}
