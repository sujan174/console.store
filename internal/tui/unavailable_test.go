package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	swiggysnap "consolestore/internal/catalog/swiggy"

	"consolestore/internal/broker/api"
	"consolestore/internal/catalog"
	"consolestore/internal/tui/datasource"
	"consolestore/internal/tui/render"
	"consolestore/internal/tui/screens"
)

// A cart whose synced response flags an item out of stock marks that line and
// blocks the order (Swiggy would reject it), surfacing a clear reason.
func TestUnavailableItemBlocksOrder(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "acct-1", ""))
	m.addr = catalog.Address{ID: "a1"}
	m.screen = scrCheckout
	m.lines = []screens.CartLine{
		{Item: catalog.Item{ID: "1", SwiggyID: "1", Name: "Fine"}, Qty: 1, Price: 100},
		{Item: catalog.Item{ID: "2", SwiggyID: "2", Name: "Gone"}, Qty: 1, Price: 200},
	}
	m.cartRestaurant = "Diner"

	// Sync result reports item 2 as unavailable.
	out, _ := m.Update(datasource.CartSyncedMsg{Cart: api.Cart{Total: 300, Lines: []api.CartLine{
		{ItemID: "1", Name: "Fine", Quantity: 1, Price: 100, Available: true},
		{ItemID: "2", Name: "Gone", Quantity: 1, Price: 200, Available: false},
	}}})
	m = out.(Model)

	if !m.unavailableItems["2"] {
		t.Fatal("item 2 should be recorded as unavailable")
	}
	if !m.hasUnavailableLine() {
		t.Fatal("hasUnavailableLine should be true")
	}

	// The checkout flags the sold-out line.
	if v := m.buildCheckout().View(0); !strings.Contains(v, "sold out") {
		t.Fatalf("checkout should mark the sold-out line:\n%s", v)
	}

	// Pressing enter must NOT place the order; it shows a clear reason instead.
	out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = out.(Model)
	if m.placingOrder {
		t.Fatal("order must be blocked while a line is sold out")
	}
	if cmd != nil {
		t.Fatal("no place-order command should fire when blocked")
	}
	if m.orderErr == "" {
		t.Fatal("a blocked order must explain why")
	}
}

// An all-available cart clears the unavailable set and allows the order.
func TestAvailableCartAllowsOrder(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "acct-1", ""))
	m.addr = catalog.Address{ID: "a1"}
	m.screen = scrCheckout
	m.lines = []screens.CartLine{{Item: catalog.Item{ID: "1", SwiggyID: "1", Name: "Fine"}, Qty: 1, Price: 100}}
	m.cartRestaurant = "Diner"

	out, _ := m.Update(datasource.CartSyncedMsg{Cart: api.Cart{Total: 100, Lines: []api.CartLine{
		{ItemID: "1", Name: "Fine", Quantity: 1, Price: 100, Available: true},
	}}})
	m = out.(Model)
	if m.hasUnavailableLine() {
		t.Fatal("no line should be unavailable")
	}
	out, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // opens order-confirm modal
	m = out.(Model)
	out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // confirm (default "yes")
	m = out.(Model)
	if !m.placingOrder || cmd == nil {
		t.Fatal("an all-available cart should place the order")
	}
}

// A failed place_food_order surfaces the real error on the checkout page.
func TestOrderFailureSurfacedOnCheckout(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "acct-1", ""))
	m.screen = scrCheckout
	m.lines = []screens.CartLine{{Item: catalog.Item{ID: "1", SwiggyID: "1", Name: "Fine"}, Qty: 1}}

	out, _ := m.Update(datasource.OrderPlacedMsg{Err: errForTest()})
	m = out.(Model)
	if m.orderErr == "" {
		t.Fatal("a place-order failure must set orderErr")
	}
	if v := m.checkout.View(0); !strings.Contains(v, "order failed") {
		t.Fatalf("the checkout must show the order failure:\n%s", v)
	}
}

func errForTest() error { return errString("swiggy: item unavailable") }

type errString string

func (e errString) Error() string { return string(e) }
