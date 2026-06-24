package store

import (
	"bytes"
	"context"
	"crypto/rand"
	"testing"

	"console.store/internal/store/kms"
)

func newTestKMS(t *testing.T) kms.KMS {
	t.Helper()
	m := make([]byte, 32)
	rand.Read(m)
	k, err := kms.NewLocal(m)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestSealOpenRoundTrip(t *testing.T) {
	k := newTestKMS(t)
	ctx := context.Background()
	tok := []byte("swiggy-access-token-abc123")
	s, err := sealToken(ctx, k, tok)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(s.Ciphertext, tok) {
		t.Fatal("ciphertext must not contain plaintext token")
	}
	got, err := openToken(ctx, k, s)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, tok) {
		t.Fatalf("round-trip mismatch: %q != %q", got, tok)
	}
}

func TestOpenTamperedCiphertextFails(t *testing.T) {
	k := newTestKMS(t)
	ctx := context.Background()
	s, _ := sealToken(ctx, k, []byte("token"))
	s.Ciphertext[0] ^= 0xFF
	if _, err := openToken(ctx, k, s); err == nil {
		t.Fatal("expected GCM auth failure on tampered ciphertext")
	}
}
