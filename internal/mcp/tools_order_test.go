package mcp

import (
	"context"
	"errors"
	"testing"

	"consolestore/internal/broker/api"
	"consolestore/internal/localstore"
)

func TestPrepareThenPlaceSucceeds(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{
		cart:  api.Cart{Restaurant: "McDonald's", Total: 250, Lines: []api.CartLine{{ItemID: "i1", Quantity: 1, Price: 250, Available: true}}},
		order: api.Order{ID: "OID1", Status: "placed", Restaurant: "McDonald's", Total: 250, ETA: "30 mins"},
	}
	s := NewServer(be, &fakeAuth{token: true})

	_, prep, err := s.handlePrepareOrder(context.Background(), nil, PrepareOrderIn{AddressID: "a1"})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if prep.ConfirmationID == "" || prep.Bill.Total != 250 {
		t.Fatalf("prep = %+v", prep)
	}
	_, plc, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: prep.ConfirmationID})
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	if plc.Order.ID != "OID1" {
		t.Fatalf("order = %+v", plc.Order)
	}
	if be.placed != 1 {
		t.Fatalf("placed = %d, want 1", be.placed)
	}
}

// An ad-hoc prepare_order→place_order carries no Swiggy restaurant id, so it must
// NOT create a name-keyed favorite — only the default address is recorded.
func TestAdhocPlaceRecordsNoFavorite(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{
		cart:  api.Cart{Restaurant: "McDonald's", Total: 250, Lines: []api.CartLine{{ItemID: "i1", Quantity: 1, Available: true}}},
		order: api.Order{ID: "OID1", Restaurant: "McDonald's", Total: 250},
	}
	s := NewServer(be, &fakeAuth{token: true})
	_, prep, _ := s.handlePrepareOrder(context.Background(), nil, PrepareOrderIn{AddressID: "a1"})
	if _, _, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: prep.ConfirmationID}); err != nil {
		t.Fatalf("place: %v", err)
	}
	c, _ := localstore.LoadCard()
	if len(c.Favorites) != 0 {
		t.Fatalf("ad-hoc order should not create a favorite, got %+v", c.Favorites)
	}
	if c.DefaultAddrID != "a1" {
		t.Fatalf("default address = %q, want a1", c.DefaultAddrID)
	}
}

// order_preset carries the real Swiggy restaurant id, so the favorite must be keyed
// by that id (never the restaurant name).
func TestOrderPresetRecordsFavoriteWithRealID(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_ = localstore.SavePresets(localstore.Presets{Version: 1, Items: []localstore.Preset{{
		Name: "dinner", AddrID: "a1", AddrLine: "Home", RestaurantID: "R123", RestaurantName: "Dominos",
		Lines: []localstore.PresetLine{{ItemID: "i1", Qty: 1}},
	}}})
	be := &fakeBackend{
		cart:  api.Cart{Restaurant: "Dominos", Total: 300, Lines: []api.CartLine{{ItemID: "i1", Quantity: 1, Available: true}}},
		order: api.Order{ID: "O2", Restaurant: "Dominos", Total: 300},
	}
	s := NewServer(be, &fakeAuth{token: true})
	_, prep, err := s.handleOrderPreset(context.Background(), nil, OrderPresetIn{Name: "dinner"})
	if err != nil {
		t.Fatalf("order_preset: %v", err)
	}
	if _, _, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: prep.ConfirmationID}); err != nil {
		t.Fatalf("place: %v", err)
	}
	c, _ := localstore.LoadCard()
	if len(c.Favorites) != 1 || c.Favorites[0].RestaurantID != "R123" || c.Favorites[0].RestaurantName != "Dominos" {
		t.Fatalf("favorite must be keyed by real id R123, got %+v", c.Favorites)
	}
	if c.DefaultAddrID != "a1" || c.AddrLabel != "Home" {
		t.Fatalf("default = %q/%q, want a1/Home", c.DefaultAddrID, c.AddrLabel)
	}
}

func TestPlaceRejectsUnknownConfirmation(t *testing.T) {
	be := &fakeBackend{cart: api.Cart{Total: 250}}
	s := NewServer(be, &fakeAuth{token: true})
	_, _, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: "nope"})
	if err == nil {
		t.Fatalf("expected rejection for unknown confirmation id")
	}
	if be.placed != 0 {
		t.Fatalf("placed = %d, want 0", be.placed)
	}
}

func TestPlaceRejectsChangedCart(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{cart: api.Cart{Restaurant: "X", Total: 250, Lines: []api.CartLine{{ItemID: "i1", Quantity: 1, Available: true}}}}
	s := NewServer(be, &fakeAuth{token: true})
	_, prep, _ := s.handlePrepareOrder(context.Background(), nil, PrepareOrderIn{AddressID: "a1"})
	be.cart.Total = 999 // cart drifted after prepare
	_, _, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: prep.ConfirmationID})
	if err == nil {
		t.Fatalf("expected rejection for changed cart")
	}
	if be.placed != 0 {
		t.Fatalf("placed = %d, want 0", be.placed)
	}
}

func TestPlaceOrderNeverRetries(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{
		cart:     api.Cart{Restaurant: "X", Total: 250, Lines: []api.CartLine{{ItemID: "i1", Quantity: 1, Available: true}}},
		placeErr: errors.New("502 bad gateway"),
	}
	s := NewServer(be, &fakeAuth{token: true})
	_, prep, _ := s.handlePrepareOrder(context.Background(), nil, PrepareOrderIn{AddressID: "a1"})
	_, _, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: prep.ConfirmationID})
	if err == nil {
		t.Fatalf("expected place error")
	}
	if be.placed != 1 {
		t.Fatalf("PlaceOrder called %d times, want exactly 1 (no retry)", be.placed)
	}
}
