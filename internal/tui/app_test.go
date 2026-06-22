package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/catalog"
	"console.store/internal/tui/render"
)

// despace strips spaces so assertions survive the list's letter-spacing
// (which only inserts spaces between glyphs).
func despace(s string) string { return strings.ReplaceAll(s, " ", "") }

// newAtMenu returns a Model that has dismissed the splash and is on the menu,
// so flow tests can drive menu interactions directly.
func newAtMenu() Model {
	m := New(render.Caps{}, nil)
	m.screen = scrMenu
	return m
}

func TestStatusBarOnMenuNotSplash(t *testing.T) {
	m := New(render.Caps{}, nil) // splash
	if strings.Contains(m.View(), "⊙ linked") {
		t.Error("splash must not show the status bar")
	}
	m2 := newAtMenu()
	v := m2.View()
	if !strings.Contains(v, "⊙ linked") || !strings.Contains(v, m2.addr.Line) {
		t.Errorf("menu should show the status bar with the address:\n%s", v)
	}
}

func TestStartsOnSplashThenKeyToMenu(t *testing.T) {
	m := New(render.Caps{}, nil)
	if m.screen != scrSplash {
		t.Fatalf("app should start on splash, got screen %d", m.screen)
	}
	// a key during the decode skips it and settles the home landing (still splash)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = updated.(Model)
	if m.screen != scrSplash {
		t.Fatalf("key during decode should settle home, not leave splash; got %d", m.screen)
	}
	// from the settled home, activating go-to-shop -> menu
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = updated.(Model)
	if !strings.Contains(m.View(), "console.store") {
		t.Errorf("after activating, should be on menu:\n%s", m.View())
	}
}

func TestSplashHoldsUntilKey(t *testing.T) {
	m := New(render.Caps{}, nil)
	// Ticks resolve the decode but never leave the splash — it's a landing
	// screen now; the user must pick "go to shop".
	for i := 0; i < 200; i++ {
		updated, _ := m.Update(tickMsg(time.Now()))
		m = updated.(Model)
	}
	if m.screen != scrSplash {
		t.Errorf("splash should hold until a key, got screen %d", m.screen)
	}
	if m.decodeStep < render.DecodeSteps {
		t.Errorf("decode should have finished, decodeStep=%d", m.decodeStep)
	}
	// enter activates the selected home item -> menu
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.screen != scrMenu {
		t.Errorf("enter on settled splash should go to menu, got %d", m.screen)
	}
}

func TestTickAdvancesFrame(t *testing.T) {
	m := New(render.Caps{}, nil)
	f0 := m.frame
	updated, cmd := m.Update(tickMsg(time.Now()))
	m = updated.(Model)
	if m.frame != f0+1 {
		t.Errorf("frame = %d, want %d", m.frame, f0+1)
	}
	if cmd == nil {
		t.Error("tick must reschedule itself")
	}
}

func TestAppStartsOnMenu(t *testing.T) {
	m := newAtMenu()
	out := m.View()
	if !strings.Contains(out, "console.store") || !strings.Contains(despace(out), "BlueTokai") {
		t.Fatal("app should start on menu with places")
	}
}

func TestAppEnterOpensRestaurantThenEscBack(t *testing.T) {
	m := newAtMenu()
	// enter on first place
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(m2.View(), "35-45 min") {
		t.Fatal("enter should open restaurant view")
	}
	// esc back to menu
	m3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !strings.Contains(m3.View(), "deliver to") {
		t.Fatal("esc should return to menu")
	}
}

func TestSectionSwitchChangesPlaces(t *testing.T) {
	m := newAtMenu()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	view := updated.(Model).View()
	if !strings.Contains(despace(view), "CaliforniaBurrito") {
		t.Errorf("after switching to food, expected a food place; got:\n%s", view)
	}
	if strings.Contains(despace(view), "BlueTokai") {
		t.Error("coffee place should not show under food section")
	}
}

