package tui

// Regression tests for the checkout's flash-empty bug: opening the cart while
// the live fetch (or the launch cart pull) is still in flight used to render
// "your cart is empty" for a beat, then pop the real cart in. The checkout now
// holds a loading state until the fetch answers, and only then commits to
// either the lines or the honest empty state.

import (
	"errors"
	"strings"
	"testing"

	"consolestore/internal/broker/api"
	"consolestore/internal/tui/datasource"
)

// Food: opening the checkout before the launch cart pull lands shows the
// loader, and the pulled cart swaps in without ever flashing "empty".
func TestFoodCheckoutWaitsForCartPull(t *testing.T) {
	be := &liveFake{}
	m := imModel(t, be) // live model helper (address a1); screen is overwritten below
	m.screen = scrMenu

	// User opens the cart immediately at launch: no local lines, no cart
	// restaurant, and the pull hasn't answered yet (cartLoaded=false).
	cmd := m.openCartCmd()
	_ = cmd
	v := m.checkout.WithViewport(m.h).View(m.frame)
	if strings.Contains(v, "your cart is empty") {
		t.Fatalf("checkout must not claim empty while the pull is in flight; got:\n%s", v)
	}
	if !strings.Contains(v, "fetching your cart") && !strings.Contains(v, "counting what you picked") {
		t.Fatalf("checkout must show the cart loader while waiting; got:\n%s", v)
	}

	// The launch pull lands WITH a pre-existing cart → the seeded lines render.
	nm, _ := m.Update(datasource.CartPulledMsg{Cart: api.Cart{
		Restaurant: "Blue Tokai", Total: 250, ItemTotal: 250,
		Lines: []api.CartLine{{ItemID: "it1", Name: "Latte", Quantity: 1, Price: 250, Available: true}},
	}})
	m = nm.(Model)
	v = m.checkout.WithViewport(m.h).View(m.frame)
	if !strings.Contains(v, "Latte") {
		t.Fatalf("pulled cart must render its lines; got:\n%s", v)
	}
}

// Food: when the pull answers with NOTHING, the checkout settles on the
// honest empty state (no eternal spinner).
func TestFoodCheckoutEmptyAfterPullAnswers(t *testing.T) {
	be := &liveFake{}
	m := imModel(t, be)
	m.screen = scrMenu
	_ = m.openCartCmd()

	nm, _ := m.Update(datasource.CartPulledMsg{Cart: api.Cart{}})
	m = nm.(Model)
	v := m.checkout.WithViewport(m.h).View(m.frame)
	if !strings.Contains(v, "your cart is empty") {
		t.Fatalf("an answered-empty pull must show the empty state; got:\n%s", v)
	}
}

// Instamart: opening the IM checkout with no local lines waits for the
// LoadIMCart answer; an answered-empty fetch settles on the empty state.
func TestIMCheckoutWaitsForCartFetch(t *testing.T) {
	be := &liveFake{}
	m := imModel(t, be)

	cmd := m.openIMCheckoutCmd()
	if cmd == nil {
		t.Fatal("live IM checkout open must fire the cart fetch")
	}
	v := m.checkout.WithViewport(m.h).View(m.frame)
	if strings.Contains(v, "your cart is empty") {
		t.Fatalf("IM checkout must not claim empty while the fetch is in flight; got:\n%s", v)
	}

	// The fetch answers empty → the empty state is now honest.
	nm, _ := m.Update(datasource.IMCartLoadedMsg{Cart: api.IMCart{}})
	m = nm.(Model)
	v = m.checkout.WithViewport(m.h).View(m.frame)
	if !strings.Contains(v, "your cart is empty") {
		t.Fatalf("an answered-empty fetch must show the empty state; got:\n%s", v)
	}
}

// Instamart: a fetch ERROR also releases the gate (degrades to the local
// truth) instead of spinning forever.
func TestIMCheckoutFetchErrorReleasesGate(t *testing.T) {
	be := &liveFake{}
	m := imModel(t, be)

	_ = m.openIMCheckoutCmd()
	nm, _ := m.Update(datasource.IMCartLoadedMsg{Err: errors.New("boom")})
	m = nm.(Model)
	v := m.checkout.WithViewport(m.h).View(m.frame)
	if !strings.Contains(v, "your cart is empty") {
		t.Fatalf("a failed fetch must degrade to the empty state, not spin; got:\n%s", v)
	}
}
