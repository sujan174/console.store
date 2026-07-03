package tui

// Tests for the live Instamart vertical: browse/search, add (single + multi
// variant), debounced cart sync, checkout (bill + place order), launch cart
// pull, minimum-order + sold-out gates, and :alias set. Uses the same
// liveFake pattern as live_test.go, extended with Instamart fixtures.

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"consolestore/internal/broker/api"
	"consolestore/internal/catalog"
	swiggysnap "consolestore/internal/catalog/swiggy"
	"consolestore/internal/localstore"
	"consolestore/internal/tui/datasource"
	"consolestore/internal/tui/render"
	"consolestore/internal/tui/screens"
)

// imModel builds a live model on scrInstamart, ready for key-driven tests.
func imModel(t *testing.T, be *liveFake) Model {
	t.Helper()
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	m := New(render.Caps{}, WithLiveBackend(be, snap, "acct-1", ""), WithSeededSnapshot())
	m.w, m.h = 100, 40
	m.addr = catalog.Address{ID: "a1", Label: "home", Line: "HSR Layout"}
	m.screen = scrInstamart
	return m
}

// TestIMGoToRendersProducts: tab → instamart renders products from the
// your_go_to_items ("IMGoTo") fixture.
func TestIMGoToRendersProducts(t *testing.T) {
	be := &liveFake{imGoTo: []api.IMProduct{
		{ID: "p1", Name: "Amul Milk", Brand: "Amul", InStock: true,
			Variants: []api.IMVariantSel{{SpinID: "sp1", Label: "500 ml", Price: 30, InStock: true}}},
	}}
	// Start on the live menu (rail-focused, as a fresh browse landing is) and
	// press tab to enter Instamart — the real key-routing path.
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	m := New(render.Caps{}, WithLiveBackend(be, snap, "acct-1", ""), WithSeededSnapshot())
	m.w, m.h = 100, 40
	m.addr = catalog.Address{ID: "a1", Label: "home", Line: "HSR Layout"}
	m.screen = scrMenu
	m.railFocus = true

	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = nm.(Model)
	if m.screen != scrInstamart {
		t.Fatalf("tab from menu must land on scrInstamart, got %v", m.screen)
	}
	if cmd == nil {
		t.Fatal("entering instamart must fire the go-to load")
	}
	m = deliver(t, m, cmd)
	v := m.inst.View()
	if !strings.Contains(v, "Amul Milk") {
		t.Fatalf("instamart view must render go-to product; got:\n%s", v)
	}
}

// deliver runs a Cmd and feeds its message(s) back into Update, recursing
// through tea.Batch/tea.Sequence results (both are unexported []Cmd types
// under the hood — reflection unwraps either without importing internals).
func deliver(t *testing.T, m Model, c tea.Cmd) Model {
	t.Helper()
	if c == nil {
		return m
	}
	msg := c()
	if msg == nil {
		return m
	}
	if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice && v.Type().Elem() == reflect.TypeOf((*tea.Cmd)(nil)).Elem() {
		for i := 0; i < v.Len(); i++ {
			if sub, ok := v.Index(i).Interface().(tea.Cmd); ok {
				m = deliver(t, m, sub)
			}
		}
		return m
	}
	nm, _ := m.Update(msg)
	return nm.(Model)
}

// TestIMSearchFlow: / enters search mode, typing + enter fires IMSearch with
// the typed query and renders the results.
func TestIMSearchFlow(t *testing.T) {
	be := &liveFake{imSearch: []api.IMProduct{
		{ID: "p2", Name: "Maggi Noodles", Brand: "Nestle", InStock: true,
			Variants: []api.IMVariantSel{{SpinID: "sp2", Label: "70 g", Price: 14, InStock: true}}},
	}}
	m := imModel(t, be)

	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = nm.(Model)
	if !m.imSearchMode {
		t.Fatal("/ must enter Instamart search mode")
	}
	for _, r := range "maggi" {
		nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = nm.(Model)
	}
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if m.imQuery != "maggi" {
		t.Fatalf("imQuery = %q, want maggi", m.imQuery)
	}
	if cmd == nil {
		t.Fatal("submitting a search must fire LoadIMProducts")
	}
	m = deliver(t, m, cmd)
	if be.imSearchQuery != "maggi" {
		t.Fatalf("backend received query %q, want maggi", be.imSearchQuery)
	}
	v := m.inst.View()
	if !strings.Contains(v, "Maggi Noodles") {
		t.Fatalf("search results must render; got:\n%s", v)
	}
}

