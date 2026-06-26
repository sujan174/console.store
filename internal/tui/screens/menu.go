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

// plural renders "1 result" / "N results".
func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// Version is the build tag shown under the brand logo (rendered by the root).
const Version = "v1.4"

type Menu struct {
	places          []catalog.Place
	address         catalog.Address
	section         catalog.Section
	usual           catalog.Usual
	hasUsual        bool
	cartChip        string
	counts          map[catalog.Section]int
	list            components.List
	searching       bool
	chipLabels      []string
	chipActive      int
	hideSectionTabs bool
	// two-pane live fields (only active when hasRail is true)
	rail          Rail
	hasRail       bool
	railFocus     bool
	usuals        []catalog.Place
	nearby        []catalog.Place
	hasSections   bool
	searchMode    bool
	searchPending bool
	searchQuery   string
	results       []catalog.Place
	resultCount   int
}

func NewMenu(places []catalog.Place, addr catalog.Address, section catalog.Section, usual catalog.Usual, hasUsual bool, cartChip string) Menu {
	rows := make([]components.Row, len(places))
	for i, p := range places {
		rows[i] = components.Row{
			Left:  theme.ItemStyle.Render(p.Name),
			Right: theme.EtaStyle.Render(p.ETA),
		}
	}
	return Menu{
		places:   places,
		address:  addr,
		section:  section,
		usual:    usual,
		hasUsual: hasUsual,
		cartChip: cartChip,
		list:     components.List{Rows: rows, Cursor: 0},
	}
}

// Selected returns the place under the cursor (false if the list is empty).
// When a rail is set (live two-pane mode) the cursor maps into mainPlaces()
// so selection crosses usuals/nearby/results seamlessly.
func (m Menu) Selected() (catalog.Place, bool) {
	src := m.places
	if m.hasRail {
		src = m.mainPlaces()
	}
	i := m.list.Cursor
	if m.hasRail {
		// In two-pane mode cursor is a direct index into mainPlaces().
		if i < 0 || i >= len(src) {
			return catalog.Place{}, false
		}
		return src[i], true
	}
	// Mock path: use the list's filtered SelectedIndex.
	i = m.list.SelectedIndex()
	if i < 0 || i >= len(src) {
		return catalog.Place{}, false
	}
	return src[i], true
}

// WithCartTotal returns a copy with an updated cart total, preserving the cursor.
func (m Menu) WithCartChip(s string) Menu { m.cartChip = s; return m }

// ListCursor returns the current cursor position in the list.
func (m Menu) ListCursor() int { return m.list.Cursor }

// WithListCursor sets the list cursor position (used by the root for live rail nav).
func (m Menu) WithListCursor(i int) Menu {
	if i < 0 {
		i = 0
	}
	places := m.mainPlaces()
	if len(places) > 0 && i >= len(places) {
		i = len(places) - 1
	}
	m.list.Cursor = i
	return m
}

// WithMaxRows sets the list viewport height (rows). 0 = show all.
func (m Menu) WithMaxRows(n int) Menu { m.list.MaxRows = n; return m }

// WithCounts sets the per-section place counts shown on the tab bar.
func (m Menu) WithCounts(c map[catalog.Section]int) Menu { m.counts = c; return m }

// WithSectionTabsHidden hides the coffee/food/snacks tab row. Use in live mode
// where the cuisine-chip row replaces it; leave false (default) for mock mode.
func (m Menu) WithSectionTabsHidden(v bool) Menu { m.hideSectionTabs = v; return m }

// WithChips sets the cuisine chip labels and the active (highlighted) chip index.
func (m Menu) WithChips(labels []string, active int) Menu {
	m.chipLabels = labels
	m.chipActive = active
	return m
}

// ChipCount returns the number of cuisine chips.
func (m Menu) ChipCount() int { return len(m.chipLabels) }

// WithRail attaches the left rail to the menu, enabling the two-pane render
// path. When a rail is set, View() renders the two-pane layout; without it
// the existing single-pane (mock) render runs unchanged.
func (m Menu) WithRail(r Rail) Menu { m.rail = r; m.hasRail = true; return m }

// WithRailFocus sets whether the rail column has keyboard focus (the main list
// is focused when false).
func (m Menu) WithRailFocus(f bool) Menu { m.railFocus = f; return m }

// WithSections sets the Home view's usuals + nearby slices. The usuals block
// is omitted from View() entirely when usuals is empty. Clears search mode.
func (m Menu) WithSections(usuals, nearby []catalog.Place) Menu {
	m.usuals = usuals
	m.nearby = nearby
	m.hasSections = true
	m.searchMode = false
	return m
}

// WithSearchMode sets the live search state. When active is true the main pane
// shows the search input, result count, and result list. Clears sections view.
func (m Menu) WithSearchMode(active bool, query string, results []catalog.Place, count int, pending bool) Menu {
	m.searchMode = active
	m.searchQuery = query
	m.results = results
	m.resultCount = count
	m.searchPending = pending
	if active {
		m.hasSections = false
	}
	return m
}

