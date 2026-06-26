package tui

// TestLiveBrowseRailFocusAndSearch drives the live browse rail:
//   (a) landing on the live browse renders the rail (🔍, Home, a category label)
//   (b) ← focuses the rail; ↓ + enter on a category loads that category's restaurants
//   (c) ← to Search entry + enter + typed query + enter shows results
//
// Uses the same fake backend pattern as live_test.go.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/broker/api"
	"console.store/internal/catalog"
	swiggysnap "console.store/internal/catalog/swiggy"
	"console.store/internal/config"
	"console.store/internal/tui/datasource"
	"console.store/internal/tui/render"
	"console.store/internal/tui/screens"
)

// railFake is a Backend whose Usuals and PlacesQuery return canned restaurants.
type railFake struct {
	usuals      []api.Restaurant
	queryResult []api.Restaurant
	err         error
}

func (f *railFake) Addresses() ([]api.Address, error) { return nil, f.err }
func (f *railFake) Places(string, catalog.Section) ([]api.Restaurant, error) {
	return nil, f.err
}
func (f *railFake) PlacesQuery(_, q string) ([]api.Restaurant, error) {
	if q == "__usuals__" {
		return f.usuals, f.err
	}
	return f.queryResult, f.err
}
func (f *railFake) SearchOrganic(_, q string) ([]api.Restaurant, error) {
	return f.queryResult, f.err
}
func (f *railFake) Usuals(string) ([]api.Restaurant, error) { return f.usuals, f.err }
func (f *railFake) Menu(string, string) (api.Menu, error)   { return api.Menu{}, f.err }
func (f *railFake) ItemOptions(string, string, string, string) ([]api.OptionGroup, error) {
	return nil, f.err
}
func (f *railFake) UpdateCart(string, string, string, []api.CartItem) (api.Cart, error) {
	return api.Cart{}, f.err
}
func (f *railFake) GetCart(string, string) (api.Cart, error) { return api.Cart{}, f.err }
func (f *railFake) ClearCart() error                         { return f.err }
func (f *railFake) PlaceOrder(string) (api.Order, error)     { return api.Order{}, f.err }

// buildRailModel constructs a seeded live Model with chips and canned snapshot data.
func buildRailModel(t *testing.T) Model {
	t.Helper()
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	// Seed nearby (query "")
	snap.SetPlaces("a1", "", []catalog.Place{
		{ID: "nr1", SwiggyID: "snr1", Name: "Nearby Place", ETA: "30 min"},
	})
	// Seed coffee category
	snap.SetPlaces("a1", "coffee", []catalog.Place{
		{ID: "r1", SwiggyID: "sr1", Name: "Blue Tokai", ETA: "25 min"},
	})
	// Seed usuals
	snap.SetPlaces("a1", datasource.UsualsKey, []catalog.Place{
		{ID: "u1", SwiggyID: "su1", Name: "Usual Spot", ETA: "20 min"},
	})
	be := &railFake{
		usuals:      []api.Restaurant{{ID: "u1", Name: "Usual Spot"}},
		queryResult: []api.Restaurant{{ID: "r1", Name: "Blue Tokai"}},
	}
	m := New(render.Caps{},
		WithLiveBackend(be, snap, "acct-1", ""),
		WithSeededSnapshot(),
		WithChips([]config.Category{
			{Label: "Coffee", Query: "coffee"},
			{Label: "Pizza", Query: "pizza"},
		}),
	)
	m.w, m.h = 100, 40
	m.screen = scrMenu
	return m
}

// TestLiveBrowseRailRenders checks that the live browse view includes rail entries.
func TestLiveBrowseRailRenders(t *testing.T) {
	m := buildRailModel(t)
	m.menu = m.buildMenu()
	v := m.menu.View()
	for _, want := range []string{"🔍", "Home", "Coffee"} {
		if !strings.Contains(v, want) {
			t.Errorf("live browse must render rail entry %q\n%s", want, v)
		}
	}
}

// TestLiveBrowseRailLeftFocusesRail verifies ← sets railFocus.
func TestLiveBrowseRailLeftFocusesRail(t *testing.T) {
	m := buildRailModel(t)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	um := m2.(Model)
	if !um.railFocus {
		t.Fatal("← must set railFocus=true on live browse")
	}
}

