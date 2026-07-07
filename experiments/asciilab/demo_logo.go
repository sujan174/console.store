package main

import (
	"math"
	"math/rand/v2"
)

// Logo: wordmark with bobbing letters, travelling gradient sweep and
// occasional glitch flicker. Integration target: splash wordmark.
type Logo struct{ w, h, n int }

func (l *Logo) Name() string    { return "logo" }
func (l *Logo) Tagline() string { return "wordmark shimmer + glitch (splash)" }
func (l *Logo) Init(w, h int)   { l.w, l.h = w, h }
func (l *Logo) Step(n int)      { l.n = n }

var glitchGlyphs = []rune("▚▞▟▙#%&$@")

func (l *Logo) View() string {
	g := NewGrid(l.w, l.h)
	word := "consolestore"
	gap := 3
	width := len(word)*gap - (gap - 1)
	x0 := (l.w - width) / 2
	midY := l.h/2 - 1
	t := float64(l.n)

	// deterministic glitch windows: reseed per short window
	rng := rand.New(rand.NewPCG(uint64(l.n/5), 42))
	glitching := l.n%90 < 6
	glitchIdx := rng.IntN(len(word))

	sweep := []RGB{Comment, Blue, Cyan, Fg, Cyan, Blue, Comment}
	for i, r := range word {
		bob := int(math.Round(math.Sin(t*0.12+float64(i)*0.55) * 1.4))
		pos := math.Mod(float64(i)/float64(len(word))-t*0.015, 1)
		if pos < 0 {
			pos++
		}
		c := gradient(sweep, pos)
		ch := r
		if glitching && i == glitchIdx {
			ch = glitchGlyphs[rng.IntN(len(glitchGlyphs))]
			c = Red
		}
		g.Set(x0+i*gap, midY+bob, ch, c)
	}

	// underline wave
	for x := 0; x < width; x++ {
		ph := math.Sin(t*0.18 + float64(x)*0.4)
		ch := '▁'
		if ph > 0.3 {
			ch = '▂'
		}
		g.Set(x0+x, midY+3, ch, lerp(BgHi, Magenta, (ph+1)/2))
	}

	tag := "terminal-native ordering"
	g.Text((l.w-len(tag))/2, midY+5, tag, Comment)
	// blinking cursor after tagline
	if (l.n/9)%2 == 0 {
		g.Set((l.w-len(tag))/2+len(tag)+1, midY+5, '▉', Green)
	}
	return g.String()
}