// TestIMAddSingleVariantSyncsCart: enter on a single-variant product
// increments the local line immediately and, once the debounce settles,
// fires IMUpdateCart with the product's spinId.
func TestIMAddSingleVariantSyncsCart(t *testing.T) {
	be := &liveFake{}
	m := imModel(t, be)
	m.imQuery = ""
	m.inst = screens.NewInstamart([]catalog.Item{
		{ID: "p1", SwiggyID: "sp1", Name: "Amul Milk", Price: 30, Section: catalog.SectionInstamart},
	}, nil, "")

	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if cmd != nil {
		t.Fatal("adding an item must debounce the sync, not fire it immediately")
	}
	if len(m.imLines) != 1 || m.imLines[0].Qty != 1 {
		t.Fatalf("expected one line qty 1, got %+v", m.imLines)
	}
	if !m.imCartSyncPending {
		t.Fatal("add must arm a pending IM cart sync")
	}

	var synced tea.Cmd
	for i := 0; i < cartSettleFrames+2 && synced == nil; i++ {
		m.frame++
		var c tea.Cmd
		m, c = m.onTick()
		synced = c
	}
	if synced == nil {
		t.Fatal("settled debounce must fire the IM cart sync")
	}
	m = deliver(t, m, synced)
	if len(be.imUpdateCalls) != 1 || be.imUpdateCalls[0].SpinID != "sp1" {
		t.Fatalf("IMUpdateCart must be called with spinId sp1, got %+v", be.imUpdateCalls)
	}
}

// TestIMAddMultiVariantOpensCustomize: a multi-variant product opens the
// customize modal; choosing a pack size adds a line carrying that variant's
// spinId.
func TestIMAddMultiVariantOpensCustomize(t *testing.T) {
	be := &liveFake{}
	m := imModel(t, be)
	item := catalog.Item{
		ID: "p3", SwiggyID: "sp-small", Name: "Coke", Price: 20, Section: catalog.SectionInstamart,
		Customizable: true,
		Options: []catalog.OptionGroup{{
			ID: "im-size", Name: "pack size", Min: 1, Max: 1, Variant: true, Absolute: true,
			Choices: []catalog.Choice{
				{ID: "sp-small", Name: "250 ml", Price: 20, InStock: true},
				{ID: "sp-big", Name: "1 L", Price: 60, InStock: true},
			},
		}},
	}
	m.inst = screens.NewInstamart([]catalog.Item{item}, nil, "")

	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if !m.customizeOpen {
		t.Fatal("a multi-variant Instamart product must open the customize modal")
	}
	// Move to the second choice (1 L) and confirm.
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = nm.(Model)
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	m = nm.(Model)
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)

	if m.customizeOpen {
		t.Fatal("confirming a valid selection must close the customize modal")
	}
	if len(m.imLines) != 1 {
		t.Fatalf("expected one IM line, got %d", len(m.imLines))
	}
	if m.imLines[0].Item.SwiggyID != "sp-big" {
		t.Fatalf("chosen variant's spinId must replace the line's SwiggyID; got %q", m.imLines[0].Item.SwiggyID)
	}
	// The food cart must be untouched by an Instamart add.
	if len(m.lines) != 0 {
		t.Fatalf("food cart must stay empty after an Instamart add, got %+v", m.lines)
	}
}

// TestIMCheckoutBillAndPlaceOrder: c opens checkout showing the Instamart
// bill with handling folded into "taxes & charges"; enter → y places the
// order via IMPlaceOrder, leaving the food cart untouched.
func TestIMCheckoutBillAndPlaceOrder(t *testing.T) {
	be := &liveFake{
		imCart: api.IMCart{
			ItemTotal: 100, Delivery: 25, Handling: 5, Taxes: 3, Total: 133,
			Lines: []api.IMCartLine{{SpinID: "sp1", Name: "Amul Milk", Quantity: 2, Price: 50, Available: true}},
		},
		imOrder: api.Order{ID: "im-ord-1", Status: "placed", Total: 133, ETA: "10-20 mins"},
	}
	m := imModel(t, be)
	m.imLines = []screens.CartLine{
		{Item: catalog.Item{ID: "p1", SwiggyID: "sp1", Name: "Amul Milk", Price: 50, Section: catalog.SectionInstamart}, Qty: 2, Price: 50},
	}
	m.lines = []screens.CartLine{
		{Item: catalog.Item{ID: "f1", SwiggyID: "f1", Name: "Latte", Price: 250}, Qty: 1},
	}
	m.cartRestaurant = "Blue Tokai"

	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	m = nm.(Model)
	if m.screen != scrCheckout || m.checkoutVertical != 1 {
		t.Fatalf("c on scrInstamart must open the merged checkout in IM mode; screen=%v vertical=%d", m.screen, m.checkoutVertical)
	}
	m = deliver(t, m, cmd)

	v := m.checkout.WithViewport(m.h).View(m.frame)
	if !strings.Contains(v, "taxes & charges") {
		t.Fatalf("Instamart checkout must fold handling into the taxes & charges row; got:\n%s", v)
	}
	if !strings.Contains(v, "133") {
		t.Fatalf("checkout bill must show the live IM total 133; got:\n%s", v)
	}

	// Enter opens the order-confirm modal.
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if !m.orderConfirmOpen {
		t.Fatal("enter on the IM checkout must open the order-confirm modal")
	}
	// y confirms placement.
	nm, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = nm.(Model)
	if !m.placingOrder {
		t.Fatal("confirming must set placingOrder=true")
	}
	if cmd == nil {
		t.Fatal("confirming must return the place-order sequence")
	}
	m = deliver(t, m, cmd)
	if be.imPlacedAddr != "a1" {
		t.Fatalf("IMPlaceOrder must be called with the address id, got %q", be.imPlacedAddr)
	}
	if m.screen != scrConfirm {
		t.Fatalf("screen = %v after IM order placed; want scrConfirm", m.screen)
	}
	if len(m.imLines) != 0 {
		t.Fatal("IM cart must be cleared after placement")
	}
	if len(m.lines) != 1 {
		t.Fatalf("food cart must stay untouched by an Instamart order; got %+v", m.lines)
	}
	if m.activeOrder.Vertical != "instamart" {
		t.Fatalf("activeOrder.Vertical = %q, want instamart", m.activeOrder.Vertical)
	}
}

