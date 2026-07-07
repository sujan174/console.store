package main

import (
	"math/rand/v2"
)

// Rain: code-glyph rain in palette colors (matrix, Tokyo Night flavored).
// Integration target: idle/screensaver background, auth-wait screen.
type Rain struct {
	w, h int
	n    int
	cols []rainCol
	rng  *rand.Rand
}

type rainCol struct {
	y      float64
	speed  float64
	length int
	glyphs []rune
}

func (r *Rain) Name() string    { return "rain" }
func (r *Rain) Tagline() string { return "code rain, Tokyo Night flavored" }

var rainSet = []rune("01{}[]()<>=+-*/;:._$#@%&|~^!?ｱｲｳｴｵｶｷｸｹｺﾊﾋﾌﾍﾎ")

func (r *Rain) Init(w, h int) {
	r.w, r.h = w, h
	r.rng = rand.New(rand.NewPCG(2, 13))
	r.cols = make([]rainCol, w)
	for i := range r.cols {
		r.cols[i] = r.newCol(true)
	}
}

func (r *Rain) newCol(scatter bool) rainCol {
	c := rainCol{
		speed:  0.25 + r.rng.Float64()*0.9,
		length: 4 + r.rng.IntN(12),
	}
	c.glyphs = make([]rune, r.h+c.length)
	for i := range c.glyphs {
		c.glyphs[i] = rainSet[r.rng.IntN(len(rainSet))]
	}
	if scatter {
		c.y = float64(r.rng.IntN(r.h * 2))
	} else {
		c.y = -float64(r.rng.IntN(r.h))
	}
	return c
}

func (r *Rain) Step(n int) {
	r.n = n
	for i := range r.cols {
		c := &r.cols[i]
		c.y += c.speed
		if int(c.y)-c.length > r.h {
			r.cols[i] = r.newCol(false)
		}
		// occasional glyph mutation
		if r.rng.IntN(6) == 0 {
			c.glyphs[r.rng.IntN(len(c.glyphs))] = rainSet[r.rng.IntN(len(rainSet))]
		}
	}
}

func (r *Rain) View() string {
	g := NewGrid(r.w, r.h)
	for x, c := range r.cols {
		head := int(c.y)
		for i := 0; i <= c.length; i++ {
			y := head - i
			if y < 0 || y >= r.h {
				continue
			}
			gi := ((y+x*7)%len(c.glyphs) + len(c.glyphs)) % len(c.glyphs)
			var col RGB
			switch {
			case i == 0:
				col = Fg
			case i == 1:
				col = Cyan
			default:
				col = lerp(Blue, Bg, float64(i)/float64(c.length+1))
			}
			g.Set(x, y, c.glyphs[gi], col)
		}
	}
	return g.String()
}
