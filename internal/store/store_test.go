package store

import (
	"context"
	"testing"
)

func TestFindOrCreateAccountIsStable(t *testing.T) {
	owner := testPool(t)
	bpool := brokerPool(t, owner)
	s := New(bpool, newTestKMS(t))
	ctx := context.Background()

	id1, err := s.FindOrCreateAccount(ctx, "+919000000001")
	if err != nil {
		t.Fatal(err)
	}
	id2, err := s.FindOrCreateAccount(ctx, "+919000000001")
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 || id1 == "" {
		t.Fatalf("expected stable non-empty id, got %q and %q", id1, id2)
	}
}

func TestLinkAndLookupPubkey(t *testing.T) {
	owner := testPool(t)
	bpool := brokerPool(t, owner)
	s := New(bpool, newTestKMS(t))
	ctx := context.Background()

	id, _ := s.FindOrCreateAccount(ctx, "+919000000002")
	if err := s.LinkPubkey(ctx, id, "ssh-ed25519 AAAAkey user"); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.AccountForPubkey(ctx, "ssh-ed25519 AAAAkey user")
	if err != nil || !ok {
		t.Fatalf("lookup failed: ok=%v err=%v", ok, err)
	}
	if got != id {
		t.Fatalf("pubkey mapped to %q, want %q", got, id)
	}
}
