package screens_test

import (
	"strings"
	"testing"

	"console.store/internal/catalog"
	"console.store/internal/tui/screens"
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
}