// mainPlaces is the flat, cursor-addressable slice for the active view:
// search → results; Home (hasSections) → usuals then nearby; else → places.
// The components.List rows must be in the same order.
func (m Menu) mainPlaces() []catalog.Place {
	switch {
	case m.searchMode:
		return m.results
	case m.hasSections:
		out := make([]catalog.Place, 0, len(m.usuals)+len(m.nearby))
		out = append(out, m.usuals...)
		out = append(out, m.nearby...)
		return out
	default:
		return m.places
	}
}

// sectionedListView renders the Home main pane: optional usuals block (omitted
// when empty) + always-present nearby block. Section header labels are dim
// hairlines drawn between the restaurant rows; the cursor list spans both.
func (m Menu) sectionedListView() string {
	var b strings.Builder

	// Build a cursor list over mainPlaces() for selection highlighting.
	places := m.mainPlaces()
	cursor := m.list.Cursor

	renderRow := func(p catalog.Place, idx int) {
		name := theme.ItemStyle.Render(p.Name)
		eta := theme.EtaStyle.Render(p.ETA)
		rating := ""
		if p.Rating > 0 {
			rating = " " + theme.Fg(theme.Gold).Render(fmt.Sprintf("★%.1f", p.Rating))
		}
		body := name + rating + "   " + eta
		if idx == cursor {
			brightName := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Bold(true).Render(p.Name)
			selBody := brightName + rating + "   " + eta
			b.WriteString("  > " + selBody + "\n")
		} else {
			b.WriteString("    " + body + "\n")
		}
	}

	renderHeader := func(label string) {
		rule := theme.Fg(theme.Div2).Render(strings.Repeat("─", 20))
		b.WriteString("  " + rule + " " + theme.DimStyle.Render(label) + " " + rule + "\n")
	}

	idx := 0
	if len(m.usuals) > 0 {
		renderHeader("your usuals")
		for _, p := range m.usuals {
			renderRow(p, idx)
			idx++
		}
	}

	if len(m.nearby) > 0 || len(m.usuals) == 0 {
		renderHeader("popular near you")
		for _, p := range m.nearby {
			renderRow(p, idx)
			idx++
		}
	}

	if len(places) == 0 {
		b.WriteString("  " + theme.DimStyle.Render("no restaurants nearby") + "\n")
	}
	return b.String()
}

// twoPaneView renders the rail + main pane layout used in live mode.
func (m Menu) twoPaneView() string {
	railH := m.list.MaxRows + 6
	if railH < m.rail.Len()+1 {
		railH = m.rail.Len() + 1
	}
	left := m.rail.WithFocus(m.railFocus).WithActive(m.rail.Active()).WithHeight(railH).View()

	var main strings.Builder

	header := theme.DimStyle.Render("deliver to ") +
		theme.BrightStyle.Render(m.address.Line)
	if m.address.Label != "" {
		header += theme.DimStyle.Render(" · " + m.address.Label)
	}
	main.WriteString(header + "\n\n")

	switch {
	case m.searchMode:
		main.WriteString(theme.CursorStyle.Render("🔍 "+m.searchQuery+"▏") + "\n")
		switch {
		case m.searchPending:
			// A query is in flight (search paginates, so it can take a moment).
			main.WriteString(theme.GoldStyle.Render("searching…") + "\n")
		case m.searchQuery == "":
			main.WriteString(theme.DimStyle.Render("type to search restaurants, ↵ to search") + "\n")
		case len(m.results) == 0:
			main.WriteString(theme.DimStyle.Render(fmt.Sprintf(`no restaurants for "%s"`, m.searchQuery)) + "\n")
		default:
			main.WriteString(theme.DimStyle.Render(plural(m.resultCount, "result", "results")) + "\n\n")
		}
		if !m.searchPending && len(m.results) > 0 {
			// Render results list
			for i, p := range m.results {
				name := theme.ItemStyle.Render(p.Name)
				eta := theme.EtaStyle.Render(p.ETA)
				rating := ""
				if p.Rating > 0 {
					rating = " " + theme.Fg(theme.Gold).Render(fmt.Sprintf("★%.1f", p.Rating))
				}
				if i == m.list.Cursor {
					brightName := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Bold(true).Render(p.Name)
					main.WriteString("> " + brightName + rating + "   " + eta + "\n")
				} else {
					main.WriteString("  " + name + rating + "   " + eta + "\n")
				}
			}
		}
	case m.hasSections:
		main.WriteString(m.sectionedListView())
	default:
		// plain flat list (live mode, non-Home category, no sections set)
		for i, p := range m.places {
			name := theme.ItemStyle.Render(p.Name)
			eta := theme.EtaStyle.Render(p.ETA)
			rating := ""
			if p.Rating > 0 {
				rating = " " + theme.Fg(theme.Gold).Render(fmt.Sprintf("★%.1f", p.Rating))
			}
			if i == m.list.Cursor {
				brightName := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Bold(true).Render(p.Name)
				main.WriteString("> " + brightName + rating + "   " + eta + "\n")
			} else {
				main.WriteString("  " + name + rating + "   " + eta + "\n")
			}
		}
	}

	mainStr := main.String()
	return lipgloss.JoinHorizontal(lipgloss.Top, left, "  "+strings.ReplaceAll(mainStr, "\n", "\n  "))
}

