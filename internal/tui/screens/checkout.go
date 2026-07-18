package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"consolestore/internal/catalog"
	"consolestore/internal/tui/components"
	"consolestore/internal/tui/theme"
)

type Checkout struct {
	restaurant string
	addr       catalog.Address
	lines      []CartLine
	placed     bool
	orderID    string
	eta        string
	placing    bool // true while PlaceOrderCmd is in-flight
	bill       Bill // Swiggy's real pricing breakdown (live mode)
	cursor     int
	liveMode   bool
	syncErr    string
	orderErr   string // last place-order failure / blocked-order reason
	mutating   bool
	viewportH  int  // terminal height; windows the item list so the page never overflows
	cartWait   bool // live cart fetch in flight — an empty cart shows a loader, never "empty"
	hour       int  // local hour (0–23) for the loaders' late-night copy
	// UPI payment sub-state: 0 idle, 1 waiting, 2 confirming, 3 failed.
	payStage       int
	payLink        string // hosted pay page (our /pay?upi=… or Swiggy bridge) — opened in the browser
	payTotal       int
	payExpS        int    // seconds left in the payment window (0 = unknown/expired)
	payMethodLabel string // bill "to pay (…)" method tag; "" renders as "COD"
	// Real per-order speed metrics, measured by the root model (elapsed from the
	// first add to placement + keystrokes over that span). Zero = unmeasured, in
	// which case the speed receipt is omitted entirely rather than faked.
	orderSecs float64
	orderKeys int
	bestSecs  float64 // fastest order this session (0 = none yet)
}

// WithPayment attaches the UPI payment sub-state (stage 0=idle,1=waiting,
// 2=confirming,3=failed): the hosted browser payment link, the amount, and the
// seconds left before the payment window closes. No in-terminal QR — the browser
// page (opened with enter) renders a reliable QR that can't go stale here.
func (c Checkout) WithPayment(stage int, payLink string, amount, expiresInSec int) Checkout {
	c.payStage = stage
	c.payLink = payLink
	c.payTotal = amount
	c.payExpS = expiresInSec
	return c
}

// WithPayLink sets the http(s) payment page the in-terminal QR encodes (and that
// enter opens in the browser). Empty leaves the payment page QR-less — exactly
// today's enter-to-open rendering.
func (c Checkout) WithPayLink(l string) Checkout { c.payLink = l; return c }

// WithPayMethod sets the bill's "to pay (…)" method tag (e.g. "UPI"). Empty
// renders as "COD" — the default for a cash-on-delivery bill.
func (c Checkout) WithPayMethod(label string) Checkout { c.payMethodLabel = label; return c }

// payLabel is the method shown in the bill's "to pay (…)" row, defaulting to
// "COD" when unset.
func (c Checkout) payLabel() string {
	if c.payMethodLabel == "" {
		return "COD"
	}
	return c.payMethodLabel
}

func NewCheckout(restaurant string, addr catalog.Address, lines []CartLine, eta string) Checkout {
	return Checkout{restaurant: restaurant, addr: addr, lines: lines, eta: eta}
}

// WithBill attaches Swiggy's real pricing breakdown (live mode).
func (c Checkout) WithBill(b Bill) Checkout { c.bill = b; return c }

// Placed returns a confirm-state copy carrying the order id and eta.
func (c Checkout) Placed(orderID, eta string) Checkout {
	c.placed = true
	c.orderID = orderID
	c.eta = eta
	return c
}

// WithSpeedStats attaches the real measured speed of this order: elapsed
// seconds from the first add to placement, keystrokes over that span, and the
// session-best time. Any zero value suppresses the speed receipt (see
// speedReceipt) so a placeholder is never shown as if measured.
func (c Checkout) WithSpeedStats(secs float64, keys int, bestSecs float64) Checkout {
	c.orderSecs = secs
	c.orderKeys = keys
	c.bestSecs = bestSecs
	return c
}

