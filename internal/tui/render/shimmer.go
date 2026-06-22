package render

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

const (
	shimmerTopHex    = "#7aa2f7" // gradient top (blue) — matches Logo
	shimmerBotHex    = "#bb9af7" // gradient bottom (violet) — matches Logo
	shimmerHighlight = "#e8ecff" // near-white sheen the band blends toward
	shimmerBandHalf  = 3.0       // columns each side of the sweep centre
	shimmerGap       = 26        // dark columns between sweeps (rest beat)
	shimmerBoost     = 0.9       // peak blend toward the highlight at band centre
)

// ShimmerWordmark renders the settled block wordmark with a bright highlight
// band that sweeps left→right across the glyphs and loops, like a sheen
// travelling over metal. It is the settled-splash signature; purely a function
// of frame, so it stays on the app's single animation tick.
//
// Truecolor terminals get the moving sheen over the blue→violet gradient.
// Non-truecolor falls back to the flat block-art (a colour effect can't read
// without colour). The Kitty graphics path returns the static bloom logo — a
// rasterized bitmap can't be per-column re-tinted — so callers should cache it
// rather than re-encode a PNG every frame.
func ShimmerWordmark(caps Caps, frame int) string {
	if caps.KittyGraphics && KittyFlag {
		return Logo(caps, 64)
	}
	if !caps.Truecolor {
		return strings.Join(asciiLogo, "\n") + "\n"
	}

	w := 0
	for _, ln := range asciiLogo {
		if r := len([]rune(ln)); r > w {
			w = r
		}
	}
	top, _ := colorful.Hex(shimmerTopHex)
	bot, _ := colorful.Hex(shimmerBotHex)
	hi, _ := colorful.Hex(shimmerHighlight)
	n := len(asciiLogo)

	cycle := w + shimmerGap
	pos := frame % cycle // sweep centre column; > w means resting off the wordmark

	var out strings.Builder
	for y, ln := range asciiLogo {
		runes := []rune(ln)
		frac := 0.0
		if n > 1 {
			frac = float64(y) / float64(n-1)
		}
		base := top.BlendLab(bot, frac).Clamped()
		baseStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(base.Hex()))

		for x := 0; x < w; x++ {
			r := ' '
			if x < len(runes) {
				r = runes[x]
			}
			if r == ' ' {
				out.WriteByte(' ') // silhouette gaps stay empty
				continue
			}
			if d := math.Abs(float64(x - pos)); d <= shimmerBandHalf {
				inten := 1 - d/shimmerBandHalf // 1 at centre → 0 at band edge
				inten *= inten                 // ease the falloff
				hex := base.BlendLab(hi, shimmerBoost*inten).Clamped().Hex()
				out.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Render(string(r)))
			} else {
				out.WriteString(baseStyle.Render(string(r)))
			}
		}
		out.WriteByte('\n')
	}
	return out.String()
}