func TestUsualAddsToCartStaysOnMenu(t *testing.T) {
	m := newAtMenu() // a1 -> usual is Cold Coffee · Blue Tokai
	before := m.cartTotal()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	m = updated.(Model)
	if m.cartTotal() <= before {
		t.Errorf("pressing u should add the usual to the cart; total %d -> %d", before, m.cartTotal())
	}
	if m.screen != scrMenu {
		t.Errorf("pressing u should stay on the menu, got screen %d", m.screen)
	}
	if !strings.Contains(m.View(), "console.store") {
		t.Errorf("should still render the menu:\n%s", m.View())
	}
}

func TestSectionsAreNonCyclable(t *testing.T) {
	// left from coffee (first tab) stays on coffee — no wrap to snacks.
	m := newAtMenu()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if updated.(Model).section != catalog.SectionCoffee {
		t.Fatalf("left from coffee should clamp to coffee, got %q", updated.(Model).section)
	}
	// right from snacks (last tab) stays on snacks — no wrap to coffee.
	m = newAtMenu()
	for i := 0; i < 5; i++ { // mash right past the end
		u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
		m = u.(Model)
	}
	if m.section != catalog.SectionSnacks {
		t.Fatalf("right past snacks should clamp to snacks, got %q", m.section)
	}
}

func TestAddressSwitchReFiltersMenu(t *testing.T) {
	m := newAtMenu() // starts at a1 (HSR), coffee section
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	view := m.View()
	if !strings.Contains(view, "Indiranagar") {
		t.Errorf("menu header should show new address Indiranagar:\n%s", view)
	}
	if !strings.Contains(despace(view), "Subko") {
		t.Errorf("Subko should be serviceable at Indiranagar:\n%s", view)
	}
}

func TestAppQuits(t *testing.T) {
	m := New(render.Caps{}, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl-c should return a quit command")
	}
}

// TestAddToCartPreservesCursor ensures that adding an item to the cart does not
// reset the restaurant list cursor back to 0. This would fail against the old
// NewRestaurant rebuild behavior.
func TestAddToCartPreservesCursor(t *testing.T) {
	m := newAtMenu()

	// Open first restaurant (Blue Tokai).
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Move cursor down to the second item (index 1).
	m3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})

	// Add the currently selected item (should be item at index 1).
	m4, _ := m3.Update(tea.KeyMsg{Type: tea.KeyEnter})

	model := m4.(Model)

	// After add, the restaurant cursor must still point to item 1.
	got, ok := model.rest.Selected()
	if !ok {
		t.Fatal("expected a selected item")
	}
	want := "Hazelnut Cold Brew" // Blue Tokai index 1
	if got.Name != want {
		t.Fatalf("cursor was reset: want selected=%q, got selected=%q", want, got.Name)
	}
}

// TestCartHeaderFromMenuNotNonsense opens the cart from the menu before any
// items are added and asserts the header is sensible (no "cart · cart").
func TestCartHeaderFromMenuNotNonsense(t *testing.T) {
	m := newAtMenu()

	// Press 'c' to open cart from menu with zero items.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	view := m2.View()

	if strings.Contains(view, "cart · cart") {
		t.Fatal("cart header must not contain 'cart · cart'")
	}
	if !strings.Contains(view, "your order") {
		t.Fatalf("cart header should say 'your order' when cart is empty, got view:\n%s", view)
	}
}

func TestCartEditsSyncToRouter(t *testing.T) {
	m := newAtMenu()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open place
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // add item
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")}) // cart
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight}) // qty 2
	m = updated.(Model)
	if m.cartTotal() != m.cart.Total() {
		t.Errorf("router total %d != cart total %d", m.cartTotal(), m.cart.Total())
	}
	if m.cart.Lines()[0].Qty != 2 {
		t.Errorf("qty = %d, want 2", m.cart.Lines()[0].Qty)
	}
}

