package render

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// DecodeSteps is the number of animation ticks the decode runs before it locks.
// ~0.8s at the 60ms tick (13 * 60ms ≈ 0.78s).
const DecodeSteps = 13

// glitchChars are the sparse noise glyphs that flicker ahead of the resolve
// front. None appear in asciiLogo, so their presence reliably signals "mid-decode".
const glitchChars = `01<>/\{}[]#%&$*+=`

const (
	decodeGhostDark   = "#1a1b26" // dark the unresolved silhouette/glitch sink toward
	decodeGhostBlend  = 0.70      // how far the ghost silhouette sinks (dimmer = more)
	decodeGlitchBlend = 0.52      // glitch flicker sits a touch brighter than the ghost
)

// DecodeWordmark renders the block wordmark mid-reveal. step is decode progress
// (0..DecodeSteps); frame drives the glitch flicker. A near-white sheen rides the
// resolve front — the SAME band the idle shimmer uses — so the reveal hands off
// to ShimmerWordmark with no change of colour language. Behind the front cells
// are full gradient; ahead they show a dim gradient ghost of the real glyph with
// sparse glitch flicker, keeping the wordmark silhouette readable throughout.
//
// The Kitty graphics path renders a rasterized bloom that cannot be glyph-
// decoded, so it settles straight to the bloom logo.
func DecodeWordmark(caps Caps, step, frame int) string {
	if caps.KittyGraphics && KittyFlag {
		return Logo(caps, 64)
	}

	w := 0
	for _, ln := range asciiLogo {
		if r := len([]rune(ln)); r > w {
			w = r
		}
	}

	// Eased resolve front: decelerates as it lands so the final columns settle
	// gently rather than snapping clean (ease-out quad).
	resolved := float64(w)
	if step < DecodeSteps {
		p := float64(step) / float64(DecodeSteps)
		resolved = (1 - (1-p)*(1-p)) * float64(w)
	}

	top, _ := colorful.Hex(shimmerTopHex)
	bot, _ := colorful.Hex(shimmerBotHex)
	hi, _ := colorful.Hex(shimmerHighlight)
	dark, _ := colorful.Hex(decodeGhostDark)
	n := len(asciiLogo)

	var out strings.Builder
	for y, ln := range asciiLogo {
		runes := []rune(ln)
		frac := 0.0
		if n > 1 {
			frac = float64(y) / float64(n-1)
		}
		base := top.BlendLab(bot, frac).Clamped()
		baseStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(base.Hex()))
		ghostStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(base.BlendLab(dark, decodeGhostBlend).Clamped().Hex()))
		glitchStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(base.BlendLab(dark, decodeGlitchBlend).Clamped().Hex()))

		for x := 0; x < w; x++ {
			r := ' '
			if x < len(runes) {
				r = runes[x]
			}
			if r == ' ' {
				out.WriteByte(' ') // silhouette gaps stay empty
				continue
			}
			fx := float64(x)
			switch {
			case fx <= resolved:
				// Resolved: full gradient, with a near-white sheen on the
				// leading edge (the shimmer band riding the resolve front).
				if d := resolved - fx; d <= shimmerBandHalf {
					inten := 1 - d/shimmerBandHalf
					inten *= inten
					out.WriteString(lipgloss.NewStyle().
						Foreground(lipgloss.Color(base.BlendLab(hi, shimmerBoost*inten).Clamped().Hex())).
						Render(string(r)))
				} else {
					out.WriteString(baseStyle.Render(string(r)))
				}
			default:
				// Ahead of the front: dim ghost silhouette, with ~1-in-3 cells
				// flickering a glitch glyph — the decrypt texture kept subtle.
				if (x*31+y*7+frame)%3 == 0 {
					g := rune(glitchChars[(x*17+y*5+frame)%len(glitchChars)])
					out.WriteString(glitchStyle.Render(string(g)))
				} else {
					out.WriteString(ghostStyle.Render(string(r)))
				}
			}
		}
		out.WriteByte('\n')
	}
	return out.String()
}
