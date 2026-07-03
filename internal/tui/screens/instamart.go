package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"

	"consolestore/internal/catalog"
	"consolestore/internal/tui/components"
	"consolestore/internal/tui/theme"
)

// InstamartETA is the honest fast-lane window.
const InstamartETA = "~12 min"

type Instamart struct {
	items    []catalog.Item
	cartChip string
	list     components.List
	loading  bool // an IMProducts load is in flight (browse or search)

	// search state — submit-only (mirrors the menu search box's visuals).
	searchActive bool
	searchQuery  string
	searchCaret  int

	// two-pane rail fields — mirror Menu's hasRail/rail/railFocus exactly, so
	// the Instamart browse is visually and behaviorally identical to Food.
	rail      Rail
	hasRail   bool
	railFocus bool
}

// NewInstamart builds the Instamart fast-lane screen, rendering in-cart checks
// and inline qty steppers from the current cart quantities (keyed by item ID),
// mirroring the restyled restaurant screen.
func NewInstamart(items []catalog.Item, qtyByItemID map[string]int, cartChip string) Instamart {
	rows := make([]components.Row, 0, len(items))
	for _, it := range items {
		rows = append(rows, itemRow(it, qtyByItemID[it.ID]))
	}
	return Instamart{items: items, cartChip: cartChip, list: components.List{Rows: rows}}
}

// itemRow renders one product row (name/desc/sold-out + price/stepper),
// factored out of NewInstamart so the two-pane rail path can reuse it.
func itemRow(it catalog.Item, qty int) components.Row {
	name := theme.ItemStyle.Render(it.Name)
	if qty > 0 {
		name = theme.BrightStyle.Render(it.Name)
	}
	if it.Desc != "" {
		name += theme.FaintStyle.Render("  " + it.Desc)
	}
	if it.OutOfStock {
		name = theme.FaintStyle.Render(it.Name) + theme.FavStyle.Render("  · sold out")
	}

	price := theme.PriceStyle.Render(fmt.Sprintf("₹%d", it.Price))
	right := price
	if qty > 0 {
		stepper := theme.FavStyle.Render("−") + " " +
			theme.GreenStyle.Render(fmt.Sprintf("×%d", qty)) + " " +
			theme.GreenStyle.Render("+") + "   "
		right = stepper + price
	}

	return components.Row{Left: name, Right: right, BarGreen: qty > 0}
}

// WithLoading marks a browse/search load in flight (shows "loading…" instead
// of the list or empty hint).
func (s Instamart) WithLoading(v bool) Instamart { s.loading = v; return s }

// WithSearch renders a submit-only search input in the header area: query is
// the typed (not-yet-submitted) text, caret its rune position, active whether
// search mode is currently capturing keys.
func (s Instamart) WithSearch(query string, caret int, active bool) Instamart {
	s.searchQuery = query
	s.searchCaret = caret
	s.searchActive = active
	return s
}

// WithRail attaches the left rail (Search, Usuals, categories) enabling the
// two-pane render path — mirrors Menu.WithRail exactly.
func (s Instamart) WithRail(r Rail) Instamart { s.rail = r; s.hasRail = true; return s }

// WithRailFocus sets whether the rail column has keyboard focus (the product
// list is focused when false).
func (s Instamart) WithRailFocus(f bool) Instamart { s.railFocus = f; return s }

// searchInputLine renders the 🔍 search field with a block caret, matching the
// menu screen's search box exactly.
func (s Instamart) searchInputLine() string {
	r := []rune(s.searchQuery)
	c := s.searchCaret
	if c < 0 {
		c = 0
	}
	if c > len(r) {
		c = len(r)
	}
	before := string(r[:c])
	at := " "
	after := ""
	if c < len(r) {
		at = string(r[c])
		after = string(r[c+1:])
	}
	caret := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Bg)).
		Background(lipgloss.Color(theme.Cursor)).
		Render(at)
	return theme.CursorStyle.Render("⌕ "+before) + caret + theme.CursorStyle.Render(after)
}

func (s Instamart) Selected() (catalog.Item, bool) {
	i := s.list.Cursor
	if !s.hasRail {
		i = s.list.SelectedIndex()
	}
	if i < 0 || i >= len(s.items) {
		return catalog.Item{}, false
	}
	return s.items[i], true
}

