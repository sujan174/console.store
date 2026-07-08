package mcp

import (
	"context"
	"testing"

	"consolestore/internal/broker/api"
)

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
	be := &fakeBackend{search: []api.Restaurant{{ID: "r1", Name: "McDonald's", ETA: "30 mins"}}}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleSearchRestaurants(context.Background(), nil, SearchRestaurantsIn{AddressID: "a1", Query: "mcd"})
	if err != nil {
		t.Fatalf("handleSearchRestaurants: %v", err)
	}
	if len(out.Restaurants) != 1 || out.Restaurants[0].Name != "McDonald's" {
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
