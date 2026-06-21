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

// NewRestaurant builds the restaurant screen, rendering in-cart checks and
// inline qty steppers from the current cart quantities (keyed by item ID).
func NewRestaurant(p catalog.Place, qtyByItemID map[string]int, cartTotal int) Restaurant {
	rows := make([]components.Row, 0, len(p.Items))
	for _, it := range p.Items {
		qty := qtyByItemID[it.ID]

		// in-cart items read brighter; the green left-bar + stepper already
		// signal "in cart", so no extra ✓ column (keeps the cursor→name gap
		// identical to the menu).
		nameStyle := theme.ItemStyle
		if qty > 0 {
			nameStyle = theme.BrightStyle
		}
		left := nameStyle.Render(it.Name)

		price := theme.PriceStyle.Render(fmt.Sprintf("₹%d", it.Price))
		right := price
		if qty > 0 {
			stepper := theme.FavStyle.Render("−") + " " +
				theme.GreenStyle.Render(fmt.Sprintf("×%d", qty)) + " " +
				theme.GreenStyle.Render("+") + "   "
			right = stepper + price
		}

		rows = append(rows, components.Row{Left: left, Right: right, BarGreen: qty > 0})
	}
	return Restaurant{p: p, cartTotal: cartTotal, list: components.List{Rows: rows}}
}

// itemDetail builds the fixed metadata strip for the highlighted item:
// "veg  ★ 4.8  180 kcal  blended double espresso · lightly sweet"
// (veg green / non-veg red, rating gold, kcal dim, description blue). Empty
// fields are omitted.
func itemDetail(it catalog.Item) string {
	veg := theme.GreenStyle.Render("veg")
	if !it.Veg {
		veg = theme.FavStyle.Render("non-veg")
	}
	parts := []string{veg}
	if it.Rating > 0 {
		parts = append(parts, theme.GoldStyle.Render(fmt.Sprintf("★ %.1f", it.Rating)))
	}
	if it.Kcal > 0 {
		parts = append(parts, theme.DimStyle.Render(fmt.Sprintf("%d kcal", it.Kcal)))
	}
	if it.Desc != "" {
		parts = append(parts, theme.CursorStyle.Render(it.Desc))
	}
	return strings.Join(parts, "  ")
}

func (s Restaurant) Selected() (catalog.Item, bool) {
	i := s.list.SelectedIndex()
	if i < 0 || i >= len(s.p.Items) {
		return catalog.Item{}, false
	}
	return s.p.Items[i], true
}

func (s Restaurant) WithCartTotal(t int) Restaurant { s.cartTotal = t; return s }

// PlaceData returns the underlying place (used by the app router).
func (s Restaurant) PlaceData() catalog.Place { return s.p }

// CursorIndex returns the current list cursor so the router can preserve it
// across a rebuild (NewRestaurant resets the cursor to 0).
func (s Restaurant) CursorIndex() int { return s.list.Cursor }

// WithCursor restores a previously captured cursor position.
func (s Restaurant) WithCursor(i int) Restaurant { s.list.Cursor = i; return s }

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
	w := components.ContentWidth()

	header := justify(
		theme.PriceStyle.Render("← "+strings.ToLower(s.p.Name)),
		theme.CartStyle.Render(fmt.Sprintf("cart · ₹%d", s.cartTotal)),
		w,
	)
	b.WriteString("  " + header + "\n")
	b.WriteString("  " + theme.EtaStyle.Render(s.p.ETA) + "\n")
	b.WriteString("  " + components.Divider())
	b.WriteString("\n\n") // padding above the list
	b.WriteString(s.list.View())
	b.WriteString("\n\n\n") // padding below the list
	// Fixed detail strip for the highlighted item — stable position so the rows
	// above don't shift as the cursor moves.
	if it, ok := s.Selected(); ok {
		b.WriteString(components.DashRule())
		b.WriteString("  " + itemDetail(it) + "\n")
	}
	b.WriteString("\n")
	b.WriteString(components.Hint("↑↓", "move", "↵/→", "add", "←", "remove", "esc", "back", "c", "cart"))
	return b.String()
}
