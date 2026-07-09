package mcp

import (
	"fmt"
	"strings"

	"rsc.io/qr"
)

// qrSVG encodes data as a QR code and returns a self-contained, theme-neutral SVG
// string (dark modules on a white quiet zone). The widget renders it inline on the
// payment screen so the user can scan-to-pay without leaving the app — the terminal
// couldn't paint a QR reliably (transparent cell backgrounds), but a web SVG is
// exact. Level M gives good density for a upi:// intent string while staying
// scannable at a phone's distance. Returns "" on an encode error; the caller always
// also offers the hosted /pay link as a fallback.
func qrSVG(data string) string {
	code, err := qr.Encode(data, qr.M)
	if err != nil {
		return ""
	}
	const quiet = 4 // modules of white border, per the QR spec
	size := code.Size
	dim := size + quiet*2

	var b strings.Builder
	// viewBox is in module units; the widget sizes the element via CSS width.
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges" role="img" aria-label="UPI payment QR code">`, dim, dim)
	b.WriteString(`<rect width="100%" height="100%" fill="#ffffff"/>`)
	b.WriteString(`<path fill="#0b0b12" d="`)
	// One path of 1x1 rects for every black module — compact and crisp.
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if code.Black(x, y) {
				fmt.Fprintf(&b, "M%d %dh1v1h-1z", x+quiet, y+quiet)
			}
		}
	}
	b.WriteString(`"/></svg>`)
	return b.String()
}
