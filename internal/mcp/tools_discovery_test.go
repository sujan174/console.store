package mcp

import (
	"context"
	"strings"
	"testing"

	"consolestore/internal/broker/api"
)

// When no address can be resolved (no explicit id, no locked/last-used pref, and
// the address list is empty/unavailable), the resolution tools must return a
// typed no_address error instead of forwarding an empty addressId to Swiggy —
// which hard-fails "addressId is required" and makes the agent retry in a loop
// (the behavior that burned through the rate limit, seen live 2026-07-09).
func TestSearchRestaurantsNoAddressReturnsTypedError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // isolate: no locked/last-used pref
	be := &fakeBackend{}                     // addrs empty → resolveAddress yields ""
	s := NewServer(be, &fakeAuth{token: true})
	_, _, err := s.handleSearchRestaurants(context.Background(), nil, SearchRestaurantsIn{Query: "burger"})
	if err == nil || !strings.Contains(err.Error(), codeNoAddress) {
		t.Fatalf("want a %q error, got %v", codeNoAddress, err)
	}
	if be.searchPageCalls != 0 {
		t.Fatalf("must NOT call Swiggy with an empty address; searchPageCalls=%d", be.searchPageCalls)
	}
}

func TestListAddressesRequiresAuth(t *testing.T) {
	s := NewServer(&fakeBackend{}, &fakeAuth{token: false})
	_, _, err := s.handleListAddresses(context.Background(), nil, ListAddressesIn{})
	if err == nil {
		t.Fatalf("expected not-signed-in error")
	}
}

func TestListAddressesReturnsAddresses(t *testing.T) {
	be := &fakeBackend{addrs: []api.Address{{ID: "a1", Label: "Home", Full: "12 Main St"}}}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleListAddresses(context.Background(), nil, ListAddressesIn{})
	if err != nil {
		t.Fatalf("handleListAddresses: %v", err)
	}
	if len(out.Addresses) != 1 || out.Addresses[0].ID != "a1" || out.Addresses[0].Label != "Home" {
		t.Fatalf("addresses = %+v", out.Addresses)
	}
}

func TestSearchRestaurantsReturnsResults(t *testing.T) {
	be := &fakeBackend{
		search:     []api.Restaurant{{ID: "r1", Name: "McDonald's", ETA: "30 mins"}},
		searchNext: 8,
		searchMore: true,
	}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleSearchRestaurants(context.Background(), nil, SearchRestaurantsIn{AddressID: "a1", Query: "mcd"})
	if err != nil {
		t.Fatalf("handleSearchRestaurants: %v", err)
	}
	if len(out.Restaurants) != 1 || out.Restaurants[0].Name != "McDonald's" {
		t.Fatalf("restaurants = %+v", out.Restaurants)
	}
	// Pagination fields propagate so the app can offer "load more".
	if out.NextOffset != 8 || !out.HasMore {
		t.Fatalf("pagination not propagated: next=%d more=%v", out.NextOffset, out.HasMore)
	}
}

// TestSearchRestaurantsSelfResolvesAddress verifies the speedup contract: an
// agent can call search_restaurants WITHOUT an address_id and the server
// fills the active address itself (here, falling back to the account's first
// saved address) — so no initialize/list_addresses round trip is forced first.
func TestSearchRestaurantsSelfResolvesAddress(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // no AddrPref written → falls back to Addresses()[0]
	be := &fakeBackend{
		addrs:  []api.Address{{ID: "a1", Label: "Home"}},
		search: []api.Restaurant{{ID: "r1", Name: "Truffles"}},
	}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleSearchRestaurants(context.Background(), nil, SearchRestaurantsIn{Query: "truffles"})
	if err != nil {
		t.Fatalf("handleSearchRestaurants with no address_id: %v", err)
	}
	if len(out.Restaurants) != 1 || out.Restaurants[0].Name != "Truffles" {
		t.Fatalf("restaurants = %+v", out.Restaurants)
	}
}

func TestToMenuItemDTOsKeepsCategory(t *testing.T) {
	in := []api.MenuItem{{ID: "1", Name: "Veg Wrap", Price: 120, Veg: true, InStock: true, Category: "Wraps"}}
	got := toMenuItemDTOs(in)
	if len(got) != 1 || got[0].Category != "Wraps" {
		t.Fatalf("category dropped: %+v", got)
	}
}

// get_menu must surface description + rating so the agent can describe/rank
// dishes — both were being dropped at the MCP DTO boundary even though the
// broker (internal/broker/mapping.go mapMenu) already fills api.MenuItem.
func TestToMenuItemDTOsKeepsDescriptionAndRating(t *testing.T) {
	in := []api.MenuItem{{ID: "1", Name: "Veg Wrap", Price: 120, Description: "Whole wheat wrap with grilled veggies", Rating: 4.6}}
	got := toMenuItemDTOs(in)
	if len(got) != 1 || got[0].Description != "Whole wheat wrap with grilled veggies" || got[0].Rating != 4.6 {
		t.Fatalf("description/rating dropped: %+v", got)
	}
}

// The MCP menu repeats the same item id across category pages (e.g. it shows
// up under "Recommended" and again under its real category). toMenuItemDTOs
// must dedupe by id, keeping the FIRST occurrence, matching the TUI's
// MergeMenuPage (internal/catalog/swiggy/snapshot.go). Punctuation-variant
// names with distinct ids must NOT be collapsed.
func TestToMenuItemDTOsDedupesByID(t *testing.T) {
	in := []api.MenuItem{
		{ID: "1", Name: "Veg Wrap", Price: 120, Category: "Recommended"},
		{ID: "2", Name: "Paneer Roll", Price: 150, Category: "Recommended"},
		{ID: "1", Name: "Veg Wrap (dup)", Price: 999, Category: "Wraps"}, // repeat of id 1, later page
		{ID: "3", Name: "Veg Wrap!", Price: 130, Category: "Wraps"},      // distinct id, punctuation-variant name
		{ID: "", Name: "No ID Item", Price: 50},                          // empty id must be dropped
	}
	got := toMenuItemDTOs(in)
	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3 (deduped by id, empty id dropped): %+v", len(got), got)
	}
	if got[0].ID != "1" || got[0].Name != "Veg Wrap" || got[0].Category != "Recommended" {
		t.Fatalf("first occurrence of id 1 not preserved: %+v", got[0])
	}
	if got[1].ID != "2" {
		t.Fatalf("order not preserved: %+v", got)
	}
	if got[2].ID != "3" || got[2].Name != "Veg Wrap!" {
		t.Fatalf("distinct id with punctuation-variant name dropped: %+v", got)
	}
}
