package tui

// Regression tests for Instamart cart DELETION syncing to Swiggy. The live
// server's update_cart REPLACES the whole cart (verified live 2026-07-03 via
// IMPROBE_DELSEM), so any local removal must fire either an IMUpdateCart
// without the removed spinId or an IMClearCart when the cart goes empty.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"consolestore/internal/broker/api"
	"consolestore/internal/catalog"
	"consolestore/internal/tui/datasource"
	"consolestore/internal/tui/screens"
)

// settleIM pumps ticks until the debounced IM cart sync fires, then delivers it.
func settleIM(t *testing.T, m Model) Model {
	t.Helper()
	var synced tea.Cmd
	for i := 0; i < cartSettleFrames+2 && synced == nil; i++ {
		m.frame++
		var c tea.Cmd
		m, c = m.onTick()
		synced = c
	}
	if synced == nil {
		t.Fatal("settled debounce did not fire an IM cart sync")
	}
	return deliver(t, m, synced)
}

func imTwoLines() []screens.CartLine {
	return []screens.CartLine{
		{Item: catalog.Item{ID: "p1", SwiggyID: "sp1", Name: "Amul Milk", Price: 30, Section: catalog.SectionInstamart}, Qty: 1, Price: 30},
		{Item: catalog.Item{ID: "p2", SwiggyID: "sp2", Name: "Bread", Price: 40, Section: catalog.SectionInstamart}, Qty: 1, Price: 40},
	}
}

// Browse list: removing an item with `-` must sync an update_cart WITHOUT the
// removed spinId (replace semantics remove it server-side).
func TestIMBrowseDeleteSyncsRemoval(t *testing.T) {
	be := &liveFake{}
	m := imModel(t, be)
	m.imLines = imTwoLines()
	m.imRailFocus = false
	m.inst = screens.NewInstamart([]catalog.Item{
		{ID: "p1", SwiggyID: "sp1", Name: "Amul Milk", Price: 30, Section: catalog.SectionInstamart},
		{ID: "p2", SwiggyID: "sp2", Name: "Bread", Price: 40, Section: catalog.SectionInstamart},
	}, m.imQtyMap(), "")

	// Cursor on p1 (row 0); `-` removes its only unit.
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("-")})
	m = nm.(Model)
	if len(m.imLines) != 1 || m.imLines[0].Item.SwiggyID != "sp2" {
		t.Fatalf("local delete must drop sp1, got %+v", m.imLines)
	}
	m = settleIM(t, m)
	if be.imUpdateCount == 0 {
		t.Fatal("browse delete never reached IMUpdateCart — server cart keeps the item")
	}
	for _, it := range be.imUpdateCalls {
		if it.SpinID == "sp1" {
			t.Fatalf("sync payload still contains deleted sp1: %+v", be.imUpdateCalls)
		}
	}
}

// Browse list: removing the LAST item must clear the server cart (update_cart
// cannot express empty — an omitted-everything call is a clear_cart).
func TestIMBrowseDeleteLastItemClearsCart(t *testing.T) {
	be := &liveFake{}
	m := imModel(t, be)
	m.imLines = imTwoLines()[:1]
	m.imRailFocus = false
	m.inst = screens.NewInstamart([]catalog.Item{
		{ID: "p1", SwiggyID: "sp1", Name: "Amul Milk", Price: 30, Section: catalog.SectionInstamart},
	}, m.imQtyMap(), "")

	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("-")})
	m = nm.(Model)
	if len(m.imLines) != 0 {
		t.Fatalf("local delete must empty the cart, got %+v", m.imLines)
	}
	m = settleIM(t, m)
	if be.imClearCalls == 0 {
		t.Fatal("deleting the last item never reached IMClearCart — server cart keeps the item")
	}
}