// WithPlacing returns a copy in the "placing order" in-flight state (disables the CTA).
func (c Checkout) WithPlacing(placing bool) Checkout {
	c.placing = placing
	return c
}

// WithViewport sets the terminal height so the item list windows to fit (the
// bill + place bar + COD line are fixed chrome; the cart rows scroll within
// whatever height remains, keeping the brand header and footer on screen).
func (c Checkout) WithViewport(h int) Checkout { c.viewportH = h; return c }

// lineRows is the item-list viewport: the height minus checkout's fixed chrome
// (header, delivery meta, the 4-line bill, place bar, COD line, hints, brand,
// footer). 0 when the height is unknown (show all).
func (c Checkout) lineRows() int {
	if c.viewportH == 0 {
		return 0
	}
	chrome := 22 // measured: everything on the page that isn't an item row
	if c.compactBill() {
		chrome = 16 // compact bill drops the itemized split + COD line
	}
	if n := c.viewportH - chrome; n >= 2 {
		return n
	}
	return 2
}

// compactBill reports whether the page should collapse the itemized bill to a
// single "to pay" line and drop the COD reminder, so a short terminal still fits
// the header, a couple of items, the total, and the place-order bar on screen.
func (c Checkout) compactBill() bool { return c.viewportH > 0 && c.viewportH < 24 }

// WithCursor sets the focused line index (clamped in View).
func (c Checkout) WithCursor(i int) Checkout { c.cursor = i; return c }

// Cursor returns the focused line index.
func (c Checkout) Cursor() int { return c.cursor }

// WithLiveSync marks the page live and carries the last sync error (drives the
// bill's syncing/error state, same as the old cart screen).
func (c Checkout) WithLiveSync(live bool, syncErr string) Checkout {
	c.liveMode = live
	c.syncErr = syncErr
	return c
}

// WithMutating marks a reduce/delete sync as in flight (freezes the CTA + line).
func (c Checkout) WithMutating(m bool) Checkout { c.mutating = m; return c }

// WithCartWait marks the live cart fetch as still in flight: while true, an
// EMPTY line list renders the CartLoading scene instead of "your cart is
// empty" — the empty state must never flash before the truth arrives.
func (c Checkout) WithCartWait(wait bool) Checkout { c.cartWait = wait; return c }

// WithHour sets the local hour (0–23), which flips the loaders' copy to the
// late-night set. Chained at render time by the root, like WithPlacing.
func (c Checkout) WithHour(h int) Checkout { c.hour = h; return c }

// WithOrderErr carries the last place-order failure (or the blocked-order
// reason, e.g. a sold-out item), shown prominently above the place-order bar.
func (c Checkout) WithOrderErr(s string) Checkout { c.orderErr = s; return c }

// hasUnavailable reports whether any line is flagged out of stock.
func (c Checkout) hasUnavailable() bool {
	for _, l := range c.lines {
		if l.Unavailable {
			return true
		}
	}
	return false
}

func (c Checkout) clampCursor() int {
	i := c.cursor
	if i >= len(c.lines) {
		i = len(c.lines) - 1
	}
	if i < 0 {
		i = 0
	}
	return i
}

// Up / Down move the line cursor.
func (c Checkout) Up() Checkout   { c.cursor--; c.cursor = c.clampCursor(); return c }
func (c Checkout) Down() Checkout { c.cursor++; c.cursor = c.clampCursor(); return c }

func (c Checkout) IsPlaced() bool { return c.placed }

// Place returns the restaurant/store name (used to seed tracking).
func (c Checkout) Place() string { return c.restaurant }

// OrderID returns the placed order's id (used to seed tracking).
func (c Checkout) OrderID() string { return c.orderID }

// ETA is the delivery estimate captured at Placed(); empty if not yet placed.
func (c Checkout) ETA() string { return c.eta }

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

// payAmount is the amount shown as due — Swiggy's real to-pay in live mode.
func (c Checkout) payAmount() int {
	if c.bill.Live {
		return c.bill.ToPay
	}
	return c.toPay()
}

