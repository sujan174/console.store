package screens

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"consolestore/internal/catalog"
	"consolestore/internal/catalog/mem"
)

func TestMenuHeaderShowsAddressAndCart(t *testing.T) {
	repo := mem.New()
	addr := repo.Addresses()[0]
	places := repo.Places(addr, catalog.SectionCoffee)
	usual, ok := repo.Usual(addr)
	m := NewMenu(places, addr, catalog.SectionCoffee, usual, ok, "cart · ₹338")
	out := m.View()
	// The brand logo now lives in the root banner, not the menu header; the menu
	// header shows the delivery address.
	if !strings.Contains(out, "deliver to") || !strings.Contains(out, addr.Line) {
		t.Fatalf("missing delivery address header:\n%s", out)
	}
	if !strings.Contains(out, "cart · ₹338") {
		t.Fatalf("missing cart total:\n%s", out)
	}
}

func TestMenuNoUsualHidesUsualLine(t *testing.T) {
	places := []catalog.Place{{ID: "x", Name: "X", ETA: "10 min"}}
	m := NewMenu(places, catalog.Address{Line: "HSR"}, catalog.SectionCoffee, catalog.Usual{}, false, "")
	if strings.Contains(m.View(), "the usual") {
		t.Fatal("usual line should be hidden when hasUsual is false")
	}
}

func TestMenuShowsThreeTabs(t *testing.T) {
	repo := mem.New()
	addr := repo.Addresses()[0]
	places := repo.Places(addr, catalog.SectionCoffee)
	usual, ok := repo.Usual(addr)
	m := NewMenu(places, addr, catalog.SectionCoffee, usual, ok, "")
	out := m.View()
	for _, tab := range []string{"coffee", "food", "snacks"} {
		if !strings.Contains(out, tab) {
			t.Fatalf("missing tab %q:\n%s", tab, out)
		}
	}
	if strings.Contains(out, "instamart") {
		t.Fatalf("menu should not show an instamart tab:\n%s", out)
	}
}

func TestMenuPlacesOnly(t *testing.T) {
	repo := mem.New()
	addr := repo.Addresses()[0]
	places := repo.Places(addr, catalog.SectionCoffee)
	usual, ok := repo.Usual(addr)
	m := NewMenu(places, addr, catalog.SectionCoffee, usual, ok, "")
	// cursor starts on the first place (no usual row offset)
	got, ok := m.Selected()
	if !ok {
		t.Fatal("Selected() returned ok=false at cursor 0")
	}
	if got.Name != places[0].Name {
		t.Fatalf("Selected() = %s, want %s", got.Name, places[0].Name)
	}
}

func TestMenuSearchFiltersList(t *testing.T) {
	places := []catalog.Place{
		{ID: "blue-tokai", Name: "Blue Tokai", ETA: "35-45 min"},
		{ID: "third-wave", Name: "Third Wave", ETA: "30-40 min"},
	}
	m := NewMenu(places, catalog.Address{Line: "HSR"}, catalog.SectionCoffee, catalog.Usual{}, false, "")
	for _, r := range []rune{'/', 'w', 'a', 'v', 'e'} {
		key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		nm, _ := m.Update(key)
		m = nm.(Menu)
	}
	view := m.View()
	if strings.Contains(despace(view), "BlueTokai") {
		t.Errorf("Blue Tokai should be filtered out:\n%s", view)
	}
	if !strings.Contains(despace(view), "ThirdWave") {
		t.Errorf("Third Wave should remain:\n%s", view)
	}
}

func TestMenuEnterSelectsRestaurant(t *testing.T) {
	repo := mem.New()
	addr := repo.Addresses()[0]
	places := repo.Places(addr, catalog.SectionCoffee)
	usual, ok := repo.Usual(addr)
	m := NewMenu(places, addr, catalog.SectionCoffee, usual, ok, "cart · ₹338")
	// cursor starts on the first place (Blue Tokai); Down moves to Third Wave.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	mm := m2.(Menu)
	got, ok := mm.Selected()
	if !ok {
		t.Fatal("Selected() returned ok=false")
	}
	if got.Name != "Third Wave" {
		t.Fatalf("Selected() = %s, want Third Wave", got.Name)
	}
}

func TestMenuRendersChips(t *testing.T) {
	m := NewMenu(
		[]catalog.Place{{ID: "r1", Name: "Blue Tokai"}},
		catalog.Address{ID: "a1", Label: "home"},
		catalog.SectionCoffee, catalog.Usual{}, false, "",
	).WithChips([]string{"Coffee & Refreshments", "Pizza", "Burgers"}, 1)

	v := m.View()
	for _, want := range []string{"Coffee & Refreshments", "Pizza", "Burgers", "Blue Tokai"} {
		if !strings.Contains(v, want) {
			t.Errorf("browse view missing %q", want)
		}
	}
	// vertical-toggle row must appear when chips are set (live mode)
	if !strings.Contains(v, "Restaurants") {
		t.Errorf("browse view missing vertical-toggle active label %q", "Restaurants")
	}
	if !strings.Contains(v, "Instamart") {
		t.Errorf("browse view missing vertical-toggle inactive label %q", "Instamart")
	}
	if !strings.Contains(v, "soon") {
		t.Errorf("browse view missing vertical-toggle soon marker")
	}
}

