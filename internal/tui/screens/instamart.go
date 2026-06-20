package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/catalog"
	"console.store/internal/tui/components"
	"console.store/internal/tui/theme"
)

// InstamartETA is the honest fast-lane window.
const InstamartETA = "~12 min"

type Instamart struct {
	items     []catalog.Item
	cartTotal int
	list      components.List
}

func NewInstamart(items []catalog.Item, cartTotal int) Instamart {
	rows := make([]components.Row, len(items))
	for i, it := range items {
		rows[i] = components.Row{Left: it.Name, Right: fmt.Sprintf("₹%d", it.Price), Tag: it.Tag}
	}
	return Instamart{items: items, cartTotal: cartTotal, list: components.List{Rows: rows}}
}

func (s Instamart) Selected() catalog.Item { return s.items[s.list.SelectedIndex()] }

func (s Instamart) WithCartTotal(t int) Instamart { s.cartTotal = t; return s }

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

func (s Instamart) View() string {
	var b strings.Builder
	back := theme.PriceStyle.Render("← instamart")
	cart := theme.CartStyle.Render(fmt.Sprintf("cart · ₹%d", s.cartTotal))
	b.WriteString("  " + back + "              " + cart + "\n")
	b.WriteString("  " + theme.EtaStyle.Render(InstamartETA+" · fast lane") + "\n\n")
	b.WriteString(s.list.View())
	b.WriteString("\n")
	b.WriteString(components.KeyHints("j/k move   ↵ add   esc back   c cart"))
	return b.String()
}
