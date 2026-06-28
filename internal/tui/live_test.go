package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/broker/api"
	"console.store/internal/catalog"
	swiggysnap "console.store/internal/catalog/swiggy"
	"console.store/internal/config"
	"console.store/internal/localstore"
	"console.store/internal/tui/datasource"
	"console.store/internal/tui/render"
	"console.store/internal/tui/screens"
)

type liveFake struct {
	addrs []api.Address
	err   error
}

func (f *liveFake) Addresses() ([]api.Address, error) { return f.addrs, f.err }
func (f *liveFake) Places(string, catalog.Section) ([]api.Restaurant, error) {
	return nil, f.err
}
func (f *liveFake) PlacesQuery(string, string) ([]api.Restaurant, error) { return nil, f.err }
func (f *liveFake) SearchOrganic(string, string) ([]api.Restaurant, string, error) {
	return nil, "", f.err
}
func (f *liveFake) Usuals(string) ([]api.Restaurant, error) { return nil, f.err }
func (f *liveFake) Menu(string, string) (api.Menu, error)   { return api.Menu{}, f.err }
func (f *liveFake) ItemOptions(string, string, string, string) ([]api.OptionGroup, error) {
	return nil, f.err
}
func (f *liveFake) UpdateCart(string, string, string, []api.CartItem) (api.Cart, error) {
	return api.Cart{}, f.err
}
func (f *liveFake) GetCart(string, string) (api.Cart, error) { return api.Cart{}, f.err }
func (f *liveFake) ClearCart() error                         { return f.err }
func (f *liveFake) PlaceOrder(string) (api.Order, error)     { return api.Order{}, f.err }
func (f *liveFake) TrackOrder(string) (api.Tracking, error)  { return api.Tracking{}, f.err }
func (f *liveFake) ActiveOrders(string) ([]api.Order, error) { return nil, f.err }
func (f *liveFake) OrderHistory(string) ([]api.Order, error) { return nil, f.err }

func TestMockPathUnaffected(t *testing.T) {
	m := New(render.Caps{})
	if m.live {
		t.Fatal("default New must not be live")
	}
	if len(m.repo.Addresses()) == 0 {
		t.Fatal("mock repo should have seed addresses")
	}
}

func TestLiveAddressesMsgAdoptsAddress(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	be := &liveFake{addrs: []api.Address{{ID: "live-1", Label: "home"}}}
	m := New(render.Caps{}, WithLiveBackend(be, snap, "acct-1", "https://authz/x"))
	if !m.live {
		t.Fatal("expected live model")
	}
	snap.SetAddresses([]catalog.Address{{ID: "live-1", Label: "home"}})
	updated, _ := m.Update(datasource.AddressesLoadedMsg{})
	if updated.(Model).addr.ID != "live-1" {
		t.Fatalf("model did not adopt live address: %+v", updated.(Model).addr)
	}
}

func TestLiveNeedsAuthOnAuthError(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "acct-1", "https://authz/x"))
	updated, _ := m.Update(datasource.AddressesLoadedMsg{Err: datasource.ErrNeedsAuth})
	if !updated.(Model).needsAuth {
		t.Fatal("expected needsAuth after ErrNeedsAuth load")
	}
}

func TestLiveMenuEnterFiresLoadMenu(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	// Home's "popular near you" reads the first category's query ("coffee").
	snap.SetPlaces("a1", "coffee", []catalog.Place{
		{ID: "r1", SwiggyID: "swiggy-r1", Name: "Blue Tokai", Section: catalog.SectionCoffee},
	})
	be := &liveFake{}
	m := New(render.Caps{},
		WithLiveBackend(be, snap, "acct-1", ""),
		WithSeededSnapshot(),
		WithChips([]config.Category{{Label: "Coffee", Query: "coffee"}}),
	)
	// Force window size so menu renders.
	m.w, m.h = 100, 40
	// Navigate to the menu screen (model starts at scrSplash).
	m.screen = scrMenu
	// Focus the main list (the browse now lands on the rail; → moves to the list).
	m.railFocus = false

	// Simulate pressing enter on the menu (restaurant is first in the Home list).
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um := updated.(Model)
	if um.screen != scrRestaurant {
		t.Fatalf("screen = %v after enter; want scrRestaurant", um.screen)
	}
	if cmd == nil {
		t.Fatal("live mode: enter on menu must return a non-nil LoadMenu Cmd")
	}
}

