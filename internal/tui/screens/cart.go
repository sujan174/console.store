package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/catalog"
	"console.store/internal/tui/components"
	"console.store/internal/tui/theme"
)

// Bill constants mirror the design (script line 606: toPay = item + 29 − 50).
// NOTE: these are duplicated in package tui (app.go DeliveryFee/CouponAmount)
// because screens does not import tui; keep the two in sync.
const (
	DeliveryFee  = 29
	CouponCode   = "DEVFRIDAY"
	CouponAmount = 50
)

type CartLine struct {
	Item catalog.Item
	Qty  int
}

type Cart struct {
	restaurant string
	lines      []CartLine
	cursor     int
	eta        string
	minNotice  string
}

func NewCart(restaurant string, lines []CartLine) Cart {
	// copy so the cart owns its slice (router keeps its own m.lines)
	cp := make([]CartLine, len(lines))
	copy(cp, lines)
	return Cart{restaurant: restaurant, lines: cp}
}

func (c Cart) Lines() []CartLine { return c.lines }

// WithEta sets the cart header ETA (e.g. "~45 min"), shown top-right.
func (c Cart) WithEta(s string) Cart { c.eta = s; return c }

// WithMinNotice sets a notice shown when the cart is below a minimum.
func (c Cart) WithMinNotice(s string) Cart { c.minNotice = s; return c }

// billToPay applies the design bill: item + delivery − coupon, or 0 when empty.
// Shared by cart and checkout so the two screens never disagree on the total.
func billToPay(itemTotal int) int {
	if itemTotal <= 0 {
		return 0
	}
	return itemTotal + DeliveryFee - CouponAmount
}

// toPay applies the design bill to the cart's item total.
func (c Cart) toPay() int { return billToPay(c.Total()) }

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

// Right increments the selected line's quantity.
func (c Cart) Right() Cart { return c.Inc() }

// Left decrements the selected line; if its quantity is 1 it removes the line.
func (c Cart) Left() Cart {
	if len(c.lines) == 0 {
		return c
	}
	if c.lines[c.cursor].Qty <= 1 {
		return c.Remove()
	}
	return c.Dec()
}

func (c Cart) Init() tea.Cmd { return nil }

func (c Cart) View() string {
	var b strings.Builder
	w := components.ContentWidth()

	// Header: "cart · {restaurant}" (brand) … "{eta}" (eta), 2-space indent.
	// An empty cart carries no restaurant binding, so fall back to the neutral
	// "your order" label (and drop the ETA) rather than leaving a stale
	// "cart · {restaurant}" once everything is removed.
	title := "cart · your order"
	eta := ""
	if len(c.lines) > 0 {
		eta = c.eta
		if c.restaurant != "" {
			title = "cart · " + c.restaurant
		}
	}
	header := justify(
		theme.BrandStyle.Render(title),
		theme.EtaStyle.Render(eta),
		w,
	)
	b.WriteString("  " + header + "\n")
	b.WriteString(components.Divider())
	b.WriteString("\n")

	if len(c.lines) == 0 {
		b.WriteString("  " + theme.DimStyle.Render("your cart is empty — press ") +
			theme.CursorStyle.Render("esc") + theme.DimStyle.Render(" to browse.") + "\n\n")
		b.WriteString(components.Hint("↑↓", "move", "←→", "qty", "↵", "checkout", "esc", "back"))
		return b.String()
	}

	// Line rows reuse the List full-bleed selected bar.
	list := components.List{Cursor: c.cursor}
	for _, l := range c.lines {
		list.Rows = append(list.Rows, components.Row{
			Left:  l.Item.Name + theme.DimStyle.Render(fmt.Sprintf("    x%d", l.Qty)),
			Right: theme.PriceStyle.Render(fmt.Sprintf("₹%d", l.Item.Price*l.Qty)),
		})
	}
	b.WriteString(list.View())

	// Bill breakdown.
	b.WriteString(components.DashRule())
	b.WriteString("  " + justify(theme.DimStyle.Render("item total"),
		theme.TextStyle.Render(fmt.Sprintf("₹%d", c.Total())), w) + "\n")
	b.WriteString("  " + justify(theme.DimStyle.Render("delivery"),
		theme.TextStyle.Render(fmt.Sprintf("₹%d", DeliveryFee)), w) + "\n")
	b.WriteString("  " + justify(
		theme.GreenStyle.Render(fmt.Sprintf("%s  −₹%d", CouponCode, CouponAmount)),
		theme.GreenStyle.Render("applied"), w) + "\n")
	b.WriteString(components.DashRule())
	b.WriteString("  " + justify(theme.BrightStyle.Render("to pay (COD)"),
		theme.BrightStyle.Render(fmt.Sprintf("₹%d", c.toPay())), w) + "\n")

	if c.minNotice != "" {
		b.WriteString("\n  " + theme.FavStyle.Render(c.minNotice) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(components.Hint("↑↓", "move", "←→", "qty", "↵", "checkout", "esc", "back"))
	return b.String()
}
