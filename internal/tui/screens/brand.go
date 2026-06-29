package screens

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"consolestore/internal/tui/theme"
)

// BrandHeaderLines is the rendered height of BrandBanner (brand line + gold
// rule), so the root can reserve list space for it.
const BrandHeaderLines = 2

// brandGlint is the white highlight that sweeps across the wordmark.
var brandGlint = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Bold(true)

// shimmerBrand renders text with a cool blue→cyan→bright gradient and a white
// glint that sweeps left-to-right on the frame — the splash's shimmer language,
// shrunk to a single line so the top bar carries the same life.
func shimmerBrand(s string, frame int) string {
	runes := []rune(s)
	n := len(runes)
	if n == 0 {
		return ""
	}
	hues := []string{theme.Cursor, theme.Price, theme.Bright} // blue → cyan → bright
	head := (frame / 3) % (n + 8)                             // sweep, then rest in the gap
	var b strings.Builder
	for i, r := range runes {
		if i == head || i == head-1 {
			b.WriteString(brandGlint.Render(string(r)))
			continue
		}
		stop := i * len(hues) / n
		if stop >= len(hues) {
			stop = len(hues) - 1
		}
		b.WriteString(theme.Fg(hues[stop]).Bold(true).Render(string(r)))
	}
	return b.String()
}

// BrandBanner is the single top bar above every post-landing screen: the
// shimmering consolestore.in wordmark (left, with version), then the compact
// delivery address + cart chip (right), under a full-width gold rule. The
// address shows just its label (e.g. "Home") — the full street lives in the
// address modal — so it stays one line, not a paragraph. Any of
// addrLine/addrLabel/cartChip may be "" to omit.
func BrandBanner(width, frame int, addrLine, addrLabel, cartChip string) string {
	inner := width - 4 // 2-space margin each side, matching the body grid
	if inner < 24 {
		inner = 24
	}

	brand := theme.Fg(theme.Cursor).Bold(true).Render("▍ ") +
		shimmerBrand("consolestore.in", frame) + "  " + theme.PurpleStyle.Render(Version)

	// Compact address: prefer the short label ("Home"); fall back to a tightly
	// truncated street so the bar never wraps.
	addr := ""
	if who := addrLabel; who != "" || addrLine != "" {
		if who == "" {
			who = railTrunc2(addrLine, 18)
		}
		addr = theme.DimStyle.Render("deliver to ") + theme.GreenStyle.Render("⊕ ") +
			theme.BrightStyle.Render(who) + theme.FaintStyle.Render(" ⌄")
	}

	chip := ""
	if cartChip != "" {
		cs := theme.CartStyle
		if strings.Contains(cartChip, "empty") {
			cs = theme.DimStyle
		}
		chip = cs.Render(cartChip)
	}

	// Right cluster: address then cart, separated when both present.
	right := addr
	if addr != "" && chip != "" {
		right += theme.FaintStyle.Render("   ·   ") + chip
	} else {
		right += chip
	}

	gap := inner - lipgloss.Width(brand) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	line1 := "  " + brand + strings.Repeat(" ", gap) + right + "  "
	rule := "  " + theme.GoldStyle.Render(strings.Repeat("─", inner)) + "  "
	return line1 + "\n" + rule + "\n"
}
