package screens

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"console.store/internal/tui/theme"
)

// BrandHeaderLines is the rendered height of BrandBanner (wordmark + gold rule),
// so the root can reserve list space for it.
const BrandHeaderLines = 2

// BrandBanner is the centered consolestore.in wordmark shown at the top of every
// post-landing screen. The version sits inline on the same line as the brand
// name so it stays visually anchored to the left of the wordmark.
func BrandBanner(width int) string {
	center := func(s string) string {
		pad := (width - lipgloss.Width(s)) / 2
		if pad < 0 {
			pad = 0
		}
		return strings.Repeat(" ", pad) + s
	}
	const name = "consolestore.in"
	headline := theme.BrandStyle.Render(name) + "  " + theme.FaintStyle.Render(Version)
	// Rule spans the whole headline (brand + version), not just the wordmark.
	rule := theme.GoldStyle.Render(strings.Repeat("─", lipgloss.Width(headline)))
	return center(headline) + "\n" + center(rule) + "\n"
}
