package tui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"consolestore/internal/catalog"
	"consolestore/internal/tui/render"
	"consolestore/internal/tui/screens"
)

var ansiSeq = regexp.MustCompile("\x1b\\[[0-9;]*m")

func plainLines(s string) []string { return strings.Split(ansiSeq.ReplaceAllString(s, ""), "\n") }

func manyPlaces(n int) []catalog.Place {
	ps := make([]catalog.Place, n)
	for i := range ps {
		ps[i] = catalog.Place{Name: fmt.Sprintf("Restaurant %d", i+1), Rating: 4.2, ETA: "30 MINS"}
	}
	return ps
}

// REGRESSION: at a short terminal the live browse screen used to render the full
// restaurant list (it never windowed to the height), overflowing the viewport so
// the brand header scrolled off the top. The list must window and the header must
// stay on screen.
func TestBrowseShortHeightKeepsHeaderAndWindows(t *testing.T) {
	menu := screens.NewMenu(nil, catalog.Address{Line: "HSR Layout", Label: "Home"},
		catalog.SectionCoffee, catalog.Usual{}, false, "cart · 1 · ₹328").
		WithRail(screens.NewRail([]string{"Coffee", "Pizza", "Biriyani", "Burgers"})).
		WithSectionTabsHidden(true).WithChips([]string{"Coffee"}, 0).
		WithSections(nil, manyPlaces(25))

	for _, h := range []int{18, 24, 28} {
		m := New(render.Caps{})
		m.live = true
		m.screen = scrMenu
		m.menu = menu
		m.w, m.h = 110, h

		lines := plainLines(m.View())
		if len(lines) > h {
			t.Fatalf("h=%d: render overflows (%d lines) — header would scroll off", h, len(lines))
		}
		if !strings.Contains(lines[0], "consolestore.in") {
			t.Fatalf("h=%d: brand header is not the first line:\n%s", h, strings.Join(lines, "\n"))
		}
		if !strings.Contains(strings.Join(lines, "\n"), "more") {
			t.Fatalf("h=%d: 25 restaurants in a short window should show a '↓ N more' indicator:\n%s", h, strings.Join(lines, "\n"))
		}
	}
}

// The in-restaurant menu must also keep its header on screen at a short height
// (this one already windowed; guard it so the chrome tuning can't regress it).
func TestRestaurantShortHeightKeepsHeader(t *testing.T) {
	items := make([]catalog.Item, 30)
	for i := range items {
		items[i] = catalog.Item{ID: fmt.Sprintf("i%d", i), Name: fmt.Sprintf("Dish %d", i+1), Price: 300, Rating: 4.5}
	}
	rest := screens.NewRestaurant(catalog.Place{Name: "Starbucks", Rating: 4.5, ETA: "35-40 MINS", Items: items}, map[string]int{}, "cart")
	for _, h := range []int{18, 24, 28} {
		m := New(render.Caps{})
		m.live = true
		m.screen = scrRestaurant
		m.rest = rest
		m.w, m.h = 110, h
		lines := plainLines(m.View())
		if len(lines) > h {
			t.Fatalf("rest h=%d: overflow (%d lines)", h, len(lines))
		}
		if !strings.Contains(lines[0], "consolestore.in") {
			t.Fatalf("rest h=%d: brand header not first line:\n%s", h, strings.Join(lines, "\n"))
		}
	}
}
