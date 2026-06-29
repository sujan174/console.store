package cli

import (
	"fmt"
	"io"
	"strings"
)

// style applies the Tokyo Night palette to the headless output. It no-ops when
// off (output isn't a terminal, or under tests) so piped/redirected output and
// test assertions stay plain text.
//
// The truecolor codes below MIRROR internal/tui/theme exactly so the CLI and the
// TUI share one brand palette — kept in sync by hand rather than importing across
// the tui boundary (the cli package must not depend on internal/tui).
type style struct{ on bool }

func newStyle(on bool) style { return style{on: on} }

// Tokyo Night — hex values copied verbatim from internal/tui/theme.
const (
	cGold   = "38;2;224;175;104" // #e0af68 — brand / restaurant names / headings
	cGreen  = "38;2;158;206;106" // #9ece6a — prices, success, confirm (TUI prices are green)
	cBlue   = "38;2;122;162;247" // #7aa2f7 — numbers, the ❯ cursor, nav
	cCyan   = "38;2;125;207;255" // #7dcfff — links / accents (→)
	cRed    = "38;2;247;118;142" // #f7768e — errors / warnings
	cDim    = "38;2;86;95;137"   // #565f89 — labels / secondary / rules
	cText   = "38;2;169;177;214" // #a9b1d6 — default body
	cBright = "38;2;192;202;245" // #c0caf5 — emphasis / selected
)

func (s style) wrap(code, txt string) string {
	if !s.on || txt == "" {
		return txt
	}
	return "\x1b[" + code + "m" + txt + "\x1b[0m"
}

func (s style) head(t string) string   { return s.wrap("1;"+cGold, t) } // bold gold — brand/restaurant
func (s style) num(t string) string    { return s.wrap(cBlue, t) }      // numbers / cursor
func (s style) money(t string) string  { return s.wrap(cGreen, t) }     // prices (green, like the TUI)
func (s style) ok(t string) string     { return s.wrap("1;"+cGreen, t) }
func (s style) warn(t string) string   { return s.wrap(cRed, t) }
func (s style) dim(t string) string    { return s.wrap(cDim, t) }
func (s style) text(t string) string   { return s.wrap(cText, t) }
func (s style) bright(t string) string { return s.wrap("1;"+cBright, t) }
func (s style) link(t string) string   { return s.wrap(cCyan, t) }

// rowKind selects the colour language for a bill row.
type rowKind int

const (
	rowItem  rowKind = iota // an ordered line: text label, green amount
	rowLabel                // a bill subtotal: dim label, green amount
	rowTotal                // the to-pay line: bold gold label, bold green amount
)

// row prints "left … amount" with the amount right-aligned to billWidth, padding
// computed on the PLAIN widths (ANSI codes have no display width).
func (s style) row(out io.Writer, left, amount string, kind rowKind) {
	pad := billWidth - len([]rune(left)) - len([]rune(amount))
	if pad < 1 {
		pad = 1
	}
	var l, a string
	switch kind {
	case rowTotal:
		l, a = s.head(left), s.ok(amount)
	case rowLabel:
		l, a = s.dim(left), s.money(amount)
	default:
		l, a = s.text(left), s.money(amount)
	}
	fmt.Fprintf(out, "  %s%s%s\n", l, strings.Repeat(" ", pad), a)
}

func (s style) rule(out io.Writer) {
	fmt.Fprintf(out, "  %s\n", s.dim(strings.Repeat("─", billWidth)))
}
