package screens_test

import (
	"strings"
	"testing"

	"consolestore/internal/catalog"
	"consolestore/internal/tui/screens"
)

func TestAddressScreenListsAllAndMarksCurrent(t *testing.T) {
	addrs := []catalog.Address{
		{ID: "a1", Label: "home", Line: "HSR Layout"},
		{ID: "a2", Label: "work", Line: "Koramangala"},
	}
	s := screens.NewAddress(addrs, "a2")
	view := s.View()
	if !strings.Contains(view, "HSR Layout") || !strings.Contains(view, "Koramangala") {
		t.Errorf("address screen missing entries:\n%s", view)
	}
	if got := s.Selected().ID; got != "a2" {
		t.Errorf("cursor should start on current address a2, got %q", got)
	}
	if !strings.Contains(view, "home") || !strings.Contains(view, "work") {
		t.Errorf("address screen missing labels:\n%s", view)
	}
}

func TestAddressScreenTitleAndEntries(t *testing.T) {
	addrs := []catalog.Address{
		{ID: "a1", Label: "home", Line: "HSR Layout"},
		{ID: "a2", Label: "work", Line: "Koramangala"},
	}
	v := screens.NewAddress(addrs, "a1").View()
	for _, want := range []string{"╭", "╰", "deliver to", "HSR Layout", "Koramangala", "↵ choose", "esc cancel"} {
		if !strings.Contains(v, want) {
			t.Errorf("address modal missing %q:\n%s", want, v)
		}
	}
}