func TestSeededPathSkipsLiveLoads(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	// Pre-seed the snapshot with an address (simulates what sshd does from config).
	snap.SetAddresses([]catalog.Address{{ID: "seed-addr", Label: "home"}})
	be := &liveFake{}
	m := New(render.Caps{},
		WithLiveBackend(be, snap, "acct-1", "https://authz/x"),
		WithSeededSnapshot(),
	)
	if !m.seeded {
		t.Fatal("expected seeded=true")
	}
	// addr should be picked up from the seeded snapshot, not the mock fallback.
	if m.addr.ID != "seed-addr" {
		t.Fatalf("addr.ID = %q; want seed-addr", m.addr.ID)
	}
	// Init() should not return a batch that includes LoadAddresses/LoadPlaces when seeded.
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init must return tick() even when seeded")
	}
	// Seeded skips re-fetching ADDRESSES, but still loads the Home view (usuals +
	// the popular list) for the seeded address — otherwise the browse is empty.
	if c := m.liveInitCmds(); c == nil {
		t.Fatal("seeded liveInitCmds must load Home (usuals + popular) for the seeded address")
	}
}

func TestLiveCartSyncFires(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	place := catalog.Place{
		ID: "r1", SwiggyID: "swiggy-r1", Name: "Blue Tokai",
		Section: catalog.SectionCoffee,
		Items:   []catalog.Item{{ID: "i1", SwiggyID: "swiggy-i1", Name: "Latte", Price: 250, Veg: true}},
	}
	snap.SetPlaces("a1", string(catalog.SectionCoffee), []catalog.Place{place})
	snap.SetMenu(place)
	be := &liveFake{}
	m := New(render.Caps{},
		WithLiveBackend(be, snap, "acct-1", ""),
		WithSeededSnapshot(),
	)
	m.w, m.h = 100, 40

	// Skip to scrRestaurant with the restaurant loaded.
	m.screen = scrRestaurant
	m.rest = screens.NewRestaurant(place, m.qtyMap(), m.cartChip()).WithAddr(m.addr)

	// Add an item — in live mode with SwiggyID set, must return a SyncCart cmd.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("adding item in live mode must return a SyncCart cmd")
	}
}

// TestCartPlaceIDResolvesNonCoffeeChip is the regression for "Pizza Hut didn't
// sync": cartPlaceID resolved the cart restaurant only across the fixed mock
// sections, so a restaurant opened under a chip whose query != a section name
// (e.g. "pizza") resolved to "" and cart sync silently no-op'd.
func TestCartPlaceIDResolvesNonCoffeeChip(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	// Pizza Hut lives under the "pizza" chip query — NOT a mock section key.
	ph := catalog.Place{ID: "ph1", SwiggyID: "ph1", Name: "Pizza Hut"}
	snap.SetPlaces("a1", "pizza", []catalog.Place{ph})
	snap.SetMenu(catalog.Place{ID: "ph1", SwiggyID: "ph1", Items: []catalog.Item{
		{ID: "p1", SwiggyID: "p1", Name: "Margherita", Price: 300},
	}})
	be := &liveFake{}
	m := New(render.Caps{},
		WithLiveBackend(be, snap, "acct-1", ""),
		WithSeededSnapshot(),
		WithChips([]config.Category{{Label: "Pizza", Query: "pizza"}}),
	)
	m.w, m.h = 100, 40
	m.cartRestaurant = "Pizza Hut"

	if got := m.cartPlaceID(); got != "ph1" {
		t.Fatalf("cartPlaceID for a pizza-chip restaurant = %q, want \"ph1\"", got)
	}
}

func TestLivePlaceOrderTransitionsToConfirm(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	be := &liveFake{}
	m := New(render.Caps{},
		WithLiveBackend(be, snap, "acct-1", ""),
		WithSeededSnapshot(),
	)
	// Put model on scrCheckout with a line in the cart.
	m.screen = scrCheckout
	m.lines = []screens.CartLine{{Item: catalog.Item{ID: "i1", Name: "Latte", Price: 250}, Qty: 1}}
	m.cartRestaurant = "Blue Tokai"
	m.checkout = screens.NewCheckout("Blue Tokai", m.addr, m.lines, "~35 min")

	// Press enter → should set placingOrder=true and return a PlaceOrderCmd.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um := updated.(Model)
	if !um.placingOrder {
		t.Fatal("expected placingOrder=true after checkout enter in live mode")
	}
	if cmd == nil {
		t.Fatal("expected PlaceOrderCmd to be returned")
	}

	// Simulate OrderPlacedMsg success.
	updated2, _ := um.Update(datasource.OrderPlacedMsg{
		Order: api.Order{ID: "order-99", Status: "placed"},
	})
	um2 := updated2.(Model)
	if um2.screen != scrConfirm {
		t.Fatalf("screen = %v after OrderPlacedMsg; want scrConfirm", um2.screen)
	}
	if um2.placingOrder {
		t.Fatal("placingOrder must be cleared after success")
	}
}

