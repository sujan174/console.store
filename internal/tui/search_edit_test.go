package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	swiggysnap "console.store/internal/catalog/swiggy"
	"console.store/internal/config"
	"console.store/internal/tui/render"
)

// searchModel returns a live Model parked in search mode with the given query
// and caret, ready to receive an edit key.
func searchModel(query string, caret int) Model {
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "local", ""))
	m.screen = scrMenu
	m.chips = []config.Category{{Label: "Pizza", Query: "pizza"}}
	m.searchMode = true
	m.railFocus = false
	m.searchQuery = query
	m.searchCaret = caret
	return m
}

func TestSearchDeleteRemovesRuneAtCaret(t *testing.T) {
	m := searchModel("abc", 1) // caret between 'a' and 'b'
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyDelete})
	got := out.(Model)
	if got.searchQuery != "ac" {
		t.Fatalf("forward-delete: query = %q, want %q", got.searchQuery, "ac")
	}
	if got.searchCaret != 1 {
		t.Fatalf("forward-delete: caret = %d, want 1 (unchanged)", got.searchCaret)
	}
}

func TestSearchDeleteAtEndIsNoOp(t *testing.T) {
	m := searchModel("abc", 3) // caret at end
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyDelete})
	got := out.(Model)
	if got.searchQuery != "abc" {
		t.Fatalf("delete at end should be no-op: query = %q, want %q", got.searchQuery, "abc")
	}
}

func TestSearchBackspaceStillDeletesBeforeCaret(t *testing.T) {
	m := searchModel("abc", 2) // caret between 'b' and 'c'
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	got := out.(Model)
	if got.searchQuery != "ac" || got.searchCaret != 1 {
		t.Fatalf("backspace: query=%q caret=%d, want %q caret 1", got.searchQuery, got.searchCaret, "ac")
	}
}
