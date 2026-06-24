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

// Version is the build tag shown under the brand logo (rendered by the root).
const Version = "v1.4"

type Menu struct {
	places    []catalog.Place
	address   catalog.Address
	section   catalog.Section
	usual     catalog.Usual
	hasUsual  bool
	cartChip  string
	counts    map[catalog.Section]int
	list      components.List
	searching bool
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

func (m Menu) View() string {
	var b strings.Builder
	w := components.ContentWidth()

	// row 1: deliver to ⊕ <addr> · <label> ⌄  (the brand logo is rendered as a
	// centered banner above every screen by the root, so it isn't repeated here).
	deliver := theme.DimStyle.Render("deliver to ") + theme.CursorStyle.Render("⊕ ") +
		theme.BrightStyle.Render(m.address.Line) + theme.DimStyle.Render(" · "+m.address.Label) +
		theme.FaintStyle.Render(" ⌄")
	b.WriteString("  " + justify("", deliver, w) + "\n")

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
