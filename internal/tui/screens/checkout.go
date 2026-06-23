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

// Total is the bare item total (including each line's selected add-ons).
func (c Checkout) Total() int {
	t := 0
	for _, l := range c.lines {
		t += l.UnitPrice() * l.Qty
	}
	return t
}

// toPay is the design bill: item + delivery − coupon.
func (c Checkout) toPay() int { return billToPay(c.Total()) }

func (c Checkout) Init() tea.Cmd { return nil }

func (c Checkout) View(frame int) string {
	if c.placed {
		return c.confirmView(frame)
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

func (c Checkout) confirmView(frame int) string {
	var b strings.Builder

	// A steaming coffee cup marks the placed order. The steam wisps waver each
	// frame; the mug has a clear C-handle on the right and a saucer beneath, so
	// it reads as a cup (not a battery).
	steam := []string{"     ( (", "      ) )"}
	if (frame/8)%2 == 1 {
		steam = []string{"      ) )", "     ( ("}
	}
	for _, s := range steam {
		b.WriteString("  " + theme.DimStyle.Render(s) + "\n")
	}

	cup := []string{
		"   ╭───────╮",
		"   │~~~~~~~│╮",
		"   │▒▒▒▒▒▒▒│ )",
		"   │▒▒▒▒▒▒▒│ )",
		"   │▒▒▒▒▒▒▒│╯",
		"   ╰───────╯",
		"  ╰─────────╯",
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

	b.WriteString(c.speedReceipt())

	b.WriteString("  " + theme.GreenStyle.Render("↵") + " " + theme.FaintStyle.Render("track") +
		"     " + theme.CursorStyle.Render("esc") + " " + theme.FaintStyle.Render("back to menu"))
	return b.String()
}

// Speed-receipt dummies. The "mastery flex" pillar: prove how fast the order
// was vs. tapping a phone app. TODO: wire to real per-order measurement —
// keystroke count + elapsed time from menu-open to "order placed", tracked in
// the root model; session best held in-memory (no cross-session persistence yet).
const (
	dummyOrderSecs   = 2.1 // TODO: elapsed time of this order
	dummyOrderKeys   = 4   // TODO: keystrokes for this order
	dummySessionBest = 1.8 // TODO: fastest order this session (in-memory)
	phoneAppAvgLabel = "~45s"
)

// speedReceipt renders the post-order speed flex. Values are placeholders.
func (c Checkout) speedReceipt() string {
	var b strings.Builder
	b.WriteString("  " + theme.GoldStyle.Render(fmt.Sprintf("⚡ ordered in %.1fs · %d keystrokes", dummyOrderSecs, dummyOrderKeys)) + "\n")
	b.WriteString("     " + theme.DimStyle.Render(fmt.Sprintf("this session best %.1fs  ·  phone app %s", dummySessionBest, phoneAppAvgLabel)) + "\n")
	b.WriteString("  " + theme.FaintStyle.Render(strings.Repeat("─", 44)) + "\n")
	b.WriteString("  " + theme.DimStyle.Render("flex it:  ") + theme.BrandStyle.Render("ssh consolestore.in") + "\n\n")
	return b.String()
}

func addrLine(a catalog.Address) string {
	if a.Full != "" {
		return a.Full
	}
	return a.Line
}
