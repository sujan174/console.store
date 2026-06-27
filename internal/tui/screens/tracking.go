package screens

import (
	"strings"

	"console.store/internal/tui/components"
	"console.store/internal/tui/theme"
)

// Tracking is the live order-tracking screen: an animated route map, a step
// timeline, ETA (live or time-estimated), and a rider contact note.
type Tracking struct {
	place, addrLine, orderID string
	placedAt                 int64
	etaLo, etaHi             int
	liveStatus, liveETA      string
}

// NewTracking constructs a Tracking screen with order metadata and ETA bounds.
func NewTracking(place, addrLine, orderID string, placedAt int64, etaLo, etaHi int) Tracking {
	return Tracking{
		place:    place,
		addrLine: addrLine,
		orderID:  orderID,
		placedAt: placedAt,
		etaLo:    etaLo,
		etaHi:    etaHi,
	}
}

// WithLive returns a copy of t with the live status and ETA override applied.
func (t Tracking) WithLive(status, eta string) Tracking {
	t.liveStatus = status
	t.liveETA = eta
	return t
}

// Resolve picks the live status when available, else the time fallback.
func (t Tracking) Resolve(nowUnix int64) TrackState {
	if st, delivered, ok := StageFromStatus(t.liveStatus); ok {
		eta := t.liveETA
		if eta == "" {
			eta = "arriving"
		}
		if delivered {
			eta = "delivered"
		}
		return TrackState{Stage: st, Delivered: delivered, ETAText: eta, Estimated: false}
	}
	return TrackProgressByTime(t.placedAt, t.etaLo, t.etaHi, nowUnix)
}

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
	delivered := step >= len(TrackStages)

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

// View renders the full tracking screen. nowUnix is the current Unix timestamp
// (clock as parameter for testability). frame and spin drive the animation.
func (t Tracking) View(nowUnix int64, frame int, spin string) string {
	ts := t.Resolve(nowUnix)
	var b strings.Builder
	w := components.ContentWidth()

	// header: ← tracking · {orderID}  …  {place}
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
	for _, line := range routeScene(ts.Stage, frame, w) {
		b.WriteString("  " + line + "\n")
	}
	b.WriteString("\n")

	// step rows from TrackStages: i<Stage ● done, i==Stage spinner, else ○;
	// append " (est.)" to the active row label when ts.Estimated.
	for i, label := range TrackStages {
		var mark, text string
		activeLabel := label
		if i == ts.Stage && ts.Estimated {
			activeLabel = label + " (est.)"
		}
		switch {
		case i < ts.Stage:
			mark = theme.GreenStyle.Render("●")
			text = theme.TextStyle.Render(label)
		case i == ts.Stage:
			mark = theme.GoldStyle.Render(spin)
			text = theme.BrightStyle.Render(activeLabel)
		default:
			mark = theme.FaintStyle.Render("○")
			text = theme.DimStyle.Render(label)
		}
		b.WriteString("  " + padTo(mark, 2) + text + "\n")
	}
	b.WriteString("\n")

	if ts.Delivered {
		if ts.Estimated {
			// Time-based delivered estimate — can't confirm yet.
			b.WriteString("  " + theme.DimStyle.Render(padTo("status", 7)) + theme.GoldStyle.Render("est. delivered") + "\n")
			b.WriteString("  " + theme.DimStyle.Render("confirm delivery & contact rider → open the Swiggy app") + "\n\n")
		} else {
			// Live confirmed delivery.
			b.WriteString("  " + theme.DimStyle.Render(padTo("status", 7)) + theme.GreenStyle.Render("delivered ✓") + "\n")
			b.WriteString("  " + theme.DimStyle.Render("rider contact & live map → open the Swiggy app") + "\n\n")
			b.WriteString("  " + theme.GreenStyle.Bold(true).Render("enjoy your order!") + "\n")
			b.WriteString("  " + theme.DimStyle.Render("rate the delivery in the Swiggy app — thank you!") + "\n\n")
		}
	} else {
		// ETA line: ts.ETAText; when !Estimated also show verbatim liveStatus.
		etaLine := ts.ETAText
		if !ts.Estimated && t.liveStatus != "" {
			etaLine = t.liveStatus + " · " + ts.ETAText
		}
		b.WriteString("  " + theme.DimStyle.Render(padTo("ETA", 7)) + theme.GreenStyle.Render(etaLine) + "\n")
		b.WriteString("  " + theme.DimStyle.Render("rider contact & live map → open the Swiggy app") + "\n\n")
	}

	b.WriteString(components.Hint("d", "done", "esc", "back to menu"))
	return b.String()
}
