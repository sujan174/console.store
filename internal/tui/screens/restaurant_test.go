package screens

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"consolestore/internal/catalog"
	"consolestore/internal/catalog/mem"
)

// despace strips spaces so assertions survive the list's letter-spacing.
func despace(s string) string { return strings.ReplaceAll(s, " ", "") }

// key builds a rune key-press message (e.g. key("i")).
func key(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func TestRestaurantRendersItemsWithPrices(t *testing.T) {
	repo := mem.New()
	p, _ := repo.Menu("blue-tokai")
	r := NewRestaurant(p, map[string]int{}, "")
	out := r.View()
	if !strings.Contains(out, "Blue Tokai") {
		t.Fatal("missing restaurant name header")
	}
	if !strings.Contains(out, "35-45 min") {
		t.Fatal("missing delivery window")
	}
	if !strings.Contains(despace(out), "ColdCoffee") || !strings.Contains(despace(out), "₹149") {
		t.Fatal("missing item + price")
	}
}

func TestRestaurantSoldOutItemMarkedNotStepper(t *testing.T) {
	p := catalog.Place{Name: "Blue Tokai", ETA: "35-45 min", Items: []catalog.Item{
		{ID: "a", Name: "Available Latte", Price: 200},
		{ID: "b", Name: "Gone Mocha", Price: 250, OutOfStock: true},
	}}
	out := NewRestaurant(p, map[string]int{}, "").View()
	if !strings.Contains(despace(out), "soldout") {
		t.Fatalf("out-of-stock item should render a 'sold out' tag:\n%s", out)
	}
	// A sold-out row shows no price (it's not orderable); the available one does.
	if !strings.Contains(despace(out), "₹200") {
		t.Fatalf("available item should still show its price:\n%s", out)
	}
	if strings.Contains(despace(out), "₹250") {
		t.Fatalf("sold-out item must not show a price/stepper:\n%s", out)
	}
}

func TestRestaurantNoQuickLookCard(t *testing.T) {
	repo := mem.New()
	p, _ := repo.Menu("blue-tokai")
	out := NewRestaurant(p, map[string]int{}, "").View()
	// The quick-look card was removed; the screen must not render it.
	if strings.Contains(out, "quick look") {
		t.Errorf("quick-look card must be gone:\n%s", out)
	}
}

func TestRestaurantSelectedItem(t *testing.T) {
	repo := mem.New()
	p, _ := repo.Menu("blue-tokai")
	r := NewRestaurant(p, map[string]int{}, "")
	if got, ok := r.Selected(); !ok || got.Name != "Cold Coffee" {
		t.Fatalf("Selected() = %s (ok=%v), want Cold Coffee", got.Name, ok)
	}
}

func TestRestaurantInCartRowShowsStepper(t *testing.T) {
	p := catalog.Place{Name: "Blue Tokai", ETA: "35-45 min", Items: []catalog.Item{
		{ID: "x", Name: "Cold Coffee", Price: 149},
	}}
	s := NewRestaurant(p, map[string]int{"x": 2}, "")
	v := s.View()
	for _, want := range []string{"×2", "−", "+", "₹149"} {
		if !strings.Contains(despace(v), want) {
			t.Errorf("missing %q:\n%s", want, v)
		}
	}
}

func TestRestaurantNotInCartShowsPlainPrice(t *testing.T) {
	p := catalog.Place{Name: "X", ETA: "30 min", Items: []catalog.Item{{ID: "y", Name: "Tea", Price: 50}}}
	s := NewRestaurant(p, map[string]int{}, "")
	if strings.Contains(s.View(), "×") {
		t.Error("no stepper when not in cart")
	}
}

func TestRestaurantInfoPanelTogglesWithI(t *testing.T) {
	repo := mem.New()
	p, _ := repo.Menu("blue-tokai")
	s := NewRestaurant(p, map[string]int{}, "")

	// Closed by default: no panel.
	if s.InfoOpen() {
		t.Fatal("info panel should start closed")
	}
	if s.InfoView(60) != "" {
		t.Fatal("InfoView should be empty while closed")
	}

	// 'i' opens it.
	ns, _ := s.Update(key("i"))
	s = ns.(Restaurant)
	if !s.InfoOpen() {
		t.Fatal("'i' should open the info panel")
	}

	out := s.InfoView(80)
	// Bordered modal card (top + bottom rule) with the selected item's real data.
	for _, want := range []string{"╭", "╰", "Cold Coffee", "180 kcal", "veg", "i/esc close"} {
		if !strings.Contains(out, want) {
			t.Errorf("info modal missing %q:\n%s", want, out)
		}
	}

	// 'i' again closes it.
	ns, _ = s.Update(key("i"))
	s = ns.(Restaurant)
	if s.InfoOpen() || s.InfoView(80) != "" {
		t.Fatal("second 'i' should close the info panel")
	}
}

func TestRestaurantInfoPanelFollowsSelection(t *testing.T) {
	repo := mem.New()
	p, _ := repo.Menu("blue-tokai")
	s := NewRestaurant(p, map[string]int{}, "").WithInfo(true)

	// Move the cursor down one and the panel should describe the new item.
	ns, _ := s.Update(key("j"))
	s = ns.(Restaurant)
	sel, _ := s.Selected()
	if !strings.Contains(s.InfoView(80), sel.Name) {
		t.Errorf("info panel should describe the selected item %q", sel.Name)
	}
}

func TestPlaceHasDescription(t *testing.T) {
	repo := mem.New()
	p, _ := repo.Menu("blue-tokai")
	if p.Description == "" {
		t.Fatal("Blue Tokai Place.Description must be non-empty after seed")
	}
}

// System 4: dish rows carry an aligned rating★ column (real Item.Rating).
func TestDishRowShowsRatingColumn(t *testing.T) {
	p := catalog.Place{Name: "X", ETA: "30 min", Items: []catalog.Item{
		{ID: "a", Name: "Hot Americano", Price: 139, Rating: 4.2},
	}}
	v := NewRestaurant(p, map[string]int{}, "").View()
	if !strings.Contains(v, "4.2 ★") {
		t.Fatalf("dish row missing aligned rating column:\n%s", v)
	}
	if !strings.Contains(v, "₹139") {
		t.Fatalf("dish row missing price:\n%s", v)
	}
}

func TestRestaurantSearchUI(t *testing.T) {
	repo := mem.New()
	p, _ := repo.Menu("blue-tokai")

	// Idle: the key hint advertises "/ search".
	if !strings.Contains(NewRestaurant(p, map[string]int{}, "").View(), "search") {
		t.Fatal("idle hint should advertise / search")
	}

	// Enter search via "/", type a query: ⌕ prompt + a focused search hint.
	s := NewRestaurant(p, map[string]int{}, "")
	ns, _ := s.Update(key("/"))
	s = ns.(Restaurant)
	for _, r := range "cold" {
		ns, _ = s.Update(key(string(r)))
		s = ns.(Restaurant)
	}
	v := s.View()
	if !strings.Contains(v, "⌕") {
		t.Fatalf("search input missing ⌕ prompt:\n%s", v)
	}
	if !strings.Contains(v, "done") || !strings.Contains(v, "clear") {
		t.Fatalf("search-mode hint missing done/clear:\n%s", v)
	}
}

func TestRestaurantClearSearch(t *testing.T) {
	repo := mem.New()
	p, _ := repo.Menu("blue-tokai")
	s := NewRestaurant(p, map[string]int{}, "")
	ns, _ := s.Update(key("/"))
	s = ns.(Restaurant)
	for _, r := range "cold" {
		ns, _ = s.Update(key(string(r)))
		s = ns.(Restaurant)
	}
	// Enter commits the search: input mode off, filter retained.
	ns, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	s = ns.(Restaurant)
	if s.Searching() {
		t.Fatal("enter should exit search input")
	}
	if s.Filter() == "" {
		t.Fatal("enter should retain the committed filter")
	}
	// ClearSearch undoes it.
	s = s.ClearSearch()
	if s.Filter() != "" {
		t.Fatalf("ClearSearch should drop the filter, got %q", s.Filter())
	}
}
