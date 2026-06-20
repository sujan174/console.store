package screens

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"console.store/internal/mock"
)

func TestMenuRendersPlacesAndUsual(t *testing.T) {
	m := NewMenu(mock.Restaurants, mock.Addresses[0], 338)
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

func TestMenuEnterSelectsRestaurant(t *testing.T) {
	m := NewMenu(mock.Restaurants, mock.Addresses[0], 338)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	mm := m2.(Menu)
	if mm.list.Cursor != 1 {
		t.Fatalf("down should move cursor to 1, got %d", mm.list.Cursor)
	}
	if got := mm.Selected(); got.Name != "Third Wave" {
		t.Fatalf("Selected() = %s, want Third Wave", got.Name)
	}
}