func TestRailCategoryFiresQuery(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	be := &liveFake{}
	m := New(render.Caps{},
		WithLiveBackend(be, snap, "acct-1", ""),
		WithSeededSnapshot(),
		WithChips([]config.Category{{Label: "Coffee", Query: "coffee"}, {Label: "Pizza", Query: "pizza"}}),
	)
	m.w, m.h = 100, 40
	m.screen = scrMenu
	// ← focuses the rail
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	um2 := m2.(Model)
	if !um2.railFocus {
		t.Fatal("← must focus the rail")
	}
	// ↓ moves rail to Coffee (index 2, first category)
	m3, _ := um2.Update(tea.KeyMsg{Type: tea.KeyDown})
	um3 := m3.(Model)
	// Enter commits to that category and must fire a LoadPlacesQuery cmd
	m4, cmd := um3.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = m4
	if cmd == nil {
		t.Fatal("selecting a category on the rail must fire a places query")
	}
}

func TestLivePlaceOrderErrShowsError(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	be := &liveFake{}
	m := New(render.Caps{},
		WithLiveBackend(be, snap, "acct-1", ""),
		WithSeededSnapshot(),
	)
	m.screen = scrCheckout
	m.lines = []screens.CartLine{{Item: catalog.Item{ID: "i1", Name: "Latte", Price: 250}, Qty: 1}}
	m.placingOrder = true

	updated, _ := m.Update(datasource.OrderPlacedMsg{
		Err: errors.New("order failed: restaurant closed"),
	})
	um := updated.(Model)
	if um.screen != scrCheckout {
		t.Fatalf("screen = %v after error; want scrCheckout", um.screen)
	}
	if um.placingOrder {
		t.Fatal("placingOrder must be cleared after error")
	}
	if um.orderErr == "" {
		t.Fatal("orderErr must be set after PlaceOrder error")
	}
}

func (f *liveFake) Logout() error { return f.err }

// TestStaleMenuLoadIgnored is the regression for the cross-restaurant race:
// open A (slow load) → open B before A lands → A's late MenuLoadedMsg must NOT
// overwrite B's screen/menu. Before the fix, A's items were merged onto B's
// identity, so adding a dish sent A's item to B's cart and Swiggy rejected it.
func TestStaleMenuLoadIgnored(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	rA := catalog.Place{ID: "A", SwiggyID: "A", Name: "Restaurant A", Section: catalog.SectionCoffee,
		Items: []catalog.Item{{ID: "ai", SwiggyID: "ai", Name: "Aitem", Price: 100}}}
	rB := catalog.Place{ID: "B", SwiggyID: "B", Name: "Restaurant B", Section: catalog.SectionCoffee,
		Items: []catalog.Item{{ID: "bi", SwiggyID: "bi", Name: "Bitem", Price: 200}}}
	snap.SetMenu(rA)
	snap.SetMenu(rB)
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "acct-1", ""), WithSeededSnapshot())
	m.w, m.h = 100, 40

	// Now viewing B (opened second; A's load is still in flight).
	m.screen = scrRestaurant
	m.rest = screens.NewRestaurant(rB, m.qtyMap(), m.cartChip()).WithAddr(m.addr)

	// A's slow MenuLoadedMsg lands late — must be dropped as stale.
	nm, _ := m.Update(datasource.MenuLoadedMsg{PlaceID: "A"})
	m = nm.(Model)

	got := m.rest.PlaceData()
	if got.SwiggyID != "B" {
		t.Fatalf("stale A load swapped restaurant identity to %q, want B", got.SwiggyID)
	}
	if len(got.Items) == 0 || got.Items[0].Name != "Bitem" {
		t.Fatalf("stale A load injected wrong menu into B: items=%+v", got.Items)
	}
}