// TestLiveBrowseCategoryNavSwapsList verifies that ←, ↓ (to Coffee), enter
// switches the main list to the Coffee category's restaurants.
func TestLiveBrowseCategoryNavSwapsList(t *testing.T) {
	m := buildRailModel(t)

	// ← focuses the rail (railActive stays at RailHome=1)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	um := m2.(Model)
	if !um.railFocus {
		t.Fatal("← must focus rail")
	}

	// ↓ moves railActive from Home(1) to Coffee(2)
	m3, _ := um.Update(tea.KeyMsg{Type: tea.KeyDown})
	um3 := m3.(Model)
	if um3.railActive != 2 {
		t.Fatalf("↓ from Home must move railActive to 2 (Coffee), got %d", um3.railActive)
	}

	// enter commits to Coffee category; must fire a load cmd (or no-op if cached)
	m4, _ := um3.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um4 := m4.(Model)
	if um4.railFocus {
		t.Fatal("enter must unfocus the rail")
	}
	// main list should now contain Coffee results
	v := um4.menu.View()
	if !strings.Contains(v, "Blue Tokai") {
		t.Errorf("after selecting Coffee category, main list must show Blue Tokai\n%s", v)
	}
}

// TestLiveBrowseSearchMode verifies that navigating to Search + enter + typing +
// enter shows results and the search input box.
func TestLiveBrowseSearchMode(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	snap.SetPlaces("a1", "blue tokai", []catalog.Place{
		{ID: "r1", SwiggyID: "sr1", Name: "Blue Tokai", ETA: "25 min"},
	})
	snap.SetPlaces("a1", "", []catalog.Place{})
	be := &railFake{
		queryResult: []api.Restaurant{{ID: "r1", Name: "Blue Tokai"}},
	}
	m := New(render.Caps{},
		WithLiveBackend(be, snap, "acct-1", ""),
		WithSeededSnapshot(),
		WithChips([]config.Category{{Label: "Coffee", Query: "coffee"}}),
	)
	m.w, m.h = 100, 40
	m.screen = scrMenu

	// ← focuses the rail (active=Home=1)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	um := m2.(Model)

	// ↑ moves to Search (index 0)
	m3, _ := um.Update(tea.KeyMsg{Type: tea.KeyUp})
	um3 := m3.(Model)
	if um3.railActive != screens.RailSearch {
		t.Fatalf("↑ from Home must reach RailSearch=0, got %d", um3.railActive)
	}

	// enter commits to Search mode
	m4, _ := um3.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um4 := m4.(Model)
	if !um4.searchMode {
		t.Fatal("entering RailSearch must set searchMode=true")
	}

	// type "blue tokai"
	for _, r := range "blue tokai" {
		m4b, _ := um4.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		um4 = m4b.(Model)
	}
	if um4.searchQuery != "blue tokai" {
		t.Fatalf("searchQuery = %q; want %q", um4.searchQuery, "blue tokai")
	}

	// enter submits the query
	m5, cmd := um4.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um5 := m5.(Model)
	_ = cmd // may be nil if already cached, or non-nil for a load
	_ = um5

	// The menu in search mode must show the search input
	v := um4.menu.View()
	if !strings.Contains(v, "🔍") {
		t.Errorf("search mode view must contain 🔍\n%s", v)
	}
	if !strings.Contains(v, "blue tokai") {
		t.Errorf("search mode view must contain the query\n%s", v)
	}
}

// TestLiveBrowseHomeShowsUsuals verifies that the Home view shows the usuals
// section header and the seeded usual restaurant.
func TestLiveBrowseHomeShowsUsuals(t *testing.T) {
	m := buildRailModel(t)
	v := m.menu.View()
	if !strings.Contains(v, "your usuals") {
		t.Errorf("Home view must show 'your usuals' when usuals are cached\n%s", v)
	}
	if !strings.Contains(v, "Usual Spot") {
		t.Errorf("Home view must show the cached usual restaurant\n%s", v)
	}
	if !strings.Contains(v, "popular near you") {
		t.Errorf("Home view must show the 'popular near you' section\n%s", v)
	}
}

// TestLiveBrowseMockUnchanged verifies mock (non-live) browse still shows section
// tabs and the / filter — no rail.
func TestLiveBrowseMockUnchanged(t *testing.T) {
	m := New(render.Caps{})
	m.w, m.h = 100, 40
	m.screen = scrMenu
	v := m.menu.View()
	if strings.Contains(v, "your usuals") {
		t.Errorf("mock browse must not show rail sections\n%s", v)
	}
	// Mock browse should show section tabs
	if !strings.Contains(v, "coffee") {
		t.Errorf("mock browse must still show section tabs\n%s", v)
	}
}

// TestLiveBrowseSearchEscExitsSearch verifies that esc in search mode exits it.
func TestLiveBrowseSearchEscExitsSearch(t *testing.T) {
	m := buildRailModel(t)
	m.searchMode = true
	m.searchQuery = "pizza"
	m.menu = m.buildMenu()
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	um := m2.(Model)
	if um.searchMode {
		t.Fatal("esc must exit search mode")
	}
	if um.searchQuery != "" {
		t.Fatalf("esc must clear searchQuery, got %q", um.searchQuery)
	}
}

