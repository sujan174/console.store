package screens

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/catalog"
	"console.store/internal/catalog/mem"
)

func TestMenuRendersPlacesAndUsual(t *testing.T) {
	repo := mem.New()
	addr := repo.Addresses()[0]
	places := repo.Places(addr, catalog.SectionCoffee)
	usual, ok := repo.Usual(addr)
	m := NewMenu(places, addr, catalog.SectionCoffee, usual, ok, 338)
	out := m.View()
	if !strings.Contains(out, "the usual") {
		t.Fatal("missing the usual pin")
	}
	if !strings.Contains(out, "Blue Tokai") || !strings.Contains(out, "35-45 min") {
		t.Fatal("missing places/eta")
	}
	if !strings.Contains(out, "coffee") {
		t.Fatal("missing category tabs")
	}
}

func TestMenuSearchFiltersList(t *testing.T) {
	places := []catalog.Place{
		{ID: "blue-tokai", Name: "Blue Tokai", ETA: "35-45 min"},
		{ID: "third-wave", Name: "Third Wave", ETA: "30-40 min"},
	}
	m := NewMenu(places, catalog.Address{Line: "HSR"}, catalog.SectionCoffee, catalog.Usual{}, false, 0)
	for _, r := range []rune{'/', 'w', 'a', 'v', 'e'} {
		key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		nm, _ := m.Update(key)
		m = nm.(Menu)
	}
	view := m.View()
	if strings.Contains(view, "Blue Tokai") {
		t.Errorf("Blue Tokai should be filtered out:\n%s", view)
	}
	if !strings.Contains(view, "Third Wave") {
		t.Errorf("Third Wave should remain:\n%s", view)
	}
}

func TestMenuEnterSelectsRestaurant(t *testing.T) {
	repo := mem.New()
	addr := repo.Addresses()[0]
	places := repo.Places(addr, catalog.SectionCoffee)
	usual, ok := repo.Usual(addr)
	m := NewMenu(places, addr, catalog.SectionCoffee, usual, ok, 338)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	mm := m2.(Menu)
	if mm.list.Cursor != 1 {
		t.Fatalf("down should move cursor to 1, got %d", mm.list.Cursor)
	}
	got, ok := mm.Selected()
	if !ok {
		t.Fatal("Selected() returned ok=false")
	}
	if got.Name != "Third Wave" {
		t.Fatalf("Selected() = %s, want Third Wave", got.Name)
	}
}
