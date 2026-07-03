package screens_test

import (
	"strings"
	"testing"

	"consolestore/internal/catalog"
	"consolestore/internal/tui/screens"
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

// liveInstamart builds a two-pane Instamart screen with the rail attached,
// mirroring liveMenu() in menu_test.go — the render path exercised once a
// rail has been attached (live mode).
func liveInstamart() screens.Instamart {
	return screens.NewInstamart(nil, nil, "🛒 empty").
		WithRail(screens.NewRailCategories([]string{"Energy Drinks", "Chips"})).
		WithRailFocus(true).WithMaxRows(20)
}

// TestInstamartTwoPaneShowsStoreSwitcher: the Instamart page renders the
// mirror-image switcher — INSTAMART as the active gold pill, Food dim, no
// "soon" tag (Instamart is live), plus the rail with the curated categories.
func TestInstamartTwoPaneShowsStoreSwitcher(t *testing.T) {
	v := liveInstamart().View()
	for _, want := range []string{"INSTAMART", "Food", "tab", "switch", "Energy Drinks"} {
		if !strings.Contains(v, want) {
			t.Fatalf("instamart two-pane switcher/rail missing %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "soon") {
		t.Fatalf("instamart is live now — must not show a soon marker:\n%s", v)
	}
}

// TestInstamartRailIsHomeless: the IM rail is Search then categories — no
// "Usuals"/go-to slot, so browsing starts straight on a product category.
func TestInstamartRailIsHomeless(t *testing.T) {
	v := liveInstamart().View()
	if !strings.Contains(v, "Search") {
		t.Fatalf("instamart rail must show Search:\n%s", v)
	}
	if strings.Contains(v, "Usuals") {
		t.Fatalf("instamart rail must NOT show a Usuals slot:\n%s", v)
	}
	if !strings.Contains(v, "Energy Drinks") {
		t.Fatalf("instamart rail must show the first category:\n%s", v)
	}
}