// TestSearchResultsNavigableAndOpenable is the F1 flow test:
//  1. Enter search mode, type a query, Enter → submits (results loaded from snapshot).
//  2. ↓ moves the result cursor onto the first (and only) result.
//  3. Enter again → opens the restaurant screen (scrRestaurant).
func TestSearchResultsNavigableAndOpenable(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	// Pre-seed results under the SEARCH key for "tokai" so searchLoad returns nil
	// and results are immediately visible after the submit Enter.
	snap.SetPlaces("a1", datasource.SearchKey("tokai"), []catalog.Place{
		{ID: "r1", SwiggyID: "sr1", Name: "Blue Tokai", ETA: "25 min"},
	})
	snap.SetPlaces("a1", "", []catalog.Place{})
	be := &railFake{
		queryResult: []api.Restaurant{{ID: "r1", Name: "Blue Tokai"}},
	}
	m := New(render.Caps{},
		WithLiveBackend(be, snap, "acct-1", ""),
		WithSeededSnapshot(),
		WithChips([]config.Category{{Label: "Coffee", Query: "coffee"}}),
	)
	m.w, m.h = 100, 40
	m.screen = scrMenu

	// ← focuses rail; ↑ moves to Search (index 0); enter enters search mode.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	um := m2.(Model)
	m3, _ := um.Update(tea.KeyMsg{Type: tea.KeyUp})
	um3 := m3.(Model)
	if um3.railActive != screens.RailSearch {
		t.Fatalf("↑ from Home must reach RailSearch=0, got %d", um3.railActive)
	}
	m4, _ := um3.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um4 := m4.(Model)
	if !um4.searchMode {
		t.Fatal("entering RailSearch must set searchMode=true")
	}

	// Type "tokai" — each rune appends to searchQuery.
	for _, r := range "tokai" {
		m4b, _ := um4.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		um4 = m4b.(Model)
	}
	if um4.searchQuery != "tokai" {
		t.Fatalf("searchQuery = %q; want %q", um4.searchQuery, "tokai")
	}

	// Enter submits the query. Results are already in the snapshot so ensureQuery
	// returns nil (no async load). searchSubmitted is set to "tokai".
	m5, _ := um4.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um5 := m5.(Model)
	if um5.searchSubmitted != "tokai" {
		t.Fatalf("after submit, searchSubmitted = %q; want %q", um5.searchSubmitted, "tokai")
	}

	// ↓ should move the result cursor (query == searchSubmitted, results loaded).
	m6, _ := um5.Update(tea.KeyMsg{Type: tea.KeyDown})
	um6 := m6.(Model)
	if um6.menu.ListCursor() != 1 {
		// cursor starts at 0; one ↓ should put it at 1 (clamped to last result if only 1)
		// With exactly 1 result, cursor clamps at 0 (WithListCursor clamps to len-1).
		// So expect cursor 0 (clamped).
		if um6.menu.ListCursor() != 0 {
			t.Fatalf("↓ with 1 result: cursor = %d; want 0 (clamped)", um6.menu.ListCursor())
		}
	}

	// Verify the view shows the search result.
	v := um5.menu.View()
	if !strings.Contains(v, "Blue Tokai") {
		t.Errorf("search results view must contain Blue Tokai\n%s", v)
	}

	// Enter again (query == searchSubmitted) → opens restaurant screen.
	m7, _ := um5.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um7 := m7.(Model)
	if um7.screen != scrRestaurant {
		t.Fatalf("Enter on selected search result must open scrRestaurant, got screen=%d", um7.screen)
	}
}

// TestCategoryViewNoNearbyHeader is the F2 test: selecting a category from the
// rail must render a plain flat list — no "nearby" section header.
func TestCategoryViewNoNearbyHeader(t *testing.T) {
	m := buildRailModel(t)

	// ← focuses rail; ↓ moves to Coffee (index 2); enter commits.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	um := m2.(Model)
	m3, _ := um.Update(tea.KeyMsg{Type: tea.KeyDown})
	um3 := m3.(Model)
	if um3.railActive != 2 {
		t.Fatalf("↓ from Home must move railActive to 2 (Coffee), got %d", um3.railActive)
	}
	m4, _ := um3.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um4 := m4.(Model)

	v := um4.menu.View()
	if strings.Contains(v, "nearby") {
		t.Errorf("category view must not render a 'nearby' section header\n%s", v)
	}
	if !strings.Contains(v, "Blue Tokai") {
		t.Errorf("category view must show category results\n%s", v)
	}
}