// Checkout page: delete/backspace on a line must immediately sync an
// update_cart WITHOUT the removed spinId.
func TestIMCheckoutDeleteSyncsRemoval(t *testing.T) {
	be := &liveFake{}
	m := imModel(t, be)
	m.imLines = imTwoLines()
	m.screen = scrCheckout
	m.checkoutVertical = 1
	m.checkout = m.buildIMCheckout()

	// Cursor starts on line 0 (sp1); delete removes the whole line.
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = nm.(Model)
	if len(m.imLines) != 1 || m.imLines[0].Item.SwiggyID != "sp2" {
		t.Fatalf("local delete must drop sp1, got %+v", m.imLines)
	}
	if cmd == nil {
		t.Fatal("checkout delete must fire a sync immediately")
	}
	m = deliver(t, m, cmd)
	if be.imUpdateCount == 0 {
		t.Fatal("checkout delete never reached IMUpdateCart")
	}
	for _, it := range be.imUpdateCalls {
		if it.SpinID == "sp1" {
			t.Fatalf("sync payload still contains deleted sp1: %+v", be.imUpdateCalls)
		}
	}
}

// Checkout page: reducing the last line to zero must clear the server cart.
func TestIMCheckoutReduceLastToZeroClearsCart(t *testing.T) {
	be := &liveFake{}
	m := imModel(t, be)
	m.imLines = imTwoLines()[:1]
	m.screen = scrCheckout
	m.checkoutVertical = 1
	m.checkout = m.buildIMCheckout()

	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("-")})
	m = nm.(Model)
	if len(m.imLines) != 0 {
		t.Fatalf("local reduce must empty the cart, got %+v", m.imLines)
	}
	if cmd == nil {
		t.Fatal("checkout reduce-to-zero must fire a clear immediately")
	}
	m = deliver(t, m, cmd)
	if be.imClearCalls == 0 {
		t.Fatal("reduce-to-zero never reached IMClearCart")
	}
}

// After a successful placement, the server cart must be force-cleared so a
// server-side leftover (seen live: items lingering in the app cart after
// checkout) can't resurface or double-charge on the next order.
func TestIMPlaceOrderForceClearsCart(t *testing.T) {
	be := &liveFake{
		imCart: api.IMCart{
			ItemTotal: 130, Total: 130,
			Lines: []api.IMCartLine{{SpinID: "sp1", Name: "Milk", Quantity: 1, Price: 130, Available: true}},
		},
		imOrder: api.Order{ID: "im-1", Status: "placed", Total: 130},
	}
	m := imModel(t, be)
	m.imLines = []screens.CartLine{
		{Item: catalog.Item{ID: "p1", SwiggyID: "sp1", Name: "Milk", Price: 130, Section: catalog.SectionInstamart}, Qty: 1, Price: 130},
	}
	// The checkout-entry load populates imLiveCart from the server; simulate it
	// so the pre-confirm freshness guard sees a cart that matches the lines.
	m.imLiveCart = be.imCart
	m.screen = scrCheckout
	m.checkoutVertical = 1
	m.checkout = m.buildIMCheckout()

	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if !m.orderConfirmOpen {
		t.Fatal("enter must open the confirm modal")
	}
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = nm.(Model)
	m = deliver(t, m, cmd)
	if be.imPlacedAddr != "a1" {
		t.Fatalf("order not placed, addr=%q", be.imPlacedAddr)
	}
	// deliver() drops Cmds returned by Update, so feed the placed message once
	// more and run ITS follow-up commands — that's where the flush lives.
	nm, cmd = m.Update(datasource.IMOrderPlacedMsg{Order: be.imOrder})
	m = nm.(Model)
	if cmd == nil {
		t.Fatal("IM placement success must return follow-up commands")
	}
	m = deliver(t, m, cmd)
	if be.imClearCalls == 0 {
		t.Fatal("placement must force-clear the server cart (leftover-items defense)")
	}
	if len(m.imLines) != 0 {
		t.Fatalf("local IM cart must stay empty after placement, got %+v", m.imLines)
	}
}