// TestCartChipShowsLiveGrandTotal: once Swiggy's real cart is known, the chip
// (shown on every page) reflects the true line count + grand total including
func checkoutModel(t *testing.T) Model {
	t.Helper()
	snap := swiggysnap.NewSnapshot()
	addr := catalog.Address{ID: "a1", Label: "home"}
	place := catalog.Place{ID: "bt1", Name: "Blue Tokai", SwiggyID: "swiggy-bt1", Section: catalog.SectionCoffee}
	snap.SetAddresses([]catalog.Address{addr})
	snap.SetPlaces("a1", string(catalog.SectionCoffee), []catalog.Place{place})
	snap.SetMenu(catalog.Place{ID: "bt1", Name: "Blue Tokai", SwiggyID: "swiggy-bt1",
		Items: []catalog.Item{{ID: "i1", Name: "Latte", Price: 250, SwiggyID: "swiggy-i1"}}})
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "acct-1", ""), WithSeededSnapshot())
	m.w, m.h = 100, 40
	m.screen = scrCheckout
	m.cartRestaurant = "Blue Tokai"
	m.lines = []screens.CartLine{
		{Item: catalog.Item{ID: "i1", Name: "Latte", Price: 250, SwiggyID: "swiggy-i1"}, Qty: 2},
	}
	m.checkout = m.buildCheckout()
	return m
}

func TestCheckoutIncrementOptimistic(t *testing.T) {
	m := checkoutModel(t)
	nm, cmd := m.Update(keyRunes("+"))
	m = nm.(Model)
	if m.lines[0].Qty != 3 {
		t.Fatalf("+ should bump qty to 3, got %d", m.lines[0].Qty)
	}
	if m.cartMutating {
		t.Fatal("+ is optimistic and must NOT freeze input")
	}
	if cmd == nil {
		t.Fatal("+ must return a sync cmd")
	}
}

func TestCheckoutReduceFreezesUntilSynced(t *testing.T) {
	m := checkoutModel(t)
	// − reduces qty and freezes.
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("-")})
	m = nm.(Model)
	if m.lines[0].Qty != 1 {
		t.Fatalf("− should reduce qty to 1, got %d", m.lines[0].Qty)
	}
	if !m.cartMutating {
		t.Fatal("− must freeze input (cartMutating) until confirmed")
	}
	if cmd == nil {
		t.Fatal("− must return a sync cmd")
	}
	// While frozen, another key is ignored.
	nm, _ = m.Update(keyRunes("+"))
	m2 := nm.(Model)
	if m2.lines[0].Qty != 1 {
		t.Fatalf("input must be frozen while mutating; qty changed to %d", m2.lines[0].Qty)
	}
	// Sync confirmation clears the freeze.
	nm, _ = m.Update(datasource.CartSyncedMsg{})
	m = nm.(Model)
	if m.cartMutating {
		t.Fatal("CartSyncedMsg must clear cartMutating")
	}
}

func TestCheckoutEnterBlockedWhileMutating(t *testing.T) {
	m := checkoutModel(t)
	m.cartMutating = true
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if m.placingOrder {
		t.Fatal("enter must be a no-op while mutating")
	}
	if cmd != nil {
		t.Fatal("enter must not start the place sequence while mutating")
	}
}

func TestCheckoutCursorClampsAfterRemove(t *testing.T) {
	m := checkoutModel(t)
	// Two lines; cursor on the second (last).
	m.lines = append(m.lines, screens.CartLine{Item: catalog.Item{ID: "i2", Name: "Mocha", Price: 300}, Qty: 1})
	m.checkout = m.buildCheckout().WithCursor(1)
	// Delete the last line -> list shrinks to 1, cursor 1 now out of range.
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyDelete})
	m = nm.(Model)
	// Confirm the reduce sync so the freeze clears and checkout rebuilds.
	nm, _ = m.Update(datasource.CartSyncedMsg{})
	m = nm.(Model)
	if c := m.checkout.Cursor(); c != 0 {
		t.Fatalf("cursor must clamp to 0 after removing the last line, got %d", c)
	}
	// A subsequent + must act on the remaining line (not be swallowed by the bounds guard).
	nm, _ = m.Update(keyRunes("+"))
	m = nm.(Model)
	if m.lines[0].Qty != 3 {
		t.Fatalf("+ after remove must increment the remaining line; qty=%d", m.lines[0].Qty)
	}
}

