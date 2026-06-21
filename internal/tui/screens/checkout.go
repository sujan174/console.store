package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	eta        string
}

func NewCheckout(restaurant string, addr catalog.Address, lines []CartLine, eta string) Checkout {
	return Checkout{restaurant: restaurant, addr: addr, lines: lines, eta: eta}
}

// Placed returns a confirm-state copy carrying the order id and eta.
func (c Checkout) Placed(orderID, eta string) Checkout {
	c.placed = true
	c.orderID = orderID
	c.eta = eta
	return c
}

func (c Checkout) IsPlaced() bool { return c.placed }

// Place returns the restaurant/store name (used to seed tracking).
func (c Checkout) Place() string { return c.restaurant }

// OrderID returns the placed order's id (used to seed tracking).
func (c Checkout) OrderID() string { return c.orderID }

// Lines returns the order's cart lines (used to derive the order id).
func (c Checkout) Lines() []CartLine { return c.lines }

// Total is the bare item total.
func (c Checkout) Total() int {
	t := 0
	for _, l := range c.lines {
		t += l.Item.Price * l.Qty
	}
	return t
}

// toPay is the design bill: item + delivery − coupon.
func (c Checkout) toPay() int { return billToPay(c.Total()) }

func (c Checkout) Init() tea.Cmd { return nil }

func (c Checkout) View() string {
	if c.placed {
		return c.confirmView()
	}
	return c.summaryView()
}

// padTo right-pads s with spaces to the given display width.
func padTo(s string, width int) string {
	if pad := width - lipgloss.Width(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

func (c Checkout) summaryView() string {
	var b strings.Builder
	w := components.ContentWidth()

	b.WriteString("  " + theme.BrandStyle.Render("checkout") + "\n")
	b.WriteString(components.Divider())
	b.WriteString("\n")

	label := func(s string) string { return theme.DimStyle.Render(padTo(s, 10)) }
	b.WriteString("  " + label("deliver to") + theme.TextStyle.Render(addrLine(c.addr)+" · "+c.addr.Label) + "\n")
	from := c.restaurant
	if c.eta != "" {
		from += " · " + c.eta
	}
	b.WriteString("  " + label("from") + theme.TextStyle.Render(from) + "\n")
	b.WriteString("  " + label("pay") + theme.GoldStyle.Render("Cash / UPI to rider on delivery") + "\n")

	b.WriteString(components.DashRule())
	b.WriteString("  " + justify(
		theme.BrightStyle.Render("to pay (COD)"),
		theme.BrightStyle.Render(fmt.Sprintf("₹%d", c.toPay())), w) + "\n")
	b.WriteString(components.DashRule())

	// Full-bleed place-order bar: green left bar + selected-row background.
	bar := theme.GreenStyle.Render("▌") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.Bright)).
			Background(lipgloss.Color(theme.SelRowBg)).
			Render(padTo(" > place order ", components.FrameWidth()-1))
	b.WriteString(bar + "\n\n")

	b.WriteString("  " + theme.FavStyle.Render("no online payment — pay the rider on delivery") + "\n")
	b.WriteString("  " + theme.DimStyle.Render("orders can't be cancelled once placed") + "\n\n")
	b.WriteString(components.Hint("↵", "place order", "esc", "back"))
	return b.String()
}

func (c Checkout) confirmView() string {
	var b strings.Builder

	// steam
	b.WriteString("  " + theme.GreenStyle.Render("˜ ˷ ˜") + "\n")

	// coffee cup (reference 368-371)
	cup := []string{
		"╭────────╮",
		"│ ▒▒▒▒▒▒ │╮",
		"│ ▒▒▒▒▒▒ │╯",
		"╰────────╯",
	}
	for _, line := range cup {
		b.WriteString("  " + theme.GoldStyle.Render(line) + "\n")
	}
	b.WriteString("\n")

	// order-placed box (reference 375-377)
	box := []string{
		"╔══════════════════════╗",
		"║   order placed  ✓     ║",
		"╚══════════════════════╝",
	}
	for _, line := range box {
		b.WriteString("  " + theme.GreenStyle.Render(line) + "\n")
	}
	b.WriteString("\n")

	b.WriteString("  " + theme.BrightStyle.Render(c.restaurant+" · ETA "+c.eta+" · ") +
		theme.DimStyle.Render(c.orderID) + "\n")
	b.WriteString("  " + theme.DimStyle.Render(fmt.Sprintf("pay ₹%d to rider (cash/UPI)", c.toPay())) + "\n")
	b.WriteString("  " + theme.FavStyle.Render("can't be cancelled now") + "\n\n")

	b.WriteString("  " + theme.GreenStyle.Render("↵") + " " + theme.FaintStyle.Render("track") +
		"     " + theme.CursorStyle.Render("esc") + " " + theme.FaintStyle.Render("back to menu"))
	return b.String()
}

func addrLine(a catalog.Address) string {
	if a.Full != "" {
		return a.Full
	}
	return a.Line
}
