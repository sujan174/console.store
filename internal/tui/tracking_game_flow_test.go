package tui

// End-to-end flow test for the loading -> confetti -> tracking+dodge sequence
// that fires after an order is placed. Drives the real Update/tick pipeline
// through teatest against the package's existing live-fake backend (the same
// liveFake used by place_sync_gate_test.go / dualorder_test.go / live_test.go),
// starting from an already-built checkout (mirroring liveCheckoutModel) so the
// test only has to drive the confirm -> place -> confetti -> tracking leg.

import (
	"bytes"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"consolestore/internal/broker/api"
	"consolestore/internal/catalog"
	swiggysnap "consolestore/internal/catalog/swiggy"
	"consolestore/internal/tui/render"
	"consolestore/internal/tui/screens"
)

// gameFlowFake wraps liveFake and scripts UpdateCart/GetCart to always echo
// back the same priced cart, so the pre-confirm sync and the final pre-place
// sync (fired by confirmPlaceOrder) re-price to IDENTICAL totals — the
// place-safety gate (see place_sync_gate_test.go) requires an exact match
// before it will fire PlaceOrder.
type gameFlowFake struct {
	*liveFake
	cart api.Cart
}

// UpdateCart sleeps briefly, like a real network round-trip. bubbletea
// coalesces Update calls that land within one render tick, so an instant fake
// would resolve confirm -> sync -> place before the renderer ever paints the
// intermediate "placing your order" loader frame; the small delay gives it a
// real frame to render, matching what a live backend would look like.
func (f *gameFlowFake) UpdateCart(string, string, string, []api.CartItem) (api.Cart, error) {
	time.Sleep(150 * time.Millisecond)
	return f.cart, nil
}
func (f *gameFlowFake) GetCart(string, string) (api.Cart, error) { return f.cart, nil }
func (f *gameFlowFake) PlaceOrder(addressID string) (api.Order, error) {
	f.liveFake.placeCalls++
	return api.Order{ID: "ord-e2e", Restaurant: "Blue Tokai", ETA: "35-45 mins", Total: f.cart.Total}, nil
}

// PlacesQuery is scripted to keep echoing the same Blue Tokai restaurant for
// every query. The real Init() fires a live LoadPlacesQuery for the home
// chip's query (the default first chip is "coffee" — the same key the test
// seeds directly into the snapshot); without this override that async load
// would land AFTER construction and blank the seeded "coffee" bucket back to
// empty, breaking cartPlaceID()'s chip-query lookup mid-flow.
func (f *gameFlowFake) PlacesQuery(string, string) ([]api.Restaurant, error) {
	return []api.Restaurant{{ID: "bt1", Name: "Blue Tokai"}}, nil
}

// gameFlowModel builds a live model already sitting on the merged
// checkout/cart page with one syncable line, matching liveCheckoutModel /
// checkoutModel in place_sync_gate_test.go and live_test.go.
func gameFlowModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	snap := swiggysnap.NewSnapshot()
	addr := catalog.Address{ID: "a1", Label: "home", Line: "HSR"}
	place := catalog.Place{ID: "bt1", Name: "Blue Tokai", SwiggyID: "swiggy-bt1", Section: catalog.SectionCoffee}
	snap.SetAddresses([]catalog.Address{addr})
	snap.SetPlaces("a1", string(catalog.SectionCoffee), []catalog.Place{place})
	snap.SetMenu(catalog.Place{ID: "bt1", Name: "Blue Tokai", SwiggyID: "swiggy-bt1",
		Items: []catalog.Item{{ID: "i1", Name: "Latte", Price: 250, SwiggyID: "swiggy-i1"}}})

	cart := api.Cart{
		Total: 279, ItemTotal: 250, Delivery: 29,
		Lines: []api.CartLine{{ItemID: "swiggy-i1", Name: "Latte", Quantity: 1, Price: 250, Available: true}},
	}
	be := &gameFlowFake{liveFake: &liveFake{}, cart: cart}
	m := New(render.Caps{}, WithLiveBackend(be, snap, "acct-1", ""), WithSeededSnapshot())
	m.w, m.h = 80, 40
	m.addr = addr
	m.screen = scrCheckout
	m.checkoutVertical = 0
	m.cartRestaurant = "Blue Tokai"
	m.lines = []screens.CartLine{{Item: catalog.Item{ID: "i1", SwiggyID: "swiggy-i1", Name: "Latte", Price: 250}, Qty: 1}}
	m.liveCart = cart
	m.cartLoaded = true
	m.checkout = m.buildCheckout()
	return m
}

// TestTrackingGameFlow drives confirm -> place -> loader -> confetti ->
// tracking(+dodge attract) -> a jump, on an 80x40 term so the game panel has
// room below the tracking body (trackingGameRows needs >= 6 spare rows).
func TestTrackingGameFlow(t *testing.T) {
	m := gameFlowModel(t)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 40))

	// Enter on checkout opens the order-confirm modal (default focus "yes").
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("place this order at"))
	}, teatest.WithDuration(3*time.Second))

	// Enter again confirms "yes": fires the final pre-place sync, which (once
	// it re-prices to the same confirmed total) fires PlaceOrder. Until the
	// OrderPlacedMsg lands, the screen renders the full-page loader.
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("placing your order"))
	}, teatest.WithDuration(3*time.Second))

	// The fake resolves quickly; OrderPlacedMsg lands and the confetti page
	// (scrConfirm) shows the placed banner.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("order placed"))
	}, teatest.WithDuration(3*time.Second))

	// confirmAdvanceFrames (25 ticks at 60ms, ~1.5s) auto-advances confetti to
	// tracking; on an 80x40 term there's room for the dodge attract prompt
	// below the tracking body. teatest's Output() reader is drained on every
	// WaitFor poll, so the tracking header ("Blue Tokai") and the dodge
	// attract prompt ("ENTER") can land in different polls once the game is
	// lazily created a tick or two after the screen switches — check each
	// independently rather than requiring both in the same snapshot.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Blue Tokai"))
	}, teatest.WithDuration(3*time.Second))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("ENTER"))
	}, teatest.WithDuration(3*time.Second))

	// Enter starts the game, Space jumps — the frame must keep rendering
	// without panicking. Once Playing, the dodge status strip switches from
	// the attract prompt to "score N   best N".
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("score")) && bytes.Contains(b, []byte("best"))
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}
