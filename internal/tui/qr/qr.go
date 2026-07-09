// Package qr renders a string (a UPI intent URL) as a QR code drawn with
// terminal cells, scannable straight off the screen by a phone camera. QR codes
// must be dark modules on a light field with a light "quiet zone" border, so we
// paint each module as two space cells with an explicit black/white background
// (independent of the terminal's own theme) and frame it with a light border.
package qr

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	goqr "rsc.io/qr"
)

// quietZone is the mandatory light border around a QR, in modules. The spec
// minimum is 4; we use 4 so cheap scanners lock on reliably.
const quietZone = 4

// Matrix encodes data at error-correction level M and returns the module grid
// (true = dark module). The grid is square. Returns an error only if the data
// can't be encoded (too long).
func Matrix(data string) ([][]bool, error) {
	code, err := goqr.Encode(data, goqr.M)
	if err != nil {
		return nil, err
	}
	n := code.Size
	m := make([][]bool, n)
	for y := 0; y < n; y++ {
		m[y] = make([]bool, n)
		for x := 0; x < n; x++ {
			m[y][x] = code.Black(x, y)
		}
	}
	return m, nil
}

// Render returns a multi-line string that draws the QR for data. Each module is
// two cells wide (one cell tall) so the code reads roughly square on screen —
// terminal cells are about twice as tall as wide. Dark modules get a black
// background, light modules (and the quiet zone) a white one, so it scans
// regardless of the terminal's theme. Empty data renders nothing.
func Render(data string) string {
	if data == "" {
		return ""
	}
	m, err := Matrix(data)
	if err != nil {
		// Can't encode (payload too long) — fall back to the raw string so the
		// user can still pay by copying the link to their phone.
		return data
	}
	dark := lipgloss.NewStyle().Background(lipgloss.Color("#000000"))
	light := lipgloss.NewStyle().Background(lipgloss.Color("#ffffff"))
	n := len(m)
	full := n + 2*quietZone

	var b strings.Builder
	for y := 0; y < full; y++ {
		for x := 0; x < full; x++ {
			inQuiet := y < quietZone || y >= n+quietZone || x < quietZone || x >= n+quietZone
			isDark := !inQuiet && m[y-quietZone][x-quietZone]
			if isDark {
				b.WriteString(dark.Render("  "))
			} else {
				b.WriteString(light.Render("  "))
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}