func (m Menu) Init() tea.Cmd { return nil }

func (m Menu) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.searching {
		switch k.String() {
		case "esc":
			m.searching = false
			m.list.SetFilter("")
		case "enter":
			m.searching = false
		case "backspace":
			f := m.list.Filter()
			if f != "" {
				m.list.SetFilter(f[:len(f)-1])
			}
		default:
			if k.Type == tea.KeyRunes {
				m.list.SetFilter(m.list.Filter() + string(k.Runes))
			}
		}
		return m, nil
	}
	switch k.String() {
	case "/":
		m.searching = true
	case "j", "down":
		m.list.Down()
	case "k", "up":
		m.list.Up()
	}
	return m, nil
}

// Searching reports whether the menu is in search-input mode.
func (m Menu) Searching() bool { return m.searching }

// justify spreads left and right across width with the gap padded by spaces.
func justify(left, right string, width int) string {
	pad := width - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + right
}

func (m Menu) View() string {
	// Two-pane live render: rail column + sectioned main pane.
	// This branch is ONLY taken when a rail has been attached via WithRail().
	// The mock single-pane path below runs byte-for-byte as before.
	if m.hasRail {
		return m.twoPaneView()
	}

	var b strings.Builder
	w := components.ContentWidth()

	// row 1: deliver to ⊕ <addr> · <label> ⌄  (the brand logo is rendered as a
	// centered banner above every screen by the root, so it isn't repeated here).
	deliver := theme.DimStyle.Render("deliver to ") + theme.CursorStyle.Render("⊕ ") +
		theme.BrightStyle.Render(m.address.Line) + theme.DimStyle.Render(" · "+m.address.Label) +
		theme.FaintStyle.Render(" ⌄")
	b.WriteString("  " + justify("", deliver, w) + "\n")

	b.WriteString("\n")

	// tab bar with per-section counts + cart chip (suppressed in live mode where
	// the cuisine-chip row below replaces it):
	//   coffee 4 │ food 5 │ quick snacks 5            🛒 cart empty
	cartStyle := theme.CartStyle
	if strings.Contains(m.cartChip, "empty") {
		cartStyle = theme.DimStyle
	}
	if !m.hideSectionTabs {
		labels := map[catalog.Section]string{
			catalog.SectionCoffee: "coffee",
			catalog.SectionFood:   "food",
			catalog.SectionSnacks: "quick snacks",
		}
		var tabs []string
		for _, s := range catalog.MenuSections {
			cnt := theme.DimStyle.Render(fmt.Sprintf(" %d", m.counts[s]))
			if s == m.section {
				tabs = append(tabs, theme.Fg(theme.Gold).Underline(true).Render(labels[s])+cnt)
			} else {
				tabs = append(tabs, theme.CatOffStyle.Render(labels[s])+cnt)
			}
		}
		sep := theme.Fg(theme.Div2).Render(" │ ")
		b.WriteString("  " + justify(strings.Join(tabs, sep), cartStyle.Render(m.cartChip), w) + "\n")
	} else {
		// In live mode the section tabs are hidden; still render the cart chip
		// right-aligned so it stays visible on the browse screen.
		b.WriteString("  " + justify("", cartStyle.Render(m.cartChip), w) + "\n")
	}

	b.WriteString("\n")

	// vertical-toggle row + cuisine chip row — rendered only in live mode (when
	// chips have been set via WithChips). Mock path (no chips) is unchanged.
	if len(m.chipLabels) > 0 {
		// vertical toggle: Restaurants (active) · Instamart soon   tab ·
		vertSep := theme.Fg(theme.Div2).Render("  ·  ")
		activeV := theme.Fg(theme.Gold).Underline(true).Render("Restaurants")
		inactiveV := theme.CatOffStyle.Render("Instamart") + "  " + theme.FaintStyle.Render("soon")
		tabHint := theme.FaintStyle.Render("tab ·")
		b.WriteString("  " + activeV + vertSep + inactiveV + "   " + tabHint + "\n")

		// cuisine chip row — scrolls horizontally (windowed around the active chip
		// with ‹ / › markers) so a long chip list never truncates.
		b.WriteString("  " + windowedBar(m.chipLabels, m.chipActive, components.ContentWidth()-4, " │ ") + "\n")
		b.WriteString("\n")
	}

	// search prompt (when active)
	if m.searching || m.list.Filter() != "" {
		b.WriteString("  " + theme.CursorStyle.Render("/"+m.list.Filter()) + "\n")
	}

	if len(m.places) == 0 && !m.hasUsual {
		b.WriteString("  " + theme.DimStyle.Render("no curated spots deliver here right now") + "\n")
	} else {
		b.WriteString(m.list.View())
	}

	b.WriteString("\n\n\n") // padding below the list

	hint := components.Hint("↑↓", "move", "←→", "category", "↵", "open", "a", "address", "c", "cart") +
		"   " + theme.PurpleStyle.Render(":") + " " + theme.FaintStyle.Render("cmd")
	b.WriteString(hint)

	return b.String()
}
