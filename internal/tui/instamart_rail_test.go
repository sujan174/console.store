package tui

// Tests for the Instamart rail: navigation/debounce mirrors the Food rail
// exactly (armIMRailLoad/settledIMRailLoad), category loads dedupe against
// the snapshot (ensureIMQuery/imLoadedQueries), and the rail's Search/Usuals
// entries route into search mode / the go-to list.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"consolestore/internal/broker/api"
	"consolestore/internal/catalog"
	swiggysnap "consolestore/internal/catalog/swiggy"
	"consolestore/internal/tui/render"
	"consolestore/internal/tui/screens"
)

// imRailModel builds a live model already on scrInstamart with the rail
// focused on Usuals (the state imEnterCmd leaves it in), ready for key-driven
// rail tests.
func imRailModel(t *testing.T, be *liveFake) Model {
	t.Helper()
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	m := New(render.Caps{}, WithLiveBackend(be, snap, "acct-1", ""), WithSeededSnapshot())
	m.w, m.h = 100, 40
	m.addr = catalog.Address{ID: "a1", Label: "home", Line: "HSR Layout"}
	m.screen = scrMenu
	m.railFocus = true
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = nm.(Model)
	m = deliver(t, m, cmd)
	return m
}

// TestIMRailShowsEnergyDrinksFirst: entering Instamart renders the rail with
// "Energy Drinks" as the first curated category (after Search/Usuals).
func TestIMRailShowsEnergyDrinksFirst(t *testing.T) {
	be := &liveFake{}
	m := imRailModel(t, be)
	if m.screen != scrInstamart {
		t.Fatalf("screen = %v, want scrInstamart", m.screen)
	}
	v := m.inst.View()
	for _, want := range []string{"INSTAMART", "Usuals", "Energy Drinks"} {
		if !strings.Contains(v, want) {
			t.Fatalf("instamart rail view missing %q:\n%s", want, v)
		}
	}
	if len(m.imChips) == 0 || m.imChips[0].Label != "Energy Drinks" {
		t.Fatalf("imChips[0] = %+v, want Energy Drinks first", m.imChips)
	}
}

// TestIMEnterLandsOnUsualsWithRailFocus: imEnterCmd starts on Usuals with the
// rail focused, matching Food's landing-on-Home.
func TestIMEnterLandsOnUsualsWithRailFocus(t *testing.T) {
	be := &liveFake{}
	m := imRailModel(t, be)
	if !m.imRailFocus {
		t.Fatal("entering Instamart must land with the rail focused")
	}
	if m.imRailActive != screens.RailHome {
		t.Fatalf("imRailActive = %d, want RailHome (Usuals slot)", m.imRailActive)
	}
	if m.imQuery != "" {
		t.Fatalf("imQuery = %q, want empty (Usuals/go-to list)", m.imQuery)
	}
}

// TestIMRailArrowToCategoryDebouncesThenFires: arrowing the rail to a
// category doesn't fire immediately — it arms a pending load that only fires
// once the cursor settles (onTick), mirroring the Food rail's debounce.
func TestIMRailArrowToCategoryDebouncesThenFires(t *testing.T) {
	be := &liveFake{imSearch: []api.IMProduct{
		{ID: "p1", Name: "Red Bull", InStock: true,
			Variants: []api.IMVariantSel{{SpinID: "sp1", Label: "250 ml", Price: 110, InStock: true}}},
	}}
	m := imRailModel(t, be)

	// Usuals → Energy Drinks (one down-arrow).
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = nm.(Model)
	if cmd != nil {
		t.Fatal("arrowing the rail must not fire a load immediately (debounced)")
	}
	if !m.imRailSettlePending {
		t.Fatal("arrowing the rail must arm a pending IM rail load")
	}

	var loadCmd tea.Cmd
	for i := 0; i < railSettleFrames+2 && loadCmd == nil; i++ {
		m.frame++
		var c tea.Cmd
		m, c = m.onTick()
		loadCmd = c
	}
	if loadCmd == nil {
		t.Fatal("settled debounce must fire the IM category load")
	}
	if m.imQuery != "energy drink" {
		t.Fatalf("imQuery = %q, want energy drink", m.imQuery)
	}
	m = deliver(t, m, loadCmd)
	if be.imSearchCalls != 1 {
		t.Fatalf("IMSearch call count = %d, want 1", be.imSearchCalls)
	}
	if be.imSearchQuery != "energy drink" {
		t.Fatalf("backend received query %q, want energy drink", be.imSearchQuery)
	}
	v := m.inst.View()
	if !strings.Contains(v, "Red Bull") {
		t.Fatalf("category results must render; got:\n%s", v)
	}
}

