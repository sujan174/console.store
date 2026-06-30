package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"consolestore/internal/catalog"
	swiggysnap "consolestore/internal/catalog/swiggy"
	"consolestore/internal/config"
	"consolestore/internal/tui/render"
	"consolestore/internal/tui/screens"
)

func liveRailModel() Model {
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "acct-1", "https://authz/x"))
	m.live = true
	m.screen = scrMenu
	m.railFocus = true
	m.railActive = screens.RailHome
	m.chips = []config.Category{{Label: "Coffee", Query: "coffee"}, {Label: "Pizza", Query: "pizza"}}
	m.addr = catalog.Address{ID: "a1"}
	return m
}

// Arrowing through the rail must NOT fire a search per step — it arms a pending
// load that only fires once the cursor settles (onTick), so fast-scrolling can't
// spray search_restaurants calls at Swiggy.
func TestRailNavDebouncesCategoryLoads(t *testing.T) {
	m := liveRailModel()

	// Two quick arrow-downs (Home → Coffee → Pizza) — neither fires a load.
	for _, k := range []string{"j", "j"} {
		u, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
		m = u.(Model)
		if cmd != nil {
			t.Fatalf("rail %q should not fire a load immediately (debounced)", k)
		}
		if !m.railSettlePending {
			t.Fatalf("rail %q should arm a pending load", k)
		}
	}

	// Tick past the settle window — the debounced load then fires once. (Update
	// always returns a non-nil cmd because of the next tick(), so we assert on
	// state, not the cmd: pending clears and catPending flips on the fired load.)
	for i := 0; i < railSettleFrames+2; i++ {
		u, _ := m.Update(tickMsg(time.Now()))
		m = u.(Model)
	}
	if m.railSettlePending {
		t.Fatal("pending load should clear after the cursor settles")
	}
	if !m.catPending {
		t.Fatal("a settled rail cursor on an uncached category should fire its load (catPending)")
	}
}

// Enter on a rail category loads immediately (explicit pick), not on settle.
func TestRailEnterLoadsImmediately(t *testing.T) {
	m := liveRailModel()
	m = func() Model { u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}); return u.(Model) }() // Home → Coffee
	if !m.railSettlePending {
		t.Fatal("precondition: arrow should arm a pending load")
	}
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if m.railSettlePending {
		t.Fatal("Enter should cancel the debounce and load immediately")
	}
}
