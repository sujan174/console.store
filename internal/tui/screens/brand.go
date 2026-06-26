package screens

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"console.store/internal/tui/theme"
)

// BrandHeaderLines is the rendered height of BrandBanner (brand line + address
// line + gold rule), so the root can reserve list space for it.
const BrandHeaderLines = 3

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

// BrandBanner is the top bar above every post-landing screen. Line 1: the
// shimmering consolestore.in wordmark (left, with version) + the cart chip
// (right). Line 2: the current delivery address, right-aligned, with semantic
// glyph colours. Line 3: a full-width gold rule. addrLine/addrLabel/cartChip may
// be "" to omit.
func BrandBanner(width, frame int, addrLine, addrLabel, cartChip string) string {
	inner := width - 4 // 2-space margin each side, matching the body grid
	if inner < 24 {
		inner = 24
	}

	// Line 1 — wordmark + version + cart chip.
	brand := theme.Fg(theme.Cursor).Bold(true).Render("▍ ") +
		shimmerBrand("consolestore.in", frame) + "  " + theme.PurpleStyle.Render(Version)
	chip := ""
	if cartChip != "" {
		cs := theme.CartStyle
		if strings.Contains(cartChip, "empty") {
			cs = theme.DimStyle
		}
		chip = cs.Render(cartChip)
	}
	gap1 := inner - lipgloss.Width(brand) - lipgloss.Width(chip)
	if gap1 < 1 {
		gap1 = 1
	}
	line1 := "  " + brand + strings.Repeat(" ", gap1) + chip + "  "

	// Line 2 — address, right-aligned, semantic colours (green pin, gold label).
	addr := ""
	if addrLine != "" {
		label := ""
		if addrLabel != "" {
			label = theme.GoldStyle.Render(" · " + addrLabel)
		}
		budget := inner - lipgloss.Width("deliver to ⊕  ⌄") - lipgloss.Width(addrLabel) - 4
		if budget < 10 {
			budget = 10
		}
		addr = theme.DimStyle.Render("deliver to ") + theme.GreenStyle.Render("⊕ ") +
			theme.BrightStyle.Render(railTrunc2(addrLine, budget)) + label +
			theme.FaintStyle.Render(" ⌄")
	}
	gap2 := inner - lipgloss.Width(addr)
	if gap2 < 0 {
		gap2 = 0
	}
	line2 := "  " + strings.Repeat(" ", gap2) + addr + "  "

	rule := "  " + theme.GoldStyle.Render(strings.Repeat("─", inner)) + "  "
	return line1 + "\n" + line2 + "\n" + rule + "\n"
}
