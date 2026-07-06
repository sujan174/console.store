package tui

import (
	"strings"
	"testing"

	"consolestore/internal/tui/render"
	"consolestore/internal/tui/screens"
)

// On a tall terminal the game panel appears in attract mode below tracking,
// once onTick has had a chance to lazily create + size it.
func TestDodgeShownWhenRoom(t *testing.T) {
	m := New(render.Caps{})
	m.w, m.h = 100, 40
	m.screen = scrTracking
	m.track = screens.NewTracking("Test Diner", "MG Road", "OID1", 0, 20, 30)
	m, _ = m.onTick()
	if !strings.Contains(ansiStrip(m.View()), "ENTER") {
		t.Fatal("tall tracking page should show the dodge attract prompt")
	}
}

// On a short terminal the game is hidden; tracking still renders.
func TestDodgeHiddenWhenCramped(t *testing.T) {
	m := New(render.Caps{})
	m.w, m.h = 80, 14
	m.screen = scrTracking
	m.track = screens.NewTracking("Test Diner", "MG Road", "OID1", 0, 20, 30)
	m, _ = m.onTick()
	out := ansiStrip(m.View())
	if strings.Contains(out, "dodge the traffic") {
		t.Fatal("cramped tracking page must not show the game body")
	}
	if !strings.Contains(out, "Test Diner") && !strings.Contains(out, "MG Road") {
		t.Fatal("cramped tracking page should still render tracking content")
	}
}
