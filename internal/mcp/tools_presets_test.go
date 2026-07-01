package mcp

import (
	"context"
	"testing"

	"consolestore/internal/broker/api"
	"consolestore/internal/localstore"
)

// update_cart must populate the cart-write slot so save_preset can persist it,
// and the saved preset must then be visible via list_presets.
func TestUpdateCartThenSavePresetThenListPresets(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{cart: api.Cart{
		Restaurant: "Dominos", Total: 300,
		Lines: []api.CartLine{{ItemID: "i1", Name: "Margherita", Quantity: 2, Price: 150, Available: true}},
	}}
	s := NewServer(be, &fakeAuth{token: true})
	if err := localstore.SetDefaultAddress("a1", "Home", nowUnix()); err != nil {
		t.Fatalf("seed default address: %v", err)
	}

	_, _, err := s.handleUpdateCart(context.Background(), nil, UpdateCartIn{
		AddressID: "a1", RestaurantID: "r1", RestaurantName: "Dominos",
		Items: []CartItemIn{{ItemID: "i1", Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("update_cart: %v", err)
	}

	_, saveOut, err := s.handleSavePreset(context.Background(), nil, SavePresetIn{Name: "dinner"})
	if err != nil {
		t.Fatalf("save_preset: %v", err)
	}
	if !saveOut.Saved || saveOut.Name != "dinner" {
		t.Fatalf("saveOut = %+v", saveOut)
	}

	_, listOut, err := s.handleListPresets(context.Background(), nil, ListPresetsIn{})
	if err != nil {
		t.Fatalf("list_presets: %v", err)
	}
	if len(listOut.Presets) != 1 {
		t.Fatalf("presets = %+v", listOut.Presets)
	}
	p := listOut.Presets[0]
	if p.Name != "dinner" || p.RestaurantName != "Dominos" || p.AddrLine != "Home" || p.Lines != 1 {
		t.Fatalf("preset = %+v", p)
	}
}

func TestSavePresetWithoutRecentCartFails(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s := NewServer(&fakeBackend{}, &fakeAuth{token: true})
	_, _, err := s.handleSavePreset(context.Background(), nil, SavePresetIn{Name: "dinner"})
	if err == nil {
		t.Fatalf("expected error when there is no recent cart")
	}
}

func TestForgetPresetRemoves(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{cart: api.Cart{Restaurant: "Dominos", Total: 300,
		Lines: []api.CartLine{{ItemID: "i1", Name: "Margherita", Quantity: 1, Available: true}}}}
	s := NewServer(be, &fakeAuth{token: true})

	_, _, err := s.handleUpdateCart(context.Background(), nil, UpdateCartIn{
		AddressID: "a1", RestaurantID: "r1", RestaurantName: "Dominos",
		Items: []CartItemIn{{ItemID: "i1", Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("update_cart: %v", err)
	}
	if _, _, err := s.handleSavePreset(context.Background(), nil, SavePresetIn{Name: "dinner"}); err != nil {
		t.Fatalf("save_preset: %v", err)
	}

	_, fout, err := s.handleForgetPreset(context.Background(), nil, ForgetPresetIn{Name: "dinner"})
	if err != nil {
		t.Fatalf("forget_preset: %v", err)
	}
	if !fout.Removed {
		t.Fatalf("expected removed=true")
	}
	_, listOut, _ := s.handleListPresets(context.Background(), nil, ListPresetsIn{})
	if len(listOut.Presets) != 0 {
		t.Fatalf("presets should be empty, got %+v", listOut.Presets)
	}
}
