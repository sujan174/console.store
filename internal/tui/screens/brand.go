package screens

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"console.store/internal/tui/theme"
)

// BrandHeaderLines is the rendered height of BrandBanner (wordmark + gold rule +
// version), so the root can reserve list space for it.
const BrandHeaderLines = 3

// BrandBanner is the centered consolestore.in wordmark shown at the top of every
// post-landing screen — the app's running logo — with a gap below it. width is
// the full frame width to center within. The name is rendered as a single span
// (so "consolestore.in" stays a searchable substring), underscored by a short
// gold rule, with the build version beneath.
func BrandBanner(width int) string {
	center := func(s string) string {
		pad := (width - lipgloss.Width(s)) / 2
		if pad < 0 {
			pad = 0
		}
		return strings.Repeat(" ", pad) + s
	}
	const name = "consolestore.in"
	logo := theme.BrandStyle.Render(name)
	rule := theme.GoldStyle.Render(strings.Repeat("─", lipgloss.Width(name)))
	ver := theme.FaintStyle.Render(Version)
	return center(logo) + "\n" + center(rule) + "\n" + center(ver) + "\n"
}
