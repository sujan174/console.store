package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/mock"
	"console.store/internal/tui/components"
	"console.store/internal/tui/theme"
)

type Restaurant struct {
	r         mock.Restaurant
	cartTotal int
	list      components.List
}

func NewRestaurant(r mock.Restaurant, cartTotal int) Restaurant {
	rows := make([]components.Row, len(r.Items))
	for i, it := range r.Items {
		rows[i] = components.Row{Left: it.Name, Right: fmt.Sprintf("₹%d", it.Price), Tag: it.Tag}
	}
	return Restaurant{r: r, cartTotal: cartTotal, list: components.List{Rows: rows}}
}

func (s Restaurant) Selected() mock.Item { return s.r.Items[s.list.Cursor] }

// WithCartTotal returns a copy of the restaurant with an updated cart total,
// preserving the list cursor and selection.
func (s Restaurant) WithCartTotal(t int) Restaurant { s.cartTotal = t; return s }

// RestaurantData returns the underlying restaurant (used by the app router).
func (s Restaurant) RestaurantData() mock.Restaurant { return s.r }

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
	back := theme.PriceStyle.Render("← " + strings.ToLower(s.r.Name))
	cart := theme.CartStyle.Render(fmt.Sprintf("cart · ₹%d", s.cartTotal))
	b.WriteString("  " + back + "              " + cart + "\n")
	b.WriteString("  " + theme.EtaStyle.Render(s.r.ETA) + "\n\n")
	b.WriteString(s.list.View())
	b.WriteString("\n")
	b.WriteString(components.KeyHints("j/k move   ↵ add   / search   esc back   c cart"))
	return b.String()
}
