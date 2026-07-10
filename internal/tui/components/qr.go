// Package-internal QR rendering for the payment page: encodes data with
// rsc.io/qr and renders two bitmap rows per terminal line using half-blocks,
// dark-on-light (a 2-cell quiet border of light cells), so phone cameras can
// scan straight off the terminal.
package components

import (
	"strings"

	"rsc.io/qr"
)

// QRBlock renders data as terminal lines. Returns nil when encoding fails
// (callers fall back to the pay-link text — never block the payment on art).
func QRBlock(data string) []string {
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
			// Dark ink on a light background: light = full block (we render
			// WHITE blocks and leave dark cells as spaces on the terminal's
			// dark canvas — the inversion that keeps QRs scannable there).
			switch {
			case !top && !bot:
				b.WriteRune('█') // light-light
			case !top && bot:
				b.WriteRune('▀') // light over dark
			case top && !bot:
				b.WriteRune('▄') // dark over light
			default:
				b.WriteRune(' ') // dark-dark
			}
		}
		lines = append(lines, b.String())
	}
	return lines
}
