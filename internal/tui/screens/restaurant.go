package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/catalog"
	"console.store/internal/tui/components"
	"console.store/internal/tui/theme"
)

type Restaurant struct {
	p         catalog.Place
	cartTotal int
	list      components.List
}

func NewRestaurant(p catalog.Place, cartTotal int) Restaurant {
	rows := make([]components.Row, len(p.Items))
	for i, it := range p.Items {
		rows[i] = components.Row{Left: it.Name, Right: fmt.Sprintf("₹%d", it.Price), Tag: it.Tag}
	}
	return Restaurant{p: p, cartTotal: cartTotal, list: components.List{Rows: rows}}
}

func (s Restaurant) Selected() catalog.Item { return s.p.Items[s.list.Cursor] }

func (s Restaurant) WithCartTotal(t int) Restaurant { s.cartTotal = t; return s }

// PlaceData returns the underlying place (used by the app router).
func (s Restaurant) PlaceData() catalog.Place { return s.p }

func (s Restaurant) Init() tea.Cmd { return nil }

func (s Restaurant) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (s Restaurant) View() string {
	var b strings.Builder
	back := theme.PriceStyle.Render("← " + strings.ToLower(s.p.Name))
	cart := theme.CartStyle.Render(fmt.Sprintf("cart · ₹%d", s.cartTotal))
	b.WriteString("  " + back + "              " + cart + "\n")
	b.WriteString("  " + theme.EtaStyle.Render(s.p.ETA) + "\n\n")
	b.WriteString(s.list.View())
	b.WriteString("\n")
	b.WriteString(components.KeyHints("j/k move   ↵ add   / search   esc back   c cart"))
	return b.String()
}
