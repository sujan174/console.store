package localstore

import (
	"context"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

func TestTokenRoundTrip(t *testing.T) {
	keyring.MockInit()
	ctx := context.Background()
	s := New()

	// No token yet.
	if _, ok, _ := s.AccountForPubkey(ctx, "ignored"); ok {
		t.Fatal("expected no token before PutToken")
	}
	if _, _, _, ok, _ := s.GetTokenFull(ctx, LocalAccountID); ok {
		t.Fatal("GetTokenFull ok should be false before PutToken")
	}

	exp := time.Unix(1_900_000_000, 0)
	if err := s.PutToken(ctx, LocalAccountID, "acc", "ref", exp); err != nil {
		t.Fatalf("PutToken: %v", err)
	}

	acc, ref, got, ok, err := s.GetTokenFull(ctx, LocalAccountID)
	if err != nil || !ok {
		t.Fatalf("GetTokenFull ok=%v err=%v", ok, err)
	}
	if acc != "acc" || ref != "ref" || !got.Equal(exp) {
		t.Fatalf("round-trip mismatch: acc=%q ref=%q exp=%v", acc, ref, got)
	}
	if id, ok, _ := s.AccountForPubkey(ctx, "ignored"); !ok || id != LocalAccountID {
		t.Fatalf("AccountForPubkey = %q,%v; want %q,true", id, ok, LocalAccountID)
	}

	if err := s.PurgeToken(ctx, LocalAccountID); err != nil {
		t.Fatalf("PurgeToken: %v", err)
	}
	if _, _, _, ok, _ := s.GetTokenFull(ctx, LocalAccountID); ok {
		t.Fatal("GetTokenFull ok should be false after purge")
	}
}

func TestFindOrCreateAccountAlwaysLocal(t *testing.T) {
	keyring.MockInit()
	s := New()
	id, err := s.FindOrCreateAccount(context.Background(), "+919999999999")
	if err != nil || id != LocalAccountID {
		t.Fatalf("FindOrCreateAccount = %q,%v; want %q,nil", id, err, LocalAccountID)
	}
}

func TestRegistrationRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, ok, err := LoadRegistration(); ok || err != nil {
		t.Fatalf("LoadRegistration on empty dir = ok %v err %v; want false,nil", ok, err)
	}
	want := Registration{ClientID: "cid", AuthorizationEndpoint: "https://a/authz", TokenEndpoint: "https://a/token"}
	if err := SaveRegistration(want); err != nil {
		t.Fatalf("SaveRegistration: %v", err)
	}
	got, ok, err := LoadRegistration()
	if err != nil || !ok || got != want {
		t.Fatalf("LoadRegistration = %+v,%v,%v; want %+v,true,nil", got, ok, err, want)
	}
}
