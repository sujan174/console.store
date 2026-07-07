package main

import "math"

// Donut: the classic spinning torus (a1k0n's donut.c), luminance-shaded
// glyphs colored across the palette. Integration target: fun loading state.
type Donut struct{ w, h, n int }

func (d *Donut) Name() string    { return "donut" }
func (d *Donut) Tagline() string { return "donut.c torus, palette shaded" }
func (d *Donut) Init(w, h int)   { d.w, d.h = w, h }
func (d *Donut) Step(n int)      { d.n = n }

var donutChars = []rune(".,-~:;=!*#$@")

func (d *Donut) View() string {
	g := NewGrid(d.w, d.h)
	if d.w < 4 || d.h < 4 {
		return g.String()
	}
	A := float64(d.n) * 0.05
	B := float64(d.n) * 0.026
	cA, sA := math.Cos(A), math.Sin(A)
	cB, sB := math.Cos(B), math.Sin(B)

	const R1, R2, K2 = 1.0, 2.0, 5.0
	K1 := float64(d.h) * K2 * 3 / (8 * (R1 + R2))

	zbuf := make([]float64, d.w*d.h)
	for th := 0.0; th < 2*math.Pi; th += 0.07 {
		ct, st := math.Cos(th), math.Sin(th)
		for ph := 0.0; ph < 2*math.Pi; ph += 0.02 {
			cp, sp := math.Cos(ph), math.Sin(ph)
			circX := R2 + R1*ct
			circY := R1 * st

			x := circX*(cB*cp+sA*sB*sp) - circY*cA*sB
			y := circX*(sB*cp-sA*cB*sp) + circY*cA*cB
			z := K2 + cA*circX*sp + circY*sA
			ooz := 1 / z

			px := int(float64(d.w)/2 + K1*2*ooz*x)
			py := int(float64(d.h)/2 - K1*ooz*y)
			if px < 0 || px >= d.w || py < 0 || py >= d.h {
				continue
			}
			lum := cp*ct*sB - cA*ct*sp - sA*st + cB*(cA*st-ct*sA*sp)
			if lum <= 0 {
				continue
			}
			idx := py*d.w + px
			if ooz <= zbuf[idx] {
				continue
			}
			zbuf[idx] = ooz
			li := int(lum * 8)
			if li >= len(donutChars) {
				li = len(donutChars) - 1
			}
			col := gradient([]RGB{Magenta, Blue, Cyan, Fg}, lum/math.Sqrt2)
			g.Set(px, py, donutChars[li], col)
		}
	}
	return g.String()
}