func liveMenu() Menu {
	return NewMenu(nil, catalog.Address{Line: "Home"}, catalog.SectionFood, catalog.Usual{}, false, "🛒 empty").
		WithRail(NewRail([]string{"Coffee", "Pizza"})).WithRailFocus(false).WithMaxRows(20)
}

func TestMenuTwoPaneHomeSections(t *testing.T) {
	m := liveMenu().WithSections(
		[]catalog.Place{{Name: "Blue Tokai", ETA: "25 min"}},
		[]catalog.Place{{Name: "Pizza Hut", ETA: "20 min"}},
	)
	v := m.View()
	for _, want := range []string{"⌕", "Home", "your usuals", "Blue Tokai", "popular near you", "Pizza Hut"} {
		if !strings.Contains(v, want) {
			t.Errorf("home view missing %q:\n%s", want, v)
		}
	}
}

func TestMenuUsualsOmittedWhenEmpty(t *testing.T) {
	m := liveMenu().WithSections(nil, []catalog.Place{{Name: "Pizza Hut"}})
	if strings.Contains(m.View(), "your usuals") {
		t.Errorf("empty usuals must omit the section:\n%s", m.View())
	}
}

func TestMenuSearchModeResults(t *testing.T) {
	m := liveMenu().WithSearchMode(true, "pizza", []catalog.Place{{Name: "Pizza Hut"}}, 1, false)
	v := m.View()
	if !strings.Contains(v, "pizza") || !strings.Contains(v, "Pizza Hut") || !strings.Contains(v, "1 result") {
		t.Errorf("search view missing query/results/count:\n%s", v)
	}
}

func TestMenuSearchPendingShowsSearching(t *testing.T) {
	m := liveMenu().WithSearchMode(true, "blue", nil, 0, true)
	v := m.View()
	if !strings.Contains(v, "searching…") {
		t.Errorf("pending search must show a searching cue:\n%s", v)
	}
	if strings.Contains(v, `no restaurants for`) {
		t.Errorf("must NOT show no-results while a search is pending:\n%s", v)
	}
}

func TestMenuSearchNoResults(t *testing.T) {
	m := liveMenu().WithSearchMode(true, "xyz", nil, 0, false)
	if !strings.Contains(m.View(), `no restaurants for "xyz"`) {
		t.Errorf("empty-results copy missing:\n%s", m.View())
	}
}

func TestMenuMockPaneUnchanged(t *testing.T) {
	// No rail set → the existing single-pane mock render (section tabs present).
	m := NewMenu([]catalog.Place{{Name: "Blue Tokai"}}, catalog.Address{Line: "Home"},
		catalog.SectionCoffee, catalog.Usual{}, false, "🛒 empty")
	v := m.View()
	if !strings.Contains(v, "coffee") || strings.Contains(v, "your usuals") {
		t.Errorf("mock pane must be unchanged (tabs, no rail sections):\n%s", v)
	}
}

func TestMenuCategoryShowsHeader(t *testing.T) {
	m := liveMenu().WithCategoryHeader("Coffee")
	m.places = []catalog.Place{{Name: "Blue Tokai"}}
	if v := m.View(); !strings.Contains(v, "Coffee") {
		t.Fatalf("category flat list missing its section header:\n%s", v)
	}
}

func TestBrowseDetailStripShowsFocused(t *testing.T) {
	m := liveMenu().WithSections(nil, []catalog.Place{
		{Name: "Blue Tokai", Rating: 4.7, ETA: "30 MINS", City: "HSR"},
	})
	v := m.View()
	for _, want := range []string{"Blue Tokai", "★ 4.7", "30 MINS", "HSR"} {
		if !strings.Contains(v, want) {
			t.Fatalf("focused-restaurant detail strip missing %q:\n%s", want, v)
		}
	}
}

func TestBrowseRowsHideRatingOutsideSearch(t *testing.T) {
	// Strip uses "★ 4.7" (spaced); the per-row compact form "★4.7" must be absent.
	m := liveMenu().WithSections(nil, []catalog.Place{{Name: "Blue Tokai", Rating: 4.7, ETA: "30 MINS"}})
	if strings.Contains(m.View(), "★4.7") {
		t.Fatalf("browse rows must not carry per-row rating outside search:\n%s", m.View())
	}
}

func TestSearchRowsKeepRating(t *testing.T) {
	m := liveMenu().WithSearchMode(true, "blue", []catalog.Place{{Name: "Blue Tokai", Rating: 4.7, ETA: "30 MINS"}}, 1, false)
	if !strings.Contains(m.View(), "★4.7") {
		t.Fatalf("search results should keep per-row rating:\n%s", m.View())
	}
}

func TestTwoPaneShowsStoreSwitcher(t *testing.T) {
	v := liveMenu().WithSections(nil, []catalog.Place{{Name: "Blue Tokai"}}).View()
	for _, want := range []string{"FOOD", "Instamart", "soon", "tab", "switch"} {
		if !strings.Contains(v, want) {
			t.Fatalf("two-pane store switcher missing %q:\n%s", want, v)
		}
	}
}

func TestTwoPaneShowsNavHint(t *testing.T) {
	v := liveMenu().WithSections(nil, []catalog.Place{{Name: "Blue Tokai"}}).View()
	for _, want := range []string{"move", "open", "search", "cart", "cmd"} {
		if !strings.Contains(v, want) {
			t.Fatalf("two-pane browse missing nav hint %q:\n%s", want, v)
		}
	}
}
