package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"consolestore/internal/broker/api"
	"consolestore/internal/localstore"
)

func TestGetCartReturnsBill(t *testing.T) {
	be := &fakeBackend{cart: api.Cart{Total: 250, ItemTotal: 200, Delivery: 30, Taxes: 20,
		Lines: []api.CartLine{{ItemID: "i1", Name: "Burger", Quantity: 1, Price: 200, Available: true}}}}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleGetCart(context.Background(), nil, GetCartIn{AddressID: "a1"})
	if err != nil {
		t.Fatalf("handleGetCart: %v", err)
	}
	if out.Cart.Total != 250 || len(out.Cart.Lines) != 1 || out.Cart.Lines[0].Name != "Burger" {
		t.Fatalf("cart = %+v", out.Cart)
	}
}

func TestUpdateCartReturnsCart(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{cart: api.Cart{Total: 200}}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleUpdateCart(context.Background(), nil, UpdateCartIn{
		AddressID: "a1", RestaurantID: "r1",
		Items: []CartItemIn{{ItemID: "i1", Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("handleUpdateCart: %v", err)
	}
	if out.Cart.Total != 200 {
		t.Fatalf("total = %d", out.Cart.Total)
	}
	if out.ReplacedCart != nil {
		t.Fatalf("clean write must carry no replaced_cart receipt, got %+v", out.ReplacedCart)
	}
}

// A write rejected because another restaurant's cart is in the way gets the cart
// auto-replaced (clear + one retry) and reports a replaced_cart receipt.
func TestUpdateCartAutoReplacesConflictingCart(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	oldCart := api.Cart{Restaurant: "KFC", Total: 340,
		Lines: []api.CartLine{{ItemID: "k1", Quantity: 2, Available: true}}}
	newCart := api.Cart{Restaurant: "Pizza Palace", Total: 500,
		Lines: []api.CartLine{{ItemID: "p1", Name: "Margherita", Quantity: 1, Available: true}}}
	be := &fakeBackend{cart: oldCart}
	be.updateFn = func(addressID, restaurantID, restaurantName string, items []api.CartItem) (api.Cart, error) {
		if be.cleared == 0 {
			return api.Cart{}, errors.New("cannot add items from a different restaurant")
		}
		return newCart, nil
	}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleUpdateCart(context.Background(), nil, UpdateCartIn{
		AddressID: "a1", RestaurantID: "r2", RestaurantName: "Pizza Palace",
		Items: []CartItemIn{{ItemID: "p1", Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("handleUpdateCart: %v", err)
	}
	if be.cleared != 1 || be.updates != 2 {
		t.Fatalf("cleared=%d updates=%d, want 1/2", be.cleared, be.updates)
	}
	if out.Cart.Total != 500 {
		t.Fatalf("cart = %+v", out.Cart)
	}
	if out.ReplacedCart == nil || out.ReplacedCart.Restaurant != "KFC" ||
		out.ReplacedCart.ItemCount != 1 || out.ReplacedCart.Total != 340 {
		t.Fatalf("replaced_cart = %+v", out.ReplacedCart)
	}
}

// A nameless (foreign) conflicting cart is still replaced; the receipt says
// "an existing cart" instead of inventing a restaurant name.
func TestUpdateCartReplacesNamelessForeignCart(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{cart: api.Cart{Restaurant: "", Total: 120,
		Lines: []api.CartLine{{ItemID: "x1", Quantity: 1, Available: true}}}}
	be.updateFn = func(addressID, restaurantID, restaurantName string, items []api.CartItem) (api.Cart, error) {
		if be.cleared == 0 {
			return api.Cart{}, errors.New("different restaurant")
		}
		return api.Cart{Restaurant: "Pizza Palace", Total: 500,
			Lines: []api.CartLine{{ItemID: "p1", Quantity: 1, Available: true}}}, nil
	}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleUpdateCart(context.Background(), nil, UpdateCartIn{
		AddressID: "a1", RestaurantID: "r2", RestaurantName: "Pizza Palace",
		Items: []CartItemIn{{ItemID: "p1", Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("handleUpdateCart: %v", err)
	}
	if out.ReplacedCart == nil || out.ReplacedCart.Restaurant != "an existing cart" {
		t.Fatalf("replaced_cart = %+v", out.ReplacedCart)
	}
}

// A failure with the SAME restaurant's cart in place is not a conflict — the
// original error passes through and nothing is cleared.
func TestUpdateCartSameRestaurantErrorPassesThrough(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{cart: api.Cart{Restaurant: "Pizza Palace", Total: 300,
		Lines: []api.CartLine{{ItemID: "p1", Quantity: 1, Available: true}}}}
	be.updateFn = func(addressID, restaurantID, restaurantName string, items []api.CartItem) (api.Cart, error) {
		return api.Cart{}, errors.New("INVALID_ADDON")
	}
	s := NewServer(be, &fakeAuth{token: true})
	_, _, err := s.handleUpdateCart(context.Background(), nil, UpdateCartIn{
		AddressID: "a1", RestaurantID: "r2", RestaurantName: "Pizza Palace",
		Items: []CartItemIn{{ItemID: "p1", Quantity: 1}},
	})
	if err == nil || !strings.Contains(err.Error(), "INVALID_ADDON") {
		t.Fatalf("want original error, got %v", err)
	}
	if be.cleared != 0 {
		t.Fatalf("cleared=%d, want 0", be.cleared)
	}
}

// When the retry after a replace also fails, the error is typed cart_conflict.
func TestUpdateCartConflictRetryFailureIsTyped(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{cart: api.Cart{Restaurant: "KFC", Total: 340,
		Lines: []api.CartLine{{ItemID: "k1", Quantity: 1, Available: true}}}}
	be.updateFn = func(addressID, restaurantID, restaurantName string, items []api.CartItem) (api.Cart, error) {
		return api.Cart{}, errors.New("nope")
	}
	s := NewServer(be, &fakeAuth{token: true})
	_, _, err := s.handleUpdateCart(context.Background(), nil, UpdateCartIn{
		AddressID: "a1", RestaurantID: "r2", RestaurantName: "Pizza Palace",
		Items: []CartItemIn{{ItemID: "p1", Quantity: 1}},
	})
	if err == nil || !strings.HasPrefix(err.Error(), "cart_conflict:") {
		t.Fatalf("want cart_conflict error, got %v", err)
	}
}

// clear_cart drops the persisted cart cache along with the live cart, so a
// later save_preset can't snapshot a cart the user discarded.
func TestClearCartDropsCartCache(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{cart: api.Cart{Restaurant: "Pizza Palace", Total: 300,
		Lines: []api.CartLine{{ItemID: "p1", Name: "Margherita", Quantity: 1, Available: true}}}}
	s := NewServer(be, &fakeAuth{token: true})
	_, _, err := s.handleUpdateCart(context.Background(), nil, UpdateCartIn{
		AddressID: "a1", RestaurantID: "r2", Items: []CartItemIn{{ItemID: "p1", Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if _, ok, _ := localstore.LoadCartCache(); !ok {
		t.Fatal("cart cache should exist after update_cart")
	}
	if _, _, err := s.handleClearCart(context.Background(), nil, ClearCartIn{}); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, ok, _ := localstore.LoadCartCache(); ok {
		t.Fatal("cart cache should be gone after clear_cart")
	}
	if _, _, err := s.handleSavePreset(context.Background(), nil, SavePresetIn{Name: "gone"}); err == nil {
		t.Fatal("save_preset should fail after clear_cart")
	}
}
