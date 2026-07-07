package mcp

import (
	"context"
	"testing"

	"consolestore/internal/localstore"
)

func TestInitializeSignedInWithAddress(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := localstore.SaveAddrPref(localstore.AddrPref{}.SetActive("a1", "Home")); err != nil {
		t.Fatal(err)
	}
	s := NewServer(&fakeBackend{}, &fakeAuth{token: true})
	_, out, err := s.handleInitialize(context.Background(), nil, InitializeIn{})
	if err != nil || !out.SignedIn || out.Address == nil || out.Address.ID != "a1" {
		t.Fatalf("init out=%+v err=%v", out, err)
	}
}

func TestInitializeSignedOut(t *testing.T) {
	s := NewServer(&fakeBackend{}, &fakeAuth{token: false})
	_, out, err := s.handleInitialize(context.Background(), nil, InitializeIn{})
	if err != nil || out.SignedIn {
		t.Fatalf("want signed-out; out=%+v err=%v", out, err)
	}
}