// PayAmount exposes payAmount to callers outside the package (the root
// model's order-confirm modal, which shows the same due amount).
func (c Checkout) PayAmount() int { return c.payAmount() }

// SwiggyBetaOrderCap is Swiggy's MCP beta ceiling: place_food_order is rejected
// for carts of ₹1000 or more. We surface it in-app and block the CTA before it
// can fail server-side.
const SwiggyBetaOrderCap = 1000

// OverCap reports whether the amount due hits Swiggy's ₹1000 beta ceiling, so
// the order can only be placed through the Swiggy app.
func (c Checkout) OverCap() bool { return c.payAmount() >= SwiggyBetaOrderCap }

// capNotice is the evident callout shown when the order is at/over the beta cap:
// a gold-bordered box telling the user to use the Swiggy app for this one.
func capNotice() string {
	inner := theme.GoldStyle.Bold(true).Render("⚠  order is ₹1000 or more") + "\n" +
		theme.BrightStyle.Render("can't be placed here — Swiggy's MCP is in beta.") + "\n" +
		theme.DimStyle.Render("place this one in the Swiggy app instead.")
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.Gold)).
		Padding(0, 2).
		Render(inner)
	lines := strings.Split(box, "\n")
	for i, l := range lines {
		lines[i] = "  " + l // indent to the page gutter
	}
	return strings.Join(lines, "\n")
}

func (c Checkout) Init() tea.Cmd { return nil }

func (c Checkout) View(frame int) string {
	if c.placed {
		return c.confirmView(frame)
	}
	if c.payStage != 0 {
		return c.paymentView(frame)
	}
	return c.summaryView(frame)
}

// paymentView renders the UPI scan-to-pay flow: a QR of the UPI intent the user
// scans with any UPI app, then a "paid → placing" beat, or a failure with a way
// back. Shown when Swiggy has disabled Cash and the order awaits online payment.
func (c Checkout) paymentView(frame int) string {
	var b strings.Builder
	title := theme.BrandStyle.Render("payment")
	if c.restaurant != "" {
		title += theme.FaintStyle.Render("  ·  ") + theme.DimStyle.Render(c.restaurant)
	}
	b.WriteString("  " + title + "\n")
	b.WriteString(components.Divider() + "\n\n")

	switch c.payStage {
	case 3: // failed / timed out
		msg := c.orderErr
		if msg == "" {
			msg = "payment failed — nothing was charged."
		}
		b.WriteString("  " + theme.FavStyle.Render("⚠ "+msg) + "\n\n")
		b.WriteString(components.Hint("esc", "back to cart"))
	case 2: // paid, finalizing
		b.WriteString("  " + theme.GreenStyle.Render("✓ paid") +
			theme.DimStyle.Render("  ·  placing your order… ") + theme.GoldStyle.Render(spinAt(frame)) + "\n")
	default: // waiting for payment
		amt := c.payTotal
		if amt == 0 { // place response didn't carry an amount — use the synced bill
			amt = c.payAmount()
		}
		b.WriteString("  " + theme.BrightStyle.Render(fmt.Sprintf("pay  ₹%d", amt)) + theme.DimStyle.Render("  ·  UPI") + "\n\n")

		// In-terminal QR of the payment page: bright half-block cells scan
		// straight off the terminal with any phone camera / UPI app. Never blocks
		// payment — a nil block (encode failed / no link) falls back to just the
		// enter-to-open hint below, exactly as before.
		if c.payLink != "" {
			if qr := components.QRBlock(c.payLink); qr != nil {
				for _, ln := range qr {
					b.WriteString("  " + theme.BrightStyle.Render(ln) + "\n")
				}
				b.WriteString("  " + theme.DimStyle.Render("scan with your phone — opens the payment page") + "\n\n")
			}
		}

		// A scannable QR that can't go stale also lives on the browser page
		// (opened with enter). Show the enter-to-open button + a live countdown so
		// a hours-old screen never invites a (refund-causing) payment.
		b.WriteString("  " + theme.GreenStyle.Render("❯ press enter") +
			theme.BrightStyle.Render(" to open the payment page & scan the QR") + "\n")
		if c.payExpS > 0 {
			b.WriteString("  " + theme.DimStyle.Render("expires in ") +
				theme.GoldStyle.Render(fmt.Sprintf("%d:%02d", c.payExpS/60, c.payExpS%60)) + "\n")
		}
		b.WriteString("\n  " + theme.DimStyle.Render("waiting for payment ") + theme.GoldStyle.Render(spinAt(frame)) + "\n\n")
		b.WriteString(components.Hint("esc", "cancel") + theme.FaintStyle.Render("   pay within the window or place again"))
	}
	return b.String()
}