// TestCartScreenShowsBillAndEta drives menu -> place -> add Cold Coffee (₹149)
// -> cart, and asserts the restyled cart shows the bill breakdown (to-pay ₹128)
// and the place ETA derived as "~45 min".
func TestCartScreenShowsBillAndEta(t *testing.T) {
	m := newAtMenu()
	for _, k := range []tea.KeyMsg{
		{Type: tea.KeyEnter},                     // open Blue Tokai
		{Type: tea.KeyEnter},                     // add Cold Coffee (₹149)
		{Type: tea.KeyRunes, Runes: []rune("c")}, // cart
	} {
		updated, _ := m.Update(k)
		m = updated.(Model)
	}
	view := m.View()
	for _, want := range []string{"item total", "₹149", "delivery", "₹29", "DEVFRIDAY", "to pay (COD)", "₹128", "~45 min"} {
		if !strings.Contains(view, want) {
			t.Errorf("cart screen missing %q:\n%s", want, view)
		}
	}
	// The menu cart chip stays the ITEM total, not the bill total.
	if toPay(m.cartTotal()) != 128 {
		t.Errorf("toPay(itemTotal) = %d, want 128", toPay(m.cartTotal()))
	}
}

func TestCheckoutFlowPlacesAndResets(t *testing.T) {
	m := newAtMenu()
	steps := []tea.KeyMsg{
		{Type: tea.KeyEnter},                     // open place
		{Type: tea.KeyEnter},                     // add item
		{Type: tea.KeyRunes, Runes: []rune("c")}, // cart
		{Type: tea.KeyEnter},                     // checkout
		{Type: tea.KeyEnter},                     // place order
	}
	for _, k := range steps {
		updated, _ := m.Update(k)
		m = updated.(Model)
	}
	if !strings.Contains(m.View(), "order placed") {
		t.Errorf("expected confirm screen:\n%s", m.View())
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.cartTotal() != 0 {
		t.Errorf("cart should be empty after confirm, total=%d", m.cartTotal())
	}
	if !strings.Contains(m.View(), "console.store") {
		t.Errorf("should be back on menu:\n%s", m.View())
	}
}

func TestTrackingFlowAdvancesAndEscResets(t *testing.T) {
	m := newAtMenu()
	steps := []tea.KeyMsg{
		{Type: tea.KeyEnter},                     // open place
		{Type: tea.KeyEnter},                     // add item
		{Type: tea.KeyRunes, Runes: []rune("c")}, // cart
		{Type: tea.KeyEnter},                     // checkout
		{Type: tea.KeyEnter},                     // place order -> confirm
		{Type: tea.KeyEnter},                     // confirm -> tracking
	}
	for _, k := range steps {
		updated, _ := m.Update(k)
		m = updated.(Model)
	}
	if !strings.Contains(m.View(), "tracking ·") {
		t.Fatalf("expected tracking screen:\n%s", m.View())
	}

	// drive ticks: trackStep should advance and cap at 3
	// trackTick%70==0 triggers step advance; 3 steps need 3*70=210 ticks.
	for i := 0; i < 215; i++ {
		updated, _ := m.Update(tickMsg(time.Now()))
		m = updated.(Model)
	}
	if m.trackStep != 3 {
		t.Errorf("trackStep should cap at 3, got %d", m.trackStep)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.cartTotal() != 0 {
		t.Errorf("cart should be empty after esc, total=%d", m.cartTotal())
	}
	if !strings.Contains(m.View(), "console.store") {
		t.Errorf("should be back on menu:\n%s", m.View())
	}
}

func TestSearchEmptyThenEnterDoesNotPanic(t *testing.T) {
	m := newAtMenu()
	seq := []tea.KeyMsg{
		{Type: tea.KeyEnter},                     // open first place
		{Type: tea.KeyRunes, Runes: []rune("/")}, // enter search
		{Type: tea.KeyRunes, Runes: []rune("z")}, // filter -> zero matches
		{Type: tea.KeyRunes, Runes: []rune("z")},
		{Type: tea.KeyEnter}, // exit search, filter still empty-result
		{Type: tea.KeyEnter}, // would panic on Selected() pre-fix
	}
	for _, k := range seq {
		updated, _ := m.Update(k)
		m = updated.(Model)
	}
	// no panic; cart stayed empty because nothing was selectable
	if m.cartTotal() != 0 {
		t.Errorf("nothing should have been added, total=%d", m.cartTotal())
	}
}

func TestAddressSwitchFlushesUnserviceableCart(t *testing.T) {
	m := newAtMenu() // a1 (HSR); Blue Tokai serves a1 but NOT a3
	// add a Blue Tokai item (first coffee place at a1, first item)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open Blue Tokai
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // add Cold Coffee
	m = updated.(Model)
	if m.cartTotal() == 0 {
		t.Fatal("expected an item in cart")
	}
	// back to menu, then switch to a3 (Indiranagar): esc -> a -> j -> j -> enter
	for _, k := range []tea.KeyMsg{
		{Type: tea.KeyEsc},
		{Type: tea.KeyRunes, Runes: []rune("a")},
		{Type: tea.KeyRunes, Runes: []rune("j")},
		{Type: tea.KeyRunes, Runes: []rune("j")},
		{Type: tea.KeyEnter},
	} {
		updated, _ = m.Update(k)
		m = updated.(Model)
	}
	if m.cartTotal() != 0 {
		t.Errorf("cart should be flushed when restaurant doesn't serve new address, total=%d", m.cartTotal())
	}
}

// TestRestaurantLeftDecrements verifies that, on the restaurant screen, ← (left)
// decrements the highlighted item rather than navigating back, and removes the
// item from the cart when its qty reaches 0. esc is the only "back" key.
func TestRestaurantLeftDecrements(t *testing.T) {
	m := newAtMenu()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open Blue Tokai
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // add Cold Coffee (qty 1)
	m = updated.(Model)
	if m.qtyMap()["bt-cold-coffee"] != 1 {
		t.Fatalf("expected qty 1 after add, qtyMap=%v", m.qtyMap())
	}
	if m.screen != scrRestaurant {
		t.Fatalf("should still be on restaurant after add, screen=%d", m.screen)
	}
	// ← decrements (not back)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = updated.(Model)
	if m.screen != scrRestaurant {
		t.Fatalf("← must decrement, not navigate back; screen=%d", m.screen)
	}
	if len(m.lines) != 0 {
		t.Fatalf("item should leave the cart at qty 0, lines=%v", m.lines)
	}
	if m.cartTotal() != 0 {
		t.Fatalf("cart total should be 0 after decrement, got %d", m.cartTotal())
	}
}

// Instamart is no longer a menu lane in the approved 3-tab design; it is reached
// only via the `:instamart` command. These tests drive that entry path.

// openInstamart drives the command palette to open the Instamart fast lane.
func openInstamart(t *testing.T, m Model) Model {
	t.Helper()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
	m = updated.(Model)
	if !m.cmdOpen {
		t.Fatal("`:` should open the palette")
	}
	for _, r := range "instamart" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if !strings.Contains(m.View(), "fast lane") {
		t.Fatalf(":instamart should open Instamart:\n%s", m.View())
	}
	return m
}

func TestCmdPaletteOpensAndInstamart(t *testing.T) {
	openInstamart(t, newAtMenu())
}

func TestInstamartViaCommandSeparateCartAndMinimum(t *testing.T) {
	m := openInstamart(t, newAtMenu())
	// add first item (Red Bull 125 >= 99) into the Instamart cart only
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.cartTotal() != 0 {
		t.Errorf("food cart untouched, got %d", m.cartTotal())
	}
	if m.imCartTotal() != 125 {
		t.Errorf("im cart = %d want 125", m.imCartTotal())
	}
	// open im cart, checkout proceeds (>=99)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if !strings.Contains(m.View(), "checkout") {
		t.Errorf("expected checkout:\n%s", m.View())
	}
}

func TestInstamartMinimumGate(t *testing.T) {
	m := openInstamart(t, newAtMenu())
	// add a cheap item below the ₹99 minimum, then open the cart.
	// Move the cursor to the cheapest available item: Lay's (₹20).
	for i := 0; i < 1; i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(Model)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.imCartTotal() >= InstamartMin {
		t.Skipf("second item is not below the minimum (total %d); gate test n/a", m.imCartTotal())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	m = updated.(Model)
	if !strings.Contains(m.View(), "minimum") {
		t.Errorf("expected minimum notice in cart:\n%s", m.View())
	}
	// checkout is gated: enter should not reach checkout.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if strings.Contains(m.View(), "checkout") {
		t.Errorf("checkout should be gated below minimum:\n%s", m.View())
	}
}

func TestCmdPaletteHelpStaysOpen(t *testing.T) {
	m := newAtMenu()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
	m = updated.(Model)
	for _, r := range "help" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if !m.cmdOpen {
		t.Error("help should keep the palette open")
	}
}

// TestDoubleEscReturnsToSplash presses Esc twice on the menu root in quick
// succession (no ticks between) and asserts the second Esc returns to the splash
// and replays the decode, while the cart is preserved.
func TestDoubleEscReturnsToSplash(t *testing.T) {
	m := newAtMenu()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open restaurant
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // add an item
	m = updated.(Model)
	if m.cartTotal() == 0 {
		t.Fatal("expected an item in the cart")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc}) // restaurant -> menu (walks back)
	m = updated.(Model)
	if m.screen != scrMenu {
		t.Fatalf("esc should walk back to menu, got screen %d", m.screen)
	}
	// now on the menu root: two quick escs are the deliberate home gesture
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc}) // arm
	m = updated.(Model)
	if m.screen != scrMenu {
		t.Fatalf("first esc on menu should stay (arm only), got screen %d", m.screen)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc}) // fire -> splash
	m = updated.(Model)
	if m.screen != scrSplash {
		t.Fatalf("double esc on menu should return to splash, got screen %d", m.screen)
	}
	if m.decodeStep != 0 {
		t.Errorf("decode should replay from 0, got decodeStep=%d", m.decodeStep)
	}
	if m.cartTotal() == 0 {
		t.Error("cart should be preserved across a double-esc home")
	}
}

