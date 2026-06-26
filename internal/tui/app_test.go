package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/catalog"
	swiggysnap "console.store/internal/catalog/swiggy"
	"console.store/internal/tui/render"
	"console.store/internal/tui/screens"
)

// despace strips spaces so assertions survive the list's letter-spacing
// (which only inserts spaces between glyphs).
func despace(s string) string { return strings.ReplaceAll(s, " ", "") }

// newAtMenu returns a Model that has dismissed the splash and is on the menu,
// so flow tests can drive menu interactions directly.
func newAtMenu() Model {
	m := New(render.Caps{})
	m.screen = scrMenu
	return m
}

func TestStatusBarOnMenuNotSplash(t *testing.T) {
	m := New(render.Caps{}) // splash
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
	m := New(render.Caps{})
	if m.screen != scrSplash {
		t.Fatalf("app should start on splash, got screen %d", m.screen)
	}
	// Enter mid-animation (decode not finished) goes straight to the shop —
	// no waiting for the animation, no two-step settle-then-press.
	if m.decodeStep >= render.DecodeSteps {
		t.Fatalf("precondition: decode should still be running, got %d", m.decodeStep)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.screen != scrMenu {
		t.Fatalf("enter during decode should go straight to the shop, got screen %d", m.screen)
	}
}

func TestSplashHoldsUntilKey(t *testing.T) {
	m := New(render.Caps{})
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
	m := New(render.Caps{})
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

// keyRunes builds a rune key-press (e.g. keyRunes("c")).
func keyRunes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

// addFocused adds one unit of the dish under the cursor: Enter adds it directly
// (a customizable dish opens the customise modal instead).
func addFocused(m Model) Model {
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // add
	return u.(Model)
}

// enterFirstRestaurantWithItem drives menu → restaurant → add one item, returning
// the model with a non-empty cart bound to a restaurant.
func enterFirstRestaurantWithItem(t *testing.T) Model {
	t.Helper()
	m := newAtMenu()
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open first restaurant
	m = u.(Model)
	m = addFocused(m) // select + ↑ to add the first item
	if len(m.lines) == 0 || m.cartRestaurant == "" {
		t.Fatalf("precondition failed: lines=%d cartRestaurant=%q", len(m.lines), m.cartRestaurant)
	}
	return m
}

// Emptying the cart must release the restaurant binding so a later visit to the
// merged checkout shows a truly empty cart — not a stale "cart · {restaurant}".
// Now that the cart screen is no longer navigated (c goes straight to checkout),
// emptying is done via the restaurant screen's − key (the same invariant holds).
func TestEmptyingCartViaCartScreenClearsRestaurant(t *testing.T) {
	m := enterFirstRestaurantWithItem(t)
	rest := m.cartRestaurant

	// Remove the only line via the restaurant screen (cart screen retired from nav).
	u, _ := m.Update(keyRunes("-"))
	m = u.(Model)

	if len(m.lines) != 0 {
		t.Fatalf("cart should be empty, lines=%d", len(m.lines))
	}
	if m.cartRestaurant != "" {
		t.Fatalf("cartRestaurant should be cleared, got %q", m.cartRestaurant)
	}

	// Navigate to menu and open checkout — confirm no stale restaurant name leaks.
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = u.(Model)
	u, _ = m.Update(keyRunes("c"))
	m = u.(Model)
	if v := m.View(); strings.Contains(v, rest) {
		t.Errorf("empty cart still shows stale restaurant %q:\n%s", rest, v)
	}
}

// Emptying the cart from the RESTAURANT screen must likewise clear the binding.
func TestEmptyingCartViaRestaurantScreenClearsRestaurant(t *testing.T) {
	m := enterFirstRestaurantWithItem(t)
	// − decrements the focused dish back to zero, removing it from the cart.
	u, _ := m.Update(keyRunes("-")) // remove the only line
	m = u.(Model)
	if len(m.lines) != 0 {
		t.Fatalf("cart should be empty, lines=%d", len(m.lines))
	}
	if m.cartRestaurant != "" {
		t.Errorf("cartRestaurant should be cleared, got %q", m.cartRestaurant)
	}
}

// openCustomizeForHazelnut enters Blue Tokai, selects the customizable Hazelnut
// Cold Brew (item index 1), and presses add — leaving the customise modal open.
func openCustomizeForHazelnut(t *testing.T) Model {
	t.Helper()
	m := newAtMenu()
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open Blue Tokai
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // navigate to Hazelnut Cold Brew
	m = u.(Model)
	sel, _ := m.rest.Selected()
	if len(sel.AddOns) == 0 {
		t.Fatalf("precondition: %q should be customizable", sel.Name)
	}
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // add -> opens modal
	m = u.(Model)
	if !m.customizeOpen {
		t.Fatal("adding a customizable item should open the customise modal")
	}
	return m
}

func TestCustomizableAddOpensModalAndAppliesAddOns(t *testing.T) {
	m := openCustomizeForHazelnut(t)
	// Nothing added yet — the modal hasn't been confirmed.
	if m.cartCount() != 0 {
		t.Fatalf("cart should be empty until confirm, count=%d", m.cartCount())
	}
	// Toggle "Extra espresso shot" (+40): it's add-on index 1.
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = u.(Model)
	u, _ = m.Update(keyRunes(" "))
	m = u.(Model)
	// Confirm.
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)

	if m.customizeOpen {
		t.Fatal("confirm should close the modal")
	}
	if m.cartCount() != 1 {
		t.Fatalf("cart count = %d, want 1", m.cartCount())
	}
	if m.cartTotal() != 169+40 {
		t.Errorf("cart total = %d, want %d (base 169 + shot 40)", m.cartTotal(), 169+40)
	}
	if m.cartRestaurant != "Blue Tokai" {
		t.Errorf("cartRestaurant = %q, want Blue Tokai", m.cartRestaurant)
	}
}

func TestCustomizeEscCancelsAddsNothing(t *testing.T) {
	m := openCustomizeForHazelnut(t)
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = u.(Model)
	if m.customizeOpen {
		t.Fatal("esc should close the modal")
	}
	if m.cartCount() != 0 {
		t.Errorf("esc must not add anything, count=%d", m.cartCount())
	}
}

func TestSameAddOnsStackDifferentAddOnsSplit(t *testing.T) {
	// First add: Hazelnut + Extra shot.
	m := openCustomizeForHazelnut(t)
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown}) // to Extra shot
	m = u.(Model)
	u, _ = m.Update(keyRunes(" ")) // select
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // confirm
	m = u.(Model)

	// Second add: identical selection -> should stack (qty 2, one line).
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // re-add -> modal
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = u.(Model)
	u, _ = m.Update(keyRunes(" "))
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if len(m.lines) != 1 || m.lines[0].Qty != 2 {
		t.Fatalf("identical customisation should stack: lines=%d qty=%d", len(m.lines), m.lines[0].Qty)
	}

	// Third add: different selection (no add-ons) -> separate line.
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // re-add -> modal
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // confirm with nothing selected
	m = u.(Model)
	if len(m.lines) != 2 {
		t.Fatalf("different customisation should be a new line: lines=%d", len(m.lines))
	}
}

