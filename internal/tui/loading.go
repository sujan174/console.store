package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"consolestore/internal/tui/theme"
)

// scooterFrames are the two-frame "wheels turning" animation for the loader's
// scooter glyph, cycled on m.frame like the braille spinner. Width-1,
// emoji-free glyphs only (see screens/tracking.go's routeScene and
// dodge/sprites.go's scooterSprite for the same convention).
var scooterFrames = []string{"o", "O"}

// loaderView renders a full-page, chrome-free loading screen shown while an
// order placement is in flight (m.placingOrder on the checkout screen). It is
// a dead end in View: the caller returns it directly instead of assembling
// the usual brand banner / footer chrome, so it must be exactly m.h lines on
// its own.
func loaderView(m Model) string {
	w, h := m.w, m.h
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}

	message := theme.BrightStyle.Bold(true).Render("placing your order…") + " " + theme.GoldStyle.Render(m.spin())

	road := roadLine(m.frame, w)
	shimmer := shimmerLine(m.frame, w)

	lines := make([]string, 0, h)
	// Vertically center the message + road + shimmer block in the viewport.
	blockHeight := 3
	top := (h - blockHeight) / 2
	if top < 0 {
		top = 0
	}
	for i := 0; i < top; i++ {
		lines = append(lines, "")
	}
	lines = append(lines, lipgloss.PlaceHorizontal(w, lipgloss.Center, message))
	lines = append(lines, lipgloss.PlaceHorizontal(w, lipgloss.Center, road))
	lines = append(lines, lipgloss.PlaceHorizontal(w, lipgloss.Center, shimmer))
	for len(lines) < h {
		lines = append(lines, "")
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	return strings.Join(lines, "\n")
}

// clampWidth clamps w to the inclusive [min, max] range.
func clampWidth(w, min, max int) int {
	if w < min {
		return min
	}
	if w > max {
		return max
	}
	return w
}

// roadLine draws a dotted road with a small scooter driving across it,
// looping position from m.frame so it appears to drive endlessly.
func roadLine(frame, w int) string {
	roadWidth := clampWidth(w-4, 8, 48)

	dots := make([]rune, roadWidth)
	for i := range dots {
		if i%3 == 0 {
			dots[i] = '·'
		} else {
			dots[i] = ' '
		}
	}

	pos := frame % roadWidth
	scooter := scooterFrames[(frame/6)%len(scooterFrames)]

	road := theme.DimStyle.Render(string(dots[:pos])) +
		scooter +
		theme.DimStyle.Render(string(dots[pos:]))
	return road
}

// shimmerLine is a subtle moving highlight strip beneath the road, giving the
// loader a sense of motion even when nothing else changes.
func shimmerLine(frame, w int) string {
	barWidth := clampWidth(w-4, 8, 48)

	bar := make([]rune, barWidth)
	for i := range bar {
		bar[i] = '─'
	}
	pos := frame % barWidth
	return theme.FaintStyle.Render(string(bar[:pos])) +
		theme.GoldStyle.Render("─") +
		theme.FaintStyle.Render(string(bar[pos+1:]))
}
