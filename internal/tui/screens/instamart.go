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

// NewInstamart builds the Instamart fast-lane screen, rendering in-cart checks
// and inline qty steppers from the current cart quantities (keyed by item ID),
// mirroring the restyled restaurant screen.
func NewInstamart(items []catalog.Item, qtyByItemID map[string]int, cartTotal int) Instamart {
	rows := make([]components.Row, 0, len(items))
	for _, it := range items {
		qty := qtyByItemID[it.ID]

		name := theme.ItemStyle.Render(it.Name)
		if qty > 0 {
			name = theme.BrightStyle.Render(it.Name)
		}

		price := theme.PriceStyle.Render(fmt.Sprintf("₹%d", it.Price))
		right := price
		if qty > 0 {
			stepper := theme.FavStyle.Render("−") + " " +
				theme.GreenStyle.Render(fmt.Sprintf("×%d", qty)) + " " +
				theme.GreenStyle.Render("+") + "   "
			right = stepper + price
		}

		rows = append(rows, components.Row{Left: name, Right: right, BarGreen: qty > 0})
	}
	return Instamart{items: items, cartTotal: cartTotal, list: components.List{Rows: rows}}
}

func (s Instamart) Selected() (catalog.Item, bool) {
	i := s.list.SelectedIndex()
	if i < 0 || i >= len(s.items) {
		return catalog.Item{}, false
	}
	return s.items[i], true
}

func (s Instamart) WithCartTotal(t int) Instamart { s.cartTotal = t; return s }

// CursorIndex returns the current list cursor so the router can preserve it
// across a rebuild (NewInstamart resets the cursor to 0).
func (s Instamart) CursorIndex() int { return s.list.Cursor }

// WithCursor restores a previously captured cursor position.
func (s Instamart) WithCursor(i int) Instamart { s.list.Cursor = i; return s }

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
	w := components.ContentWidth()

	header := justify(
		theme.PriceStyle.Render("← instamart"),
		theme.CartStyle.Render(fmt.Sprintf("cart · ₹%d", s.cartTotal)),
		w,
	)
	b.WriteString("  " + header + "\n")
	b.WriteString("  " + theme.EtaStyle.Render(InstamartETA+" · fast lane") + "\n")
	b.WriteString("  " + components.Divider())
	b.WriteString("\n") // padding above the list
	b.WriteString(s.list.View())
	b.WriteString("\n\n") // padding below the list
	b.WriteString(components.Hint("↑↓", "move", "↵/→", "add", "←", "remove", "c", "cart", "esc", "back"))
	return b.String()
}
