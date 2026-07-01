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
