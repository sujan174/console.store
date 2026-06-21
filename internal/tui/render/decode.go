package render

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// DecodeSteps is the number of animation ticks the decode runs before it locks.
// ~0.8s at the 60ms tick (13 * 60ms ≈ 0.78s).
const DecodeSteps = 13

// glitchChars are the noise glyphs shown for not-yet-resolved cells. None of
// these appear in asciiLogo, so their presence reliably signals "mid-decode".
const glitchChars = `01<>/\{}[]#%&$*+=`

const (
	decodeEdgeHex  = "#7aa2f7" // bright cyan-blue render head
	decodeGlitchHx = "#565f89" // dim glitch noise
)

// DecodeWordmark renders the block wordmark mid-decode. step is decode progress
// (0..DecodeSteps); frame is the global animation tick (drives glitch shimmer).
// Columns left of the resolve front show the real glyph (gradient-tinted on
// truecolor); the front column is a bright edge; columns to the right show a
// deterministic glitch glyph. At step==DecodeSteps the wordmark is fully clean.
//
// The Kitty graphics path renders a rasterized bloom that cannot be glyph-
// decoded, so it settles straight to the bloom logo.
func DecodeWordmark(caps Caps, step, frame int) string {
	if caps.KittyGraphics && KittyFlag {
		return Logo(caps, 64)
	}

	W := 0
	for _, ln := range asciiLogo {
		if r := len([]rune(ln)); r > W {
			W = r
		}
	}
	resolved := step * W / DecodeSteps
	if step >= DecodeSteps {
		resolved = W
	}

	top, _ := colorful.Hex("#7aa2f7")
	bot, _ := colorful.Hex("#bb9af7")
	edge := lipgloss.NewStyle().Foreground(lipgloss.Color(decodeEdgeHex))
	glitch := lipgloss.NewStyle().Foreground(lipgloss.Color(decodeGlitchHx))
	n := len(asciiLogo)

	var out strings.Builder
	for y, ln := range asciiLogo {
		runes := []rune(ln)
		frac := 0.0
		if n > 1 {
			frac = float64(y) / float64(n-1)
		}
		lineHex := top.BlendLab(bot, frac).Clamped().Hex()
		grad := lipgloss.NewStyle().Foreground(lipgloss.Color(lineHex))

		for x := 0; x < W; x++ {
			r := ' '
			if x < len(runes) {
				r = runes[x]
			}
			switch {
			case r == ' ':
				out.WriteByte(' ') // silhouette gaps stay empty
			case x < resolved:
				if caps.Truecolor {
					out.WriteString(grad.Render(string(r)))
				} else {
					out.WriteString(string(r))
				}
			case x == resolved:
				out.WriteString(edge.Render(string(r)))
			default:
				g := rune(glitchChars[(x*31+y*7+frame)%len(glitchChars)])
				out.WriteString(glitch.Render(string(g)))
			}
		}
		out.WriteByte('\n')
	}
	return out.String()
}
