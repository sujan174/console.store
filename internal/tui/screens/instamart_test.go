package screens_test

import (
	"strings"
	"testing"

	"console.store/internal/catalog"
	"console.store/internal/tui/screens"
)

func TestInstamartListsItemsWithFastEta(t *testing.T) {
	items := []catalog.Item{
		{ID: "im-red-bull", Name: "Red Bull (250ml)", Price: 125, Section: catalog.SectionInstamart},
		{ID: "im-lays", Name: "Lay's Classic Salted", Price: 20, Section: catalog.SectionInstamart},
	}
	s := screens.NewInstamart(items, 0)
	view := s.View()
	if !strings.Contains(view, "Red Bull") || !strings.Contains(view, "min") {
		t.Errorf("instamart list missing items or eta:\n%s", view)
	}
	if got, ok := s.Selected(); !ok || got.Name != "Red Bull (250ml)" {
		t.Errorf("first selection = %q (ok=%v)", got.Name, ok)
	}
}