func TestNonCustomizableAddsDirectly(t *testing.T) {
	m := newAtMenu()
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // Blue Tokai
	m = u.(Model)
	// Item index 0 (Cold Coffee) has no add-ons. Select + ↑ adds it directly.
	m = addFocused(m)
	if m.customizeOpen {
		t.Fatal("non-customizable item should not open the modal")
	}
	if m.cartCount() != 1 {
		t.Errorf("non-customizable item should add directly, count=%d", m.cartCount())
	}
}

func TestAppStartsOnMenu(t *testing.T) {
	m := newAtMenu()
	out := m.View()
	if !strings.Contains(out, "consolestore.in") || !strings.Contains(despace(out), "BlueTokai") {
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
	if !strings.Contains(m.View(), "consolestore.in") {
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
	m := New(render.Caps{})
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

	// Select + ↑ on the currently focused item (index 1).
	model := addFocused(m3.(Model))

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

// TestCartEditsSyncToRouter verifies that after adding an item and opening the
// merged checkout (c), the checkout screen's line count and total reflect m.lines.
// (Cart-screen qty editing was retired when c was re-routed to scrCheckout.)
func TestCartEditsSyncToRouter(t *testing.T) {
	m := newAtMenu()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open place
	m = updated.(Model)
	m = addFocused(m)                                                         // select + ↑ to add item
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")}) // merged checkout
	m = updated.(Model)
	// checkout must show the authoritative lines from m.lines.
	if len(m.checkout.Lines()) != len(m.lines) {
		t.Errorf("checkout lines %d != router lines %d", len(m.checkout.Lines()), len(m.lines))
	}
	if len(m.lines) == 0 {
		t.Fatal("expected at least one line in the cart")
	}
	if m.checkout.Lines()[0].Qty != m.lines[0].Qty {
		t.Errorf("checkout qty %d != router qty %d", m.checkout.Lines()[0].Qty, m.lines[0].Qty)
	}
}

// TestCartScreenShowsBillAndEta drives menu -> place -> add Cold Coffee (₹149)
// -> checkout (merged), and asserts the merged checkout shows the bill total
// (to-pay ₹128) and the place ETA derived as "~45 min".
func TestCartScreenShowsBillAndEta(t *testing.T) {
	m := newAtMenu()
	for _, k := range []tea.KeyMsg{
		{Type: tea.KeyEnter},                     // open Blue Tokai
		{Type: tea.KeyEnter},                     // add Cold Coffee (₹149)
		{Type: tea.KeyRunes, Runes: []rune("c")}, // merged checkout
	} {
		updated, _ := m.Update(k)
		m = updated.(Model)
	}
	view := m.View()
	for _, want := range []string{"item total", "₹149", "delivery", "₹29", "DEVFRIDAY", "to pay (COD)", "₹128", "~45 min"} {
		if !strings.Contains(view, want) {
			t.Errorf("checkout screen missing %q:\n%s", want, view)
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
		{Type: tea.KeyRunes, Runes: []rune("c")}, // merged checkout (no separate cart step)
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
	if !strings.Contains(m.View(), "consolestore.in") {
		t.Errorf("should be back on menu:\n%s", m.View())
	}
}

func TestTrackingFlowAdvancesAndEscResets(t *testing.T) {
	m := newAtMenu()
	steps := []tea.KeyMsg{
		{Type: tea.KeyEnter},                     // open place
		{Type: tea.KeyEnter},                     // add item
		{Type: tea.KeyRunes, Runes: []rune("c")}, // merged checkout (no separate cart step)
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

	// drive ticks: trackStep should advance and cap at 4 (delivered).
	// trackTick%70==0 triggers step advance; from step 1 to 4 needs 3*70=210 ticks.
	for i := 0; i < 215; i++ {
		updated, _ := m.Update(tickMsg(time.Now()))
		m = updated.(Model)
	}
	if m.trackStep != 4 {
		t.Errorf("trackStep should cap at 4 (delivered), got %d", m.trackStep)
	}
	if !strings.Contains(m.View(), "enjoy your order") {
		t.Errorf("delivered tracking should show the thank-you note:\n%s", m.View())
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.cartTotal() != 0 {
		t.Errorf("cart should be empty after esc, total=%d", m.cartTotal())
	}
	if !strings.Contains(m.View(), "consolestore.in") {
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
	m = addFocused(m) // select + ↑ to add Cold Coffee
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

// TestRestaurantMinusDecrements verifies that − decrements the focused dish and
// removes it from the cart at qty 0, staying on the restaurant screen.
func TestRestaurantMinusDecrements(t *testing.T) {
	m := newAtMenu()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open Blue Tokai
	m = updated.(Model)
	m = addFocused(m) // Enter adds Cold Coffee (qty 1)
	if m.qtyMap()["bt-cold-coffee"] != 1 {
		t.Fatalf("expected qty 1 after add, qtyMap=%v", m.qtyMap())
	}
	if m.screen != scrRestaurant {
		t.Fatalf("should still be on restaurant after add, screen=%d", m.screen)
	}
	// − decrements the focused dish (not back)
	updated, _ = m.Update(keyRunes("-"))
	m = updated.(Model)
	if m.screen != scrRestaurant {
		t.Fatalf("− must decrement, not navigate back; screen=%d", m.screen)
	}
	if len(m.lines) != 0 {
		t.Fatalf("item should leave the cart at qty 0, lines=%v", m.lines)
	}
	if m.cartTotal() != 0 {
		t.Fatalf("cart total should be 0 after decrement, got %d", m.cartTotal())
	}
}

// TestRestaurantArrowsNavigateQtyOnPlusMinus verifies the controls: ↑/↓ always
// move between dishes (even while a dish is in the cart), while + / − adjust the
// focused dish's quantity, and − to zero removes it.
func TestRestaurantArrowsNavigateQtyOnPlusMinus(t *testing.T) {
	m := newAtMenu()
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open Blue Tokai
	m = u.(Model)

	// Add the first dish (Cold Coffee, non-customizable).
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // add -> qty 1
	m = u.(Model)
	if m.qtyMap()["bt-cold-coffee"] != 1 {
		t.Fatalf("Enter should add the focused dish; qtyMap=%v", m.qtyMap())
	}

	// + increments the in-cart focused dish to 2.
	u, _ = m.Update(keyRunes("+"))
	m = u.(Model)
	if m.qtyMap()["bt-cold-coffee"] != 2 {
		t.Fatalf("+ on an in-cart dish should increment; qtyMap=%v", m.qtyMap())
	}

	// ↓ must MOVE to another dish even though one is in the cart (the bug fix):
	// the cold-coffee qty stays 2 and the cursor leaves it.
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = u.(Model)
	moved, _ := m.rest.Selected()
	if moved.Name == "Cold Coffee" {
		t.Fatalf("↓ must navigate off the in-cart dish, stayed on %q", moved.Name)
	}
	if m.qtyMap()["bt-cold-coffee"] != 2 {
		t.Fatalf("↓ must not change quantity; qtyMap=%v", m.qtyMap())
	}

	// Back to the cold coffee; − twice returns to 0 and removes it.
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = u.(Model)
	for i := 0; i < 2; i++ {
		u, _ = m.Update(keyRunes("-"))
		m = u.(Model)
	}
	if len(m.lines) != 0 {
		t.Fatalf("− to zero should remove the dish; lines=%v", m.lines)
	}
}

// TestRestaurantArrowsNavigateCategoryNotAdd is the reported bug fix: on the
// restaurant screen, ← / → move the top category bar and must NOT add items or
// change the cart.
func TestRestaurantArrowsNavigateCategoryNotAdd(t *testing.T) {
	m := newAtMenu()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open Blue Tokai
	m = updated.(Model)
	if len(m.rest.Categories()) < 2 {
		t.Skip("seed restaurant has no sub-categories to navigate")
	}
	before := m.rest.ActiveCategory()
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight}) // → next category
	m = updated.(Model)
	if m.cartTotal() != 0 || len(m.lines) != 0 {
		t.Fatalf("→ must not add to cart; total=%d lines=%d", m.cartTotal(), len(m.lines))
	}
	if m.rest.ActiveCategory() == before {
		t.Fatalf("→ should advance the category bar; stayed on %q", before)
	}
	if m.screen != scrRestaurant {
		t.Fatalf("→ must stay on the restaurant screen, got %d", m.screen)
	}
}

// Instamart is no longer a menu lane in the approved 3-tab design; it is reached
// only via the `:instamart` command. These tests drive that entry path.

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
	m = addFocused(m) // select + ↑ to add an item
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

// openSecondRestaurantWithFirstInCart drives: open Blue Tokai, add its first
// item, esc to menu, move to the 2nd place, open it. Returns the model sitting
// in the 2nd restaurant with the 1st restaurant's item in the cart, plus the
// 2nd restaurant's name (read from the model, not hardcoded, so the test is
// robust to seed/serviceability ordering).
func openSecondRestaurantWithFirstInCart(t *testing.T) (Model, string) {
	t.Helper()
	m := newAtMenu()
	step := func(k tea.KeyMsg) { u, _ := m.Update(k); m = u.(Model) }

	step(tea.KeyMsg{Type: tea.KeyEnter}) // open Blue Tokai
	step(tea.KeyMsg{Type: tea.KeyEnter}) // add first item
	first := m.cartRestaurant
	if first == "" || len(m.lines) == 0 {
		t.Fatalf("setup: expected an item in the cart from %q", first)
	}
	step(tea.KeyMsg{Type: tea.KeyEsc})                       // back to menu
	step(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // 2nd place
	step(tea.KeyMsg{Type: tea.KeyEnter})                     // open it
	if m.screen != scrRestaurant {
		t.Fatalf("setup: expected to be in a restaurant, got screen %d", m.screen)
	}
	second := m.rest.PlaceData().Name
	if second == first {
		t.Fatalf("setup: needed a different 2nd restaurant, both were %q", first)
	}
	return m, first
}

func TestCrossRestaurantAddOpensConflict(t *testing.T) {
	m, first := openSecondRestaurantWithFirstInCart(t)
	before := len(m.lines)

	m = addFocused(m) // select + ↑ to add from the 2nd restaurant

	if !m.conflictOpen {
		t.Fatal("adding from a different restaurant should open the conflict modal")
	}
	if len(m.lines) != before {
		t.Fatalf("cart must be untouched while the modal is open: was %d, now %d", before, len(m.lines))
	}
	if m.cartRestaurant != first {
		t.Fatalf("cart restaurant must stay %q while modal open, got %q", first, m.cartRestaurant)
	}
	if m.conflictSel != 1 {
		t.Fatalf("modal should open focused on keep-current (1), got conflictSel=%d", m.conflictSel)
	}
}

func TestConflictConfirmStartsNewCart(t *testing.T) {
	m, _ := openSecondRestaurantWithFirstInCart(t)
	second := m.rest.PlaceData().Name

	m = addFocused(m)                               // select + ↑ -> trigger conflict
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft}) // focus "start new"
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // confirm
	m = u.(Model)

	if m.conflictOpen {
		t.Fatal("confirm should close the modal")
	}
	if m.cartRestaurant != second {
		t.Fatalf("new cart restaurant should be %q, got %q", second, m.cartRestaurant)
	}
	if len(m.lines) != 1 {
		t.Fatalf("new cart should hold exactly the one new item, got %d lines", len(m.lines))
	}
}

func TestConflictEnterOnDefaultKeepsCart(t *testing.T) {
	m, first := openSecondRestaurantWithFirstInCart(t)
	before := len(m.lines)

	m = addFocused(m) // select + ↑ -> trigger conflict
	// default focus is "keep current"; a reflexive Enter must not wipe the cart.
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)

	if m.conflictOpen {
		t.Fatal("enter should close the modal")
	}
	if m.cartRestaurant != first {
		t.Fatalf("default-focus enter must keep restaurant %q, got %q", first, m.cartRestaurant)
	}
	if len(m.lines) != before {
		t.Fatalf("default-focus enter must leave the cart untouched: was %d, now %d", before, len(m.lines))
	}
}

func TestConflictCancelKeepsCart(t *testing.T) {
	m, first := openSecondRestaurantWithFirstInCart(t)
	before := len(m.lines)

	m = addFocused(m)                              // select + ↑ -> trigger conflict
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc}) // cancel
	m = u.(Model)

	if m.conflictOpen {
		t.Fatal("esc should close the modal")
	}
	if m.cartRestaurant != first {
		t.Fatalf("cancel must keep the original cart restaurant %q, got %q", first, m.cartRestaurant)
	}
	if len(m.lines) != before {
		t.Fatalf("cancel must leave the cart untouched: was %d, now %d", before, len(m.lines))
	}
}

func TestSameRestaurantNoConflict(t *testing.T) {
	m := newAtMenu()
	step := func(k tea.KeyMsg) { u, _ := m.Update(k); m = u.(Model) }
	step(tea.KeyMsg{Type: tea.KeyEnter}) // open Blue Tokai
	step(tea.KeyMsg{Type: tea.KeyEnter}) // add first item
	step(tea.KeyMsg{Type: tea.KeyEnter}) // add again, same restaurant

	if m.conflictOpen {
		t.Fatal("adding from the same restaurant must not open the modal")
	}
}

// TestSnacksCrossPlaceNoConflict verifies that adding items from two different
// snack places does not open the conflict modal — the whole snacks section
// shares one cart.
func TestSnacksCrossPlaceNoConflict(t *testing.T) {
	m := newAtMenu()
	step := func(k tea.KeyMsg) { u, _ := m.Update(k); m = u.(Model) }

	// Navigate to Snacks tab (press "3")
	step(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	if m.section != catalog.SectionSnacks {
		t.Fatal("setup: expected snacks section")
	}

	// Open first snack place and add an item
	step(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != scrRestaurant {
		t.Fatal("setup: expected restaurant screen")
	}
	step(tea.KeyMsg{Type: tea.KeyEnter}) // add first item
	first := m.cartRestaurant
	if first == "" || len(m.lines) == 0 {
		t.Fatalf("setup: expected item in cart from %q", first)
	}

	// Back to menu, move to second snack place, open it
	step(tea.KeyMsg{Type: tea.KeyEsc})
	step(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	step(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != scrRestaurant {
		t.Fatal("setup: expected second restaurant screen")
	}
	second := m.rest.PlaceData().Name
	if second == first {
		t.Skipf("only one snack place available at this address (%q), skipping cross-place test", first)
	}

	// Add from second snack place — must NOT conflict
	step(tea.KeyMsg{Type: tea.KeyEnter})

	if m.conflictOpen {
		t.Fatal("adding from a different snack place must NOT open the conflict modal")
	}
	if len(m.lines) < 2 {
		t.Fatalf("expected at least 2 cart lines after adding from 2 snack places, got %d", len(m.lines))
	}
}

// TestSnacksToFoodConflicts verifies that switching from a snacks cart to a food
// restaurant DOES trigger the conflict modal.
func TestSnacksToFoodConflicts(t *testing.T) {
	m := newAtMenu()
	step := func(k tea.KeyMsg) { u, _ := m.Update(k); m = u.(Model) }

	// Add from snacks
	step(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")}) // snacks tab
	step(tea.KeyMsg{Type: tea.KeyEnter})                     // open first snack place
	if m.screen != scrRestaurant {
		t.Fatal("setup: expected restaurant screen")
	}
	step(tea.KeyMsg{Type: tea.KeyEnter}) // add item
	if len(m.lines) == 0 {
		t.Fatal("setup: expected item in snacks cart")
	}

	// Switch to food tab and try to add
	step(tea.KeyMsg{Type: tea.KeyEsc})                       // back to menu
	step(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")}) // food tab
	step(tea.KeyMsg{Type: tea.KeyEnter})                     // open first food place
	if m.screen != scrRestaurant {
		t.Fatal("setup: expected food restaurant screen")
	}
	step(tea.KeyMsg{Type: tea.KeyEnter}) // try to add (may open customize modal if item has add-ons)
	if m.customizeOpen {
		// The selected item is customizable — confirm the customize modal so
		// commitAdd is reached and the conflict check fires.
		step(tea.KeyMsg{Type: tea.KeyEnter})
	}

	if !m.conflictOpen {
		t.Fatal("adding from food restaurant when snacks cart is active must open conflict modal")
	}
}

// TestUsualCrossRestaurantOpensConflict exercises the menu "usual" add-path
// through the conflict modal (the restaurant add-path is covered elsewhere). A
// cart seeded from a different restaurant, then pressing "u" (whose usual is a
// Blue Tokai item) must open the modal focused on keep-current, cart untouched.
func TestUsualCrossRestaurantOpensConflict(t *testing.T) {
	m := newAtMenu()
	step := func(k tea.KeyMsg) { u, _ := m.Update(k); m = u.(Model) }

	step(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // 2nd coffee place
	step(tea.KeyMsg{Type: tea.KeyEnter})                     // open it
	second := m.rest.PlaceData().Name
	step(tea.KeyMsg{Type: tea.KeyEnter}) // add its first item
	step(tea.KeyMsg{Type: tea.KeyEsc})   // back to menu
	if len(m.lines) == 0 || m.cartRestaurant != second {
		t.Fatalf("setup: expected cart seeded from %q, got %d lines / %q", second, len(m.lines), m.cartRestaurant)
	}

	before := len(m.lines)
	step(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")}) // usual belongs to Blue Tokai

	if !m.conflictOpen {
		t.Fatal("pressing u with a different restaurant in the cart should open the conflict modal")
	}
	if m.conflictSel != 1 {
		t.Fatalf("usual conflict should open focused on keep-current (1), got %d", m.conflictSel)
	}
	if m.cartRestaurant != second {
		t.Fatalf("cart restaurant must stay %q while modal open, got %q", second, m.cartRestaurant)
	}
	if len(m.lines) != before {
		t.Fatalf("cart must be untouched while modal open: was %d, now %d", before, len(m.lines))
	}
}

func TestRestaurantCGoesStraightToCheckout(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	place := catalog.Place{ID: "r1", SwiggyID: "r1", Name: "Blue Tokai",
		Items: []catalog.Item{{ID: "i1", SwiggyID: "i1", Name: "Latte", Price: 250}}}
	snap.SetMenu(place)
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "acct-1", ""), WithSeededSnapshot())
	m.w, m.h = 100, 40
	m.screen = scrRestaurant
	m.rest = screens.NewRestaurant(place, map[string]int{}, "").WithAddr(m.addr)
	m.lines = []screens.CartLine{{Item: catalog.Item{ID: "i1", Name: "Latte", Price: 250}, Qty: 1}}
	m.cartRestaurant = "Blue Tokai"

	nm, _ := m.Update(keyRunes("c"))
	m = nm.(Model)
	if m.screen != scrCheckout {
		t.Fatalf("`c` must open the merged checkout directly, got screen %v", m.screen)
	}
}
