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

// trackSteps are the timeline rows (reference line 894).
var trackSteps = []string{"order confirmed", "preparing …", "out for delivery", "delivered"}

// trackQuips are the rider status quips indexed by min(trackStep,3) (reference 919-922).
var trackQuips = []string{"picking up your order", "weaving through traffic", "almost at your gate", "knock knock"}

// progressBar renders the dotted route with a solid green portion and a 🛵 at
// the current fraction (reference 403-409).
func progressBar(step int) string {
	pct := step
	if pct > 3 {
		pct = 3
	}
	w := components.ContentWidth()
	filled := pct * w / 3
	var b strings.Builder
	for i := 0; i < w; i++ {
		if i == filled && filled < w {
			b.WriteString("🛵")
		} else if i < filled {
			b.WriteString(theme.GreenStyle.Render("━"))
		} else {
			b.WriteString(theme.FaintStyle.Render("─"))
		}
	}
	return b.String()
}

func (t Tracking) View(trackStep int, spin string) string {
	var b strings.Builder
	w := components.ContentWidth()

	// header: ← tracking · {orderId}  …  {place}
	b.WriteString("  " + justify(
		theme.PriceStyle.Render("← tracking · "+t.orderID),
		theme.DimStyle.Render(t.place), w) + "\n")
	b.WriteString(components.Divider())
	b.WriteString("\n")

	// route endpoints: ☕ {place}  …  {addr.line} ⌂
	b.WriteString("  " + justify(
		theme.GoldStyle.Render("☕ "+t.place),
		theme.PriceStyle.Render(t.addrLine+" ⌂"), w) + "\n")

	// animated route map: ● ━━━🛵──── ⌂
	b.WriteString("  " + theme.GreenStyle.Render("●") + progressBar(trackStep) + theme.PriceStyle.Render("⌂") + "\n\n")

	// step rows (reference 894-901)
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

	// ETA row
	b.WriteString("  " + theme.DimStyle.Render(padTo("ETA", 6)) + theme.GreenStyle.Render("~32 min") + "\n")

	// rider line: rider · Imran · KA 05 1234 · {quip}
	quip := trackQuips[min(trackStep, 3)]
	b.WriteString("  " + theme.DimStyle.Render("rider · Imran · KA 05 1234 · ") + theme.GreenStyle.Render(quip) + "\n\n")

	b.WriteString(components.Hint("esc", "back to menu"))
	return b.String()
}
