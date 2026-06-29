package cli

import (
	"fmt"
	"io"
	"strings"
)

// style applies a small Tokyo Night-ish ANSI palette to the headless output. It
// no-ops when off (output isn't a terminal, or under tests) so piped/redirected
// output and test assertions stay plain text.
type style struct{ on bool }

func newStyle(on bool) style { return style{on: on} }

func (s style) wrap(code, txt string) string {
	if !s.on || txt == "" {
		return txt
	}
	return "\x1b[" + code + "m" + txt + "\x1b[0m"
}

// Palette (truecolor; matches the TUI's Tokyo Night accents).
func (s style) head(t string) string  { return s.wrap("1;38;2;224;175;104", t) } // bold gold
func (s style) num(t string) string   { return s.wrap("38;2;122;162;247", t) }   // blue
func (s style) money(t string) string { return s.wrap("38;2;125;207;255", t) }   // cyan
func (s style) ok(t string) string    { return s.wrap("1;38;2;158;206;106", t) } // bold green
func (s style) warn(t string) string  { return s.wrap("38;2;247;118;142", t) }   // red
func (s style) dim(t string) string   { return s.wrap("38;2;120;130;160", t) }   // muted

// row prints "left … amount" with the amount right-aligned to billWidth, padding
// computed on the PLAIN widths (ANSI codes have no display width). emphasize
// bolds the label + amount (the "to pay" line).
func (s style) row(out io.Writer, left, amount string, emphasize bool) {
	pad := billWidth - len([]rune(left)) - len([]rune(amount))
	if pad < 1 {
		pad = 1
	}
	leftOut, amtOut := left, s.money(amount)
	if emphasize {
		leftOut, amtOut = s.head(left), s.ok(amount)
	}
	fmt.Fprintf(out, "  %s%s%s\n", leftOut, strings.Repeat(" ", pad), amtOut)
}

func (s style) rule(out io.Writer) {
	fmt.Fprintf(out, "  %s\n", s.dim(strings.Repeat("─", billWidth)))
}