// The top-right banner cart chip must follow the vertical: Instamart screens
// show the Instamart cart (server-derived once a live cart is known), food
// screens keep showing the food cart.
func TestBannerChipSwitchesToInstamart(t *testing.T) {
	be := &liveFake{}
	m := imModel(t, be)
	// Food cart: 1×₹250. Instamart live cart (Swiggy-confirmed): 2 units ₹162.
	m.lines = []screens.CartLine{
		{Item: catalog.Item{ID: "f1", SwiggyID: "f1", Name: "Latte", Price: 250}, Qty: 1, Price: 250},
	}
	m.imLiveCart = api.IMCart{
		ItemTotal: 156, Total: 162,
		Lines: []api.IMCartLine{
			{SpinID: "sp1", Name: "Milk", Quantity: 1, Price: 116, Available: true},
			{SpinID: "sp2", Name: "Bread", Quantity: 1, Price: 40, Available: true},
		},
	}
	m.imLines = imTwoLines()

	m.screen = scrInstamart
	m.inst = m.buildInstamart()
	if v := m.View(); !strings.Contains(v, "🛒 cart · 2 · ₹162") {
		t.Fatalf("instamart screen banner must show the IM cart chip (server totals); got:\n%.400s", v)
	}

	m.screen = scrMenu
	if v := m.View(); !strings.Contains(v, "₹250") || strings.Contains(v, "₹162") {
		t.Fatalf("menu screen banner must show the FOOD cart chip; got:\n%.400s", v)
	}

	// IM checkout keeps the IM chip too.
	m.screen = scrCheckout
	m.checkoutVertical = 1
	m.checkout = m.buildIMCheckout()
	if v := m.View(); !strings.Contains(v, "🛒 cart · 2 · ₹162") {
		t.Fatalf("IM checkout banner must show the IM cart chip; got:\n%.400s", v)
	}
}

// The order-confirm modal must state the delivery address and the live
// (Swiggy-derived) payable amount — the final look before money moves.
func TestOrderConfirmShowsAddressAndLiveTotal(t *testing.T) {
	be := &liveFake{imCart: api.IMCart{
		ItemTotal: 156, Total: 162,
		Lines: []api.IMCartLine{{SpinID: "sp1", Name: "Milk", Quantity: 2, Price: 78, Available: true}},
	}}
	m := imModel(t, be)
	m.imLines = []screens.CartLine{
		{Item: catalog.Item{ID: "p1", SwiggyID: "sp1", Name: "Milk", Price: 78, Section: catalog.SectionInstamart}, Qty: 2, Price: 78},
	}
	m.imLiveCart = be.imCart
	m.screen = scrCheckout
	m.checkoutVertical = 1
	m.checkout = m.buildIMCheckout()

	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if !m.orderConfirmOpen {
		t.Fatal("enter must open the confirm modal")
	}
	v := m.View()
	if !strings.Contains(v, "HSR Layout") {
		t.Fatalf("confirm modal must show the delivery address; got:\n%.600s", v)
	}
	if !strings.Contains(v, "₹162") {
		t.Fatalf("confirm modal must show the live Swiggy total ₹162; got:\n%.600s", v)
	}
}

// Regression for "added items didn't show, bill was the old item": with a
// pre-existing (seeded) Instamart cart, adding a new item must move the
// top-right chip IMMEDIATELY (optimistic local count), not keep showing the
// stale server cart until the debounced sync returns. imLiveMatchesLines gates
// the server-derived chip so the in-flight window falls back to local intent.
func TestIMChipReflectsAddBeforeSyncReturns(t *testing.T) {
	be := &liveFake{}
	m := imModel(t, be)
	// Server-confirmed cart from launch pull: one pre-existing item X (₹100).
	m.imLiveCart = api.IMCart{
		ItemTotal: 100, Total: 100,
		Lines: []api.IMCartLine{{SpinID: "spX", Name: "Old Item", Quantity: 1, Price: 100, Available: true}},
	}
	m.imLines = []screens.CartLine{
		{Item: catalog.Item{ID: "im-spX", SwiggyID: "spX", Name: "Old Item", Price: 100, Section: catalog.SectionInstamart}, Qty: 1, Price: 100},
	}
	// Sanity: converged state shows the server chip.
	if got := m.imCartChip(); !strings.Contains(got, "· 1 · ₹100") {
		t.Fatalf("converged chip should show server cart; got %q", got)
	}

	// User adds a NEW item Y on the browse list. Local lines now diverge from
	// the (still unchanged) server cart — the sync is only armed, not returned.
	m.imRailFocus = false
	m.inst = screens.NewInstamart([]catalog.Item{
		{ID: "pY", SwiggyID: "spY", Name: "New Item", Price: 50, Section: catalog.SectionInstamart},
	}, m.imQtyMap(), "")
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if len(m.imLines) != 2 {
		t.Fatalf("add must append the new line, got %+v", m.imLines)
	}

	// THE ASSERTION: chip reflects BOTH items now (count 2), not the stale
	// server count of 1. Before the fix imCartChip returned the server cart
	// verbatim → "· 1 · ₹100", hiding the just-added item.
	got := m.imCartChip()
	if strings.Contains(got, "· 1 ·") {
		t.Fatalf("chip still shows the stale server cart after an add: %q", got)
	}
	if !strings.Contains(got, "· 2 · ₹150") {
		t.Fatalf("chip must show the optimistic 2-item cart after an add; got %q", got)
	}

	// Once the debounced sync lands, the server cart catches up and the chip
	// converges to Swiggy's fee-inclusive total.
	be.imCart = api.IMCart{
		ItemTotal: 150, Total: 155, // +₹5 handling the local optimistic total can't know
		Lines: []api.IMCartLine{
			{SpinID: "spX", Name: "Old Item", Quantity: 1, Price: 100, Available: true},
			{SpinID: "spY", Name: "New Item", Quantity: 1, Price: 50, Available: true},
		},
	}
	m = settleIM(t, m)
	if got := m.imCartChip(); !strings.Contains(got, "· 2 · ₹155") {
		t.Fatalf("chip must converge to the server total after sync; got %q", got)
	}
}

