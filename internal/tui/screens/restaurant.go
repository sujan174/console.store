package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"console.store/internal/catalog"
	"console.store/internal/tui/components"
	"console.store/internal/tui/theme"
)

type Restaurant struct {
	p         catalog.Place
	addr      catalog.Address
	cartChip  string
	list      components.List
	searching bool
	infoOpen  bool // 'i' toggles the detail panel for the selected item

	category string // active category filter; "" or "All" = no filter
	vegOnly  bool
	qtyByID  map[string]int // cart quantities (for rebuilding rows after filter change)
}

// buildRows converts a slice of catalog items into display rows using the
// given cart quantity map.
func buildRows(items []catalog.Item, qtyByItemID map[string]int) []components.Row {
	rows := make([]components.Row, 0, len(items))
	for _, it := range items {
		qty := qtyByItemID[it.ID]

		// in-cart items read brighter; the green left-bar + stepper already
		// signal "in cart", so no extra ✓ column (keeps the cursor→name gap
		// identical to the menu).
		nameStyle := theme.ItemStyle
		if qty > 0 {
			nameStyle = theme.BrightStyle
		}
		left := nameStyle.Render(it.Name)

		price := theme.PriceStyle.Render(fmt.Sprintf("₹%d", it.Price))
		right := price
		if qty > 0 {
			stepper := theme.FavStyle.Render("−") + " " +
				theme.GreenStyle.Render(fmt.Sprintf("×%d", qty)) + " " +
				theme.GreenStyle.Render("+") + "   "
			right = stepper + price
		}

		rows = append(rows, components.Row{Left: left, Right: right, BarGreen: qty > 0})
	}
	return rows
}

// NewRestaurant builds the restaurant screen, rendering in-cart checks and
// inline qty steppers from the current cart quantities (keyed by item ID).
func NewRestaurant(p catalog.Place, qtyByItemID map[string]int, cartChip string) Restaurant {
	s := Restaurant{p: p, cartChip: cartChip, qtyByID: qtyByItemID}
	s.list.Rows = buildRows(p.Items, qtyByItemID)
	return s
}

// quickLook renders the ╭─ quick look ─╮ card shown below the item list.
// Returns "" when the place has no items (no popular pick to show).
func (s Restaurant) quickLook(w int) string {
	top, ok := s.topItem()
	if !ok {
		return ""
	}

	inner := w - 4
	if inner < 1 {
		inner = 1
	}

	var lines []string

	if s.p.Description != "" {
		desc := s.p.Description
		if r := []rune(desc); len(r) > inner {
			desc = string(r[:inner-1]) + "…"
		}
		lines = append(lines, theme.DimStyle.Render(desc))
	}

	popular := theme.GoldStyle.Render("★ popular") +
		"   " +
		theme.BrightStyle.Render(top.Name) +
		theme.DimStyle.Render(fmt.Sprintf(" · ₹%d", top.Price))
	lines = append(lines, popular)

	title := theme.FaintStyle.Render("quick look")
	return infoBox(title, lines, w)
}

// topItem returns the highest-rated item, used as the popular pick in the quick-look card.
func (s Restaurant) topItem() (catalog.Item, bool) {
	if len(s.p.Items) == 0 {
		return catalog.Item{}, false
	}
	best := s.p.Items[0]
	for _, it := range s.p.Items[1:] {
		if it.Rating > best.Rating {
			best = it
		}
	}
	return best, true
}

// vegCount is the number of vegetarian items on the menu.
func (s Restaurant) vegCount() int {
	n := 0
	for _, it := range s.p.Items {
		if it.Veg {
			n++
		}
	}
	return n
}

func (s Restaurant) Selected() (catalog.Item, bool) {
	// s.list.Rows holds the category+veg filtered items (as display rows).
	// s.list.SelectedIndex() resolves the cursor (which may be further narrowed
	// by the list's search filter) back to an index into s.list.Rows, which
	// corresponds 1:1 with categoryVegItems().
	i := s.list.SelectedIndex()
	items := s.categoryVegItems()
	if i < 0 || i >= len(items) {
		return catalog.Item{}, false
	}
	return items[i], true
}

func (s Restaurant) WithCartChip(c string) Restaurant { s.cartChip = c; return s }

// WithAddr sets the delivery address shown in the header.
func (s Restaurant) WithAddr(a catalog.Address) Restaurant { s.addr = a; return s }

// WithMaxRows sets the list viewport height (rows). 0 = show all.
func (s Restaurant) WithMaxRows(n int) Restaurant { s.list.MaxRows = n; return s }