func (s Instamart) WithCartChip(c string) Instamart { s.cartChip = c; return s }

// WithMaxRows sets the list viewport height (rows). 0 = show all.
func (s Instamart) WithMaxRows(n int) Instamart { s.list.MaxRows = n; return s }

// CursorIndex returns the current list cursor so the router can preserve it
// across a rebuild (NewInstamart resets the cursor to 0).
func (s Instamart) CursorIndex() int { return s.list.Cursor }

// WithCursor restores a previously captured cursor position.
func (s Instamart) WithCursor(i int) Instamart { s.list.Cursor = i; return s }

// WithListCursor sets the list cursor directly, clamped to the item count —
// mirrors Menu.WithListCursor for the two-pane rail path.
func (s Instamart) WithListCursor(i int) Instamart {
	if i < 0 {
		i = 0
	}
	if len(s.items) > 0 && i >= len(s.items) {
		i = len(s.items) - 1
	}
	s.list.Cursor = i
	return s
}

func (s Instamart) Init() tea.Cmd { return nil }

func (s Instamart) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "j", "down":
			s.list.Down()
		case "k", "up":
			s.list.Up()
		}
	}
	return s, nil
}

// browseRows renders the windowed product rows, capped to budget rows so the
// page never overflows the viewport — mirrors Menu.browseRows.
func (s Instamart) browseRows(budget int) string {
	if len(s.items) == 0 {
		if s.loading {
			return "  " + theme.GoldStyle.Render("loading…") + "\n"
		}
		return "  " + theme.DimStyle.Render("no usuals yet — press / to search") + "\n"
	}
	start, end, above, below := windowRange(s.list.Cursor, len(s.items), budget)

	var b strings.Builder
	if above > 0 {
		b.WriteString("  " + theme.FaintStyle.Render(fmt.Sprintf("↑ %d more", above)) + "\n")
	}
	for i := start; i < end; i++ {
		b.WriteString(s.productRow(s.items[i], i == s.list.Cursor) + "\n")
	}
	if below > 0 {
		b.WriteString("  " + theme.FaintStyle.Render(fmt.Sprintf("↓ %d more", below)) + "\n")
	}
	return b.String()
}

// productRow renders one product row for the two-pane list, matching
// placeRow's cursor/selection styling (▌ > lead + bright name when the list
// has focus; a faint · marker when focus is on the rail). The qty
// stepper/price on the right is pulled from the pre-built row (list.Rows,
// 1:1 with items) so it reflects the caller's qtyByItemID exactly as the
// single-pane path renders it.
func (s Instamart) productRow(it catalog.Item, selected bool) string {
	w := components.ContentWidth() - railWidth - 5
	if w < 16 {
		w = 16
	}
	const leadW = 4
	base := itemRow(it, 0)
	// Steppers/qty come from the pre-built row (list.Rows), which already
	// encodes qty>0 styling — reuse its Right side verbatim so the two-pane
	// list shows the same stepper as the single-pane path.
	if idx := s.itemIndex(it); idx >= 0 && idx < len(s.list.Rows) {
		base.Right = s.list.Rows[idx].Right
	}
	name := railTrunc2(it.Name, w-leadW-lipgloss.Width(base.Right)-1)
	styledName := theme.ItemStyle.Render(name)
	var lead string
	switch {
	case selected && !s.railFocus:
		lead = theme.CursorStyle.Render("▌ > ")
		styledName = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Bold(true).Render(name)
	case selected:
		lead = theme.FaintStyle.Render("  · ")
		styledName = theme.TextStyle.Render(name)
	default:
		lead = "    "
	}
	if it.OutOfStock {
		styledName = theme.FaintStyle.Render(name) + theme.FavStyle.Render(" · sold out")
	}
	pad := w - leadW - lipgloss.Width(name) - lipgloss.Width(base.Right)
	if pad < 1 {
		pad = 1
	}
	content := lead + styledName + strings.Repeat(" ", pad) + base.Right
	if selected && !s.railFocus {
		return lipgloss.NewStyle().Background(lipgloss.Color(theme.SelRowBg)).Render(content)
	}
	return content
}

