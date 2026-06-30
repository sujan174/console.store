package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func key(m Model, s string) Model {
	var msg tea.KeyMsg
	switch s {
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
	u, _ := m.Update(msg)
	return u.(Model)
}

// ? and H open the help modal from a normal screen; closing it returns to the
// screen behind it.
func TestHelpOpensAndCloses(t *testing.T) {
	m := newAtMenu()
	m.w, m.h = 100, 30

	m = key(m, "?")
	if !m.helpOpen {
		t.Fatal("? should open the help modal")
	}
	if v := m.View(); !strings.Contains(v, "powered by Swiggy") || !strings.Contains(v, "esc esc") {
		t.Fatalf("help view should show the intro + controls:\n%s", v)
	}
	m = key(m, "esc")
	if m.helpOpen {
		t.Fatal("esc should close the help modal")
	}

	if m = key(m, "H"); !m.helpOpen {
		t.Fatal("H should also open help")
	}
	if m = key(m, "?"); m.helpOpen {
		t.Fatal("? should toggle help closed")
	}
}

// ? must NOT open help while typing in a search box — it's real input there.
func TestHelpSuppressedWhileTyping(t *testing.T) {
	m := newAtMenu()
	m.searchMode = true
	if m.helpTriggerable() {
		t.Error("help must not trigger while a search box is active")
	}
	m.searchMode = false
	m.cmdOpen = true
	if m.helpTriggerable() {
		t.Error("help must not trigger while the command palette is open")
	}
}
