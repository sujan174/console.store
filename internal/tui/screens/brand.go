package screens

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"console.store/internal/tui/theme"
)

// BrandHeaderLines is the rendered height of BrandBanner (brand bar + gold rule),
// so the root can reserve list space for it.
const BrandHeaderLines = 2

// BrandBanner is the top bar shown above every post-landing screen: the gold
// consolestore.in wordmark sits LEFT (with the version), the current delivery
// address sits RIGHT (truncated), and a full-width gold rule underlines the bar.
// addrLine/addrLabel are the current address; pass "" to omit the address.
func BrandBanner(width int, addrLine, addrLabel string) string {
	inner := width - 4 // 2-space margin each side, matching the body grid
	if inner < 20 {
		inner = 20
	}

	// Brand: gold + bold, led by a gold accent bar so it reads larger/heavier
	// than the body — the closest a terminal gets to "bigger".
	brand := theme.Fg(theme.Gold).Bold(true).Render("▍ consolestore.in") +
		"  " + theme.FaintStyle.Render(Version)

	addr := ""
	if addrLine != "" {
		label := ""
		if addrLabel != "" {
			label = theme.DimStyle.Render(" · " + addrLabel)
		}
		// Budget for the address line so the bar never overflows the brand.
		budget := inner - lipgloss.Width(brand) - lipgloss.Width("deliver to ⊕  ⌄") - lipgloss.Width(addrLabel) - 4
		if budget < 8 {
			budget = 8
		}
		addr = theme.DimStyle.Render("deliver to ") + theme.CursorStyle.Render("⊕ ") +
			theme.BrightStyle.Render(railTrunc2(addrLine, budget)) + label +
			theme.FaintStyle.Render(" ⌄")
	}

	gap := inner - lipgloss.Width(brand) - lipgloss.Width(addr)
	if gap < 1 {
		gap = 1
	}
	bar := "  " + brand + strings.Repeat(" ", gap) + addr + "  "
	rule := "  " + theme.GoldStyle.Render(strings.Repeat("─", inner)) + "  "
	return bar + "\n" + rule + "\n"
}
