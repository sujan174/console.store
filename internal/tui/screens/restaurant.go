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
	searching bool
}

func NewRestaurant(p catalog.Place, cartTotal int) Restaurant {
	rows := make([]components.Row, len(p.Items))
	for i, it := range p.Items {
		rows[i] = components.Row{Left: it.Name, Right: fmt.Sprintf("₹%d", it.Price), Tag: it.Tag}
	}
	return Restaurant{p: p, cartTotal: cartTotal, list: components.List{Rows: rows}}
}

func (s Restaurant) Selected() (catalog.Item, bool) {
	i := s.list.SelectedIndex()
	if i < 0 {
		return catalog.Item{}, false
	}
	return s.p.Items[i], true
}

func (s Restaurant) WithCartTotal(t int) Restaurant { s.cartTotal = t; return s }

// PlaceData returns the underlying place (used by the app router).
func (s Restaurant) PlaceData() catalog.Place { return s.p }

func (s Restaurant) Init() tea.Cmd { return nil }

func (s Restaurant) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	if s.searching {
		switch k.String() {
		case "esc":
			s.searching = false
			s.list.SetFilter("")
		case "enter":
			s.searching = false
		case "backspace":
			f := s.list.Filter()
			if f != "" {
				s.list.SetFilter(f[:len(f)-1])
			}
		default:
			if k.Type == tea.KeyRunes {
				s.list.SetFilter(s.list.Filter() + string(k.Runes))
			}
		}
		return s, nil
	}
	switch k.String() {
	case "/":
		s.searching = true
	case "j", "down":
		s.list.Down()
	case "k", "up":
		s.list.Up()
	}
	return s, nil
}

// Searching reports whether the restaurant is in search-input mode.
func (s Restaurant) Searching() bool { return s.searching }

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
