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
	addr      catalog.Address
	cartChip  string
	list      components.List
	searching bool
}

// NewRestaurant builds the restaurant screen, rendering in-cart checks and
// inline qty steppers from the current cart quantities (keyed by item ID).
func NewRestaurant(p catalog.Place, qtyByItemID map[string]int, cartChip string) Restaurant {
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
		if it.Tag != "" {
			left += "  " + theme.GreenStyle.Render(it.Tag)
		}

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
	return Restaurant{p: p, cartChip: cartChip, list: components.List{Rows: rows}}
}

// topItem is the restaurant's "most ordered" hero pick (highest-rated item).
func (s Restaurant) topItem() (catalog.Item, bool) {
	if len(s.p.Items) == 0 {
		return catalog.Item{}, false
	}
	best := s.p.Items[0]
	for _, it := range s.p.Items[1:] {
		if it.Rating > best.Rating {
			best = it
		}
	}
	return best, true
}

// vegCount is the number of vegetarian items on the menu.
func (s Restaurant) vegCount() int {
	n := 0
	for _, it := range s.p.Items {
		if it.Veg {
			n++
		}
	}
	return n
}

func (s Restaurant) Selected() (catalog.Item, bool) {
	i := s.list.SelectedIndex()
	if i < 0 || i >= len(s.p.Items) {
		return catalog.Item{}, false
	}
	return s.p.Items[i], true
}

func (s Restaurant) WithCartChip(c string) Restaurant { s.cartChip = c; return s }

// WithAddr sets the delivery address shown in the header.
func (s Restaurant) WithAddr(a catalog.Address) Restaurant { s.addr = a; return s }

// WithMaxRows sets the list viewport height (rows). 0 = show all.
func (s Restaurant) WithMaxRows(n int) Restaurant { s.list.MaxRows = n; return s }

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

	b.WriteString("\n") // top padding

	// row 1: ← back  <name> ★            deliver to ⊕ <addr>
	star := ""
	if s.p.Fav {
		star = " " + theme.GoldStyle.Render("★")
	}
	left := theme.PriceStyle.Render("← back") + "  " + theme.BrightStyle.Bold(true).Render(s.p.Name) + star
	right := theme.DimStyle.Render("deliver to ") + theme.CursorStyle.Render("⊕ ") + theme.BrightStyle.Render(s.addr.Line)
	b.WriteString("  " + justify(left, right, w) + "\n")

	// row 2 meta: ★ 4.6 · 35-45 min · coffee · 10 items
	dot := theme.FaintStyle.Render("  ·  ")
	meta := theme.GoldStyle.Render(fmt.Sprintf("★ %.1f", s.p.Rating)) + dot +
		theme.DimStyle.Render(s.p.ETA) + dot +
		theme.DimStyle.Render(string(s.p.Section)) + dot +
		theme.DimStyle.Render(fmt.Sprintf("%d items", len(s.p.Items)))
	b.WriteString("  " + meta + "\n")

	// most-ordered hero card
	if top, ok := s.topItem(); ok {
		b.WriteString("\n") // gap before the hero card
		hl := theme.GoldStyle.Render("★ ") + theme.BrightStyle.Render(top.Name)
		if top.Desc != "" {
			hl += "  " + theme.DimStyle.Render(top.Desc)
		}
		hr := theme.PriceStyle.Render(fmt.Sprintf("₹%d", top.Price)) + "  " + theme.CursorStyle.Render("→")
		b.WriteString(heroBox("most ordered", hl, hr, w))
	}

	b.WriteString("\n")

	// filter row: all 10 │ veg 9   ⌄ filter            🛒 cart empty
	allTab := theme.Fg(theme.Gold).Underline(true).Render("all") + theme.DimStyle.Render(fmt.Sprintf(" %d", len(s.p.Items)))
	vegTab := theme.CatOffStyle.Render("veg") + theme.DimStyle.Render(fmt.Sprintf(" %d", s.vegCount()))
	sep := theme.Fg(theme.Div2).Render(" │ ")
	filters := allTab + sep + vegTab + "   " + theme.FaintStyle.Render("⌄ filter")
	cartStyle := theme.CartStyle
	if strings.Contains(s.cartChip, "empty") {
		cartStyle = theme.DimStyle
	}
	b.WriteString("  " + justify(filters, cartStyle.Render(s.cartChip), w) + "\n")

	// search prompt (when active)
	if s.searching || s.list.Filter() != "" {
		b.WriteString("  " + theme.CursorStyle.Render("/"+s.list.Filter()) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(s.list.View())
	b.WriteString("\n")
	b.WriteString(components.Hint("↑↓", "move", "↵/→", "add", "←", "remove", "esc", "back", "c", "cart"))
	return b.String()
}