// Final-value guarantee: if the user bumps a qty (debounced) and hits Enter
// before the sync returns, the confirm modal must NOT open on the stale total.
// It flushes the cart first and opens only when the fresh server bill lands.
func TestIMConfirmWaitsForFreshBill(t *testing.T) {
	be := &liveFake{}
	m := imModel(t, be)
	// Server-confirmed baseline: 1 unit, ₹100.
	m.imLiveCart = api.IMCart{
		ItemTotal: 100, Total: 100,
		Lines: []api.IMCartLine{{SpinID: "sp1", Name: "Milk", Quantity: 1, Price: 100, Available: true}},
	}
	m.imLines = []screens.CartLine{
		{Item: catalog.Item{ID: "p1", SwiggyID: "sp1", Name: "Milk", Price: 100, Section: catalog.SectionInstamart}, Qty: 1, Price: 100},
	}
	m.screen = scrCheckout
	m.checkoutVertical = 1
	m.checkout = m.buildIMCheckout()

	// Bump to qty 2 locally (as a debounced + would) so the server cart is now stale.
	m.imLines[0].Qty = 2
	m.checkout = m.buildIMCheckout()

	// Enter must defer: flush the cart, NOT open the modal yet.
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if m.orderConfirmOpen {
		t.Fatal("confirm modal must not open on a stale server bill")
	}
	if !m.imConfirmPending {
		t.Fatal("Enter with a stale cart must mark a pending confirm")
	}
	if cmd == nil {
		t.Fatal("Enter with a stale cart must fire the flush sync")
	}

	// The flush returns Swiggy's fresh bill for the 2-unit cart.
	be.imCart = api.IMCart{
		ItemTotal: 200, Total: 205,
		Lines: []api.IMCartLine{{SpinID: "sp1", Name: "Milk", Quantity: 2, Price: 100, Available: true}},
	}
	m = deliver(t, m, cmd)
	if !m.orderConfirmOpen {
		t.Fatal("confirm modal must open once the fresh bill lands")
	}
	if m.imConfirmPending {
		t.Fatal("pending flag must clear after the modal opens")
	}
	// The modal's payable amount is now the fresh server total, not the stale one.
	if got := m.checkout.PayAmount(); got != 205 {
		t.Fatalf("confirm total must be the fresh server bill 205, got %d", got)
	}
}

