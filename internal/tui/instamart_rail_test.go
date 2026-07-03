package tui

// Tests for the Instamart rail: navigation/debounce mirrors the Food rail
// exactly (armIMRailLoad/settledIMRailLoad), category loads dedupe against
// the snapshot (ensureIMQuery/imLoadedQueries), and the rail's Search entry
// routes into search mode. The IM rail is Home-less: it lands straight on the
// first product category (Energy Drinks) — there is no Usuals/go-to list.

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
// focused on the first category (the state imEnterCmd leaves it in), ready for
// key-driven rail tests.
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
// "Energy Drinks" as the first (landed) category, and NO Usuals slot.
func TestIMRailShowsEnergyDrinksFirst(t *testing.T) {
	be := &liveFake{}
	m := imRailModel(t, be)
	if m.screen != scrInstamart {
		t.Fatalf("screen = %v, want scrInstamart", m.screen)
	}
	v := m.inst.View()
	for _, want := range []string{"INSTAMART", "Energy Drinks"} {
		if !strings.Contains(v, want) {
			t.Fatalf("instamart rail view missing %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "Usuals") {
		t.Fatalf("instamart rail must not show a Usuals slot:\n%s", v)
	}
	if len(m.imChips) == 0 || m.imChips[0].Label != "Energy Drinks" {
		t.Fatalf("imChips[0] = %+v, want Energy Drinks first", m.imChips)
	}
}

// TestIMEnterLandsOnFirstCategory: imEnterCmd lands on the first category with
// the rail focused, its query loaded — no go-to/Usuals landing.
func TestIMEnterLandsOnFirstCategory(t *testing.T) {
	be := &liveFake{}
	m := imRailModel(t, be)
	if !m.imRailFocus {
		t.Fatal("entering Instamart must land with the rail focused")
	}
	if want := m.imRail().CatBase(); m.imRailActive != want {
		t.Fatalf("imRailActive = %d, want %d (first category)", m.imRailActive, want)
	}
	if m.imQuery != "energy drink" {
		t.Fatalf("imQuery = %q, want energy drink (first category)", m.imQuery)
	}
	if be.imSearchQuery != "energy drink" {
		t.Fatalf("entry must load the first category; backend query = %q", be.imSearchQuery)
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

	// Landed on Energy Drinks; arrow down to Chips (one down-arrow).
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
	if m.imQuery != "chips" {
		t.Fatalf("imQuery = %q, want chips", m.imQuery)
	}
	m = deliver(t, m, loadCmd)
	if be.imSearchQuery != "chips" {
		t.Fatalf("backend received query %q, want chips", be.imSearchQuery)
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
	// Entry already loaded the landed category (Energy Drinks) once.
	if be.imSearchCalls != 1 {
		t.Fatalf("entry must load the first category once, got %d", be.imSearchCalls)
	}

	// Down to Chips, Enter → its first (and only) load.
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown}) // Chips
	m = nm.(Model)
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if cmd == nil {
		t.Fatal("Enter on a rail category must load immediately")
	}
	m = deliver(t, m, cmd)
	if be.imSearchCalls != 2 {
		t.Fatalf("IMSearch calls after Chips = %d, want 2 (energy drink + chips)", be.imSearchCalls)
	}

	// Back to the rail, up to Energy Drinks (already loaded), down to Chips
	// again — must render from the snapshot with NO third IMSearch call.
	m.imRailFocus = true
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp}) // Chips → Energy Drinks
	m = nm.(Model)
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // Energy Drinks → Chips
	m = nm.(Model)
	nm, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if cmd != nil {
		m = deliver(t, m, cmd)
	}
	if be.imSearchCalls != 2 {
		t.Fatalf("IMSearch calls after revisit = %d, want still 2 (cache hit)", be.imSearchCalls)
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

	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp}) // first category → Search
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

// TestIMRailHasNoUsualsSlot: the rail is Home-less — up from the landed first
// category reaches Search directly (no Usuals slot in between), and IMGoTo is
// never called.
func TestIMRailHasNoUsualsSlot(t *testing.T) {
	be := &liveFake{imGoTo: []api.IMProduct{{ID: "p0", Name: "Bread", InStock: true,
		Variants: []api.IMVariantSel{{SpinID: "sp0", Label: "400 g", Price: 45, InStock: true}}}}}
	m := imRailModel(t, be)
	if be.imGoToCalls != 0 {
		t.Fatalf("the Home-less IM rail must never fetch the go-to list, got %d calls", be.imGoToCalls)
	}
	// Landed on the first category (index CatBase); one Up must reach Search.
	if want := m.imRail().CatBase(); m.imRailActive != want {
		t.Fatalf("imRailActive = %d, want %d (first category)", m.imRailActive, want)
	}
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = nm.(Model)
	if m.imRailActive != screens.RailSearch {
		t.Fatalf("one Up from the first category must land on Search (no Usuals slot); got %d", m.imRailActive)
	}
}
