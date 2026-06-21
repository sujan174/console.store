package screens_test

import (
	"strings"
	"testing"

	"console.store/internal/catalog"
	"console.store/internal/tui/screens"
)

func TestInstamartHeaderAndFastLane(t *testing.T) {
	items := []catalog.Item{{ID: "rb", Name: "Red Bull (250ml)", Price: 125}}
	s := screens.NewInstamart(items, map[string]int{}, "")
	v := s.View()
	if !strings.Contains(v, "fast lane") || !strings.Contains(strings.ReplaceAll(v, " ", ""), "RedBull") {
		t.Errorf("missing header/items:\n%s", v)
	}
	if got, ok := s.Selected(); !ok || got.Name != "Red Bull (250ml)" {
		t.Errorf("first selection = %q (ok=%v)", got.Name, ok)
	}
}

func TestInstamartInCartStepper(t *testing.T) {
	items := []catalog.Item{{ID: "rb", Name: "Red Bull", Price: 125}}
	s := screens.NewInstamart(items, map[string]int{"rb": 1}, "")
	v := s.View()
	for _, w := range []string{"×1", "−", "+"} {
		if !strings.Contains(v, w) {
			t.Errorf("missing %q:\n%s", w, v)
		}
	}
}
