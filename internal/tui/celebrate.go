package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"consolestore/internal/tui/theme"
)

// confGlyphs are the width-1, emoji-free confetti glyphs (ported from
// experiments/asciilab/demo_confetti.go's burst look, kept simple here).
var confGlyphs = []rune("▪●◆✦*·")

// confColors cycles the Tokyo Night accents across confetti particles.
var confColors = []string{theme.Cursor, theme.Price, theme.Green, theme.Gold, theme.Fav, theme.Purple}

// confParticle is a deterministic function of (slot, tick): given the same
// inputs it always draws the same glyph/color/position, so confettiView needs
// no stored particle state — it derives the whole frame from m.confirmTick.
type confParticle struct {
	x, y int
	ch   rune
	col  string
}

// confettiFrame computes the particle burst for a given viewport size and
// tick, pure and deterministic (no RNG, no mutable state): each particle's
// trajectory is a closed-form function of its slot index and the tick count,
// so the same (w, h, tick) always renders the same frame.
func confettiFrame(w, h, tick int) []confParticle {
	if w <= 0 || h <= 0 {
		return nil
	}
	const n = 36
	parts := make([]confParticle, 0, n)
	for i := 0; i < n; i++ {
		// Spread starting x across the width using a simple deterministic
		// hash of the slot index; each particle falls at its own rate and
		// drifts left/right based on its slot parity.
		startX := (i*37 + 5) % w
		drift := (i%7 - 3) // -3..3
		fallRate := 1 + (i % 3)
		t := tick - (i % 5) // stagger the burst starts slightly
		if t < 0 {
			continue
		}
		y := (t * fallRate) / 4
		x := startX + (t*drift)/6
		if y < 0 || y >= h || x < 0 || x >= w {
			continue
		}
		parts = append(parts, confParticle{
			x:   x,
			y:   y,
			ch:  confGlyphs[i%len(confGlyphs)],
			col: confColors[i%len(confColors)],
		})
	}
	return parts
}

// confettiView renders the full-page, chrome-free celebration shown right
// after an order is placed (scrConfirm). Like loaderView, it is a dead end in
// View: the caller returns it directly instead of assembling the usual brand
// banner / footer chrome, so it must be exactly m.h lines on its own. The
// confetti burst is a pure function of m.confirmTick, never stored particle
// state, so the view is trivially deterministic and testable.
func confettiView(m Model) string {
	w, h := m.w, m.h
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}

	grid := make([][]rune, h)
	colors := make([][]string, h)
	for y := 0; y < h; y++ {
		grid[y] = make([]rune, w)
		colors[y] = make([]string, w)
		for x := 0; x < w; x++ {
			grid[y][x] = ' '
		}
	}
	for _, p := range confettiFrame(w, h, m.confirmTick) {
		grid[p.y][p.x] = p.ch
		colors[p.y][p.x] = p.col
	}

	msg := "✓ order placed"
	eta := m.checkout.ETA()
	sub := ""
	if eta != "" {
		sub = "eta " + eta
	}

	msgY := h / 2
	subY := msgY + 1

	lines := make([]string, h)
	for y := 0; y < h; y++ {
		if y == msgY {
			lines[y] = lipgloss.PlaceHorizontal(w, lipgloss.Center, theme.GreenStyle.Bold(true).Render(msg))
			continue
		}
		if y == subY && sub != "" {
			lines[y] = lipgloss.PlaceHorizontal(w, lipgloss.Center, theme.DimStyle.Render(sub))
			continue
		}
		var b strings.Builder
		for x := 0; x < w; x++ {
			ch := grid[y][x]
			if ch == ' ' {
				b.WriteByte(' ')
				continue
			}
			b.WriteString(theme.Fg(colors[y][x]).Render(string(ch)))
		}
		lines[y] = b.String()
	}
	return strings.Join(lines, "\n")
}
