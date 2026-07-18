package cli

import (
	"strings"

	"rsc.io/qr"
)

// qrBlock renders data as terminal half-block lines, dark-on-light with a
// 2-cell quiet border, so phone cameras can scan straight off the terminal.
// Returns nil when encoding fails (callers fall back to the pay-link text —
// never block the payment on art). Duplicated from tui/components.QRBlock
// because cli must not import internal/tui.
func qrBlock(data string) []string {
	code, err := qr.Encode(data, qr.L)
	if err != nil {
		return nil
	}
	const quiet = 2
	size := code.Size
	// cell(x,y): true = dark module. Out-of-range (quiet zone) = light.
	cell := func(x, y int) bool {
		if x < 0 || y < 0 || x >= size || y >= size {
			return false
		}
		return code.Black(x, y)
	}
	var lines []string
	for y := -quiet; y < size+quiet; y += 2 {
		var b strings.Builder
		for x := -quiet; x < size+quiet; x++ {
			top, bot := cell(x, y), cell(x, y+1)
			// Light = full block (white ink); dark cells stay as spaces on the
			// terminal's dark canvas — the inversion that keeps QRs scannable.
			switch {
			case !top && !bot:
				b.WriteRune('█')
			case !top && bot:
				b.WriteRune('▀')
			case top && !bot:
				b.WriteRune('▄')
			default:
				b.WriteRune(' ')
			}
		}
		lines = append(lines, b.String())
	}
	return lines
}
