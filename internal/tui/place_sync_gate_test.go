package tui

// The order-placement safety gate: a real order must fire ONLY after the final
// pre-place cart sync succeeds AND re-prices to the total the user confirmed.
// A failed sync, a price change, or a newly sold-out line must abort — never
// place blind against whatever cart Swiggy currently holds.

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"consolestore/internal/broker/api"
	"consolestore/internal/catalog"
	swiggysnap "consolestore/internal/catalog/swiggy"
	"consolestore/internal/tui/datasource"
	"consolestore/internal/tui/render"
	"consolestore/internal/tui/screens"
)

func liveCheckoutModel(t *testing.T, be *liveFake) Model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	m := New(render.Caps{}, WithLiveBackend(be, snap, "acct-1", ""), WithSeededSnapshot())
	m.w, m.h = 100, 40
	m.addr = catalog.Address{ID: "a1", Label: "home", Line: "HSR"}
	m.screen = scrCheckout
	m.checkoutVertical = 0
	m.lines = []screens.CartLine{{Item: catalog.Item{SwiggyID: "i1", Name: "Dosa"}, Qty: 1, Price: 500}}
	m.liveCart = api.Cart{Total: 500, ItemTotal: 500, Lines: []api.CartLine{{ItemID: "i1", Available: true, Quantity: 1}}}
	m.cartLoaded = true
	m.checkout = m.buildCheckout()
	return m
}

// placedByCmd executes cmd (if any) and reports whether it placed an order.
func placedByCmd(t *testing.T, cmd tea.Cmd) bool {
	t.Helper()
	if cmd == nil {
		return false
	}
	_, ok := cmd().(datasource.OrderPlacedMsg)
	return ok
}

func TestConfirmPlaceOrderFailedSyncDoesNotPlace(t *testing.T) {
	be := &liveFake{}
	m := liveCheckoutModel(t, be)
	m.placingOrder = true
	m.placePending = true
	m.confirmedTotal = m.liveCart.Total

	nm, cmd := m.Update(datasource.CartSyncedMsg{Err: errors.New("update_food_cart 5xx")})
	mm := nm.(Model)

	if mm.placingOrder {
		t.Fatal("a failed pre-place sync must abort placement")
	}
	if mm.placePending {
		t.Fatal("placePending must clear on a failed sync")
	}
	if mm.orderErr == "" {
		t.Fatal("user must be told the order was not placed")
	}
	if placedByCmd(t, cmd) || be.placeCalls != 0 {
		t.Fatalf("must NOT place an order after a failed sync (placeCalls=%d)", be.placeCalls)
	}
}

func TestConfirmPlaceOrderSuccessfulSyncPlaces(t *testing.T) {
	be := &liveFake{}
	m := liveCheckoutModel(t, be)
	m.placingOrder = true
	m.placePending = true
	m.confirmedTotal = 500

	nm, cmd := m.Update(datasource.CartSyncedMsg{Cart: api.Cart{
		Total: 500, ItemTotal: 500,
		Lines: []api.CartLine{{ItemID: "i1", Available: true, Quantity: 1}},
	}})
	mm := nm.(Model)

	if !mm.placingOrder {
		t.Fatal("should still be in the placing state while the order fires")
	}
	if mm.placePending {
		t.Fatal("placePending should clear once the place command fires")
	}
	if cmd == nil {
		t.Fatal("a successful, price-matched sync must fire the place command")
	}
	if _, ok := cmd().(datasource.OrderPlacedMsg); !ok {
		t.Fatal("expected the place command to place the order")
	}
	if be.placeCalls != 1 {
		t.Fatalf("PlaceOrder calls = %d, want 1", be.placeCalls)
	}
}

func TestConfirmPlaceOrderRepriceAborts(t *testing.T) {
	be := &liveFake{}
	m := liveCheckoutModel(t, be)
	m.placingOrder = true
	m.placePending = true
	m.confirmedTotal = 500

	// Swiggy re-prices the cart between confirm and place (offer expiry/surge).
	nm, cmd := m.Update(datasource.CartSyncedMsg{Cart: api.Cart{
		Total: 560, ItemTotal: 560,
		Lines: []api.CartLine{{ItemID: "i1", Available: true, Quantity: 1}},
	}})
	mm := nm.(Model)

	if mm.placingOrder {
		t.Fatal("a price change must abort placement so the user re-confirms")
	}
	if placedByCmd(t, cmd) || be.placeCalls != 0 {
		t.Fatalf("must NOT place after a price change (placeCalls=%d)", be.placeCalls)
	}
	if !strings.Contains(mm.orderErr, "price") {
		t.Fatalf("expected a price-changed notice, got %q", mm.orderErr)
	}
}

func TestConfirmPlaceOrderSoldOutAborts(t *testing.T) {
	be := &liveFake{}
	m := liveCheckoutModel(t, be)
	m.placingOrder = true
	m.placePending = true
	m.confirmedTotal = 500

	// The final sync reveals a line went sold out.
	nm, cmd := m.Update(datasource.CartSyncedMsg{Cart: api.Cart{
		Total: 500, ItemTotal: 500,
		Lines: []api.CartLine{{ItemID: "i1", Available: false, Quantity: 1}},
	}})
	mm := nm.(Model)

	if mm.placingOrder {
		t.Fatal("a sold-out line on the final sync must abort placement")
	}
	if placedByCmd(t, cmd) || be.placeCalls != 0 {
		t.Fatalf("must NOT place with a sold-out line (placeCalls=%d)", be.placeCalls)
	}
}

func TestConfirmPlaceIMFailedSyncDoesNotPlace(t *testing.T) {
	be := &liveFake{}
	m := liveCheckoutModel(t, be)
	m.checkoutVertical = 1
	m.imLines = []screens.CartLine{{Item: catalog.Item{SwiggyID: "sp1", Name: "Milk"}, Qty: 1, Price: 200}}
	m.imLiveCart = api.IMCart{Total: 200, ItemTotal: 200}
	m.placingOrder = true
	m.imPlacePending = true
	m.confirmedTotal = 200

	nm, cmd := m.Update(datasource.IMCartSyncedMsg{Err: errors.New("checkout sync 5xx")})
	mm := nm.(Model)

	if mm.placingOrder {
		t.Fatal("a failed IM pre-place sync must abort placement")
	}
	if mm.imPlacePending {
		t.Fatal("imPlacePending must clear on a failed sync")
	}
	if placedByCmd(t, cmd) || be.imPlacedAddr != "" {
		t.Fatalf("must NOT place an IM order after a failed sync (placedAddr=%q)", be.imPlacedAddr)
	}
}
