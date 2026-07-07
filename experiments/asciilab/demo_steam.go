package main

import (
	"math"
	"math/rand/v2"
)

// Steam: ramen bowl with drifting steam particles.
// Integration target: first-run ramen animation / empty-cart art.
type Steam struct {
	w, h  int
	n     int
	parts []steamP
	rng   *rand.Rand
}

type steamP struct {
	x, y  float64
	vy    float64
	phase float64
	age   float64
	life  float64
}

func (s *Steam) Name() string    { return "steam" }
func (s *Steam) Tagline() string { return "ramen bowl steam particles" }

func (s *Steam) Init(w, h int) {
	s.w, s.h = w, h
	s.parts = nil
	s.rng = rand.New(rand.NewPCG(7, 21))
}

var bowlArt = []string{
	`     _______________________     `,
	`    /  ~~~  ~~~~~  ~~  ~~~  \    `,
	`   |≈≈≈≈≈≈≈≈≈≈≈≈≈≈≈≈≈≈≈≈≈≈≈≈≈|   `,
	`    \                       /    `,
	`     '.___________________.'     `,
	`         \_____________/         `,
}

func (s *Steam) bowlOrigin() (int, int) {
	bx := (s.w - len([]rune(bowlArt[0]))) / 2
	by := s.h - len(bowlArt) - 1
	return bx, by
}

func (s *Steam) Step(n int) {
	s.n = n
	bx, by := s.bowlOrigin()
	// spawn 1–2 particles near the broth surface
	for i := 0; i < 1+s.rng.IntN(2); i++ {
		s.parts = append(s.parts, steamP{
			x:     float64(bx + 4 + s.rng.IntN(len([]rune(bowlArt[0]))-8)),
			y:     float64(by + 1),
			vy:    0.22 + s.rng.Float64()*0.3,
			phase: s.rng.Float64() * math.Pi * 2,
			life:  22 + s.rng.Float64()*26,
		})
	}
	alive := s.parts[:0]
	for _, p := range s.parts {
		p.y -= p.vy
		p.age++
		if p.age < p.life && p.y > 0 {
			alive = append(alive, p)
		}
	}
	s.parts = alive
}

var steamGlyphs = []rune("~≈∿°")

func (s *Steam) View() string {
	g := NewGrid(s.w, s.h)
	bx, by := s.bowlOrigin()

	for _, p := range s.parts {
		t := p.age / p.life
		drift := math.Sin(p.phase+p.age*0.15) * (1 + t*2.2)
		x := int(p.x + drift)
		y := int(p.y)
		if y >= by { // never draw over the bowl
			continue
		}
		ch := steamGlyphs[int(t*float64(len(steamGlyphs)))%len(steamGlyphs)]
		g.Set(x, y, ch, lerp(Cyan, BgHi, t))
	}

	for i, line := range bowlArt {
		for j, r := range []rune(line) {
			if r == ' ' {
				continue
			}
			c := Red
			switch r {
			case '~':
				c = Yellow // noodles
			case '≈':
				c = Orange // broth rim
			}
			g.Set(bx+j, by+i, r, c)
		}
	}

	label := "ramen · ₹289 · 24 min"
	g.Text((s.w-len([]rune(label)))/2, by+len(bowlArt), label, Comment)
	return g.String()
}
