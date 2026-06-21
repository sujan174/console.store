package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"console.store/internal/tui/theme"
)

// InnerWidth is the default content column width used before a window size
// is known.
const InnerWidth = 60

// margin is the left/right text gutter inside the frame.
const margin = 2

// MaxFrame caps the content column. The design is a fixed-width card, not a
// full-bleed sheet — on a wide terminal the column stays this wide and is
// centred on the dark canvas (see FrameOffset) rather than stretching edge to
// edge (which marooned prices/ETAs far to the right).
const MaxFrame = 70

// frameWidth is the card width (dividers, status bar, selected rows span this).
// It tracks the terminal width up to MaxFrame.
var frameWidth = InnerWidth + 2*margin

// SetFrameWidth sets the card width from the terminal size, clamped to
// [24, MaxFrame].
func SetFrameWidth(w int) {
	if w < 24 {
		w = 24
	}
	if w > MaxFrame {
		w = MaxFrame
	}
	frameWidth = w
}

// FrameOffset is the left padding that centres the card on a terminal of the
// given width (0 when the terminal is no wider than the card).
func FrameOffset(termWidth int) int {
	if off := (termWidth - frameWidth) / 2; off > 0 {
		return off
	}
	return 0
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
	cur := sp(1)
	if blink {
		cur = seg(theme.Cursor, "▋")
	}

	// Left segment at three verbosity tiers; right segment at three. We pick the
	// richest pair that fits the interior so the bar NEVER exceeds the frame
	// (an over-wide bar wraps past the last column → a phantom "second column").
	leftFull := seg(theme.Green, "⊙ linked") + seg(theme.Faint, " · ") +
		seg(theme.Dim, addr+" · home") + seg(theme.Faint, " · ") + seg(theme.Dim, screen)
	leftMid := seg(theme.Green, "⊙ linked") + seg(theme.Faint, " · ") + seg(theme.Dim, screen)
	leftTiny := seg(theme.Green, "⊙ linked")

	rightFull := seg(theme.Dim, hint) + seg(theme.Faint, " · ↑"+latency+"ms ") + cur
	rightMid := seg(theme.Faint, "↑"+latency+"ms ") + cur
	rightTiny := cur

	avail := frameWidth - 2*margin
	fits := func(l, r string) bool {
		return lipgloss.Width(l)+1+lipgloss.Width(r) <= avail
	}
	var left, right string
	switch {
	case fits(leftFull, rightFull):
		left, right = leftFull, rightFull
	case fits(leftFull, rightMid):
		left, right = leftFull, rightMid
	case fits(leftMid, rightMid):
		left, right = leftMid, rightMid
	case fits(leftMid, rightTiny):
		left, right = leftMid, rightTiny
	default:
		left, right = leftTiny, rightTiny
	}
	gap := avail - lipgloss.Width(left) - lipgloss.Width(right)
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
