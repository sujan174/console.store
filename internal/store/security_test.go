package store

import (
	"context"
	"testing"
)

// TestSearchPathShadowingBlocked is a regression test for C1 (CRITICAL):
// SECURITY DEFINER functions without a pinned search_path allowed an attacker
// holding the broker DSN to create pg_temp.ssh_pubkeys and have
// account_for_pubkey return an attacker-controlled UUID.
//
// After the fix (SET search_path = ” on both SECURITY DEFINER functions and
// schema-qualifying all table references), pg_temp objects are invisible to
// the function body regardless of what the broker role has created.
func TestSearchPathShadowingBlocked(t *testing.T) {
	owner := testPool(t)
	bpool := brokerPool(t, owner)
	ctx := context.Background()

	// Step 1: create a real account + pubkey via the legitimate API.
	s := New(bpool, newTestKMS(t))
	realID, err := s.FindOrCreateAccount(ctx, "+919001000099")
	if err != nil {
		t.Fatalf("setup: FindOrCreateAccount: %v", err)
	}
	const realPubkey = "ssh-ed25519 AAAAattackkey attacker"
	if err := s.LinkPubkey(ctx, realID, realPubkey); err != nil {
		t.Fatalf("setup: LinkPubkey: %v", err)
	}

	// Step 2: as the broker role, create pg_temp.ssh_pubkeys with a row that
	// maps the same pubkey to a bogus (attacker-chosen) UUID.
	// Before the fix this shadowed public.ssh_pubkeys inside the SECURITY
	// DEFINER body because pg_temp led the search_path.
	const bogusUUID = "00000000-dead-beef-0000-000000000000"
	_, err = bpool.Exec(ctx, `
		CREATE TEMP TABLE IF NOT EXISTS ssh_pubkeys (
			account_id uuid,
			pubkey     text
		)
	`)
	if err != nil {
		t.Fatalf("create pg_temp.ssh_pubkeys: %v", err)
	}
	_, err = bpool.Exec(ctx,
		`INSERT INTO pg_temp.ssh_pubkeys (account_id, pubkey) VALUES ($1, $2)`,
		bogusUUID, realPubkey,
	)
	if err != nil {
		t.Fatalf("insert into pg_temp.ssh_pubkeys: %v", err)
	}

	// Step 3: call account_for_pubkey. With the fix it must return the real
	// account id (from public.ssh_pubkeys), NOT the bogus UUID.
	gotID, ok, err := s.AccountForPubkey(ctx, realPubkey)
	if err != nil {
		t.Fatalf("AccountForPubkey: %v", err)
	}
	if !ok {
		t.Fatal("AccountForPubkey returned no row; expected the real account id")
	}
	if gotID == bogusUUID {
		t.Fatalf("SECURITY DEFINER search_path not pinned: got bogus UUID %q — pg_temp shadowing succeeded", bogusUUID)
	}
	if gotID != realID {
		t.Fatalf("unexpected id: got %q, want %q", gotID, realID)
	}
}