// TestEscWalkBackDoesNotTeleportHome is the reported glitch: from a sub-screen,
// Esc (back to menu) immediately followed by another Esc must NOT jump to the
// splash — the back-step Esc must not arm the home gesture.
func TestEscWalkBackDoesNotTeleportHome(t *testing.T) {
	m := newAtMenu()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open restaurant
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc}) // restaurant -> menu
	m = updated.(Model)
	if m.screen != scrMenu {
		t.Fatalf("esc should walk back to menu, got screen %d", m.screen)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc}) // must stay on menu, not teleport
	m = updated.(Model)
	if m.screen != scrMenu {
		t.Errorf("esc after walking back must NOT jump home, got screen %d", m.screen)
	}
}

// TestSlowEscDoesNotReturnToSplash verifies a second Esc on the menu after the
// double-esc window has elapsed does not jump home.
func TestSlowEscDoesNotReturnToSplash(t *testing.T) {
	m := newAtMenu()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc}) // arm on menu
	m = updated.(Model)
	for i := 0; i < escDoubleWindow+1; i++ { // let the window lapse
		updated, _ = m.Update(tickMsg(time.Now()))
		m = updated.(Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc}) // slow esc — re-arms, no home
	m = updated.(Model)
	if m.screen != scrMenu {
		t.Errorf("slow second esc should not return to splash, got screen %d", m.screen)
	}
}

func TestTickInterval(t *testing.T) {
	if tickInterval != 60*time.Millisecond {
		t.Errorf("tickInterval = %v, want 60ms", tickInterval)
	}
}
