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

type Menu struct {
	places    []catalog.Place
	address   catalog.Address
	section   catalog.Section
	usual     catalog.Usual
	hasUsual  bool
	cartTotal int
	list      components.List
	searching bool
}

func NewMenu(places []catalog.Place, addr catalog.Address, section catalog.Section, usual catalog.Usual, hasUsual bool, cartTotal int) Menu {
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
		cartTotal: cartTotal,
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
func (m Menu) WithCartTotal(t int) Menu { m.cartTotal = t; return m }

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

	// header row: brand (left) · cart total (right)
	header := justify(
		theme.BrandStyle.Render("console.store"),
		theme.CartStyle.Render(fmt.Sprintf("cart · ₹%d", m.cartTotal)),
		w,
	)
	b.WriteString("  " + header + "\n")

	// address row: address line (left) · [a] (right)
	addrRow := justify(
		theme.DimStyle.Render(m.address.Line),
		theme.FaintStyle.Render("[a]"),
		w,
	)
	b.WriteString("  " + addrRow + "\n")

	b.WriteString("  " + components.Divider())

	// usual line: ↵ the usual   <label>            ₹<price>
	if m.hasUsual {
		left := theme.PurpleStyle.Render("↵ the usual") + "   " + theme.ItemStyle.Render(m.usual.Label)
		right := theme.PriceStyle.Render(fmt.Sprintf("₹%d", m.usual.Item.Price))
		b.WriteString("  " + justify(left, right, w) + "\n")
	}

	b.WriteString("\n")

	// tabs row: coffee  food  snacks
	labels := map[catalog.Section]string{
		catalog.SectionCoffee: "coffee",
		catalog.SectionFood:   "food",
		catalog.SectionSnacks: "snacks",
	}
	var tabs []string
	for _, s := range catalog.MenuSections {
		if s == m.section {
			tabs = append(tabs, theme.CatOnStyle.Render(labels[s]))
		} else {
			tabs = append(tabs, theme.CatOffStyle.Render(labels[s]))
		}
	}
	b.WriteString("  " + strings.Join(tabs, "   ") + "\n")

	b.WriteString("\n")

	// search prompt (when active)
	if m.searching || m.list.Filter() != "" {
		b.WriteString("  " + theme.CursorStyle.Render("/"+m.list.Filter()) + "\n")
	}

	b.WriteString("\n\n") // padding above the list

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
