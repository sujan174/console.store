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

// Every post-landing screen must fit the viewport across a basic range of
// terminal sizes — no overflow (which scrolls the brand header off the top), and
// the brand header stays on line 1. Centered screens (splash, gate, modals) just
// must not overflow. Locks the whole adaptive-height sweep.
func TestAllScreensFitViewport(t *testing.T) {
	addr := catalog.Address{Line: "FD 46, HSR Layout", Label: "Home"}
	cart := func(n int) screens.Checkout {
		lines := make([]screens.CartLine, n)
		for i := range lines {
			lines[i] = screens.CartLine{Item: catalog.Item{Name: fmt.Sprintf("Item %d", i+1), Price: 80}, Qty: 1}
		}
		return screens.NewCheckout("Blue Tokai", addr, lines, "35-40 mins").
			WithBill(screens.Bill{Live: true, ToPay: n*80 + 70}).WithLiveSync(true, "")
	}
	items := make([]catalog.Item, 30)
	for i := range items {
		items[i] = catalog.Item{ID: fmt.Sprintf("i%d", i), Name: fmt.Sprintf("Dish %d", i+1), Price: 300, Rating: 4.5}
	}
	menu := screens.NewMenu(nil, addr, catalog.SectionCoffee, catalog.Usual{}, false, "cart").
		WithRail(screens.NewRail([]string{"Coffee", "Pizza", "Biriyani", "Burgers"})).
		WithSectionTabsHidden(true).WithChips([]string{"Coffee"}, 0).WithSections(nil, manyPlaces(20))
	rest := screens.NewRestaurant(catalog.Place{Name: "Starbucks", Rating: 4.5, ETA: "35 MINS", Items: items}, map[string]int{}, "cart")

	cases := []struct {
		name   string
		banner bool // body screens must keep the brand header on line 1
		build  func(*Model)
	}{
		{"menu", true, func(m *Model) { m.live = true; m.screen = scrMenu; m.menu = menu }},
		{"restaurant", true, func(m *Model) { m.live = true; m.screen = scrRestaurant; m.rest = rest }},
		{"checkout", true, func(m *Model) { m.live = true; m.screen = scrCheckout; m.checkout = cart(10) }},
		{"checkout-empty", true, func(m *Model) { m.live = true; m.screen = scrCheckout; m.checkout = cart(0) }},
		// confirm is now the full-page confetti celebration (chrome-free, like
		// the order-placing loader) — no brand banner, just no-overflow.
		{"confirm", false, func(m *Model) { m.live = true; m.screen = scrConfirm; m.checkout = cart(3).Placed("999", "35-40 mins") }},
		{"track-transit", true, func(m *Model) {
			m.live = true
			m.screen = scrTracking
			m.nowUnix = 1_000_600
			m.track = screens.NewTracking("Blue Tokai", "HSR", "X1", 1_000_000, 30, 40).WithLive("Out for delivery", "10 mins")
		}},
		{"track-delivered", true, func(m *Model) {
			m.live = true
			m.screen = scrTracking
			m.nowUnix = 1_900_000
			m.track = screens.NewTracking("Blue Tokai", "HSR", "X1", 1_000_000, 30, 40).WithLive("Delivered", "")
		}},
		{"instamart", true, func(m *Model) { m.live = true; m.screen = scrInstamart }},
		{"splash", false, func(m *Model) { m.screen = scrSplash }},
		{"authgate", false, func(m *Model) { m.needsAuth = true }},
		{"settings", false, func(m *Model) { m.settingsOpen = true }},
		{"conflict", false, func(m *Model) {
			m.conflictOpen = true
			m.conflict = screens.NewCartConflict("Blue Tokai", "Third Wave", "Flat White")
		}},
	}

	for _, h := range []int{18, 20, 24, 30, 44} {
		for _, w := range []int{80, 110, 140} {
			for _, tc := range cases {
				m := New(render.Caps{})
				m.w, m.h = w, h
				tc.build(&m)
				lines := plainLines(m.View())
				if len(lines) > h {
					t.Errorf("%s w=%d h=%d: overflow (%d lines > %d)", tc.name, w, h, len(lines), h)
				}
				if tc.banner && (len(lines) == 0 || !strings.Contains(lines[0], "consolestore.in")) {
					t.Errorf("%s w=%d h=%d: brand header not on line 1", tc.name, w, h)
				}
			}
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
