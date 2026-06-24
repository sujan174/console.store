package kms

import (
	"bytes"
	"context"
	"crypto/rand"
	"testing"
)

func TestLocalGenerateAndUnwrap(t *testing.T) {
	master := make([]byte, 32)
	rand.Read(master)
	k, err := NewLocal(master)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pt, wrapped, err := k.GenerateDataKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(pt) != 32 {
		t.Fatalf("DEK len = %d, want 32", len(pt))
	}
	if bytes.Equal(pt, wrapped) {
		t.Fatal("wrapped DEK must not equal plaintext DEK")
	}
	got, err := k.Unwrap(ctx, wrapped)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, pt) {
		t.Fatal("unwrapped DEK != original plaintext DEK")
	}
}

func TestLocalUnwrapWrongKeyFails(t *testing.T) {
	m1 := make([]byte, 32)
	rand.Read(m1)
	m2 := make([]byte, 32)
	rand.Read(m2)
	ctx := context.Background()
	k1, _ := NewLocal(m1)
	_, wrapped, _ := k1.GenerateDataKey(ctx)
	k2, _ := NewLocal(m2)
	if _, err := k2.Unwrap(ctx, wrapped); err == nil {
		t.Fatal("expected unwrap with wrong master key to fail")
	}
}

func TestNewLocalRejectsBadKeyLen(t *testing.T) {
	if _, err := NewLocal(make([]byte, 16)); err == nil {
		t.Fatal("expected error for 16-byte master key")
	}
}
