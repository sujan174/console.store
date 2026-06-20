package screens

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

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
		rows[i] = components.Row{Left: p.Name, Right: p.ETA, Fav: p.Fav}
	}
	return Menu{places: places, address: addr, section: section, usual: usual, hasUsual: hasUsual, cartTotal: cartTotal, list: components.List{Rows: rows}}
}

// Selected returns the place under the cursor. Returns ok=false if the list is empty.
func (m Menu) Selected() (catalog.Place, bool) {
	i := m.list.SelectedIndex()
	if i < 0 {
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

func (m Menu) View() string {
	var b strings.Builder
	b.WriteString(components.Header("console.store", m.address.Line, m.cartTotal))
	b.WriteString("\n")
	if m.hasUsual {
		b.WriteString("  " + theme.CursorStyle.Render("u  the usual") + "   " +
			theme.ItemStyle.Render(m.usual.Label) + "\n\n")
	}
	b.WriteString(components.SectionTabs(m.section))
	b.WriteString("\n")
	if m.searching || m.list.Filter() != "" {
		b.WriteString("  " + theme.PriceStyle.Render("/"+m.list.Filter()) + "\n")
	}
	if len(m.places) == 0 {
		b.WriteString("  " + theme.DimStyle.Render("no curated spots deliver here right now") + "\n")
	} else {
		b.WriteString(m.list.View())
	}
	b.WriteString("\n")
	b.WriteString(components.KeyHints("j/k move   ↵ open   u usual   1/2/3 section   i instamart   a address   c cart"))
	return b.String()
}