// THE rapid-add lost-update regression: 7 fast adds produce ONE in-flight
// update_cart at a time; edits arriving mid-flight are held and re-fired with
// the LATEST quantities once the flight lands — the server can never be left
// on a stale smaller cart by out-of-order writes.
func TestIMRapidAddsSingleFlightConverges(t *testing.T) {
	be := &liveFake{}
	m := imModel(t, be)
	m.imRailFocus = false
	m.imQuery = ""
	// Seed the snapshot too: each add repaints the browse list from it
	// (refreshInstamart), so an empty snapshot would swallow later presses.
	m.snap.SetInstamart(m.addr.ID, "", []catalog.Item{
		{ID: "p1", SwiggyID: "sp1", Name: "Red Bull", Price: 125, Section: catalog.SectionInstamart},
	})
	m.inst = m.buildInstamart()

	press := func(n int) {
		for i := 0; i < n; i++ {
			nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = nm.(Model)
		}
	}

	// 3 fast adds → debounce settles → sync #1 fires with qty 3.
	press(3)
	var sync1 tea.Cmd
	for i := 0; i < cartSettleFrames+2 && sync1 == nil; i++ {
		m.frame++
		var c tea.Cmd
		m, c = m.onTick()
		sync1 = c
	}
	if sync1 == nil {
		t.Fatal("settled debounce must fire sync #1")
	}
	if !m.imSyncInFlight {
		t.Fatal("firing a sync must mark the write in flight")
	}

	// 4 more adds while sync #1 is STILL IN FLIGHT (not delivered yet).
	press(4)
	if m.imLines[0].Qty != 7 {
		t.Fatalf("local qty = %d, want 7", m.imLines[0].Qty)
	}
	// The debounce must HOLD — no second write while one is in flight.
	for i := 0; i < cartSettleFrames+2; i++ {
		m.frame++
		var c tea.Cmd
		m, c = m.onTick()
		if c != nil {
			t.Fatal("a second sync must NOT fire while one is in flight (write reorder risk)")
		}
	}
	if !m.imCartSyncPending {
		t.Fatal("mid-flight edits must stay pending, not be dropped")
	}

	// sync #1 lands (server confirms qty 3 — already stale vs local 7).
	be.imCart = api.IMCart{ItemTotal: 375, Total: 375,
		Lines: []api.IMCartLine{{SpinID: "sp1", Name: "Red Bull", Quantity: 3, Price: 125, Available: true}}}
	m = deliver(t, m, sync1)
	if m.imSyncInFlight {
		t.Fatal("the response must release the in-flight flag")
	}

	// Next tick: the held edit fires sync #2 with the LATEST qty 7.
	var sync2 tea.Cmd
	for i := 0; i < 2 && sync2 == nil; i++ {
		m.frame++
		var c tea.Cmd
		m, c = m.onTick()
		sync2 = c
	}
	if sync2 == nil {
		t.Fatal("the held edit must fire a follow-up sync after the flight lands")
	}
	be.imCart = api.IMCart{ItemTotal: 875, Total: 875,
		Lines: []api.IMCartLine{{SpinID: "sp1", Name: "Red Bull", Quantity: 7, Price: 125, Available: true}}}
	m = deliver(t, m, sync2)
	if len(be.imUpdateCalls) != 1 || be.imUpdateCalls[0].Quantity != 7 {
		t.Fatalf("final update_cart must carry qty 7, got %+v", be.imUpdateCalls)
	}
	if got := m.imCartChip(); !strings.Contains(got, "· 7 · ₹875") {
		t.Fatalf("chip must converge to the 7-item server cart; got %q", got)
	}
}

// Rapid adds then an INSTANT `c`: the checkout must flush the pending write
// (not race it with a read), hold the bill in the "updating" state until the
// chain converges, and disable placing meanwhile.
func TestIMCheckoutFlushesPendingAddsBeforeBill(t *testing.T) {
	be := &liveFake{}
	m := imModel(t, be)
	m.imRailFocus = false
	m.imQuery = ""
	m.snap.SetInstamart(m.addr.ID, "", []catalog.Item{
		{ID: "p1", SwiggyID: "sp1", Name: "Red Bull", Price: 125, Section: catalog.SectionInstamart},
	})
	m.inst = m.buildInstamart()

	// 7 rapid adds, then `c` before the debounce ever settles.
	for i := 0; i < 7; i++ {
		nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = nm.(Model)
	}
	if !m.imCartSyncPending {
		t.Fatal("precondition: adds must leave a pending sync")
	}
	cmd := m.openIMCheckoutCmd()
	if cmd == nil {
		t.Fatal("opening the checkout with pending edits must flush the write now")
	}
	if !m.imSyncInFlight {
		t.Fatal("the flush must be marked in flight")
	}
	// While the flush is in flight the bill must read "updating", never a
	// stale total, and the place bar must be disabled.
	v := m.checkout.WithViewport(m.h).View(m.frame)
	if !strings.Contains(v, "updating bill…") {
		t.Fatalf("checkout must hold the bill while the write settles; got:\n%s", v)
	}

	// The flush lands with the full 7-item cart → bill + place bar unlock.
	be.imCart = api.IMCart{ItemTotal: 875, Total: 880,
		Lines: []api.IMCartLine{{SpinID: "sp1", Name: "Red Bull", Quantity: 7, Price: 125, Available: true}}}
	m = deliver(t, m, cmd)
	if len(be.imUpdateCalls) != 1 || be.imUpdateCalls[0].Quantity != 7 {
		t.Fatalf("the flushed update_cart must carry qty 7, got %+v", be.imUpdateCalls)
	}
	v = m.checkout.WithViewport(m.h).View(m.frame)
	if strings.Contains(v, "updating bill…") {
		t.Fatalf("converged checkout must show the real bill; got:\n%s", v)
	}
	if !strings.Contains(v, "880") {
		t.Fatalf("bill must show the live 7-item total 880; got:\n%s", v)
	}
}

