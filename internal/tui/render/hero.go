package render

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// asciiLogo is the bold uppercase block-art wordmark — the design's actual
// logo shape (web design lines 211-216). It is the floor for terminals without
// truecolor (rendered flat) and the source for the gradient-text and
// Kitty-glow treatments, so every tier renders the same bold shape.
var asciiLogo = []string{
	`██████╗ ██████╗ ███╗  ██╗███████╗ ██████╗ ██╗     ███████╗`,
	`██╔════╝██╔═══██╗████╗ ██║██╔════╝██╔═══██╗██║     ██╔════╝`,
	`██║     ██║   ██║██╔██╗██║███████╗██║   ██║██║     █████╗  `,
	`██║     ██║   ██║██║╚████║╚════██║██║   ██║██║     ██╔══╝  `,
	`╚██████╗╚██████╔╝██║ ╚███║███████║╚██████╔╝███████╗███████╗`,
	` ╚═════╝ ╚═════╝ ╚═╝  ╚══╝╚══════╝ ╚═════╝ ╚══════╝╚══════╝`,
}

// Logo returns the best wordmark for the terminal's capabilities. w is the
// available frame width (reserved for future centring; unused today). The
// caller indents/places the result.
//
// Fidelity ladder: on truecolor terminals the iconic block-art is tinted with
// a vertical blue→purple gradient (the design's blue glow hue) — keeping the
// bold uppercase shape the design uses while adding its colour sheen. On
// non-truecolor terminals it falls back to the flat block-art. (Kitty graphics
// — real gaussian bloom — is layered on later behind a flag.)
func Logo(caps Caps, w int) string {
	if caps.Truecolor {
		return GradientText(asciiLogo, "#7aa2f7", "#bb9af7")
	}
	return strings.Join(asciiLogo, "\n") + "\n"
}

// GradientText colour-tints each line of block-art with a vertical gradient
// interpolated top→bottom across the lines (Lab space for perceptual
// smoothness). Glyph shapes are preserved exactly — only colour changes — so
// bold wordmarks stay bold. Each line is newline-terminated.
func GradientText(lines []string, top, bottom string) string {
	t, _ := colorful.Hex(orDefault(top, "#c0caf5"))
	b, _ := colorful.Hex(orDefault(bottom, "#c0caf5"))
	n := len(lines)
	var out strings.Builder
	for i, ln := range lines {
		frac := 0.0
		if n > 1 {
			frac = float64(i) / float64(n-1)
		}
		hex := t.BlendLab(b, frac).Clamped().Hex()
		out.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Render(ln) + "\n")
	}
	return out.String()
}

// boxArtBitmap converts the block-art wordmark into a 1-bit Bitmap (any
// non-space rune is a lit pixel) so the Kitty backend rasterizes and blurs the
// SAME bold shape as the text path.
func boxArtBitmap() Bitmap {
	h := len(asciiLogo)
	w := 0
	for _, ln := range asciiLogo {
		if r := len([]rune(ln)); r > w {
			w = r
		}
	}
	on := make([]bool, w*h)
	for y, ln := range asciiLogo {
		for x, r := range []rune(ln) {
			if r != ' ' {
				on[y*w+x] = true
			}
		}
	}
	return Bitmap{W: w, H: h, on: on}
}
