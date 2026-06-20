package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"console.store/internal/tui/theme"
)

// InnerWidth is the content column width all screens render to.
const InnerWidth = 60

// Divider is the full-width section rule under a screen header (design: 1px #232539).
func Divider() string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Div)).Render(strings.Repeat("─", InnerWidth)) + "\n"
}

// DashRule is the dashed bill separator (design: 1px dashed #2c2e44).
func DashRule() string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Div2)).Render(strings.Repeat("╌", InnerWidth)) + "\n"
}

// StatusBar renders the persistent bottom bar (design lines 459-463):
//
//	⊙ linked · <addr> · home · <screen>            <hint> · ↑<lat>ms ▋
func StatusBar(addr, screen, hint, latency string, blink bool) string {
	left := theme.GreenStyle.Render("⊙ linked") +
		theme.FaintStyle.Render(" · ") + theme.DimStyle.Render(addr+" · home") +
		theme.FaintStyle.Render(" · ") + theme.DimStyle.Render(screen)
	cur := " "
	if blink {
		cur = theme.CursorStyle.Render("▋")
	}
	right := theme.DimStyle.Render(hint) + theme.FaintStyle.Render(" · ↑"+latency+"ms ") + cur
	gap := InnerWidth - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	bar := left + strings.Repeat(" ", gap) + right
	return lipgloss.NewStyle().Background(lipgloss.Color(theme.PanelLo)).Render(bar)
}

// Hint renders a footer hint line from alternating (keyGlyph, label) pairs:
// keys in Dim, labels in Faint (design lines 263-265).
func Hint(pairs ...string) string {
	var b strings.Builder
	b.WriteString("  ")
	for i := 0; i+1 < len(pairs); i += 2 {
		b.WriteString(theme.DimStyle.Render(pairs[i]) + " " + theme.FaintStyle.Render(pairs[i+1]))
		if i+2 < len(pairs) {
			b.WriteString("   ")
		}
	}
	return b.String()
}
