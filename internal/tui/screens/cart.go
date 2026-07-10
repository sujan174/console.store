package screens

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"consolestore/internal/catalog"
	"consolestore/internal/tui/components"
	"consolestore/internal/tui/theme"
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
	Item        catalog.Item
	Qty         int
	AddOns      []catalog.AddOn     // selected customizations for this line
	Selections  []catalog.Selection // live variant/addon selections (cart-send + key)
	Price       int                 // resolved per-unit price (0 = compute from Item+AddOns)
	Unavailable bool                // Swiggy reports this cart item out of stock (display-only)
}

// UnitPrice is the per-unit price including any selected add-ons. A resolved
// Price (e.g. set by a live variant, which is an absolute price) wins.
func (l CartLine) UnitPrice() int {
	if l.Price > 0 {
		return l.Price
	}
	p := l.Item.Price
	for _, a := range l.AddOns {
		p += a.Price
	}
	return p
}

// Key returns the cart-line identity: item id + its sorted add-on ids + live
// selection ids. Two lines stack only when item, add-ons AND selections match
// (so e.g. different pizza sizes are distinct lines).
func (l CartLine) Key() string { return lineKeyFull(l.Item, l.AddOns, l.Selections) }

func lineKeyFull(item catalog.Item, addons []catalog.AddOn, sels []catalog.Selection) string {
	key := LineKey(item, addons)
	if len(sels) == 0 {
		return key
	}
	ids := make([]string, len(sels))
	for i, s := range sels {
		ids[i] = s.GroupID + ":" + s.ChoiceID
	}
	sort.Strings(ids)
	return key + "|" + strings.Join(ids, ",")
}

// LineKey computes the stacking key for an item plus a set of add-ons. Add-on
// ids are sorted so selection order never produces a duplicate line.
func LineKey(item catalog.Item, addons []catalog.AddOn) string {
	ids := make([]string, len(addons))
	for i, a := range addons {
		ids[i] = a.ID
	}
	sort.Strings(ids)
	return item.ID + "|" + strings.Join(ids, ",")
}

// AddOnSummary is a short comma-joined list of add-on names (for the cart line).
func AddOnSummary(addons []catalog.AddOn) string {
	if len(addons) == 0 {
		return ""
	}
	names := make([]string, len(addons))
	for i, a := range addons {
		names[i] = a.Name
	}
	return strings.Join(names, ", ")
}

type Cart struct {
	restaurant string
	lines      []CartLine
	cursor     int
	eta        string
	minNotice  string
	bill       Bill
	liveMode   bool
	syncErr    string
}

// Bill carries Swiggy's real cart pricing breakdown. When Live is set, the cart
// and checkout screens render this exact split (item / delivery / taxes / to-pay)
// instead of the mock delivery-fee-minus-coupon math.
type Bill struct {
	ItemTotal int
	Delivery  int
	Taxes     int
	ToPay     int
	Live      bool
}

// renderBill renders Swiggy's real itemized split. payLabel tags the "to pay
// (…)" row with the settlement method (e.g. "COD" or "UPI").
func renderBill(w int, bill Bill, payLabel string) string {
	var b strings.Builder
	b.WriteString(components.DashRule())
	row := func(label string, amt int) {
		b.WriteString("  " + justify(theme.DimStyle.Render(label),
			theme.TextStyle.Render(fmt.Sprintf("₹%d", amt)), w) + "\n")
	}
	row("item total", bill.ItemTotal)
	row("delivery", bill.Delivery)
	row("taxes & charges", bill.Taxes)
	b.WriteString(components.DashRule())
	b.WriteString("  " + justify(theme.BrightStyle.Render("to pay ("+payLabel+")"),
		theme.BrightStyle.Render(fmt.Sprintf("₹%d", bill.ToPay)), w) + "\n")
	return b.String()
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

// WithBill attaches Swiggy's real pricing breakdown (live mode).
func (c Cart) WithBill(b Bill) Cart { c.bill = b; return c }

// WithMinNotice sets a notice shown when the cart is below a minimum.
func (c Cart) WithMinNotice(s string) Cart { c.minNotice = s; return c }

// WithLiveSync marks the cart as live and carries the last sync error. In live
// mode without real pricing yet, the bill area shows a syncing/error state
// instead of the mock placeholder split.
func (c Cart) WithLiveSync(live bool, syncErr string) Cart {
	c.liveMode = live
	c.syncErr = syncErr
	return c
}

// billToPay applies the design bill: item + delivery − coupon, or 0 when empty.
// Shared by cart and checkout so the two screens never disagree on the total.
func billToPay(itemTotal int) int {
	if itemTotal <= 0 {
		return 0
	}
	// A large coupon must never drive the amount due negative (a sub-₹21 cart
	// would otherwise render a negative "to pay").
	if total := itemTotal + DeliveryFee - CouponAmount; total > 0 {
		return total
	}
	return 0
}

// toPay applies the design bill to the cart's item total.
func (c Cart) toPay() int { return billToPay(c.Total()) }

func (c Cart) Total() int {
	t := 0
	for _, l := range c.lines {
		t += l.UnitPrice() * l.Qty
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
	if c.cursor >= 0 && c.cursor < len(c.lines) {
		c.lines[c.cursor].Qty++
	}
	return c
}

func (c Cart) Dec() Cart {
	if c.cursor >= 0 && c.cursor < len(c.lines) && c.lines[c.cursor].Qty > 1 {
		c.lines[c.cursor].Qty--
	}
	return c
}

func (c Cart) Remove() Cart {
	if c.cursor < 0 || c.cursor >= len(c.lines) {
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

	// Line rows reuse the List full-bleed selected bar. Customised lines carry a
	// faint add-on summary after the name so the same item with different add-ons
	// reads as distinct.
	list := components.List{Cursor: c.cursor}
	for _, l := range c.lines {
		left := l.Item.Name
		if s := AddOnSummary(l.AddOns); s != "" {
			left += theme.FaintStyle.Render("  + " + s)
		}
		left += theme.DimStyle.Render(fmt.Sprintf("    x%d", l.Qty))
		list.Rows = append(list.Rows, components.Row{
			Left:  left,
			Right: theme.PriceStyle.Render(fmt.Sprintf("₹%d", l.UnitPrice()*l.Qty)),
		})
	}
	b.WriteString(list.View())

	// Bill breakdown — real Swiggy split when synced; a syncing/error state in
	// live mode before pricing arrives; the design mock math otherwise.
	switch {
	case c.bill.Live:
		b.WriteString(renderBill(w, c.bill, "COD"))
	case c.liveMode:
		b.WriteString(components.DashRule())
		if c.syncErr != "" {
			b.WriteString("  " + theme.FavStyle.Render("couldn't sync — "+c.syncErr) + "\n")
		} else {
			b.WriteString("  " + theme.DimStyle.Render("syncing cart…") + "\n")
		}
		b.WriteString(components.DashRule())
	default:
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
	}

	if c.minNotice != "" {
		b.WriteString("\n  " + theme.FavStyle.Render(c.minNotice) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(components.Hint("↑↓", "move", "←→", "qty", "↵", "checkout", "esc", "back"))
	return b.String()
}
