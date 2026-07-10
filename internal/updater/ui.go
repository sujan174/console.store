package updater

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// ui renders the launch-time update as a small branded block instead of bare
// log lines. Color/carriage-return tricks are gated on Out being a real
// terminal (char device, NO_COLOR unset, TERM != dumb) — piped/captured output
// (tests, logs) gets plain text lines with identical content.
type ui struct {
	out   io.Writer
	color bool
}

// Tokyo Night, matching the installer script's palette.
const (
	cGold  = "\033[38;2;224;175;104m"
	cBlue  = "\033[38;2;122;162;247m"
	cCyan  = "\033[38;2;125;207;255m"
	cGreen = "\033[38;2;158;206;106m"
	cRed   = "\033[38;2;247;118;142m"
	cDim   = "\033[38;2;86;95;137m"
	cBold  = "\033[1m"
	cReset = "\033[0m"
)

func newUI(out io.Writer) ui {
	return ui{out: out, color: isTerminal(out)}
}

func isTerminal(out io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	f, ok := out.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

// c wraps s in the given color when the terminal supports it.
func (u ui) c(color, s string) string {
	if !u.color {
		return s
	}
	return color + s + cReset
}

// header announces the update: the gold glyph, the wordmark, and the version
// hop. Content matches the old single line ("updating X → Y") so anything
// parsing the plain output still finds it.
func (u ui) header(current, next, channel string) {
	fmt.Fprintln(u.out)
	fmt.Fprintf(u.out, "  %s  %s\n", u.c(cGold, "▟█▙"), u.c(cBlue+cBold, "consolestore update"))
	fmt.Fprintf(u.out, "  %s  updating %s %s %s %s\n",
		u.c(cGold, "▜█▛"),
		u.c(cBold, current),
		u.c(cDim, "→"),
		u.c(cCyan+cBold, next),
		u.c(cDim, "· "+channel))
}

// progress redraws an in-place download bar (falls back to no-op when not a
// terminal — the download just happens silently like before).
func (u ui) progress(done, total int64) {
	if !u.color || total <= 0 {
		return
	}
	const width = 26
	frac := float64(done) / float64(total)
	if frac > 1 {
		frac = 1
	}
	filled := int(frac * width)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	fmt.Fprintf(u.out, "\r       %s %3.0f%%  %s",
		u.c(cBlue, bar), frac*100, u.c(cDim, fmt.Sprintf("%.1f/%.1f MB", mb(done), mb(total))))
}

// progressDone clears the bar line so the closing line prints clean.
func (u ui) progressDone() {
	if !u.color {
		return
	}
	fmt.Fprintf(u.out, "\r%s\r", strings.Repeat(" ", 70))
}

func mb(n int64) float64 { return float64(n) / (1 << 20) }

// success is the final line before the re-exec swap.
func (u ui) success() {
	fmt.Fprintf(u.out, "  %s verified sha256 %s\n\n",
		u.c(cGreen+cBold, "✓"), u.c(cDim, "· relaunching…"))
}

// fail keeps the old copy (tests + users grep for "staying on").
func (u ui) fail(current string) {
	fmt.Fprintf(u.out, "  %s update failed %s\n\n",
		u.c(cRed+cBold, "✗"), u.c(cDim, "— staying on "+current))
}

// badSum keeps the old checksum-mismatch semantics.
func (u ui) badSum() {
	fmt.Fprintf(u.out, "  %s update checksum mismatch %s\n\n",
		u.c(cRed+cBold, "✗"), u.c(cDim, "— skipping"))
}
