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

		// Two fixed-width right columns so rating AND price line up vertically
		// down the whole list (the list right-aligns the Right block as a unit,
		// so each sub-cell must be a constant width to stay columnar).
		ratingW := lipgloss.Width("★0.0")
		ratingCell := strings.Repeat(" ", ratingW) // blank when the dish has no rating
		if it.Rating > 0 {
			ratingCell = theme.GoldStyle.Render(fmt.Sprintf("★%.1f", it.Rating))
		}
		priceW := lipgloss.Width("₹9999")
		priceCell := lipgloss.PlaceHorizontal(priceW, lipgloss.Right, theme.PriceStyle.Render(fmt.Sprintf("₹%d", it.Price)))

		right := ratingCell + "    " + priceCell
		if qty > 0 {
			// Stepper goes to the LEFT of the rating so the ★rating and ₹price
			// columns stay aligned with every other (non-cart) row.
			stepper := theme.FavStyle.Render("−") + " " +
				theme.GreenStyle.Render(fmt.Sprintf("×%d", qty)) + " " +
				theme.GreenStyle.Render("+")
			right = stepper + "    " + ratingCell + "    " + priceCell
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
		case "up":
			// Arrow keys browse matches WITHOUT leaving search (j/k stay typable).
			s.list.Up()
		case "down":
			s.list.Down()
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

// Filter returns the active dish-search filter ("" when none). Lets the root
// tier the esc key: clear a committed search before leaving the menu.
func (s Restaurant) Filter() string { return s.list.Filter() }

// ClearSearch drops the dish-search filter and exits search input, returning a
// copy. This is the "undo my search" path — esc clears the filter and stays on
// the menu instead of popping back to discovery.
func (s Restaurant) ClearSearch() Restaurant {
	s.searching = false
	s.list.SetFilter("")
	return s
}

func (s Restaurant) View() string {
	var b strings.Builder
	w := components.ContentWidth()

	// row 1: esc  <name>   (the delivery address now lives in the top brand bar)
	left := theme.PriceStyle.Render("esc") + "  " + theme.BrightStyle.Bold(true).Render(s.p.Name)
	b.WriteString("  " + left + "\n")

	// row 2 meta: the real live ★ rating · delivery time. (No cuisine/section — it
	// was the browse enum, not the real cuisine — and no item count, which reads 0
	// until the menu streams in.)
	dot := theme.FaintStyle.Render("  ·  ")
	meta := theme.GoldStyle.Render(fmt.Sprintf("★ %.1f", s.p.Rating))
	if s.p.ETA != "" {
		meta += dot + theme.DimStyle.Render(s.p.ETA)
	}
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
	// The cart chip now lives in the top brand bar; the category bar gets the
	// full width (minus the veg indicator when shown).
	budget := w - lipgloss.Width(veg) - 4
	if budget < 12 {
		budget = 12
	}
	catBar := s.categoryBar(budget) + veg
	b.WriteString("  " + catBar + "\n")

	// search input line + live result count (when active)
	if s.searching || s.list.Filter() != "" {
		b.WriteString("  " + s.searchLine() + "\n")
	}

	b.WriteString("\n")
	// Empty state: a filter that matches nothing reads better as a message than
	// a blank void where the list would be.
	if (s.searching || s.list.Filter() != "") && len(s.list.VisibleRows()) == 0 {
		b.WriteString("  " + theme.DimStyle.Render(fmt.Sprintf("no dishes match %q", s.list.Filter())) + "\n")
	} else {
		b.WriteString(s.list.View())
	}
	b.WriteString("\n")
	// Search mode gets its own focused hint; otherwise the full key set,
	// now including / to enter dish search.
	if s.searching {
		b.WriteString(components.Hint("type", "filter", "↑↓", "browse", "↵", "done", "esc", "clear"))
	} else {
		// ↑/↓ move between dishes; ↵/+ add the focused dish and − removes a unit.
		b.WriteString(components.Hint("↑↓", "move", "↵/+", "add", "−", "remove", "←→", "category", "/", "search", "c", "cart", "esc", "back"))
	}
	return b.String()
}

// searchLine renders the dish-search input: a ⌕ prompt with a block caret while
// typing, plus a live match count — mirroring the discovery search so the two
// feel like one system. When a filter is set but not actively typed, it shows
// the count and a "/ edit" affordance instead of the caret.
func (s Restaurant) searchLine() string {
	f := s.list.Filter()
	n := len(s.list.VisibleRows())
	if s.searching {
		caret := lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.Bg)).
			Background(lipgloss.Color(theme.Cursor)).Render(" ")
		line := theme.CursorStyle.Render("⌕ "+f) + caret
		if f == "" {
			return line + theme.FaintStyle.Render("   type to filter dishes")
		}
		return line + theme.FaintStyle.Render("   "+plural(n, "dish", "dishes"))
	}
	return theme.DimStyle.Render("⌕ "+f) +
		theme.FaintStyle.Render("   "+plural(n, "dish", "dishes")+" · / edit · esc clear")
}

