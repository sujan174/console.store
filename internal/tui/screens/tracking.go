package screens

import (
	"strings"

	"console.store/internal/tui/components"
	"console.store/internal/tui/theme"
)

// Tracking is the live order-tracking screen: an animated route map, a step
// timeline, ETA, and the rider line (design reference lines 388-423).
type Tracking struct {
	place    string
	addrLine string
	orderID  string
}

func NewTracking(place, addrLine, orderID string) Tracking {
	return Tracking{place: place, addrLine: addrLine, orderID: orderID}
}

// trackSteps are the timeline rows. The order is "delivered" once trackStep
// reaches len(trackSteps) (all four marked done).
var trackSteps = []string{"order confirmed", "preparing …", "out for delivery", "delivered"}

// trackQuips are the rider status quips indexed by min(trackStep,3).
var trackQuips = []string{"picking up your order", "weaving through traffic", "almost at your gate", "knock knock"}

// wheelSpin are the rotating wheel glyphs (quarter-turns) cycled per frame so
// the courier's wheels appear to spin.
var wheelSpin = []string{"◐", "◓", "◑", "◒"}

// routeScene renders the 3-row animated courier: a rider on a two-wheeler riding
// the road from the restaurant (left) to the delivery address (right). The
// wheels spin and speed streaks flow each frame; once delivered the bike parks
// at the destination with its wheels still. No emoji — pure box/line glyphs.
func routeScene(step, frame, w int) []string {
	if w < 16 {
		w = 16
	}
	const spriteW = 5
	delivered := step >= len(trackSteps)

	pct := step
	if pct > 3 {
		pct = 3
	}
	x := pct * (w - spriteW) / 3 // left column of the 5-wide sprite

	wheel := "O" // parked
	if !delivered {
		wheel = wheelSpin[(frame/2)%len(wheelSpin)]
	}

	// Row 2 — the road: green where travelled, the bike chassis + spinning
	// wheels, faint track ahead.
	rightLen := w - x - spriteW
	if rightLen < 0 {
		rightLen = 0
	}
	road := theme.GreenStyle.Render(strings.Repeat("═", x)) +
		theme.PriceStyle.Render(wheel) + theme.GreenStyle.Render("═══") + theme.PriceStyle.Render(wheel) +
		theme.FaintStyle.Render(strings.Repeat("─", rightLen))

	// Rows 0/1 — the rider, hunched over the chassis (col x+1).
	head := strings.Repeat(" ", x+1) + theme.BrightStyle.Render("_o")
	body := strings.Repeat(" ", x+1) + theme.BrightStyle.Render(`-\<`)

	// Speed streaks flowing behind the bike while moving.
	if !delivered && x >= 3 {
		streak := []rune("~  ~")
		for i := range streak {
			if (i+frame/2)%2 == 0 {
				streak[i] = ' '
			}
		}
		body = strings.Repeat(" ", x-3) + theme.FaintStyle.Render(string(streak)) + theme.BrightStyle.Render(`-\<`)
	}

	return []string{head, body, road}
}

func (t Tracking) View(trackStep, frame int, spin string) string {
	var b strings.Builder
	w := components.ContentWidth()
	delivered := trackStep >= len(trackSteps)

	// header: ← tracking · {orderId}  …  {place}
	b.WriteString("  " + justify(
		theme.PriceStyle.Render("← tracking · "+t.orderID),
		theme.DimStyle.Render(t.place), w) + "\n")
	b.WriteString(components.Divider())
	b.WriteString("\n")

	// route endpoints (text labels, no emoji): {place}  …  {addr.line}
	b.WriteString("  " + justify(
		theme.GoldStyle.Render(t.place),
		theme.PriceStyle.Render(t.addrLine), w) + "\n")

	// animated courier scene (3 rows)
	for _, line := range routeScene(trackStep, frame, w) {
		b.WriteString("  " + line + "\n")
	}
	b.WriteString("\n")

	// step rows
	for i, label := range trackSteps {
		var mark, text string
		switch {
		case i < trackStep:
			mark = theme.GreenStyle.Render("●")
			text = theme.TextStyle.Render(label)
		case i == trackStep:
			mark = theme.GoldStyle.Render(spin)
			text = theme.BrightStyle.Render(label)
		default:
			mark = theme.FaintStyle.Render("○")
			text = theme.DimStyle.Render(label)
		}
		b.WriteString("  " + padTo(mark, 2) + text + "\n")
	}
	b.WriteString("\n")

	if delivered {
		// status + the delivered note.
		b.WriteString("  " + theme.DimStyle.Render(padTo("status", 7)) + theme.GreenStyle.Render("delivered ✓") + "\n")
		b.WriteString("  " + theme.DimStyle.Render("rider · Imran · KA 05 1234 · ") + theme.GreenStyle.Render("handed over") + "\n\n")
		b.WriteString("  " + theme.GreenStyle.Bold(true).Render("enjoy your order!") + "\n")
		b.WriteString("  " + theme.DimStyle.Render("rate the delivery & rider later in the app when possible — thank you!") + "\n\n")
	} else {
		b.WriteString("  " + theme.DimStyle.Render(padTo("ETA", 7)) + theme.GreenStyle.Render("~32 min") + "\n")
		quip := trackQuips[min(trackStep, len(trackQuips)-1)]
		b.WriteString("  " + theme.DimStyle.Render("rider · Imran · KA 05 1234 · ") + theme.GreenStyle.Render(quip) + "\n\n")
	}

	b.WriteString(components.Hint("esc", "back to menu"))
	return b.String()
}
