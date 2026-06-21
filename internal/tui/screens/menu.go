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

// version is the build tag shown next to the brand in the header.
const version = "v1.4"

type Menu struct {
	places      []catalog.Place
	address     catalog.Address
	section     catalog.Section
	usual       catalog.Usual
	hasUsual    bool
	cartChip    string
	trending    catalog.Trending
	hasTrending bool
	counts      map[catalog.Section]int
	list        components.List
	searching   bool
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
		places:    places,
		address:   addr,
		section:   section,
		usual:     usual,
		hasUsual:  hasUsual,
		cartChip:  cartChip,
		list:      components.List{Rows: rows, Cursor: 0},
	}
}

// Selected returns the place under the cursor (false if the list is empty).
func (m Menu) Selected() (catalog.Place, bool) {
	i := m.list.SelectedIndex()
	if i < 0 || i >= len(m.places) {
		return catalog.Place{}, false
	}
	return m.places[i], true
}

// WithCartTotal returns a copy with an updated cart total, preserving the cursor.
func (m Menu) WithCartChip(s string) Menu { m.cartChip = s; return m }

// WithMaxRows sets the list viewport height (rows). 0 = show all.
func (m Menu) WithMaxRows(n int) Menu { m.list.MaxRows = n; return m }

// WithTrending sets the hero "trending now" pick.
func (m Menu) WithTrending(t catalog.Trending, ok bool) Menu {
	m.trending, m.hasTrending = t, ok
	return m
}

// WithCounts sets the per-section place counts shown on the tab bar.
func (m Menu) WithCounts(c map[catalog.Section]int) Menu { m.counts = c; return m }

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

// etaTail turns "30-40 min" into "~40 min".
func etaTail(eta string) string {
	if i := strings.LastIndex(eta, "-"); i >= 0 {
		return "~" + strings.TrimSpace(eta[i+1:])
	}
	return eta
}

// heroBox renders a rounded titled card spanning width w:
//
//	╭─ <title> ───────────────╮
//	│ <left>          <right> │
//	╰─────────────────────────╯
func heroBox(title, left, right string, w int) string {
	bd := theme.Fg(theme.Div2)
	topUsed := lipgloss.Width("╭─ ") + lipgloss.Width(title) + lipgloss.Width(" ") + 1
	fill := w - topUsed
	if fill < 0 {
		fill = 0
	}
	top := bd.Render("╭─ ") + theme.FaintStyle.Render(title) + bd.Render(" "+strings.Repeat("─", fill)+"╮")
	inner := w - 4
	gap := inner - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	mid := bd.Render("│ ") + left + strings.Repeat(" ", gap) + right + bd.Render(" │")
	bot := bd.Render("╰" + strings.Repeat("─", w-2) + "╯")
	return "  " + top + "\n  " + mid + "\n  " + bot + "\n"
}

func (m Menu) View() string {
	var b strings.Builder
	w := components.ContentWidth()

	b.WriteString("\n") // top padding

	// row 1: brand + version  |  deliver to ⊕ <addr> · <label> ⌄
	brand := theme.BrandStyle.Render("console.store") + " " + theme.FaintStyle.Render(version)
	deliver := theme.DimStyle.Render("deliver to ") + theme.CursorStyle.Render("⊕ ") +
		theme.BrightStyle.Render(m.address.Line) + theme.DimStyle.Render(" · "+m.address.Label) +
		theme.FaintStyle.Render(" ⌄")
	b.WriteString("  " + justify(brand, deliver, w) + "\n")

	// hero card: trending now
	if m.hasTrending {
		b.WriteString("\n") // gap before the hero card
		left := "🔥 " + theme.BrightStyle.Render(m.trending.Item.Name) +
			theme.DimStyle.Render(fmt.Sprintf("  ·  %d today", m.trending.Count))
		right := theme.DimStyle.Render(etaTail(m.trending.ETA)) + "   " +
			theme.PriceStyle.Render(fmt.Sprintf("₹%d", m.trending.Item.Price)) + "  " +
			theme.CursorStyle.Render("→")
		b.WriteString(heroBox("trending now", left, right, w))
	}

	b.WriteString("\n")

	// tab bar with per-section counts + cart chip:
	//   coffee 4 │ food 5 │ quick snacks 5            🛒 cart empty
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
	cartStyle := theme.CartStyle
	if strings.Contains(m.cartChip, "empty") {
		cartStyle = theme.DimStyle
	}
	b.WriteString("  " + justify(strings.Join(tabs, sep), cartStyle.Render(m.cartChip), w) + "\n")

	b.WriteString("\n")

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