// padTo right-pads s with spaces to the given display width.
func padTo(s string, width int) string {
	if pad := width - lipgloss.Width(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

func (c Checkout) summaryView(frame int) string {
	var b strings.Builder
	w := components.ContentWidth()

	// Header: "checkout · {restaurant}" — the restaurant gives the page context.
	title := theme.BrandStyle.Render("checkout")
	if c.restaurant != "" && len(c.lines) > 0 {
		title += theme.FaintStyle.Render("  ·  ") + theme.DimStyle.Render(c.restaurant)
	}
	b.WriteString("  " + title + "\n")
	b.WriteString(components.Divider())
	b.WriteString("\n")

	// Cart still in flight — never claim "empty" before the fetch lands; the
	// server may be about to hand us a cart built in the app or last session.
	if len(c.lines) == 0 && c.cartWait {
		b.WriteString(CartLoading(frame, c.hour, w))
		b.WriteString("\n" + components.Hint("esc", "back"))
		return b.String()
	}

	// Empty state — no bill, no place-order bar; a calm prompt back to browsing.
	if len(c.lines) == 0 {
		b.WriteString("\n  " + theme.DimStyle.Render("your cart is empty") + "\n\n")
		b.WriteString("  " + theme.FaintStyle.Render("press ") + theme.CursorStyle.Render("esc") +
			theme.FaintStyle.Render(" to browse the menu") + "\n\n\n")
		b.WriteString(components.Hint("esc", "back"))
		return b.String()
	}

	// Delivery meta — two aligned lines, then a blank for breathing room.
	label := func(s string) string { return theme.DimStyle.Render(padTo(s, 11)) }
	b.WriteString("  " + label("deliver to") + theme.TextStyle.Render(addrLine(c.addr)) +
		theme.DimStyle.Render("  ·  "+c.addr.Label) + "\n")
	fromLine := theme.TextStyle.Render(c.restaurant)
	if c.eta != "" {
		fromLine += theme.FaintStyle.Render("  ·  ") + theme.DimStyle.Render(c.eta)
	}
	b.WriteString("  " + label("from") + fromLine + "\n\n")

	// Item rows — full-bleed cursor bar. The focused row carries the − ×N +
	// stepper; others a dim ×N. Fixed-width qty + price cells keep the ₹ column
	// aligned straight down the list.
	cur := c.clampCursor()
	stepW := lipgloss.Width("−  ×99  +")
	priceW := lipgloss.Width("₹9999")
	list := components.List{Cursor: cur, MaxRows: c.lineRows()}
	for i, l := range c.lines {
		// Sold-out line: dim it, tag it, and keep the stepper so the user can
		// remove it (the only way to unblock checkout).
		if l.Unavailable {
			name := theme.FaintStyle.Render(l.Item.Name) +
				theme.FavStyle.Render("  · sold out")
			var qty string
			if i == cur {
				qty = theme.FavStyle.Render("−") + "  " +
					theme.FaintStyle.Render(fmt.Sprintf("×%d", l.Qty)) + "  " +
					theme.FaintStyle.Render("+")
			} else {
				qty = theme.FaintStyle.Render(fmt.Sprintf("×%d", l.Qty))
			}
			qtyCell := lipgloss.PlaceHorizontal(stepW, lipgloss.Right, qty)
			priceCell := lipgloss.PlaceHorizontal(priceW, lipgloss.Right,
				theme.FaintStyle.Render(fmt.Sprintf("₹%d", l.UnitPrice()*l.Qty)))
			list.Rows = append(list.Rows, components.Row{
				Left: name, Right: qtyCell + "    " + priceCell, BarGreen: i == cur,
			})
			continue
		}
		name := theme.BrightStyle.Render(l.Item.Name)
		if s := AddOnSummary(l.AddOns); s != "" {
			name += theme.FaintStyle.Render("  + " + s)
		}
		var qty string
		if i == cur {
			qty = theme.FavStyle.Render("−") + "  " +
				theme.GreenStyle.Render(fmt.Sprintf("×%d", l.Qty)) + "  " +
				theme.GreenStyle.Render("+")
		} else {
			qty = theme.DimStyle.Render(fmt.Sprintf("×%d", l.Qty))
		}
		qtyCell := lipgloss.PlaceHorizontal(stepW, lipgloss.Right, qty)
		priceCell := lipgloss.PlaceHorizontal(priceW, lipgloss.Right,
			theme.PriceStyle.Render(fmt.Sprintf("₹%d", l.UnitPrice()*l.Qty)))
		list.Rows = append(list.Rows, components.Row{
			Left: name, Right: qtyCell + "    " + priceCell, BarGreen: i == cur,
		})
	}
	b.WriteString(list.View())
	b.WriteString("\n")

	// Bill split — Swiggy's real numbers when synced; a syncing/error state in
	// live mode before they arrive; design math in mock mode. On a short terminal
	// the itemized split collapses to a single "to pay" line so the CTA still fits.
	compact := c.compactBill()
	switch {
	case c.cartWait:
		// A cart write is still settling — the last synced bill prices lines
		// the user has already changed, so showing it would lie. Hold with the
		// pulse-family spinner until the chain converges.
		b.WriteString(components.DashRule())
		b.WriteString("  " + theme.GoldStyle.Render(spinAt(frame)) + " " +
			theme.DimStyle.Render("updating bill…") + "\n")
		b.WriteString(components.DashRule())
	case compact && c.bill.Live:
		b.WriteString(components.DashRule())
		b.WriteString("  " + justify(theme.BrightStyle.Render("to pay ("+c.payLabel()+")"),
			theme.BrightStyle.Render(fmt.Sprintf("₹%d", c.bill.ToPay)), w) + "\n")
	case compact && !c.liveMode:
		b.WriteString(components.DashRule())
		b.WriteString("  " + justify(theme.BrightStyle.Render("to pay ("+c.payLabel()+")"),
			theme.BrightStyle.Render(fmt.Sprintf("₹%d", c.toPay())), w) + "\n")
	case c.bill.Live:
		b.WriteString(renderBill(w, c.bill, c.payLabel()))
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
		b.WriteString("  " + justify(theme.BrightStyle.Render("to pay ("+c.payLabel()+")"),
			theme.BrightStyle.Render(fmt.Sprintf("₹%d", c.toPay())), w) + "\n")
	}
	b.WriteString("\n")

	// Notices above the CTA, in priority order so the one blocker that matters
	// can't be missed: the ₹1000 beta cap (evident bordered box) first, then a
	// place-order error, then a sold-out block.
	blocked := c.hasUnavailable()
	over := c.OverCap()
	switch {
	case over:
		b.WriteString(capNotice() + "\n\n")
	case c.orderErr != "":
		b.WriteString("  " + theme.FavStyle.Render("⚠ "+c.orderErr) + "\n")
	case blocked:
		b.WriteString("  " + theme.FavStyle.Render("⚠ a sold-out item is in your cart — remove it to order") + "\n")
	}

	// Full-bleed place-order bar: green left bar + selected-row background. The
	// bar reads dim/disabled when the order is blocked (sold-out item) or over
	// the ₹1000 beta cap.
	disabled := blocked || over || c.cartWait
	barLabel := " ❯ place order "
	switch {
	case c.placing:
		barLabel = " placing order… "
	case c.mutating:
		barLabel = " syncing… "
	case over:
		barLabel = " order ₹1000+ — use the Swiggy app "
	case blocked:
		barLabel = " place order — remove sold-out item "
	}
	barBar := theme.GreenStyle.Render("▌")
	barBg := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Bright)).
		Background(lipgloss.Color(theme.SelRowBg))
	if disabled {
		barBar = theme.FaintStyle.Render("▌")
		barBg = theme.DimStyle
	}
	bar := barBar + barBg.Render(padTo(barLabel, components.FrameWidth()-1))
	b.WriteString(bar + "\n\n")

	// One tidy COD line instead of two stacked notices — dropped on a short
	// terminal where every row counts (the bar already says "place order").
	if !compact {
		b.WriteString("  " + theme.GoldStyle.Render("pay the rider — cash / UPI") +
			theme.FaintStyle.Render("   ·   ") + theme.DimStyle.Render("can't cancel once placed") + "\n\n")
	}
	b.WriteString(components.Hint("↑↓", "move", "←→", "qty", "⌫", "remove", "↵", "place order", "esc", "back"))
	return b.String()
}