// TestIMRailRevisitCategoryRendersFromSnapshot: revisiting a previously
// loaded category renders instantly from the snapshot — no second IMSearch
// call. Instamart skips disk caching (in-memory snapshot only), so this dedupe
// is purely the imLoadedQueries session guard (mirrors ensureQuery's
// live-loaded dedupe for Food).
func TestIMRailRevisitCategoryRendersFromSnapshot(t *testing.T) {
	be := &liveFake{imSearch: []api.IMProduct{
		{ID: "p1", Name: "Lays Chips", InStock: true,
			Variants: []api.IMVariantSel{{SpinID: "sp1", Label: "52 g", Price: 20, InStock: true}}},
	}}
	m := imRailModel(t, be)

	// Enter loads the category immediately (no settle wait needed).
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown}) // Energy Drinks
	m = nm.(Model)
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // Chips
	m = nm.(Model)
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if cmd == nil {
		t.Fatal("Enter on a rail category must load immediately")
	}
	m = deliver(t, m, cmd)
	if be.imSearchCalls != 1 {
		t.Fatalf("IMSearch call count after first visit = %d, want 1", be.imSearchCalls)
	}

	// Leave to Usuals, then come back to Chips — must render from the
	// snapshot with NO second IMSearch call.
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft}) // back to rail focus (list had focus after Enter)
	m = nm.(Model)
	// Re-focus the rail explicitly in case Enter already dropped focus there.
	m.imRailFocus = true
	m.imRailActive = screens.RailHome
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // Energy Drinks
	m = nm.(Model)
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // Chips
	m = nm.(Model)
	nm, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if cmd != nil {
		m = deliver(t, m, cmd)
	}
	if be.imSearchCalls != 1 {
		t.Fatalf("IMSearch call count after revisit = %d, want still 1 (cache hit)", be.imSearchCalls)
	}
	v := m.inst.View()
	if !strings.Contains(v, "Lays Chips") {
		t.Fatalf("revisited category must render from snapshot instantly; got:\n%s", v)
	}
}

// TestIMRailSearchEntryEntersSearchMode: the rail's Search entry (index 0)
// enters search mode on Enter, mirroring Food's rail Search entry.
func TestIMRailSearchEntryEntersSearchMode(t *testing.T) {
	be := &liveFake{}
	m := imRailModel(t, be)

	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp}) // Usuals → Search
	m = nm.(Model)
	if m.imRailActive != screens.RailSearch {
		t.Fatalf("imRailActive = %d, want RailSearch", m.imRailActive)
	}
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if !m.imSearchMode {
		t.Fatal("Enter on the rail Search entry must enter Instamart search mode")
	}
}

// TestIMRailUsualsEntryReturnsToGoToList: from a category, selecting Usuals
// on the rail returns to the go-to list (imQuery reset to ""). The go-to list
// was already loaded once on entry (imEnterCmd), so re-selecting it dedupes
// like Food's Home entry (ensureHomeLoaded) — no second fetch, just an
// instant render from the snapshot already holding the go-to items.
func TestIMRailUsualsEntryReturnsToGoToList(t *testing.T) {
	be := &liveFake{imGoTo: []api.IMProduct{{ID: "p0", Name: "Bread", InStock: true,
		Variants: []api.IMVariantSel{{SpinID: "sp0", Label: "400 g", Price: 45, InStock: true}}}}}
	m := imRailModel(t, be)
	if be.imGoToCalls != 1 {
		t.Fatalf("precondition: entering Instamart must fetch the go-to list once, got %d", be.imGoToCalls)
	}
	m.imQuery = "energy drink" // simulate being on a category
	m.imRailActive = 2         // first category slot

	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp}) // category → Usuals
	m = nm.(Model)
	if m.imRailActive != screens.RailHome {
		t.Fatalf("imRailActive = %d, want RailHome (Usuals)", m.imRailActive)
	}
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if m.imQuery != "" {
		t.Fatalf("imQuery = %q, want empty after selecting Usuals", m.imQuery)
	}
	if cmd != nil {
		m = deliver(t, m, cmd)
	}
	if be.imGoToCalls != 1 {
		t.Fatalf("IMGoTo call count = %d, want still 1 (session dedupe, like ensureHomeLoaded)", be.imGoToCalls)
	}
	v := m.inst.View()
	if !strings.Contains(v, "Bread") {
		t.Fatalf("Usuals must render the go-to list from the snapshot; got:\n%s", v)
	}
}
