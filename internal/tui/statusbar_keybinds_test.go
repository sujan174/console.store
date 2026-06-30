package tui

import (
	"strings"
	"testing"
)

// System 2: the status bar shows real per-screen keybinds on the two main
// screens (command-line feel), not the marketing rotation.
func TestScreenKeybinds(t *testing.T) {
	var m Model
	m.screen = scrMenu
	if kb := m.screenKeybinds(); !strings.Contains(kb, "/ search") || !strings.Contains(kb, ": cmd") {
		t.Fatalf("browse keybinds = %q", kb)
	}
	if kb := m.screenKeybinds(); !strings.Contains(kb, "? help") {
		t.Fatalf("menu keybinds must advertise help, got %q", kb)
	}
	m.screen = scrRestaurant
	if kb := m.screenKeybinds(); !strings.Contains(kb, "↵/+ add") || !strings.Contains(kb, "c cart") {
		t.Fatalf("restaurant keybinds = %q", kb)
	}
	m.screen = scrCheckout
	if kb := m.screenKeybinds(); !strings.Contains(kb, "↵ place") || !strings.Contains(kb, "? help") {
		t.Fatalf("checkout keybinds = %q", kb)
	}
	// Every other screen still advertises help at minimum (the affordance is
	// always discoverable from the bottom bar).
	m.screen = scrCart
	if kb := m.screenKeybinds(); !strings.Contains(kb, "? help") {
		t.Fatalf("other screens must advertise help, got %q", kb)
	}
}
