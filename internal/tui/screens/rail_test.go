package screens

import (
	"strings"
	"testing"
)

func TestRailEntriesOrder(t *testing.T) {
	r := NewRail([]string{"Coffee", "Pizza"})
	// ⌕ Search, Home, Coffee, Pizza
	if r.Len() != 4 {
		t.Fatalf("expected 4 entries, got %d", r.Len())
	}
	if !strings.Contains(r.EntryLabel(RailSearch), "Search") {
		t.Errorf("index 0 must be Search: %q", r.EntryLabel(RailSearch))
	}
	if r.EntryLabel(RailHome) != "Home" {
		t.Errorf("index 1 must be Home: %q", r.EntryLabel(RailHome))
	}
	if r.EntryLabel(2) != "Coffee" {
		t.Errorf("categories start at index 2: %q", r.EntryLabel(2))
	}
}

func TestRailViewShowsEntriesAndSearchIcon(t *testing.T) {
	v := NewRail([]string{"Coffee"}).WithActive(RailHome).WithHeight(10).View()
	for _, want := range []string{"⌕", "Search", "Home", "Coffee"} {
		if !strings.Contains(v, want) {
			t.Errorf("rail view missing %q:\n%s", want, v)
		}
	}
}

func TestRailFixedWidth(t *testing.T) {
	r := NewRail([]string{"Coffee"})
	if r.Width() < 10 {
		t.Fatalf("rail width should be a sensible fixed column, got %d", r.Width())
	}
}