// PlaceData returns the underlying place (used by the app router).
func (s Restaurant) PlaceData() catalog.Place { return s.p }

// CursorIndex returns the current list cursor so the router can preserve it
// across a rebuild (NewRestaurant resets the cursor to 0).
func (s Restaurant) CursorIndex() int { return s.list.Cursor }

// WithCursor restores a previously captured cursor position.
func (s Restaurant) WithCursor(i int) Restaurant { s.list.Cursor = i; return s }

// InfoOpen reports whether the detail panel is showing (so the router can
// preserve it across a NewRestaurant rebuild).
func (s Restaurant) InfoOpen() bool { return s.infoOpen }

// WithInfo restores the detail-panel open/closed state.
func (s Restaurant) WithInfo(open bool) Restaurant { s.infoOpen = open; return s }

// Categories returns "All" followed by the distinct item categories in menu order.
func (s Restaurant) Categories() []string {
	out := []string{"All"}
	seen := map[string]bool{}
	for _, it := range s.p.Items {
		c := it.Category
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out
}

// ActiveCategory returns the currently active category filter (empty = "All").
func (s Restaurant) ActiveCategory() string { return s.category }

// activeCategoryIndex is the position of the active category within Categories()
// ("All" is index 0 when no filter is set).
func (s Restaurant) activeCategoryIndex(cats []string) int {
	for i, c := range cats {
		if (c == "All" && s.category == "") || c == s.category {
			return i
		}
	}
	return 0
}

// categoryBar renders the top-nav category row as a horizontal window centred on
// the active category, so a long category list stays navigable: the active chip
// is always visible, with ‹ / › markers when categories are hidden off either
// side. budget is the character width available for the categories.
func (s Restaurant) categoryBar(budget int) string {
	cats := s.Categories()
	return windowedBar(cats, s.activeCategoryIndex(cats), budget, " · ")
}

// CategoryBarForTest exposes the windowed category bar for unit tests.
func (s Restaurant) CategoryBarForTest(budget int) string { return s.categoryBar(budget) }

// WithCategory sets the active category filter. "" or "All" = no filter.
func (s Restaurant) WithCategory(cat string) Restaurant {
	if cat == "All" {
		cat = ""
	}
	s.category = cat
	s.list.Cursor = 0
	s.list.SetFilter("") // clear search when changing category
	s.list.Rows = buildRows(s.categoryVegItems(), s.qtyByID)
	return s
}

// NextCategory advances to the next category, clamping at the last.
func (s Restaurant) NextCategory() Restaurant { return s.stepCategory(1) }

// PrevCategory retreats to the previous category (clamps at start).
func (s Restaurant) PrevCategory() Restaurant { return s.stepCategory(-1) }

func (s Restaurant) stepCategory(d int) Restaurant {
	cats := s.Categories()
	cur := 0
	for i, c := range cats {
		if (c == "All" && s.category == "") || c == s.category {
			cur = i
			break
		}
	}
	cur += d
	if cur < 0 {
		cur = 0
	}
	if cur >= len(cats) {
		cur = len(cats) - 1
	}
	return s.WithCategory(cats[cur])
}

// VegOnly reports whether the veg-only filter is active.
func (s Restaurant) VegOnly() bool { return s.vegOnly }

// WithVegOnly sets the veg-only filter and resets the cursor.
func (s Restaurant) WithVegOnly(v bool) Restaurant {
	s.vegOnly = v
	s.list.Cursor = 0
	s.list.SetFilter("") // clear search when toggling veg filter
	s.list.Rows = buildRows(s.categoryVegItems(), s.qtyByID)
	return s
}

// categoryVegItems returns items after applying the category and veg-only filters
// (but NOT the search filter). Used to populate s.list.Rows so that the list's
// own filter handles the search term, keeping cursor navigation consistent.
func (s Restaurant) categoryVegItems() []catalog.Item {
	out := []catalog.Item{}
	for _, it := range s.p.Items {
		if s.category != "" && it.Category != s.category {
			continue
		}
		if s.vegOnly && !it.Veg {
			continue
		}
		out = append(out, it)
	}
	return out
}

// visibleItems applies the category, veg-only, and dish-search filters.
// The list's rows are always the category+veg subset; the list.Filter() (search
// term) is then applied on top. This function reconstructs the full visible set
// from the underlying items so Selected() can index it correctly.
func (s Restaurant) visibleItems() []catalog.Item {
	search := strings.ToLower(s.list.Filter())
	out := []catalog.Item{}
	for _, it := range s.p.Items {
		if s.category != "" && it.Category != s.category {
			continue
		}
		if s.vegOnly && !it.Veg {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(it.Name), search) {
			continue
		}
		out = append(out, it)
	}
	return out
}

// VisibleNamesForTest exposes the filtered item names for unit tests.
func (s Restaurant) VisibleNamesForTest() []string {
	out := []string{}
	for _, it := range s.visibleItems() {
		out = append(out, it.Name)
	}
	return out
}

func (s Restaurant) Init() tea.Cmd { return nil }

func (s Restaurant) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	if s.searching {
		switch k.String() {
		case "esc":
			s.searching = false
			s.list.SetFilter("")
		case "enter":
			s.searching = false
		case "backspace":
			f := s.list.Filter()
			if f != "" {
				s.list.SetFilter(f[:len(f)-1])
			}
		default:
			if k.Type == tea.KeyRunes {
				s.list.SetFilter(s.list.Filter() + string(k.Runes))
			}
		}
		return s, nil
	}
	switch k.String() {
	case "/":
		s.searching = true
	case "i":
		s.infoOpen = !s.infoOpen
	case "j", "down":
		s.list.Down()
	case "k", "up":
		s.list.Up()
	}
	return s, nil
}

