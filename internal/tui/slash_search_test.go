package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	swiggysnap "consolestore/internal/catalog/swiggy"
	"consolestore/internal/config"
	"consolestore/internal/tui/render"
	"consolestore/internal/tui/screens"
)

// `/` jumps straight into search from the browse screen.
func TestSlashEntersSearch(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "local", ""))
	m.screen = scrMenu
	m.chips = []config.Category{{Label: "Coffee", Query: "coffee"}}

	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	mm := out.(Model)
	if !mm.searchMode {
		t.Fatal("/ should enter search mode")
	}
	if mm.railActive != screens.RailSearch {
		t.Fatalf("/ should select the Search rail entry, got %d", mm.railActive)
	}
}
