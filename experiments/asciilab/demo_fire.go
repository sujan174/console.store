package main

import (
	"math/rand/v2"
)

// Fire: classic doom-fire cellular automaton on the half-block framebuffer.
// Integration target: "spicy" badge accents, dramatic empty states.
type Fire struct {
	w, h    int // cells
	pw, ph  int // pixels
	heat    []int
	palette []RGB
	rng     *rand.Rand
}

func (f *Fire) Name() string    { return "fire" }
func (f *Fire) Tagline() string { return "doom fire, half-block truecolor" }

const fireMax = 47

func (f *Fire) Init(w, h int) {
	f.w, f.h = w, h
	f.pw, f.ph = w, h*2
	f.heat = make([]int, f.pw*f.ph)
	f.rng = rand.New(rand.NewPCG(4, 4))

	stops := []RGB{Bg, {0x4a, 0x1e, 0x33}, Red, Orange, Yellow, {0xff, 0xf4, 0xd0}}
	f.palette = make([]RGB, fireMax+1)
	for i := range f.palette {
		f.palette[i] = gradient(stops, float64(i)/fireMax)
	}
	// seed the bottom row
	for x := 0; x < f.pw; x++ {
		f.heat[(f.ph-1)*f.pw+x] = fireMax
	}
}

func (f *Fire) Step(n int) {
	// flicker the source
	for x := 0; x < f.pw; x++ {
		v := fireMax - f.rng.IntN(9)
		f.heat[(f.ph-1)*f.pw+x] = v
	}
	for y := 0; y < f.ph-1; y++ {
		for x := 0; x < f.pw; x++ {
			sx := x + f.rng.IntN(3) - 1
			if sx < 0 {
				sx = 0
			}
			if sx >= f.pw {
				sx = f.pw - 1
			}
			v := f.heat[(y+1)*f.pw+sx] - f.rng.IntN(2)
			if v < 0 {
				v = 0
			}
			f.heat[y*f.pw+x] = v
		}
	}
}

func (f *Fire) View() string {
	fb := NewFB(f.w, f.h)
	for y := 0; y < f.ph; y++ {
		for x := 0; x < f.pw; x++ {
			fb.Set(x, y, f.palette[f.heat[y*f.pw+x]])
		}
	}
	return fb.String()
}
