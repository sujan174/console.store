package screens

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/catalog"
	"console.store/internal/catalog/mem"
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

func TestBrandBannerShowsLogoAndVersion(t *testing.T) {
	out := BrandBanner(80)
	if !strings.Contains(out, "consolestore.in") {
		t.Errorf("brand banner should show the wordmark:\n%s", out)
	}
	if !strings.Contains(out, Version) {
		t.Errorf("brand banner should show the version %q:\n%s", Version, out)
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

// itoa is a tiny int-to-string helper for tests (avoids importing strconv twice).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf []byte
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