// itemIndex finds its position in s.items (small lists — linear scan is fine).
func (s Instamart) itemIndex(it catalog.Item) int {
	for i, x := range s.items {
		if x.ID == it.ID {
			return i
		}
	}
	return -1
}

func (s Instamart) twoPaneView() string {
	budget := s.list.MaxRows

	var main strings.Builder
	switch {
	case s.searchActive:
		main.WriteString(s.searchInputLine() + "\n")
		switch {
		case s.loading:
			main.WriteString(theme.GoldStyle.Render("searching…") + "\n")
		case s.searchQuery == "":
			main.WriteString(theme.DimStyle.Render("type to search instamart, ↵ to search") + "\n")
		case len(s.items) == 0:
			main.WriteString(theme.DimStyle.Render(fmt.Sprintf(`no products for "%s"`, s.searchQuery)) + "\n")
		default:
			main.WriteString(theme.DimStyle.Render(plural(len(s.items), "result", "results")) + "\n\n")
		}
		if !s.loading && len(s.items) > 0 {
			rb := budget
			if rb > 0 {
				if rb -= 2; rb < 3 {
					rb = 3
				}
			}
			start, end, above, below := windowRange(s.list.Cursor, len(s.items), rb)
			if above > 0 {
				main.WriteString("  " + theme.FaintStyle.Render(fmt.Sprintf("↑ %d more", above)) + "\n")
			}
			for i := start; i < end; i++ {
				main.WriteString(s.productRow(s.items[i], i == s.list.Cursor) + "\n")
			}
			if below > 0 {
				main.WriteString("  " + theme.FaintStyle.Render(fmt.Sprintf("↓ %d more", below)) + "\n")
			}
		}
	default:
		main.WriteString(s.browseRows(budget))
	}

	mainStr := main.String()

	railH := lipgloss.Height(mainStr)
	if railH < s.rail.Len()+1 {
		railH = s.rail.Len() + 1
	}
	left := s.rail.WithFocus(s.railFocus).WithActive(s.rail.Active()).WithHeight(railH).View()

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, mainStr)
	hint := components.Hint("↑↓", "move", "↵/→", "add", "←", "remove", "/", "search", "c", "cart") +
		"   " + theme.PurpleStyle.Render(":") + " " + theme.FaintStyle.Render("cmd")
	// The store switcher (Food ⟷ Instamart) sits above the rail/main split —
	// active=1 renders the mirror-image gold pill on Instamart.
	return verticalTabs(1) + body + "\n\n" + hint
}

func (s Instamart) View() string {
	if s.hasRail {
		return s.twoPaneView()
	}

	var b strings.Builder
	w := components.ContentWidth()

	header := justify(
		theme.PriceStyle.Render("← instamart"),
		theme.CartStyle.Render(s.cartChip),
		w,
	)
	b.WriteString("  " + header + "\n")
	b.WriteString("  " + theme.EtaStyle.Render(InstamartETA+" · fast lane") + "\n")
	b.WriteString("  " + components.Divider())
	b.WriteString("\n\n") // padding above the list

	switch {
	case s.searchActive:
		b.WriteString("  " + s.searchInputLine() + "\n\n")
		switch {
		case s.loading:
			b.WriteString("  " + theme.GoldStyle.Render("searching…") + "\n")
		case s.searchQuery == "" && len(s.items) == 0:
			b.WriteString("  " + theme.DimStyle.Render("type to search instamart, ↵ to search") + "\n")
		case len(s.items) == 0:
			b.WriteString("  " + theme.DimStyle.Render(fmt.Sprintf(`no products for "%s"`, s.searchQuery)) + "\n")
		default:
			b.WriteString(s.list.View())
		}
	case s.loading:
		b.WriteString("  " + theme.GoldStyle.Render("loading…") + "\n")
	case len(s.items) == 0:
		b.WriteString("  " + theme.DimStyle.Render("no usuals yet — press / to search") + "\n")
	default:
		b.WriteString(s.list.View())
	}

	b.WriteString("\n\n\n") // padding below the list
	if s.searchActive {
		b.WriteString(components.Hint("↵", "search", "esc", "cancel"))
	} else {
		b.WriteString(components.Hint("↑↓", "move", "↵/→", "add", "←", "remove", "/", "search", "c", "cart", "esc", "back"))
	}
	return b.String()
}
