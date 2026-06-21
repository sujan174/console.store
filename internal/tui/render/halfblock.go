package render

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// HalfBlockOpts controls colour. Top/Bottom are hex endpoints of a vertical
// gradient interpolated across the bitmap height. Glow, when non-empty, paints
// a dim one-pixel halo around lit pixels in that hex (coarse bloom).
type HalfBlockOpts struct {
	Top    string // hex, top of gradient
	Bottom string // hex, bottom of gradient
	Glow   string // hex, optional halo colour ("" = no halo)
}

// HalfBlock renders a 1-bit bitmap at 2x vertical resolution: each text cell
// is one ▀ glyph whose foreground is the upper pixel and background is the
// lower pixel. Lit pixels take the vertical gradient colour; with Glow set,
// unlit pixels adjacent to a lit one take the dim halo colour. A cell with no
// lit or haloed sub-pixel renders as a literal blank so the terminal canvas
// shows through.
func HalfBlock(bm Bitmap, opt HalfBlockOpts) string {
	top, _ := colorful.Hex(orDefault(opt.Top, "#c0caf5"))
	bot, _ := colorful.Hex(orDefault(opt.Bottom, "#c0caf5"))
	var b strings.Builder
	for y := 0; y < bm.H; y += 2 {
		for x := 0; x < bm.W; x++ {
			upLit := bm.At(x, y)
			loLit := bm.At(x, y+1)
			upHalo := opt.Glow != "" && !upLit && neighbourLit(bm, x, y)
			loHalo := opt.Glow != "" && !loLit && neighbourLit(bm, x, y+1)

			if !upLit && !loLit && !upHalo && !loHalo {
				b.WriteString(" ") // empty cell -> terminal canvas shows through
				continue
			}
			fg := pixelColor(upLit, upHalo, gradientAt(top, bot, y, bm.H), opt.Glow)
			bg := pixelColor(loLit, loHalo, gradientAt(top, bot, y+1, bm.H), opt.Glow)
			cell := lipgloss.NewStyle().
				Foreground(lipgloss.Color(fg)).
				Background(lipgloss.Color(bg)).
				Render("▀")
			b.WriteString(cell)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// pixelColor resolves a single half-cell pixel: lit -> gradient hex, halo ->
// glow hex, else a sentinel that blends with the dark canvas (the caller only
// styles a cell when at least one sub-pixel is non-empty).
func pixelColor(lit, halo bool, grad, glow string) string {
	switch {
	case lit:
		return grad
	case halo:
		return glow
	default:
		if glow != "" {
			return glow
		}
		return "#15161f"
	}
}

// gradientAt returns the hex colour at row y of an h-tall vertical gradient.
func gradientAt(top, bot colorful.Color, y, h int) string {
	if h <= 1 {
		return top.Hex()
	}
	t := float64(y) / float64(h-1)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return top.BlendLab(bot, t).Clamped().Hex()
}

// neighbourLit reports whether any 8-neighbour of (x,y) is a lit pixel.
func neighbourLit(bm Bitmap, x, y int) bool {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			if bm.At(x+dx, y+dy) {
				return true
			}
		}
	}
	return false
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
