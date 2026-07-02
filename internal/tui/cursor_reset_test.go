package tui

import (
	"fmt"
	"testing"

	"consolestore/internal/broker/api"
	"consolestore/internal/catalog"
	swiggysnap "consolestore/internal/catalog/swiggy"
	"consolestore/internal/config"
	"consolestore/internal/tui/datasource"
	"consolestore/internal/tui/render"
	"consolestore/internal/tui/screens"
)

// The "jump back to the top" bug: with a category open and the user scrolled
// down, a LATE async load landing (usuals from launch, another category's
// debounced load, a streamed page) rebuilt the menu screen and reset the list
// cursor to 0 — yanking the user back to the top seconds into scrolling.
// Data refreshes must preserve the cursor; only real view switches reset it.

// cursorModel builds a live model on scrMenu with a category open (12 places
// cached) and the cursor scrolled down to row 5.
func cursorModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	var places []catalog.Place
	for i := 0; i < 12; i++ {
		places = append(places, catalog.Place{ID: fmt.Sprintf("p%d", i), SwiggyID: fmt.Sprintf("p%d", i), Name: fmt.Sprintf("Cafe %d", i)})
	}
	chips := config.DefaultCategories()
	snap.SetPlaces("a1", chips[0].Query, places)

	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "acct-1", ""), WithChips(chips))
	m.addr = catalog.Address{ID: "a1", Label: "home"}
	m.screen = scrMenu
	m.railActive = screens.RailHome + 1 // first category entry (railCatBase)
	m.railFocus = false                 // focus in the list, scrolling
	m.menu = m.buildMenu().WithListCursor(5)
	if m.menu.ListCursor() != 5 {
		t.Fatalf("setup: cursor = %d, want 5", m.menu.ListCursor())
	}
	return m
}

func TestLateUsualsLoadKeepsListCursor(t *testing.T) {
	m := cursorModel(t)
	out, _ := m.Update(datasource.UsualsLoadedMsg{})
	m = out.(Model)
	if got := m.menu.ListCursor(); got != 5 {
		t.Fatalf("cursor after late UsualsLoadedMsg = %d, want 5 (must not jump to top)", got)
	}
}

func TestLatePlacesLoadKeepsListCursor(t *testing.T) {
	m := cursorModel(t)
	// A different query's results land (e.g. Home's popular list finishing late).
	out, _ := m.Update(datasource.PlacesLoadedMsg{Query: "unrelated"})
	m = out.(Model)
	if got := m.menu.ListCursor(); got != 5 {
		t.Fatalf("cursor after late PlacesLoadedMsg = %d, want 5 (must not jump to top)", got)
	}
}

func TestStreamedPageKeepsListCursor(t *testing.T) {
	m := cursorModel(t)
	out, _ := m.Update(datasource.PlacesPageLoadedMsg{Query: "unrelated", Page: 1, Gen: m.placesGen, Done: true})
	m = out.(Model)
	if got := m.menu.ListCursor(); got != 5 {
		t.Fatalf("cursor after streamed page = %d, want 5 (must not jump to top)", got)
	}
}

func TestAddressBackendNotNeededForCursorTests(t *testing.T) {
	// Guard: the fixture's fake returns empty lists everywhere; the cursor
	// tests above rely only on the seeded snapshot. This keeps the fixture
	// honest if liveFake ever changes.
	var _ datasource.Backend = &liveFake{}
	_ = api.Address{}
}