// Searching reports whether the restaurant is in search-input mode.
func (s Restaurant) Searching() bool { return s.searching }

func (s Restaurant) View() string {
	var b strings.Builder
	w := components.ContentWidth()

	// row 1: esc  <name>            deliver to ⊕ <addr>
	left := theme.PriceStyle.Render("esc") + "  " + theme.BrightStyle.Bold(true).Render(s.p.Name)
	right := theme.DimStyle.Render("deliver to ") + theme.CursorStyle.Render("⊕ ") + theme.BrightStyle.Render(s.addr.Line)
	b.WriteString("  " + justify(left, right, w) + "\n")

	// row 2 meta: ★ 4.6 · 35-45 min · coffee · 10 items
	dot := theme.FaintStyle.Render("  ·  ")
	meta := theme.GoldStyle.Render(fmt.Sprintf("★ %.1f", s.p.Rating)) + dot +
		theme.DimStyle.Render(s.p.ETA) + dot +
		theme.DimStyle.Render(string(s.p.Section)) + dot +
		theme.DimStyle.Render(fmt.Sprintf("%d items", len(s.p.Items)))
	b.WriteString("  " + meta + "\n")

	b.WriteString("\n")

	// category filter bar: ‹ <Cat> · <Cat*> · <Cat> ›   [veg ●]   🛒 chip
	// The bar scrolls horizontally to keep the active category in view when the
	// menu has more categories than fit on one line. The veg indicator only shows
	// while veg-only is on; otherwise the full width goes to the category bar.
	veg := ""
	if s.vegOnly {
		veg = "   " + theme.GreenStyle.Render("veg ●")
	}
	cartStyle := theme.CartStyle
	if strings.Contains(s.cartChip, "empty") {
		cartStyle = theme.DimStyle
	}
	chip := cartStyle.Render(s.cartChip)
	// Budget for the scrolling categories = full width minus the right-aligned
	// cart chip, the veg indicator (when shown), and the leading/trailing margins.
	budget := w - lipgloss.Width(chip) - lipgloss.Width(veg) - 4
	if budget < 12 {
		budget = 12
	}
	catBar := s.categoryBar(budget) + veg
	b.WriteString("  " + justify(catBar, chip, w) + "\n")

	// search prompt (when active)
	if s.searching || s.list.Filter() != "" {
		b.WriteString("  " + theme.CursorStyle.Render("/"+s.list.Filter()) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(s.list.View())
	b.WriteString("\n")
	if ql := s.quickLook(w); ql != "" {
		b.WriteString(ql + "\n")
	}
	// ↑/↓ always move between dishes; ↵/+ add the focused dish and − removes a
	// unit (− to zero drops it from the cart).
	b.WriteString(components.Hint("↑↓", "move", "↵/+", "add", "−", "remove", "←→", "category", "c", "cart", "esc", "back"))
	return b.String()
}

// InfoView renders the bordered detail panel for the currently selected item,
// shown above the keyboard hints when the user presses 'i'. It returns "" when
// the panel is closed or nothing is selected.
//
// The richer fields (allergens, spice, prep, serving) are dummy data derived
// from the item name/description for now — they'll come from the live Swiggy
// menu once the integration lands.
func (s Restaurant) InfoView(w int) string {
	if !s.infoOpen {
		return ""
	}
	it, ok := s.Selected()
	if !ok {
		return ""
	}

	dot := theme.FaintStyle.Render("  ·  ")
	inner := w - 4
	if inner < 1 {
		inner = 1
	}

	desc := it.Desc
	if desc == "" {
		desc = "no description available"
	}
	if r := []rune(desc); len(r) > inner {
		desc = string(r[:inner-1]) + "…"
	}

	veg := theme.GreenStyle.Render("veg")
	if !it.Veg {
		veg = theme.FavStyle.Render("non-veg")
	}
	kcal := theme.DimStyle.Render("— kcal")
	if it.Kcal > 0 {
		kcal = theme.DimStyle.Render(fmt.Sprintf("%d kcal", it.Kcal))
	}
	stats := theme.GoldStyle.Render(fmt.Sprintf("★ %.1f", it.Rating)) + dot +
		kcal + dot + veg + dot + theme.DimStyle.Render("serves 1")

	label := func(k, v string) string {
		return theme.DimStyle.Render(k+" · ") + theme.FaintStyle.Render(v)
	}
	row2 := label("allergens", itemAllergens(it)) + dot + label("spice", itemSpice(it))
	row3 := label("prep", itemPrep(it)) + dot + label("portion", "regular")

	lines := []string{
		theme.ItemStyle.Render(desc),
		"",
		stats,
		row2,
		row3,
	}
	title := theme.FaintStyle.Render("details") + theme.DimStyle.Render(" · ") + theme.BrightStyle.Render(it.Name)
	return infoBox(title, lines, w)
}

// infoBox renders a rounded titled card with multiple body lines spanning
// width w. Its top and bottom borders separate the detail panel from the list
// above and the keyboard hints below.
//
//	╭─ <title> ───────────────────╮
//	│ <line 1>                    │
//	│ <line 2>                    │
//	╰─────────────────────────────╯
func infoBox(title string, lines []string, w int) string {
	bd := theme.Fg(theme.Div2)
	topUsed := lipgloss.Width("╭─ ") + lipgloss.Width(title) + lipgloss.Width(" ") + 1
	fill := w - topUsed
	if fill < 0 {
		fill = 0
	}
	inner := w - 4
	if inner < 0 {
		inner = 0
	}

	var b strings.Builder
	b.WriteString("  " + bd.Render("╭─ ") + title + bd.Render(" "+strings.Repeat("─", fill)+"╮") + "\n")
	for _, ln := range lines {
		b.WriteString("  " + bd.Render("│ ") + components.PadTo(ln, inner) + bd.Render(" │") + "\n")
	}
	b.WriteString("  " + bd.Render("╰"+strings.Repeat("─", w-2)+"╯"))
	return b.String()
}

// itemBlob is the lowercased name + description used to infer the dummy detail
// fields below.
func itemBlob(it catalog.Item) string {
	return strings.ToLower(it.Name + " " + it.Desc)
}

// itemAllergens infers a dummy allergen list from the item's name/description.
func itemAllergens(it catalog.Item) string {
	s := itemBlob(it)
	groups := []struct {
		allergen string
		keys     []string
	}{
		{"dairy", []string{"milk", "cream", "cheese", "latte", "mocha", "chai", "yogurt", "butter", "oat", "parfait", "fudge", "brownie", "cappuccino", "cortado", "flat white", "horchata", "cotija"}},
		{"nuts", []string{"almond", "walnut", "hazelnut", "peanut", "cashew", "nut"}},
		{"gluten", []string{"bread", "croissant", "bun", "toast", "sandwich", "muffin", "cake", "loaf", "brownie", "burrito", "taco", "nacho", "quesadilla", "churro", "cookie", "poppers", "wheat"}},
		{"egg", []string{"egg", "mayo"}},
		{"soy", []string{"soy", "tofu"}},
	}
	var out []string
	for _, g := range groups {
		for _, k := range g.keys {
			if strings.Contains(s, k) {
				out = append(out, g.allergen)
				break
			}
		}
	}
	if len(out) == 0 {
		return "none listed"
	}
	return strings.Join(out, ", ")
}

// itemSpice infers a dummy spice level from the item's name/description.
func itemSpice(it catalog.Item) string {
	s := itemBlob(it)
	for _, k := range []string{"jalape", "chilli", "chili", "peri", "spicy", "masala", "pepper", "salsa"} {
		if strings.Contains(s, k) {
			return "medium"
		}
	}
	return "mild"
}

// itemPrep returns a dummy prep-time window, stable per item name.
func itemPrep(it catalog.Item) string {
	lo := 8 + len(it.Name)%6
	return fmt.Sprintf("%d-%d min", lo, lo+5)
}
