package mcp

import (
	"context"
	"testing"

	"consolestore/internal/broker/api"
	"consolestore/internal/localstore"
)

func TestGetCardReconcilesWarnings(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_ = localstore.SaveCard(localstore.Card{Version: 1, DefaultAddrID: "gone", AddrLabel: "Home"})
	be := &fakeBackend{addrs: []api.Address{{ID: "other", Label: "Office"}}}
	s := NewServer(be, &fakeAuth{token: true})

	_, out, err := s.handleGetCard(context.Background(), nil, GetCardIn{})
	if err != nil {
		t.Fatalf("get_card: %v", err)
	}
	if len(out.Warnings) != 1 || out.Card.DefaultAddressID != "" {
		t.Fatalf("out = %+v", out)
	}
}

func TestUpdateCardSetsPrefs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s := NewServer(&fakeBackend{}, &fakeAuth{token: true})
	_, _, err := s.handleUpdateCard(context.Background(), nil, UpdateCardIn{Prefs: []string{"vegetarian"}, DefaultAddressID: "a9"})
	if err != nil {
		t.Fatalf("update_card: %v", err)
	}
	c, _ := localstore.LoadCard()
	if len(c.Prefs) != 1 || c.Prefs[0] != "vegetarian" || c.DefaultAddrID != "a9" {
		t.Fatalf("card = %+v", c)
	}
}
