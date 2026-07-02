package mcp

import (
	"context"
	"errors"
	"strings"
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

// The ₹1000 Builders Club cap is enforced at prepare time, before a
// confirmation_id exists.
func TestPrepareRejectsOverCap(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{cart: api.Cart{Restaurant: "X", Total: 1000,
		Lines: []api.CartLine{{ItemID: "i1", Quantity: 4, Available: true}}}}
	s := NewServer(be, &fakeAuth{token: true})
	_, _, err := s.handlePrepareOrder(context.Background(), nil, PrepareOrderIn{AddressID: "a1"})
	if err == nil || !strings.HasPrefix(err.Error(), "over_cap:") {
		t.Fatalf("want over_cap error, got %v", err)
	}
}

// The cap also guards order_preset — a saved preset whose bill grew past the cap
// is refused before a confirmation_id exists.
func TestOrderPresetRejectsOverCap(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_ = localstore.SavePresets(localstore.Presets{Version: 1, Items: []localstore.Preset{{
		Name: "feast", AddrID: "a1", RestaurantID: "R1", RestaurantName: "Dominos",
		Lines: []localstore.PresetLine{{ItemID: "i1", Qty: 4}},
	}}})
	be := &fakeBackend{cart: api.Cart{Restaurant: "Dominos", Total: 1200,
		Lines: []api.CartLine{{ItemID: "i1", Quantity: 4, Available: true}}}}
	s := NewServer(be, &fakeAuth{token: true})
	_, _, err := s.handleOrderPreset(context.Background(), nil, OrderPresetIn{Name: "feast"})
	if err == nil || !strings.HasPrefix(err.Error(), "over_cap:") {
		t.Fatalf("want over_cap error, got %v", err)
	}
}

// Taste observation survives an address switch: the rebuild re-records the
// cache under the new address, so placement still finds the cart write.
func TestTasteObservedAfterAddressSwitch(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cart := api.Cart{Restaurant: "Blue Tokai", Total: 300,
		Lines: []api.CartLine{{ItemID: "i1", Name: "Latte", Quantity: 1, Available: true}}}
	be := &fakeBackend{
		cart:  cart,
		order: api.Order{ID: "O1", Restaurant: "Blue Tokai", Total: 300},
		itemOpts: []api.OptionGroup{{ID: "g1", Name: "Milk", Choices: []api.OptionChoice{
			{ID: "c1", Name: "Oat", InStock: true},
		}}},
	}
	s := NewServer(be, &fakeAuth{token: true})
	if _, _, err := s.handleGetItemOptions(context.Background(), nil, GetItemOptionsIn{
		AddressID: "a1", RestaurantID: "R9", ItemName: "Latte", MenuItemID: "i1",
	}); err != nil {
		t.Fatalf("options: %v", err)
	}
	if _, _, err := s.handleUpdateCart(context.Background(), nil, UpdateCartIn{
		AddressID: "a1", RestaurantID: "R9", RestaurantName: "Blue Tokai",
		Items: []CartItemIn{{ItemID: "i1", Quantity: 1, Addons: []CartAddonSelIn{{GroupID: "g1", ChoiceID: "c1"}}}},
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	_, prep, err := s.handlePrepareOrder(context.Background(), nil, PrepareOrderIn{AddressID: "a2"})
	if err != nil || prep.Rebuilt != "address_change" {
		t.Fatalf("prepare: rebuilt=%q err=%v", prep.Rebuilt, err)
	}
	if _, _, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: prep.ConfirmationID}); err != nil {
		t.Fatalf("place: %v", err)
	}
	ts, _ := localstore.LoadTaste()
	if len(ts.Entries) != 1 || len(ts.Entries[0].Picks) != 1 ||
		ts.Entries[0].Picks[0].ChoiceName != "Oat" {
		t.Fatalf("taste after address switch = %+v", ts.Entries)
	}
}

// prepare_order at a different address than the cart was built for rebuilds the
// same lines at the new address and reports it; the placement then records the
// real restaurant identity from the rebuilt cache.
func TestPrepareRebuildsAtNewAddress(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cart := api.Cart{Restaurant: "Blue Tokai", Total: 300,
		Lines: []api.CartLine{{ItemID: "i1", Name: "Latte", Quantity: 1, Available: true}}}
	be := &fakeBackend{cart: cart, order: api.Order{ID: "O1", Restaurant: "Blue Tokai", Total: 300}}
	s := NewServer(be, &fakeAuth{token: true})
	if _, _, err := s.handleUpdateCart(context.Background(), nil, UpdateCartIn{
		AddressID: "a1", RestaurantID: "R9", RestaurantName: "Blue Tokai",
		Items: []CartItemIn{{ItemID: "i1", Quantity: 1}},
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	var rebuiltAt string
	be.updateFn = func(addressID, restaurantID, restaurantName string, items []api.CartItem) (api.Cart, error) {
		rebuiltAt = addressID
		return cart, nil
	}
	_, prep, err := s.handlePrepareOrder(context.Background(), nil, PrepareOrderIn{AddressID: "a2"})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if prep.Rebuilt != "address_change" || rebuiltAt != "a2" {
		t.Fatalf("rebuilt=%q at %q, want address_change at a2", prep.Rebuilt, rebuiltAt)
	}
	if _, _, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: prep.ConfirmationID}); err != nil {
		t.Fatalf("place: %v", err)
	}
	c, _ := localstore.LoadCard()
	if len(c.Favorites) != 1 || c.Favorites[0].RestaurantID != "R9" {
		t.Fatalf("rebuilt order should record the real restaurant id, got %+v", c.Favorites)
	}
}

// When the outlet can't deliver to the new address, the error is typed
// unserviceable and points the agent at a same-brand search.
func TestPrepareUnserviceableAtNewAddress(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cart := api.Cart{Restaurant: "Blue Tokai", Total: 300,
		Lines: []api.CartLine{{ItemID: "i1", Quantity: 1, Available: true}}}
	be := &fakeBackend{cart: cart}
	s := NewServer(be, &fakeAuth{token: true})
	if _, _, err := s.handleUpdateCart(context.Background(), nil, UpdateCartIn{
		AddressID: "a1", RestaurantID: "R9", RestaurantName: "Blue Tokai",
		Items: []CartItemIn{{ItemID: "i1", Quantity: 1}},
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	be.updateFn = func(addressID, restaurantID, restaurantName string, items []api.CartItem) (api.Cart, error) {
		return api.Cart{}, errors.New("restaurant not serviceable")
	}
	_, _, err := s.handlePrepareOrder(context.Background(), nil, PrepareOrderIn{AddressID: "a2"})
	if err == nil || !strings.HasPrefix(err.Error(), "unserviceable:") {
		t.Fatalf("want unserviceable error, got %v", err)
	}
}

// A cart Swiggy expired server-side (empty on re-fetch, same address) is rebuilt
// from the cached lines.
func TestPrepareRebuildsExpiredCart(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cart := api.Cart{Restaurant: "Blue Tokai", Total: 300,
		Lines: []api.CartLine{{ItemID: "i1", Quantity: 1, Available: true}}}
	be := &fakeBackend{cart: cart}
	s := NewServer(be, &fakeAuth{token: true})
	if _, _, err := s.handleUpdateCart(context.Background(), nil, UpdateCartIn{
		AddressID: "a1", RestaurantID: "R9", RestaurantName: "Blue Tokai",
		Items: []CartItemIn{{ItemID: "i1", Quantity: 1}},
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	be.getFn = func(addressID string) (api.Cart, error) { return api.Cart{}, nil } // expired
	_, prep, err := s.handlePrepareOrder(context.Background(), nil, PrepareOrderIn{AddressID: "a1"})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if prep.Rebuilt != "expired" || prep.Bill.Total != 300 {
		t.Fatalf("prep = %+v", prep)
	}
}

// A cache consumed by a placed order never seeds a rebuild — yesterday's pizza
// must not resurrect when the user prepares at another address.
func TestPlacedCacheDoesNotRebuild(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cart := api.Cart{Restaurant: "Blue Tokai", Total: 300,
		Lines: []api.CartLine{{ItemID: "i1", Quantity: 1, Available: true}}}
	be := &fakeBackend{cart: cart, order: api.Order{ID: "O1", Restaurant: "Blue Tokai", Total: 300}}
	s := NewServer(be, &fakeAuth{token: true})
	if _, _, err := s.handleUpdateCart(context.Background(), nil, UpdateCartIn{
		AddressID: "a1", RestaurantID: "R9", RestaurantName: "Blue Tokai",
		Items: []CartItemIn{{ItemID: "i1", Quantity: 1}},
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	_, prep, _ := s.handlePrepareOrder(context.Background(), nil, PrepareOrderIn{AddressID: "a1"})
	if _, _, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: prep.ConfirmationID}); err != nil {
		t.Fatalf("place: %v", err)
	}
	be.getFn = func(addressID string) (api.Cart, error) { return api.Cart{}, nil }
	updatesBefore := be.updates
	_, _, err := s.handlePrepareOrder(context.Background(), nil, PrepareOrderIn{AddressID: "a2"})
	if err == nil || !strings.Contains(err.Error(), "cart is empty") {
		t.Fatalf("want plain empty-cart error, got %v", err)
	}
	if be.updates != updatesBefore {
		t.Fatal("placed cache must not trigger a rebuild")
	}
}

// A stale cache (older than the rebuild window) is ignored.
func TestStaleCacheDoesNotRebuild(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_ = localstore.SaveCartCache(localstore.CartCache{
		AddressID: "a1", RestaurantID: "R9", RestaurantName: "Blue Tokai",
		Lines:     []localstore.CartCacheLine{{ItemID: "i1", Name: "Latte", Qty: 1}},
		WrittenAt: nowUnix() - 3*60*60, // 3h old
	})
	be := &fakeBackend{} // empty live cart
	s := NewServer(be, &fakeAuth{token: true})
	_, _, err := s.handlePrepareOrder(context.Background(), nil, PrepareOrderIn{AddressID: "a2"})
	if err == nil || !strings.Contains(err.Error(), "cart is empty") {
		t.Fatalf("want plain empty-cart error, got %v", err)
	}
	if be.updates != 0 {
		t.Fatal("stale cache must not trigger a rebuild")
	}
}

// save_preset still works right after placing (the cache is marked placed, not
// dropped) and from a fresh process (disk fallback).
func TestSavePresetAfterPlaceAndAcrossRestart(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cart := api.Cart{Restaurant: "Blue Tokai", Total: 300,
		Lines: []api.CartLine{{ItemID: "i1", Name: "Latte", Quantity: 1, Available: true}}}
	be := &fakeBackend{cart: cart, order: api.Order{ID: "O1", Restaurant: "Blue Tokai", Total: 300}}
	s := NewServer(be, &fakeAuth{token: true})
	if _, _, err := s.handleUpdateCart(context.Background(), nil, UpdateCartIn{
		AddressID: "a1", RestaurantID: "R9", RestaurantName: "Blue Tokai",
		Items: []CartItemIn{{ItemID: "i1", Quantity: 1}},
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	_, prep, _ := s.handlePrepareOrder(context.Background(), nil, PrepareOrderIn{AddressID: "a1"})
	if _, _, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: prep.ConfirmationID}); err != nil {
		t.Fatalf("place: %v", err)
	}
	if _, out, err := s.handleSavePreset(context.Background(), nil, SavePresetIn{Name: "coffee"}); err != nil || !out.Saved {
		t.Fatalf("save_preset after place: %v %+v", err, out)
	}

	// Fresh process: the in-memory slot is empty; the disk cache backs it.
	s2 := NewServer(be, &fakeAuth{token: true})
	if _, out, err := s2.handleSavePreset(context.Background(), nil, SavePresetIn{Name: "coffee2"}); err != nil || !out.Saved {
		t.Fatalf("save_preset after restart: %v %+v", err, out)
	}
	ps, _ := localstore.LoadPresets()
	if len(ps.Items) != 2 || ps.Items[1].RestaurantID != "R9" || len(ps.Items[1].Lines) != 1 {
		t.Fatalf("presets = %+v", ps.Items)
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