// The cart chip carries a spinner while the write chain is unsettled — the
// "it's updating, hold on" indicator — and drops it once converged.
func TestIMChipShowsSyncingSpinner(t *testing.T) {
	be := &liveFake{}
	m := imModel(t, be)
	m.imLines = imTwoLines()
	m.imCartSyncPending = true
	if got := m.imCartChip(); !strings.Contains(got, spinFrames[m.frame%len(spinFrames)]) {
		t.Fatalf("chip must carry the spinner while a write is pending; got %q", got)
	}
	m.imCartSyncPending = false
	m.imSyncInFlight = true
	if got := m.imCartChip(); !strings.Contains(got, spinFrames[m.frame%len(spinFrames)]) {
		t.Fatalf("chip must carry the spinner while a write is in flight; got %q", got)
	}
	m.imSyncInFlight = false
	if got := m.imCartChip(); strings.Contains(got, spinFrames[m.frame%len(spinFrames)]) {
		t.Fatalf("settled chip must drop the spinner; got %q", got)
	}
}

// If Swiggy keeps disagreeing with the local lines (server-side clamp), the
// deferred-confirm chase must stop after its budget and ADOPT the server cart
// instead of writing forever — the write chain can never loop.
func TestIMConfirmChaseBudgetAdoptsServerCart(t *testing.T) {
	be := &liveFake{}
	m := imModel(t, be)
	m.imLines = []screens.CartLine{
		{Item: catalog.Item{ID: "p1", SwiggyID: "sp1", Name: "Red Bull", Price: 125, Section: catalog.SectionInstamart}, Qty: 7, Price: 125},
	}
	m.screen = scrCheckout
	m.checkoutVertical = 1
	m.checkout = m.buildIMCheckout()

	// Swiggy clamps to 5 no matter what we send.
	clamped := api.IMCart{ItemTotal: 625, Total: 630,
		Lines: []api.IMCartLine{{SpinID: "sp1", Name: "Red Bull", Quantity: 5, Price: 125, Available: true}}}
	be.imCart = clamped
	m.imLiveCart = api.IMCart{} // stale — forces the Enter gate to flush

	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if !m.imConfirmPending || cmd == nil {
		t.Fatal("Enter with a stale cart must defer and flush")
	}
	// Each delivery returns the clamped cart; the chase must stop within its
	// budget (2 re-fires) and never spin.
	for i := 0; i < 4 && cmd != nil; i++ {
		m = deliver(t, m, cmd)
		cmd = nil
		if m.imConfirmPending && m.imSyncInFlight {
			// the handler re-fired — synthesize the next clamped response
			cmd = func() tea.Msg { return datasource.IMCartSyncedMsg{Cart: clamped} }
		}
	}
	if m.imConfirmPending {
		t.Fatal("chase must terminate within its budget")
	}
	if m.orderConfirmOpen {
		t.Fatal("a clamped cart must NOT open the confirm modal — user re-reviews")
	}
	if len(m.imLines) != 1 || m.imLines[0].Qty != 5 {
		t.Fatalf("terminal state must ADOPT the server cart (qty 5), got %+v", m.imLines)
	}
	if m.imOrderErr == "" {
		t.Fatal("the adjustment must be surfaced to the user")
	}
}