// TestCheckoutReduceMockNoFreeze guards the mock-mode hard-lock: a reduce with
// no live backend must NOT freeze the screen (no CartSyncedMsg would ever
// arrive to clear it), so esc still works afterward.
func TestCheckoutReduceMockNoFreeze(t *testing.T) {
	m := New(render.Caps{}) // mock mode (not live)
	m.w, m.h = 100, 40
	m.screen = scrCheckout
	m.lines = []screens.CartLine{
		{Item: catalog.Item{ID: "i1", Name: "Latte", Price: 250}, Qty: 2},
	}
	m.checkout = m.buildCheckout()

	nm, _ := m.Update(keyRunes("-"))
	m = nm.(Model)
	if m.cartMutating {
		t.Fatal("mock-mode reduce must NOT freeze (no sync to confirm it)")
	}
	if m.lines[0].Qty != 1 {
		t.Fatalf("mock reduce should drop qty to 1, got %d", m.lines[0].Qty)
	}
	// esc must still work — not swallowed by a stuck freeze.
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = nm.(Model)
	if m.screen != scrMenu {
		t.Fatalf("esc after mock reduce must leave checkout; screen=%v", m.screen)
	}
}

// delivery + taxes, not the local item subtotal.
func TestCartChipShowsLiveGrandTotal(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "acct-1", ""), WithSeededSnapshot())
	m.liveCart = api.Cart{
		ItemTotal: 250, Delivery: 29, Taxes: 18, Total: 297,
		Lines: []api.CartLine{{ItemID: "i1", Name: "Latte", Quantity: 2, Price: 125}},
	}
	chip := m.cartChip()
	if !strings.Contains(chip, "297") {
		t.Fatalf("chip should show grand total 297, got %q", chip)
	}
	if !strings.Contains(chip, "2") {
		t.Fatalf("chip should show live line count 2, got %q", chip)
	}
}

// TestPlaceSavesActiveOrderAndConfirms verifies that OrderPlacedMsg persists the
// active order to disk and transitions the model to scrConfirm.
func TestPlaceSavesActiveOrderAndConfirms(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := checkoutModel(t)

	nm, _ := m.Update(datasource.OrderPlacedMsg{
		Order: api.Order{ID: "X1", Restaurant: "Blue Tokai", ETA: "55-65 mins", Total: 386},
	})
	m = nm.(Model)
	if m.screen != scrConfirm {
		t.Fatalf("screen=%v, want scrConfirm", m.screen)
	}
	if !m.hasActiveOrder {
		t.Fatal("hasActiveOrder must be true after OrderPlacedMsg")
	}
	if m.activeOrder.OrderID != "X1" {
		t.Fatalf("activeOrder.OrderID=%q, want X1", m.activeOrder.OrderID)
	}
	if _, ok, _ := localstore.LoadActiveOrder(); !ok {
		t.Fatal("active order not saved to disk")
	}
}

// TestTrackingPollSkippedWhenBackendNil verifies that the onTick auto-advance
// from scrConfirm to scrTracking still happens when backend is nil (mock/safestore),
// but does NOT call PollTrackingCmd (which would nil-pointer panic).
func TestTrackingPollSkippedWhenBackendNil(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := New(render.Caps{}) // mock mode: backend is nil
	m.hasActiveOrder = true
	m.activeOrder = localstore.ActiveOrder{OrderID: "X1", Restaurant: "Blue Tokai", ETAHiMin: 40, PlacedAt: 1}
	m.screen = scrConfirm
	m.confirmTick = 42
	// onTick must NOT panic and must NOT return a poll cmd when backend is nil.
	nm, cmd := m.Update(tickMsg(time.Now()))
	m = nm.(Model)
	if m.screen != scrTracking {
		t.Fatalf("should still advance to tracking, screen=%v", m.screen)
	}
	_ = cmd // must not have panicked
}

// TestConfirmAutoAdvancesToTracking verifies that enough ticks on scrConfirm
// automatically advance the model to scrTracking.
func TestConfirmAutoAdvancesToTracking(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	be := &liveFake{}
	m := New(render.Caps{},
		WithLiveBackend(be, snap, "acct-1", ""),
		WithSeededSnapshot(),
	)
	m.screen = scrConfirm
	m.hasActiveOrder = true
	m.activeOrder = localstore.ActiveOrder{
		OrderID:    "ord-99",
		Restaurant: "Blue Tokai",
		AddrLine:   "HSR",
		ETALoMin:   35,
		ETAHiMin:   45,
		PlacedAt:   time.Now().Unix(),
	}
	m.confirmTick = 0

	for i := 0; i < 60; i++ {
		nm, _ := m.Update(tickMsg(time.Now()))
		m = nm.(Model)
	}
	if m.screen != scrTracking {
		t.Fatalf("auto-advance failed, screen=%v (want scrTracking)", m.screen)
	}
}
