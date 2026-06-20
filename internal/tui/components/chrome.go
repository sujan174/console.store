package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"console.store/internal/tui/theme"
)

// InnerWidth is the default content column width used before a window size
// is known. Once the SSH session reports its size the root calls
// SetFrameWidth and the UI renders full-bleed to the terminal width.
const InnerWidth = 60

// margin is the left/right text gutter inside the full-bleed frame.
const margin = 2

// frameWidth is the full-bleed width (dividers, status bar, selected rows
// span this). It tracks the terminal width at runtime.
var frameWidth = InnerWidth + 2*margin

// SetFrameWidth sets the full-bleed width from the terminal size.
func SetFrameWidth(w int) {
	if w < 24 {
		w = 24
	}
	frameWidth = w
}

// FrameWidth is the current full-bleed width (edge to edge).
func FrameWidth() int { return frameWidth }

// ContentWidth is the text area width between the gutters.
func ContentWidth() int { return frameWidth - 2*margin }

// PadTo right-pads s with spaces to the given display width.
func PadTo(s string, width int) string {
	if pad := width - lipgloss.Width(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

// Divider is the full-bleed section rule under a screen header
// (design line 241: border-top 1px #232539, margin 0 -36px → edge to edge).
func Divider() string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Div)).Render(strings.Repeat("─", frameWidth)) + "\n"
}

// DashRule is the dashed bill separator (design line 322: margin 0 → content
// width, indented inside the gutters, not full-bleed).
func DashRule() string {
	return strings.Repeat(" ", margin) +
		lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Div2)).Render(strings.Repeat("╌", ContentWidth())) + "\n"
}

// StatusBar renders the persistent full-bleed bottom bar (design lines 459-463):
//
//	⊙ linked · <addr> · home · <screen>            <hint> · ↑<lat>ms ▋
//
// Every segment carries the panel background so the strip is continuous — a
// single outer Background() would be torn apart by the inner colour resets.
func StatusBar(addr, screen, hint, latency string, blink bool) string {
	bg := lipgloss.Color(theme.PanelLo)
	seg := func(fg, s string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(fg)).Background(bg).Render(s)
	}
	sp := func(n int) string {
		return lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", n))
	}
	left := seg(theme.Green, "⊙ linked") + seg(theme.Faint, " · ") +
		seg(theme.Dim, addr+" · home") + seg(theme.Faint, " · ") + seg(theme.Dim, screen)
	cur := sp(1)
	if blink {
		cur = seg(theme.Cursor, "▋")
	}
	right := seg(theme.Dim, hint) + seg(theme.Faint, " · ↑"+latency+"ms ") + cur
	gap := frameWidth - 2*margin - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return sp(margin) + left + sp(gap) + right + sp(margin)
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
