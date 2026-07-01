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
	if len(out.Warnings) != 1 || out.Card.Address.Default.ID != "" {
		t.Fatalf("out = %+v", out)
	}
}

func TestRememberSetsDefaultAddressAndPolicy(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s := NewServer(&fakeBackend{addrs: []api.Address{{ID: "a9", Label: "Home"}}}, &fakeAuth{token: true})
	_, _, err := s.handleRemember(context.Background(), nil, RememberIn{Policy: "vegetarian", DefaultAddressID: "a9"})
	if err != nil {
		t.Fatalf("remember: %v", err)
	}
	c, _ := localstore.LoadCard()
	if len(c.Prefs) != 1 || c.Prefs[0] != "vegetarian" || c.DefaultAddrID != "a9" || c.AddrLabel != "Home" {
		t.Fatalf("card = %+v", c)
	}

	_, out, err := s.handleGetCard(context.Background(), nil, GetCardIn{})
	if err != nil {
		t.Fatalf("get_card: %v", err)
	}
	if len(out.Card.Policies) != 1 || out.Card.Policies[0] != "vegetarian" {
		t.Fatalf("policies = %+v", out.Card.Policies)
	}
	if out.Card.Address.Default.ID != "a9" || out.Card.Address.Default.Label != "Home" {
		t.Fatalf("default address = %+v", out.Card.Address.Default)
	}
}

// Policy is additive (case-insensitive dedupe), never a wholesale replace.
func TestRememberPolicyIsAdditiveAndDeduped(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_ = localstore.SaveCard(localstore.Card{Version: 1, Prefs: []string{"no onion"}})
	s := NewServer(&fakeBackend{}, &fakeAuth{token: true})

	_, _, err := s.handleRemember(context.Background(), nil, RememberIn{Policy: "No Onion"})
	if err != nil {
		t.Fatalf("remember: %v", err)
	}
	c, _ := localstore.LoadCard()
	if len(c.Prefs) != 1 {
		t.Fatalf("expected dedupe, got %+v", c.Prefs)
	}

	_, _, err = s.handleRemember(context.Background(), nil, RememberIn{Policy: "extra spicy"})
	if err != nil {
		t.Fatalf("remember: %v", err)
	}
	c, _ = localstore.LoadCard()
	if len(c.Prefs) != 2 {
		t.Fatalf("expected 2 policies, got %+v", c.Prefs)
	}
}

func TestForgetRemovesPolicy(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_ = localstore.SaveCard(localstore.Card{Version: 1, Prefs: []string{"no onion", "vegetarian"}})
	s := NewServer(&fakeBackend{}, &fakeAuth{token: true})

	_, _, err := s.handleForget(context.Background(), nil, ForgetIn{Policy: "No Onion"})
	if err != nil {
		t.Fatalf("forget: %v", err)
	}
	c, _ := localstore.LoadCard()
	if len(c.Prefs) != 1 || c.Prefs[0] != "vegetarian" {
		t.Fatalf("prefs = %+v", c.Prefs)
	}
}

// remember with an explicit taste must surface via get_card under "taste" with
// source "explicit"; forget must remove it.
func TestRememberTasteThenForget(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s := NewServer(&fakeBackend{}, &fakeAuth{token: true})

	_, _, err := s.handleRemember(context.Background(), nil, RememberIn{
		RestaurantID: "R1", RestaurantName: "Dominos", ItemName: "Margherita",
		Picks: []TastePickIn{{GroupName: "Size", ChoiceName: "Large", Variant: true, Absolute: true}},
	})
	if err != nil {
		t.Fatalf("remember: %v", err)
	}

	_, out, err := s.handleGetCard(context.Background(), nil, GetCardIn{})
	if err != nil {
		t.Fatalf("get_card: %v", err)
	}
	if len(out.Card.Taste) != 1 {
		t.Fatalf("taste = %+v", out.Card.Taste)
	}
	e := out.Card.Taste[0]
	if e.RestaurantID != "R1" || e.ItemName != "Margherita" || len(e.Picks) != 1 {
		t.Fatalf("entry = %+v", e)
	}
	if e.Picks[0].GroupName != "Size" || e.Picks[0].ChoiceName != "Large" || e.Picks[0].Source != "explicit" {
		t.Fatalf("pick = %+v", e.Picks[0])
	}

	_, _, err = s.handleForget(context.Background(), nil, ForgetIn{RestaurantID: "R1", ItemName: "Margherita"})
	if err != nil {
		t.Fatalf("forget: %v", err)
	}
	_, out, err = s.handleGetCard(context.Background(), nil, GetCardIn{})
	if err != nil {
		t.Fatalf("get_card: %v", err)
	}
	if len(out.Card.Taste) != 0 {
		t.Fatalf("taste should be empty after forget, got %+v", out.Card.Taste)
	}
}

// A declined suggestion is silenced (not deleted): get_card no longer surfaces it
// under suggestions, but the inferred taste entry survives.
func TestForgetDeclineSuggestionSilencesButKeeps(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	taste := localstore.Taste{Version: 1, Entries: []localstore.TasteEntry{{
		RestaurantID: "R1", RestaurantName: "Starbucks", ItemName: "Latte",
		Picks:        []localstore.TastePick{{GroupName: "Milk", ChoiceName: "Oat", Source: "inferred", Count: 3}},
		LastUsedUnix: nowUnix(),
	}}}
	_ = localstore.SaveTaste(taste)
	s := NewServer(&fakeBackend{}, &fakeAuth{token: true})

	// Before: it's a live suggestion.
	_, out, _ := s.handleGetCard(context.Background(), nil, GetCardIn{})
	if len(out.Card.Suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %+v", out.Card.Suggestions)
	}

	_, _, err := s.handleForget(context.Background(), nil, ForgetIn{
		RestaurantID: "R1", ItemName: "Latte", DeclineSuggestion: true,
	})
	if err != nil {
		t.Fatalf("forget decline: %v", err)
	}

	// After: no suggestion, but the taste entry is still there.
	_, out, _ = s.handleGetCard(context.Background(), nil, GetCardIn{})
	if len(out.Card.Suggestions) != 0 {
		t.Fatalf("expected suggestion silenced, got %+v", out.Card.Suggestions)
	}
	if len(out.Card.Taste) != 1 {
		t.Fatalf("declined taste must survive, got %+v", out.Card.Taste)
	}
}

func TestRememberConfirmSuggestionPromotes(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	taste := localstore.Taste{Version: 1, Entries: []localstore.TasteEntry{{
		RestaurantID: "R1", RestaurantName: "Dominos", ItemName: "Margherita",
		Picks:        []localstore.TastePick{{GroupName: "Size", ChoiceName: "Large", Source: "inferred", Count: 3}},
		LastUsedUnix: nowUnix(),
	}}}
	_ = localstore.SaveTaste(taste)
	s := NewServer(&fakeBackend{}, &fakeAuth{token: true})

	_, _, err := s.handleRemember(context.Background(), nil, RememberIn{
		RestaurantID: "R1", ItemName: "Margherita", ConfirmSuggestion: true,
	})
	if err != nil {
		t.Fatalf("remember: %v", err)
	}
	t2, _ := localstore.LoadTaste()
	e, ok := t2.Find("R1", "Margherita")
	if !ok || len(e.Picks) != 1 || e.Picks[0].Source != "explicit" {
		t.Fatalf("expected promoted explicit pick, got %+v", e)
	}
}
