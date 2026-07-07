package main

import "math"

// Plasma: layered sine interference mapped over the palette,
// rendered on the half-block framebuffer (timg-style truecolor).
// Integration target: hero/background ambience.
type Plasma struct{ w, h, n int }

func (p *Plasma) Name() string    { return "plasma" }
func (p *Plasma) Tagline() string { return "sine plasma, half-block truecolor" }
func (p *Plasma) Init(w, h int)   { p.w, p.h = w, h }
func (p *Plasma) Step(n int)      { p.n = n }

var plasmaStops = []RGB{Bg, Blue, Magenta, Cyan, Teal, Bg}

func (p *Plasma) View() string {
	fb := NewFB(p.w, p.h)
	t := float64(p.n) * 0.06
	cx := float64(p.w) / 2
	cy := float64(p.h) // pixel-space center Y
	for y := 0; y < p.h*2; y++ {
		fy := float64(y)
		for x := 0; x < p.w; x++ {
			fx := float64(x)
			v := math.Sin(fx*0.055+t) +
				math.Sin(fy*0.045-t*0.7) +
				math.Sin((fx+fy)*0.04+t*0.4) +
				math.Sin(math.Hypot(fx-cx, (fy-cy)*1.6)*0.05-t)
			fb.Set(x, y, gradient(plasmaStops, (v+4)/8))
		}
	}
	return fb.String()
}
