package store

import (
	"context"
	"testing"
	"time"
)

func TestTokenRoundTrip(t *testing.T) {
	owner := testPool(t)
	s := New(brokerPool(t, owner), newTestKMS(t))
	ctx := context.Background()
	id, _ := s.FindOrCreateAccount(ctx, "+919000000010")

	exp := time.Now().Add(time.Hour).Truncate(time.Second)
	if err := s.PutToken(ctx, id, Token{AccessToken: "tok-xyz", ExpiresAt: exp}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.GetToken(ctx, id)
	if err != nil || !ok {
		t.Fatalf("get: ok=%v err=%v", ok, err)
	}
	if got.AccessToken != "tok-xyz" || !got.ExpiresAt.Equal(exp) {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestPurgeToken(t *testing.T) {
	owner := testPool(t)
	s := New(brokerPool(t, owner), newTestKMS(t))
	ctx := context.Background()
	id, _ := s.FindOrCreateAccount(ctx, "+919000000011")
	s.PutToken(ctx, id, Token{AccessToken: "t", ExpiresAt: time.Now().Add(time.Hour)})
	if err := s.PurgeToken(ctx, id); err != nil {
		t.Fatal(err)
	}
	_, ok, _ := s.GetToken(ctx, id)
	if ok {
		t.Fatal("token still present after purge")
	}
}

// The security-critical test: account B can never read account A's token row.
func TestRLSBlocksCrossAccountTokenRead(t *testing.T) {
	owner := testPool(t)
	s := New(brokerPool(t, owner), newTestKMS(t))
	ctx := context.Background()
	a, _ := s.FindOrCreateAccount(ctx, "+919000000020")
	b, _ := s.FindOrCreateAccount(ctx, "+919000000021")
	s.PutToken(ctx, a, Token{AccessToken: "A-secret", ExpiresAt: time.Now().Add(time.Hour)})

	// Reading as B must NOT see A's token.
	_, ok, err := s.GetToken(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("RLS breach: account B read a token while scoped to B (A's row leaked)")
	}
}
