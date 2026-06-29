package screens

import (
	"strconv"
	"strings"

	"consolestore/internal/tui/components"
	"consolestore/internal/tui/theme"
)

// Tracking is the live order-tracking screen: an animated route map, a step
// timeline, ETA (live or time-estimated), and a rider contact note.
type Tracking struct {
	place, addrLine, orderID string
	placedAt                 int64
	etaLo, etaHi             int
	liveStatus, liveETA      string
	viewportH                int // terminal height; trims the secondary notes on a short screen
}

// WithViewport sets the terminal height so the page can drop secondary "open the
// Swiggy app" / "rate the delivery" notes on a short terminal instead of
// overflowing (which scrolls the header off the top). 0 = unknown → full.
func (t Tracking) WithViewport(h int) Tracking { t.viewportH = h; return t }

// compact reports whether to trim the secondary notes for a short terminal.
func (t Tracking) compact() bool { return t.viewportH > 0 && t.viewportH < 22 }

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

// Resolve picks the live status/ETA when available, else a time estimate. The
// live ETA from track_food_order is authoritative (it's what the CLI shows), so
// it is ALWAYS preferred for the displayed time once a poll has landed — the
// local time-based countdown is only a pre-first-poll placeholder, never an
// override of real data.
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
	// No mappable live status yet — estimate the stage from elapsed time. But if a
	// live ETA arrived, trust it over the local countdown so the time stays synced
	// with Swiggy even when the status phrasing doesn't match our stage rules.
	est := TrackProgressByTime(t.placedAt, t.etaLo, t.etaHi, nowUnix)
	if t.liveETA != "" {
		est.ETAText = t.liveETA
		est.Estimated = false
	}
	return est
}

// etaMinutes pulls the leading integer out of a live ETA string ("11 mins" → 11).
// ok is false when there's no number (e.g. "arriving", "").
func etaMinutes(s string) (int, bool) {
	for _, w := range strings.Fields(s) {
		if n, err := strconv.Atoi(w); err == nil {
			return n, true
		}
	}
	return 0, false
}

func clampFrac(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// journeyFrac is the rider's continuous position along the road in [0,1] —
// proportional to progress, NOT the discrete stage. It uses Swiggy's live ETA as
// the remaining distance: position = covered / (covered + remaining), where
// covered is elapsed minutes and remaining is the live ETA. As the live ETA
// counts down the rider advances smoothly; with no live ETA yet it falls back to
// elapsed vs the initial estimate.
func (t Tracking) journeyFrac(nowUnix int64, delivered bool) float64 {
	if delivered {
		return 1
	}
	// Rider has arrived at the door — park the sprite at the destination.
	if strings.Contains(strings.ToLower(t.liveStatus), "arrived") {
		return 1
	}
	elapsedMin := float64(nowUnix-t.placedAt) / 60
	if elapsedMin < 0 {
		elapsedMin = 0
	}
	if mins, ok := etaMinutes(t.liveETA); ok {
		total := elapsedMin + float64(mins)
		if total <= 0 {
			return 0
		}
		return clampFrac(elapsedMin / total)
	}
	base := float64(t.etaHi)
	if base <= 0 {
		base = 45
	}
	return clampFrac(elapsedMin / base)
}

// wheelSpin are the rotating wheel glyphs (quarter-turns) cycled per frame so
// the courier's wheels appear to spin.
var wheelSpin = []string{"◐", "◓", "◑", "◒"}

// routeScene renders the 3-row animated courier: a rider on a two-wheeler riding
// the road from the restaurant (left) to the delivery address (right). The
// wheels spin and speed streaks flow each frame; once delivered the bike parks
// at the destination with its wheels still. No emoji — pure box/line glyphs.
func routeScene(frac float64, delivered bool, frame, w int) []string {
	if w < 16 {
		w = 16
	}
	const spriteW = 5

	x := int(clampFrac(frac) * float64(w-spriteW)) // left column of the 5-wide sprite

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

	// animated courier scene (3 rows) — the rider's position is proportional to
	// progress (elapsed vs the live ETA remaining), not the discrete stage.
	for _, line := range routeScene(t.journeyFrac(nowUnix, ts.Delivered), ts.Delivered, frame, w) {
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

	compact := t.compact()
	if !compact {
		b.WriteString("\n")
	}
	swiggyNote := func(s string) {
		if !compact {
			b.WriteString("  " + theme.DimStyle.Render(s) + "\n")
		}
	}
	if ts.Delivered {
		if ts.Estimated {
			// Time-based delivered estimate — can't confirm yet.
			b.WriteString("  " + theme.DimStyle.Render(padTo("status", 7)) + theme.GoldStyle.Render("est. delivered") + "\n")
			swiggyNote("confirm delivery & contact rider → open the Swiggy app")
		} else {
			// Live confirmed delivery.
			b.WriteString("  " + theme.DimStyle.Render(padTo("status", 7)) + theme.GreenStyle.Render("delivered ✓") + "\n")
			swiggyNote("rider contact & live map → open the Swiggy app")
			if !compact {
				b.WriteString("\n  " + theme.GreenStyle.Bold(true).Render("enjoy your order!") + "\n")
				swiggyNote("rate the delivery in the Swiggy app — thank you!")
			}
		}
	} else {
		// Live status → a friendly phrase plus a real ETA when we have one (e.g.
		// "Arrived at location" reads as "rider's outside …", with no "N/A"); with
		// no live status yet, the time-based estimate.
		label, line := "ETA", ts.ETAText
		if !ts.Estimated && t.liveStatus != "" {
			label, line = "status", StatusDisplay(t.liveStatus)
			if e := cleanETA(ts.ETAText); e != "" {
				line += " · " + e
			}
		}
		b.WriteString("  " + theme.DimStyle.Render(padTo(label, 7)) + theme.GreenStyle.Render(line) + "\n")
		swiggyNote("rider contact & live map → open the Swiggy app")
	}
	b.WriteString("\n")

	b.WriteString(components.Hint("d", "done", "esc", "back to menu"))
	return b.String()
}