// InfoView renders the centered item-detail modal for the selected dish. It
// returns "" when closed or nothing is selected; the root centers it over the
// viewport (like the customise/conflict modals). Real Swiggy fields only —
// nothing inferred or faked.
func (s Restaurant) InfoView(int) string {
	if !s.infoOpen {
		return ""
	}
	it, ok := s.Selected()
	if !ok {
		return ""
	}
	const cardW = 52
	inner := cardW - 4

	// badge row: veg/non-veg · ★rating · ₹price · kcal (rating/kcal omitted at 0)
	veg := theme.GreenStyle.Render("🟢 veg")
	if !it.Veg {
		veg = theme.FavStyle.Render("🔴 non-veg")
	}
	badge := []string{veg}
	if it.Rating > 0 {
		badge = append(badge, theme.GoldStyle.Render(fmt.Sprintf("★ %.1f", it.Rating)))
	}
	badge = append(badge, theme.BrightStyle.Render(fmt.Sprintf("₹%d", it.Price)))
	if it.Kcal > 0 {
		badge = append(badge, theme.DimStyle.Render(fmt.Sprintf("%d kcal", it.Kcal)))
	}
	badgeRow := strings.Join(badge, theme.FaintStyle.Render("    "))

	// real description, word-wrapped to the inner width
	descText := it.Desc
	if strings.TrimSpace(descText) == "" {
		descText = "no description available"
	}
	wrapped := lipgloss.NewStyle().Width(inner).Render(descText)

	// footer meta: category · serves 1 (category omitted when unknown)
	foot := ""
	if it.Category != "" {
		foot = theme.DimStyle.Render(it.Category) + theme.FaintStyle.Render(" · ")
	}
	foot += theme.DimStyle.Render("serves 1")

	lines := []string{badgeRow, ""}
	for _, dl := range strings.Split(wrapped, "\n") {
		lines = append(lines, theme.ItemStyle.Render(dl))
	}
	lines = append(lines, "", foot)

	return modalCard(it.Name, lines, "↑↓ browse  ·  i/esc close", cardW)
}

// modalCard draws a rounded, gold-bordered card of width w with the title set
// into the top border and a centered hint set into the bottom border. It is
// self-contained (no left margin) so the root can center it in the viewport.
//
// RestaurantInfoCard renders the centered restaurant-detail modal for a place in
// the browse list ('i'). Real Swiggy fields only: ★rating · delivery time · area,
// the cuisines/cost blurb, and the live offer when present.
func RestaurantInfoCard(p catalog.Place) string {
	const cardW = 52
	inner := cardW - 4

	badge := []string{}
	if p.Rating > 0 {
		badge = append(badge, theme.GoldStyle.Render(fmt.Sprintf("★ %.1f", p.Rating)))
	}
	if p.ETA != "" {
		badge = append(badge, theme.DimStyle.Render(p.ETA))
	}
	if p.City != "" {
		badge = append(badge, theme.DimStyle.Render(p.City))
	}

	var lines []string
	if len(badge) > 0 {
		lines = append(lines, strings.Join(badge, theme.FaintStyle.Render("    ")), "")
	}
	if strings.TrimSpace(p.Description) != "" {
		wrapped := lipgloss.NewStyle().Width(inner).Render(p.Description)
		for _, dl := range strings.Split(wrapped, "\n") {
			lines = append(lines, theme.ItemStyle.Render(dl))
		}
	}
	if strings.TrimSpace(p.Offer) != "" {
		wrapped := lipgloss.NewStyle().Width(inner).Render("🎁 " + p.Offer)
		lines = append(lines, "")
		for _, dl := range strings.Split(wrapped, "\n") {
			lines = append(lines, theme.GreenStyle.Render(dl))
		}
	}
	if len(lines) == 0 {
		lines = append(lines, theme.DimStyle.Render("no details available"))
	}
	return modalCard(p.Name, lines, "↑↓ browse  ·  i/esc close", cardW)
}

// autoCard is modalCard with the width sized to the content (the widest of the
// title, body lines, or footer). Used by the taller modals (customise, wizard,
// cart conflict) whose width varies with the data.
func autoCard(title string, lines []string, footer string) string {
	w := lipgloss.Width(title) + 6
	for _, l := range lines {
		if x := lipgloss.Width(l) + 4; x > w {
			w = x
		}
	}
	if x := lipgloss.Width(footer) + 6; x > w {
		w = x
	}
	if w < 36 {
		w = 36
	}
	return modalCard(title, lines, footer, w)
}

// ╭─ <title> ─────────────╮
// │ <line>                │
// ╰──── <footer> ─────────╯
func modalCard(title string, lines []string, footer string, w int) string {
	bd := theme.Fg(theme.Gold)
	inner := w - 4
	if inner < 1 {
		inner = 1
	}

	if tr := []rune(title); len(tr) > w-6 {
		title = string(tr[:w-7]) + "…"
	}
	titleStr := theme.BrightStyle.Bold(true).Render(title)
	fill := w - 5 - lipgloss.Width(titleStr) // "╭─ "(3)+title+" "(1)+fill+"╮"(1)=w
	if fill < 0 {
		fill = 0
	}

	// One blank line of padding after the top border and before the bottom
	// border, so every modal has uniform breathing room.
	body := make([]string, 0, len(lines)+2)
	body = append(body, "")
	body = append(body, lines...)
	body = append(body, "")

	var b strings.Builder
	b.WriteString(bd.Render("╭─ ") + titleStr + bd.Render(" "+strings.Repeat("─", fill)+"╮") + "\n")
	for _, ln := range body {
		b.WriteString(bd.Render("│ ") + components.PadTo(ln, inner) + bd.Render(" │") + "\n")
	}
	footStr := theme.FaintStyle.Render(footer)
	rem := w - 4 - lipgloss.Width(footStr) // "╰"(1)+l+" "(1)+foot+" "(1)+r+"╯"(1)=w
	if rem < 2 {
		rem = 2
	}
	l := rem / 2
	b.WriteString(bd.Render("╰"+strings.Repeat("─", l)+" ") + footStr + bd.Render(" "+strings.Repeat("─", rem-l)+"╯"))
	return b.String()
}
