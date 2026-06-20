package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/catalog"
	"console.store/internal/tui/components"
	"console.store/internal/tui/theme"
)

type Checkout struct {
	restaurant string
	addr       catalog.Address
	lines      []CartLine
	placed     bool
	orderID    string
}

func NewCheckout(restaurant string, addr catalog.Address, lines []CartLine) Checkout {
	return Checkout{restaurant: restaurant, addr: addr, lines: lines}
}

// Placed returns a confirm-state copy carrying the order id.
func (c Checkout) Placed(orderID string) Checkout {
	c.placed = true
	c.orderID = orderID
	return c
}

func (c Checkout) IsPlaced() bool { return c.placed }

func (c Checkout) Total() int {
	t := 0
	for _, l := range c.lines {
		t += l.Item.Price * l.Qty
	}
	return t
}

func (c Checkout) Init() tea.Cmd { return nil }

func (c Checkout) View() string {
	if c.placed {
		return c.confirmView()
	}
	return c.summaryView()
}

func (c Checkout) summaryView() string {
	var b strings.Builder
	b.WriteString("  " + theme.BrandStyle.Render("checkout") + "\n\n")
	b.WriteString("  " + theme.DimStyle.Render("delivering to "+c.addr.Label+" · "+addrLine(c.addr)) + "\n")
	b.WriteString("  " + theme.DimStyle.Render("from "+c.restaurant) + "\n\n")
	for _, l := range c.lines {
		b.WriteString(fmt.Sprintf("  %s   x%d   %s\n",
			theme.ItemStyle.Render(l.Item.Name), l.Qty,
			theme.PriceStyle.Render(fmt.Sprintf("₹%d", l.Item.Price*l.Qty))))
	}
	b.WriteString("  " + theme.FaintStyle.Render(strings.Repeat("─", 50)) + "\n")
	b.WriteString("  " + theme.BrightStyle.Render(fmt.Sprintf("to pay (COD)   ₹%d", c.Total())) + "\n\n")
	b.WriteString("  " + theme.DimStyle.Render("pay the rider on delivery · cash or UPI") + "\n")
	b.WriteString("  " + theme.FavStyle.Render("orders can't be cancelled once placed") + "\n\n")
	b.WriteString(components.KeyHints("↵ place order   esc back"))
	return b.String()
}

func (c Checkout) confirmView() string {
	var b strings.Builder
	art := []string{
		"     ___ ",
		"    ( o )    order placed",
		"   /  |  \\   ",
		"      |      ",
	}
	for _, line := range art {
		b.WriteString("  " + theme.AccentStyle.Render(line) + "\n")
	}
	b.WriteString("\n")
	b.WriteString("  " + theme.EtaStyle.Render("✓ "+c.orderID) + "  " +
		theme.DimStyle.Render("· COD · "+c.restaurant) + "\n\n")
	b.WriteString("  " + theme.DimStyle.Render("the rider is on the way. track in the Swiggy app.") + "\n\n")
	b.WriteString(components.KeyHints("esc  back to menu"))
	return b.String()
}

func addrLine(a catalog.Address) string {
	if a.Full != "" {
		return a.Full
	}
	return a.Line
}
