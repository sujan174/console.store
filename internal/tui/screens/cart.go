package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/catalog"
	"console.store/internal/tui/components"
	"console.store/internal/tui/theme"
)

type CartLine struct {
	Item catalog.Item
	Qty  int
}

type Cart struct {
	restaurant string
	lines      []CartLine
	cursor     int
	minNotice  string
}

func NewCart(restaurant string, lines []CartLine) Cart {
	// copy so the cart owns its slice (router keeps its own m.lines)
	cp := make([]CartLine, len(lines))
	copy(cp, lines)
	return Cart{restaurant: restaurant, lines: cp}
}

func (c Cart) Lines() []CartLine { return c.lines }

// WithMinNotice sets a notice shown when the cart is below a minimum.
func (c Cart) WithMinNotice(s string) Cart { c.minNotice = s; return c }

func (c Cart) Total() int {
	t := 0
	for _, l := range c.lines {
		t += l.Item.Price * l.Qty
	}
	return t
}

func (c Cart) clampCursor() Cart {
	if c.cursor >= len(c.lines) {
		c.cursor = len(c.lines) - 1
	}
	if c.cursor < 0 {
		c.cursor = 0
	}
	return c
}

func (c Cart) Up() Cart   { c.cursor--; return c.clampCursor() }
func (c Cart) Down() Cart { c.cursor++; return c.clampCursor() }

func (c Cart) Inc() Cart {
	if len(c.lines) > 0 {
		c.lines[c.cursor].Qty++
	}
	return c
}

func (c Cart) Dec() Cart {
	if len(c.lines) > 0 && c.lines[c.cursor].Qty > 1 {
		c.lines[c.cursor].Qty--
	}
	return c
}

func (c Cart) Remove() Cart {
	if len(c.lines) == 0 {
		return c
	}
	c.lines = append(c.lines[:c.cursor], c.lines[c.cursor+1:]...)
	return c.clampCursor()
}

func (c Cart) Init() tea.Cmd { return nil }

func (c Cart) View() string {
	var b strings.Builder
	b.WriteString("  " + theme.CartStyle.Render("cart · "+c.restaurant) + "\n\n")
	if len(c.lines) == 0 {
		b.WriteString("  " + theme.DimStyle.Render("your cart is empty") + "\n\n")
		b.WriteString(components.KeyHints("esc back"))
		return b.String()
	}
	for i, l := range c.lines {
		marker := theme.FaintStyle.Render("·")
		if i == c.cursor {
			marker = theme.CursorStyle.Render("❯")
		}
		b.WriteString(fmt.Sprintf("  %s %s   x%d   %s\n",
			marker, theme.ItemStyle.Render(l.Item.Name), l.Qty,
			theme.PriceStyle.Render(fmt.Sprintf("₹%d", l.Item.Price*l.Qty))))
	}
	b.WriteString("  " + theme.FaintStyle.Render(strings.Repeat("─", 50)) + "\n")
	b.WriteString("  " + theme.BrightStyle.Render(fmt.Sprintf("to pay (COD)   ₹%d", c.Total())) + "\n\n")
	b.WriteString("  " + theme.DimStyle.Render("pay the rider on delivery · cash or UPI") + "\n")
	b.WriteString("  " + theme.FavStyle.Render("orders can't be cancelled once placed") + "\n\n")
	if c.minNotice != "" {
		b.WriteString("  " + theme.FavStyle.Render(c.minNotice) + "\n\n")
	}
	b.WriteString(components.KeyHints("j/k move   +/- qty   x remove   ↵ checkout   esc back"))
	return b.String()
}
