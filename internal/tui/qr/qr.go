// Package qr renders a string (a UPI intent URL) as a QR code drawn with
// terminal cells, scannable straight off the screen by a phone camera. It uses
// HALF-BLOCK cells ("▀"): the upper half painted in the top module's colour and
// the lower half (the cell background) in the bottom module's — so one text row
// carries TWO module rows and one cell carries one module across. That makes the
// code ~2x narrower AND ~2x shorter than a full-block draw (fitting far more
// terminals) while staying square, since a terminal cell is ~2x taller than wide.
//
// Caveat: a QR needs its LIGHT modules to actually paint light, which relies on a
// background colour. Terminals with a background image / transparency don't paint
// cell backgrounds solidly, so the code can come out invisible there regardless
// of size — callers must always offer a non-QR fallback (a payment link).
package qr

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	goqr "rsc.io/qr"
)

// quietZone is the mandatory light border around a QR, in modules.
const quietZone = 4

// Matrix encodes data at error-correction level M and returns the module grid
// (true = dark module). The grid is square. Errors only if data is too long.
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

// Dims reports the rendered footprint in terminal cells (cols x rows) for data,
// so a caller can decide whether it fits before rendering. cols = modules +
// quiet zone; rows = half that (two module-rows per text row, rounded up).
func Dims(data string) (cols, rows int, err error) {
	m, err := Matrix(data)
	if err != nil {
		return 0, 0, err
	}
	full := len(m) + 2*quietZone
	return full, (full + 1) / 2, nil
}

// FitsIn reports whether the QR for data renders within maxCols x maxRows
// terminal cells. A zero/negative bound means "unbounded" on that axis.
func FitsIn(data string, maxCols, maxRows int) bool {
	cols, rows, err := Dims(data)
	if err != nil {
		return false
	}
	if maxCols > 0 && cols > maxCols {
		return false
	}
	if maxRows > 0 && rows > maxRows {
		return false
	}
	return true
}

// Render returns the half-block QR for data (empty string for empty data, or the
// raw data if it can't be encoded). Dark = black, light/quiet = white, so it
// scans regardless of the terminal's own theme (where backgrounds paint).
func Render(data string) string {
	if data == "" {
		return ""
	}
	m, err := Matrix(data)
	if err != nil {
		return data
	}
	n := len(m)
	full := n + 2*quietZone
	// module(x,y) with the quiet-zone offset applied; out-of-grid = light (false).
	module := func(x, y int) bool {
		if y < quietZone || y >= n+quietZone || x < quietZone || x >= n+quietZone {
			return false
		}
		return m[y-quietZone][x-quietZone]
	}
	color := func(dark bool) lipgloss.Color {
		if dark {
			return lipgloss.Color("#000000")
		}
		return lipgloss.Color("#ffffff")
	}

	var b strings.Builder
	for y := 0; y < full; y += 2 {
		for x := 0; x < full; x++ {
			top := module(x, y)
			bot := false
			if y+1 < full {
				bot = module(x, y+1)
			} else {
				bot = false // pad the final half-row with light (quiet)
			}
			// "▀": glyph (upper half) = foreground = top module; cell background =
			// bottom module. One cell renders both stacked modules.
			cell := lipgloss.NewStyle().Foreground(color(top)).Background(color(bot)).Render("▀")
			b.WriteString(cell)
		}
		b.WriteByte('\n')
	}
	return b.String()
}
