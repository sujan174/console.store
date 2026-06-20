package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestAppStartsOnMenu(t *testing.T) {
	m := New()
	out := m.View()
	if !strings.Contains(out, "console.store") || !strings.Contains(out, "Blue Tokai") {
		t.Fatal("app should start on menu with places")
	}
}

func TestAppEnterOpensRestaurantThenEscBack(t *testing.T) {
	m := New()
	// enter on first place
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(m2.View(), "35-45 min") {
		t.Fatal("enter should open restaurant view")
	}
	// esc back to menu
	m3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !strings.Contains(m3.View(), "the usual") {
		t.Fatal("esc should return to menu")
	}
}

func TestAppQuits(t *testing.T) {
	m := New()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl-c should return a quit command")
	}
}
