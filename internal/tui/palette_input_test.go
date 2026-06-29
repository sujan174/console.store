package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	swiggysnap "console.store/internal/catalog/swiggy"

	"console.store/internal/tui/render"
)

// The `:` command palette must accept spaces (so `alias set <name>` is typable)
// and support left-arrow caret editing — like the search bar. Space arrives as
// tea.KeySpace, not KeyRunes, which the palette used to drop.
func TestPaletteAcceptsSpacesAndCaret(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "local", ""))
	m.cmdOpen = true

	feed := func(keys ...tea.KeyMsg) {
		for _, k := range keys {
			out, _ := m.Update(k)
			m = out.(Model)
		}
	}

	feed(
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("alias")},
		tea.KeyMsg{Type: tea.KeySpace},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("set")},
		tea.KeyMsg{Type: tea.KeySpace},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("breakfast")},
	)
	if got := m.cmd.Text(); got != "alias set breakfast" {
		t.Fatalf("spaces dropped: %q, want %q", got, "alias set breakfast")
	}

	// ← once puts the caret before the final 't'; inserting 'X' lands mid-string.
	feed(
		tea.KeyMsg{Type: tea.KeyLeft},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")},
	)
	if got := m.cmd.Text(); got != "alias set breakfasXt" {
		t.Fatalf("left-arrow caret insert failed: %q", got)
	}

	// Backspace deletes the rune before the caret (the 'X'), not the end.
	feed(tea.KeyMsg{Type: tea.KeyBackspace})
	if got := m.cmd.Text(); got != "alias set breakfast" {
		t.Fatalf("caret backspace failed: %q", got)
	}
}
