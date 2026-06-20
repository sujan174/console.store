package screens

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/mock"
	"console.store/internal/tui/components"
	"console.store/internal/tui/theme"
)

type Menu struct {
	restaurants []mock.Restaurant
	address     mock.Address
	cartTotal   int
	list        components.List
}

func NewMenu(rs []mock.Restaurant, addr mock.Address, cartTotal int) Menu {
	rows := make([]components.Row, len(rs))
	for i, r := range rs {
		rows[i] = components.Row{Left: r.Name, Right: r.ETA, Fav: r.Fav}
	}
	return Menu{restaurants: rs, address: addr, cartTotal: cartTotal, list: components.List{Rows: rows}}
}

// Selected returns the restaurant under the cursor.
func (m Menu) Selected() mock.Restaurant { return m.restaurants[m.list.Cursor] }

func (m Menu) Init() tea.Cmd { return nil }

func (m Menu) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "j", "down":
			m.list.Down()
		case "k", "up":
			m.list.Up()
		}
	}
	return m, nil
}

func (m Menu) View() string {
	var b strings.Builder
	b.WriteString(components.Header("console.store", m.address.Line, m.cartTotal))
	b.WriteString("\n")
	if u, ok := mock.Usual(); ok {
		b.WriteString("  " + theme.CursorStyle.Render("↵ the usual") + "   " +
			theme.ItemStyle.Render(u.Name) + "\n\n")
	}
	b.WriteString("  " + theme.CatOnStyle.Render("coffee") + "   " +
		theme.CatOffStyle.Render("food") + "   " +
		theme.CatOffStyle.Render("snacks") + "   " +
		theme.PriceStyle.Render("instamart ↗") + "\n\n")
	b.WriteString(m.list.View())
	b.WriteString("\n")
	b.WriteString(components.KeyHints("j/k move   ↵ open   / search   a address   c cart"))
	return b.String()
}
