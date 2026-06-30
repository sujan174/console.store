package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"consolestore/internal/catalog"
	"consolestore/internal/tui/screens"
)

// Mashing + on the checkout page must NOT fire an update_food_cart per keystroke
// — that would spray write-tool calls past Swiggy's 30/min write cap. Each press
// updates the cart optimistically and (re)arms a single debounced sync that
// fires once the keys settle.
func TestCheckoutIncrementDebouncesSync(t *testing.T) {
	m := checkoutModel(t)

	// Three quick +'s: qty climbs locally, no sync cmd, one pending sync armed.
	for i := 0; i < 3; i++ {
		u, cmd := m.Update(keyRunes("+"))
		m = u.(Model)
		if cmd != nil {
			t.Fatalf("+ #%d should not sync immediately (debounced)", i+1)
		}
		if !m.cartSyncPending {
			t.Fatalf("+ #%d should arm a pending sync", i+1)
		}
	}
	if m.lines[0].Qty != 5 { // started at 2
		t.Fatalf("three +'s should bump qty 2→5 optimistically, got %d", m.lines[0].Qty)
	}

	// Tick past the settle window — the single debounced sync fires once.
	var fired tea.Cmd
	for i := 0; i < cartSettleFrames+2; i++ {
		u, _ := m.Update(tickMsg(time.Now()))
		m = u.(Model)
		if c := m.settledCartSync(); c != nil { // drained already by onTick; this re-checks state
			fired = c
		}
	}
	if m.cartSyncPending {
		t.Fatal("pending sync should clear after the keys settle")
	}
	_ = fired
}

// A settled debounce must actually produce the live sync cmd (not silently drop
// it). Drive onTick directly and assert the cmd surfaces once pending clears.
func TestCartDebounceFiresSyncOnSettle(t *testing.T) {
	m := checkoutModel(t)
	u, _ := m.Update(keyRunes("+"))
	m = u.(Model)
	if !m.cartSyncPending {
		t.Fatal("precondition: + must arm a pending sync")
	}

	// Before the settle window elapses, onTick must not fire the sync.
	for i := 0; i < cartSettleFrames-1; i++ {
		m.frame++
		nm, cmd := m.onTick()
		m = nm
		if cmd != nil {
			t.Fatalf("tick %d: sync must not fire before settle", i)
		}
		if !m.cartSyncPending {
			t.Fatal("pending sync cleared too early")
		}
	}

	// The settling tick fires the sync exactly once and clears the flag.
	m.frame++
	nm, cmd := m.onTick()
	m = nm
	if cmd == nil {
		t.Fatal("settled debounce must fire the live sync cmd")
	}
	if m.cartSyncPending {
		t.Fatal("pending flag must clear once the sync fires")
	}
}

// A freeze-path reduce already serializes one sync at a time (cartMutating), so
// the debounce must NOT pile a second sync on top while one is in flight.
func TestCartDebounceSkipsWhileMutating(t *testing.T) {
	m := checkoutModel(t)
	m.cartSyncPending = true
	m.cartSyncFrame = m.frame
	m.cartMutating = true

	for i := 0; i < cartSettleFrames+2; i++ {
		m.frame++
		nm, cmd := m.onTick()
		m = nm
		if cmd != nil {
			t.Fatal("debounce must not fire a sync while a freeze-path sync is in flight")
		}
	}
	if m.cartSyncPending {
		t.Fatal("pending flag should still clear (consumed), just without firing")
	}
}

// Menu-list + (add) must debounce its sync too, not fire one update_food_cart
// per add keystroke.
func TestMenuAddDebouncesSync(t *testing.T) {
	m := menuAddModel(t)
	u, cmd := m.Update(keyRunes("+"))
	m = u.(Model)
	if cmd != nil {
		t.Fatal("menu + must debounce the sync, not fire immediately")
	}
	if !m.cartSyncPending {
		t.Fatal("menu + must arm a pending sync")
	}
	if got := m.qtyMap()["i1"]; got != 1 {
		t.Fatalf("menu + should add the item optimistically (qty 1), got %d", got)
	}
}

// menuAddModel is a live restaurant screen with a single simple (non-customizable)
// item selected, ready for a + add.
func menuAddModel(t *testing.T) Model {
	t.Helper()
	m := checkoutModel(t) // reuses the seeded Blue Tokai / Latte snapshot + live backend
	m.lines = nil
	m.cartRestaurant = ""
	place := catalog.Place{ID: "bt1", Name: "Blue Tokai", SwiggyID: "swiggy-bt1", Section: catalog.SectionCoffee,
		Items: []catalog.Item{{ID: "i1", Name: "Latte", Price: 250, SwiggyID: "swiggy-i1"}}}
	m.screen = scrRestaurant
	m.rest = screens.NewRestaurant(place, m.qtyMap(), m.cartChip()).WithAddr(m.addr)
	return m
}
