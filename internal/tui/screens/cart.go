package screens

import (
	"fmt"
	"strings"

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
}

func NewCart(restaurant string, lines []CartLine) Cart {
	return Cart{restaurant: restaurant, lines: lines}
}

func (c Cart) Total() int {
	t := 0
	for _, l := range c.lines {
		t += l.Item.Price * l.Qty
	}
	return t
}

func (c Cart) View() string {
	var b strings.Builder
	b.WriteString("  " + theme.CartStyle.Render("cart · "+c.restaurant) + "\n\n")
	for _, l := range c.lines {
		b.WriteString(fmt.Sprintf("  %s   x%d   %s\n",
			theme.ItemStyle.Render(l.Item.Name), l.Qty,
			theme.PriceStyle.Render(fmt.Sprintf("₹%d", l.Item.Price*l.Qty))))
	}
	b.WriteString("  " + theme.FaintStyle.Render(strings.Repeat("─", 50)) + "\n")
	b.WriteString("  " + theme.BrightStyle.Render(fmt.Sprintf("to pay (COD)   ₹%d", c.Total())) + "\n\n")
	b.WriteString("  " + theme.DimStyle.Render("pay the rider on delivery · cash or UPI") + "\n")
	b.WriteString("  " + theme.FavStyle.Render("orders can't be cancelled once placed") + "\n\n")
	b.WriteString(components.KeyHints("+/- qty   x remove   ↵ checkout   esc back"))
	return b.String()
}