// TestPullIMCartSeedsLines: the launch PullIMCart fixture seeds imLines when
// the local IM cart is empty.
func TestPullIMCartSeedsLines(t *testing.T) {
	be := &liveFake{imCart: api.IMCart{
		ItemTotal: 60, Total: 60,
		Lines: []api.IMCartLine{{SpinID: "sp9", Name: "Bread", Quantity: 1, Price: 60, Available: true}},
	}}
	m := imModel(t, be)

	nm, _ := m.Update(datasource.IMCartPulledMsg{Cart: be.imCart})
	m = nm.(Model)
	if len(m.imLines) != 1 {
		t.Fatalf("expected PullIMCart to seed one line, got %+v", m.imLines)
	}
	if m.imLines[0].Item.SwiggyID != "sp9" {
		t.Fatalf("seeded line SwiggyID = %q, want sp9", m.imLines[0].Item.SwiggyID)
	}
	if !strings.HasPrefix(m.imLines[0].Item.ID, "im-") {
		t.Fatalf("seeded line ID must be prefixed im- to avoid colliding with browse ids; got %q", m.imLines[0].Item.ID)
	}
}

// TestIMCheckoutUnderMinimumRefused: an Instamart order under ₹99 is refused
// with the minimum-order message and does not fire placement.
func TestIMCheckoutUnderMinimumRefused(t *testing.T) {
	be := &liveFake{}
	m := imModel(t, be)
	m.checkoutVertical = 1
	m.screen = scrCheckout
	m.imLines = []screens.CartLine{
		{Item: catalog.Item{ID: "p1", SwiggyID: "sp1", Name: "Gum", Price: 20, Section: catalog.SectionInstamart}, Qty: 1, Price: 20},
	}
	m.checkout = m.buildIMCheckout()

	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if m.orderConfirmOpen {
		t.Fatal("an under-minimum IM cart must not open the order-confirm modal")
	}
	if !strings.Contains(m.imOrderErr, "99") {
		t.Fatalf("imOrderErr must mention the ₹99 minimum, got %q", m.imOrderErr)
	}
}

// TestIMCheckoutSoldOutBlocked: a sold-out IM line blocks placement with the
// same message food's checkout uses.
func TestIMCheckoutSoldOutBlocked(t *testing.T) {
	be := &liveFake{}
	m := imModel(t, be)
	m.checkoutVertical = 1
	m.screen = scrCheckout
	m.imLines = []screens.CartLine{
		{Item: catalog.Item{ID: "p1", SwiggyID: "sp1", Name: "Milk", Price: 200, Section: catalog.SectionInstamart}, Qty: 1, Price: 200, Unavailable: true},
	}
	m.checkout = m.buildIMCheckout()

	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if m.orderConfirmOpen {
		t.Fatal("a sold-out IM line must not open the order-confirm modal")
	}
	if !strings.Contains(m.imOrderErr, "sold-out") {
		t.Fatalf("imOrderErr must flag the sold-out item, got %q", m.imOrderErr)
	}
}

// TestAliasSetInstamartSavesVerticalPreset: :alias set on the Instamart
// screen saves a preset with Vertical="instamart".
func TestAliasSetInstamartSavesVerticalPreset(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &liveFake{}
	m := imModel(t, be)
	m.imLines = []screens.CartLine{
		{Item: catalog.Item{ID: "p1", SwiggyID: "sp1", Name: "Amul Milk", Price: 30, Section: catalog.SectionInstamart}, Qty: 2},
	}

	lines := m.runAliasCommand("set groceries")
	joined := ""
	for _, l := range lines {
		joined += l.Text + "\n"
	}
	if !strings.Contains(joined, "Instamart") {
		t.Fatalf("confirmation must mention Instamart:\n%s", joined)
	}
	ps, _ := localstore.LoadPresets()
	got := ps.ByName("groceries")
	if len(got) != 1 || !got[0].IsInstamart() || got[0].Vertical != "instamart" {
		t.Fatalf("preset not saved as instamart vertical: %+v", got)
	}
	if len(got[0].Lines) != 1 || got[0].Lines[0].ItemID != "sp1" {
		t.Fatalf("preset lines not captured: %+v", got[0].Lines)
	}
}