func (c Checkout) confirmView(frame int) string {
	var b strings.Builder

	// The full celebration (steam + cup + speed receipt) is ~25 rows tall. On a
	// short terminal that overflows and scrolls the order box off-screen, so below
	// a threshold we drop the decorative art and keep the essentials. 0 = unknown.
	compact := c.viewportH > 0 && c.viewportH < 28

	if !compact {
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
	}

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
	b.WriteString("  " + theme.DimStyle.Render(fmt.Sprintf("pay ₹%d to rider (cash/UPI)", c.payAmount())) + "\n")
	b.WriteString("  " + theme.FavStyle.Render("can't be cancelled now") + "\n\n")

	if !compact {
		if r := c.speedReceipt(); r != "" {
			b.WriteString(r)
		}
	}

	b.WriteString("  " + theme.GreenStyle.Render("↵") + " " + theme.FaintStyle.Render("track") +
		"     " + theme.CursorStyle.Render("esc") + " " + theme.FaintStyle.Render("back to menu"))
	return b.String()
}

// phoneAppAvgLabel is a deliberately-rounded, clearly-framed ESTIMATE of how
// long the same order takes tapping through a phone app — a reference point,
// not a per-order measurement.
const phoneAppAvgLabel = "~45s"

// speedReceipt renders the post-order speed flex from REAL measured values
// (elapsed + keystrokes from the first add to placement). It returns "" when the
// order wasn't measured (orderSecs/orderKeys zero) so a fabricated receipt is
// never shown — the project's real-strings stance.
func (c Checkout) speedReceipt() string {
	if c.orderSecs <= 0 || c.orderKeys <= 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("  " + theme.GoldStyle.Render(fmt.Sprintf("⚡ ordered in %.1fs · %d keystrokes", c.orderSecs, c.orderKeys)) + "\n")
	best := c.bestSecs
	if best <= 0 || c.orderSecs < best {
		best = c.orderSecs // this order is the session best (or the only one)
	}
	b.WriteString("     " + theme.DimStyle.Render(fmt.Sprintf("this session best %.1fs  ·  phone app %s", best, phoneAppAvgLabel)) + "\n")
	b.WriteString("  " + theme.FaintStyle.Render(strings.Repeat("─", 44)) + "\n")
	b.WriteString("  " + theme.DimStyle.Render("flex it:  ") + theme.BrandStyle.Render("curl consolestore.in/install") + "\n\n")
	return b.String()
}

func addrLine(a catalog.Address) string {
	if a.Full != "" {
		return a.Full
	}
	return a.Line
}
