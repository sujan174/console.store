package tui

import (
	"errors"
	"testing"

	swiggysnap "console.store/internal/catalog/swiggy"

	"console.store/internal/broker/api"
	"console.store/internal/catalog"
	"console.store/internal/tui/datasource"
	"console.store/internal/tui/render"
	"console.store/internal/tui/screens"
)

// A cart already built on the Swiggy website is pulled at launch and seeded into
// the local cart so the conflict (keep/override) modal fires when the user then
// adds an item from a different restaurant in the terminal.
func TestCartPullSeedsLocalCartAndConflicts(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "acct-1", ""))

	out, _ := m.Update(datasource.CartPulledMsg{Cart: api.Cart{
		Restaurant: "Existing Diner",
		Lines: []api.CartLine{
			{ItemID: "111", Name: "Old Biryani", Quantity: 2, Price: 300},
		},
	}})
	m = out.(Model)

	if m.cartRestaurant != "Existing Diner" {
		t.Fatalf("cartRestaurant should seed from the pulled cart, got %q", m.cartRestaurant)
	}
	if len(m.lines) != 1 || m.lines[0].Qty != 2 || m.lines[0].Item.SwiggyID != "111" {
		t.Fatalf("local lines not seeded from pulled cart: %+v", m.lines)
	}
	// Adding from a different restaurant must now be detected as a conflict.
	if !m.conflictsWithCart("Other Cafe", catalog.SectionFood) {
		t.Fatal("a cross-restaurant add should conflict after seeding the foreign cart")
	}
	// The same restaurant does not conflict.
	if m.conflictsWithCart("Existing Diner", catalog.SectionFood) {
		t.Fatal("adding from the same restaurant must not conflict")
	}
}

// The pull never clobbers a cart the user is already building this session.
func TestCartPullDoesNotClobberLocalCart(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "acct-1", ""))
	m.cartRestaurant = "My Place"
	m.cartSection = catalog.SectionFood
	m.lines = []screens.CartLine{{Item: catalog.Item{ID: "a", Name: "Mine"}, Qty: 1}}

	out, _ := m.Update(datasource.CartPulledMsg{Cart: api.Cart{
		Restaurant: "Foreign", Lines: []api.CartLine{{ItemID: "9", Name: "X", Quantity: 1}},
	}})
	m = out.(Model)
	if m.cartRestaurant != "My Place" || len(m.lines) != 1 {
		t.Fatalf("launch pull must not overwrite an in-progress cart: rest=%q lines=%d", m.cartRestaurant, len(m.lines))
	}
}

// A pull error at launch is swallowed (no nag, no seed).
func TestCartPullErrorIsSilent(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "acct-1", ""))
	out, _ := m.Update(datasource.CartPulledMsg{Err: errors.New("boom")})
	m = out.(Model)
	if m.cartSyncErr != "" {
		t.Fatalf("a launch cart-pull error must stay silent, got %q", m.cartSyncErr)
	}
	if len(m.lines) != 0 {
		t.Fatal("an errored pull must not seed lines")
	}
}
