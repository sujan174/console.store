package main

import "strings"

// Spinners: loading-glyph gallery + progress bar styles.
// Integration target: menu/cart loading states, cart "syncing" chip.
type Spinners struct{ w, h, n int }

func (s *Spinners) Name() string    { return "spinners" }
func (s *Spinners) Tagline() string { return "loading glyphs + progress bars" }
func (s *Spinners) Init(w, h int)   { s.w, s.h = w, h }
func (s *Spinners) Step(n int)      { s.n = n }

type spinSet struct {
	label  string
	frames []string
	div    int // frames per glyph
}

var spinSets = []spinSet{
	{"braille", runes("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"), 2},
	{"orbit", runes("⣾⣽⣻⢿⡿⣟⣯⣷"), 2},
	{"line", runes(`|/-\`), 2},
	{"corners", runes("◜◝◞◟"), 3},
	{"quads", runes("▖▘▝▗"), 3},
	{"pulse", runes("·•●●•·"), 3},
	{"stack", runes("▁▂▃▄▅▆▇█▇▆▅▄▃▂"), 1},
	{"arrows", runes("←↖↑↗→↘↓↙"), 2},
	{"bounce", []string{"[●    ]", "[ ●   ]", "[  ●  ]", "[   ● ]", "[    ●]", "[   ● ]", "[  ●  ]", "[ ●   ]"}, 2},
	{"cooking", []string{"cooking   ", "cooking.  ", "cooking.. ", "cooking..."}, 8},
}

func runes(s string) []string {
	rs := []rune(s)
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = string(r)
	}
	return out
}

func (s *Spinners) View() string {
	g := NewGrid(s.w, s.h)
	g.Text(3, 1, "spinner gallery", Blue)
	g.Text(3, 2, strings.Repeat("─", 30), BgHi)

	col := 0
	row := 4
	for i, set := range spinSets {
		x := 3 + col*26
		f := set.frames[(s.n/set.div)%len(set.frames)]
		g.Text(x, row, f, Cyan)
		g.Text(x+12, row, set.label, Comment)
		if i%2 == 1 {
			row += 2
			col = 0
		} else {
			col = 1
		}
	}

	// progress bars
	by := row + 2
	g.Text(3, by, "progress", Blue)
	g.Text(3, by+1, strings.Repeat("─", 30), BgHi)
	p := float64(s.n%140) / 140
	barW := s.w - 30
	if barW > 48 {
		barW = 48
	}
	if barW < 10 {
		barW = 10
	}

	// 1: eighth-block smooth fill
	drawEighthBar(g, 3, by+3, barW, p, Green)
	g.Text(3+barW+2, by+3, "eighth-block", Comment)

	// 2: gradient dot fill
	for i := 0; i < barW; i++ {
		t := float64(i) / float64(barW-1)
		ch := '·'
		c := Comment
		if t <= p {
			ch = '━'
			c = gradient([]RGB{Blue, Magenta, Red}, t)
		}
		g.Set(3+i, by+5, ch, c)
	}
	g.Text(3+barW+2, by+5, "gradient sweep", Comment)

	// 3: indeterminate shimmer
	for i := 0; i < barW; i++ {
		d := abs((s.n*2)%(barW*2) - i)
		c := lerp(Cyan, BgHi, float64(d)/7)
		g.Set(3+i, by+7, '━', c)
	}
	g.Text(3+barW+2, by+7, "indeterminate", Comment)

	return g.String()
}

func drawEighthBar(g *Grid, x, y, w int, p float64, c RGB) {
	part := []rune(" ▏▎▍▌▋▊▉█")
	fill := p * float64(w)
	for i := 0; i < w; i++ {
		r := fill - float64(i)
		switch {
		case r >= 1:
			g.Set(x+i, y, '█', c)
		case r > 0:
			g.Set(x+i, y, part[int(r*8)+1], c) // r in (0,1) → index 1..8
		default:
			g.Set(x+i, y, '─', BgHi)
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
