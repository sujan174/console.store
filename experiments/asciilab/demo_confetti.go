package main

import (
	"math/rand/v2"
)

// Confetti: celebration burst + placed banner.
// Integration target: the moment place_order succeeds.
type Confetti struct {
	w, h, n int
	parts   []confP
	rng     *rand.Rand
}

type confP struct {
	x, y   float64
	vx, vy float64
	ch     rune
	c      RGB
}

func (c *Confetti) Name() string    { return "confetti" }
func (c *Confetti) Tagline() string { return "order-placed celebration burst" }

func (c *Confetti) Init(w, h int) {
	c.w, c.h = w, h
	c.parts = nil
	c.rng = rand.New(rand.NewPCG(11, 5))
}

var confGlyphs = []rune("▪●◆✦*·▴")
var confColors = []RGB{Blue, Cyan, Magenta, Green, Red, Orange, Yellow, Teal}

func (c *Confetti) burst(cx, cy float64, count int) {
	for i := 0; i < count; i++ {
		c.parts = append(c.parts, confP{
			x:  cx,
			y:  cy,
			vx: (c.rng.Float64() - 0.5) * 3.4,
			vy: -0.4 - c.rng.Float64()*1.3,
			ch: confGlyphs[c.rng.IntN(len(confGlyphs))],
			c:  confColors[c.rng.IntN(len(confColors))],
		})
	}
}

func (c *Confetti) Step(n int) {
	c.n = n
	if n%100 == 1 {
		c.burst(float64(c.w)/2, float64(c.h)/2, 70)
	}
	if n%100 == 30 { // side pops
		c.burst(float64(c.w)/5, float64(c.h)*0.7, 30)
		c.burst(float64(c.w)*4/5, float64(c.h)*0.7, 30)
	}
	alive := c.parts[:0]
	for _, p := range c.parts {
		p.vy += 0.07 // gravity
		p.x += p.vx
		p.y += p.vy
		if p.y < float64(c.h)+1 && p.x >= -1 && p.x < float64(c.w)+1 {
			alive = append(alive, p)
		}
	}
	c.parts = alive
}

func (c *Confetti) View() string {
	g := NewGrid(c.w, c.h)
	for _, p := range c.parts {
		g.Set(int(p.x), int(p.y), p.ch, p.c)
	}

	msg := " ✓ order placed "
	sub := " eta 24 min · cash on delivery "
	bw := len([]rune(sub)) + 2
	bx := (c.w - bw) / 2
	by := c.h/2 - 2
	g.Text(bx, by, "╭"+repeat('─', bw-2)+"╮", Green)
	g.Text(bx, by+1, "│"+pad(msg, bw-2)+"│", Green)
	g.Text(bx, by+2, "│"+pad(sub, bw-2)+"│", Green)
	g.Text(bx, by+3, "╰"+repeat('─', bw-2)+"╯", Green)
	// recolor inner text
	g.Text(bx+1+(bw-2-len([]rune(msg)))/2, by+1, msg, Fg)
	g.Text(bx+1+(bw-2-len([]rune(sub)))/2, by+2, sub, Comment)
	return g.String()
}

func repeat(r rune, n int) string {
	if n < 0 {
		n = 0
	}
	out := make([]rune, n)
	for i := range out {
		out[i] = r
	}
	return string(out)
}

func pad(s string, w int) string {
	n := len([]rune(s))
	if n >= w {
		return s
	}
	left := (w - n) / 2
	return repeat(' ', left) + s + repeat(' ', w-n-left)
}
