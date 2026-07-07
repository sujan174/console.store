package main

import "strings"

// Grid is a character surface: one rune + fg color per terminal cell,
// painted on the Tokyo Night background.
type Grid struct {
	W, H  int
	cells []gcell
}

type gcell struct {
	ch rune
	c  RGB
}

func NewGrid(w, h int) *Grid {
	g := &Grid{W: w, H: h, cells: make([]gcell, w*h)}
	for i := range g.cells {
		g.cells[i] = gcell{' ', Fg}
	}
	return g
}

func (g *Grid) Set(x, y int, ch rune, c RGB) {
	if x < 0 || x >= g.W || y < 0 || y >= g.H {
		return
	}
	g.cells[y*g.W+x] = gcell{ch, c}
}

func (g *Grid) Text(x, y int, s string, c RGB) {
	for i, r := range []rune(s) {
		g.Set(x+i, y, r, c)
	}
}

func (g *Grid) String() string {
	var b strings.Builder
	for y := 0; y < g.H; y++ {
		b.WriteString(bgSeq(Bg))
		var last RGB
		lastSet := false
		for x := 0; x < g.W; x++ {
			c := g.cells[y*g.W+x]
			if !lastSet || c.c != last {
				b.WriteString(fgSeq(c.c))
				last, lastSet = c.c, true
			}
			b.WriteRune(c.ch)
		}
		b.WriteString(reset)
		if y < g.H-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// FB is a half-block "pixel" framebuffer (timg technique): every terminal
// cell renders two vertical pixels via ▀ with fg = top pixel, bg = bottom.
type FB struct {
	W, H int // H in pixels = 2 × cell rows
	px   []RGB
}

func NewFB(w, hCells int) *FB {
	f := &FB{W: w, H: hCells * 2, px: make([]RGB, w*hCells*2)}
	f.Fill(Bg)
	return f
}

func (f *FB) Fill(c RGB) {
	for i := range f.px {
		f.px[i] = c
	}
}

func (f *FB) Set(x, y int, c RGB) {
	if x < 0 || x >= f.W || y < 0 || y >= f.H {
		return
	}
	f.px[y*f.W+x] = c
}

func (f *FB) Get(x, y int) RGB {
	if x < 0 || x >= f.W || y < 0 || y >= f.H {
		return Bg
	}
	return f.px[y*f.W+x]
}

func (f *FB) String() string {
	var b strings.Builder
	rows := f.H / 2
	for r := 0; r < rows; r++ {
		var lastT, lastB RGB
		lastSet := false
		for x := 0; x < f.W; x++ {
			top := f.px[(r*2)*f.W+x]
			bot := f.px[(r*2+1)*f.W+x]
			if !lastSet || top != lastT || bot != lastB {
				b.WriteString(fgSeq(top))
				b.WriteString(bgSeq(bot))
				lastT, lastB, lastSet = top, bot, true
			}
			b.WriteRune('▀')
		}
		b.WriteString(reset)
		if r < rows-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
