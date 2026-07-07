package main

import (
	"math"
	"math/rand/v2"
)

// Rider: courier cycling past a parallax night skyline.
// Integration target: order-tracking page courier sprite.
type Rider struct {
	w, h, n int
	stars   []star
}

type star struct{ x, y, tw int }

func (r *Rider) Name() string    { return "rider" }
func (r *Rider) Tagline() string { return "parallax courier ride (tracking)" }
func (r *Rider) Step(n int)      { r.n = n }

func (r *Rider) Init(w, h int) {
	r.w, r.h = w, h
	rng := rand.New(rand.NewPCG(3, 9))
	r.stars = nil
	skyH := h - 9
	if skyH < 2 {
		skyH = 2
	}
	for i := 0; i < w/4; i++ {
		r.stars = append(r.stars, star{rng.IntN(w), rng.IntN(skyH), rng.IntN(40)})
	}
}

// skyline layers, repeated and scrolled at different speeds
var skyFar = []rune("▂▃▂▄▃▅▄▂▃▄▂▅▃▂▄▃")
var skyNear = []rune("▄▆█▅██▇▅▆██▄▇█▆▅")

func (r *Rider) View() string {
	g := NewGrid(r.w, r.h)
	roadY := r.h - 4

	// stars
	for _, s := range r.stars {
		ch := '·'
		c := Comment
		if (r.n+s.tw)%40 < 6 {
			ch, c = '✦', Fg
		}
		g.Set(s.x, s.y, ch, c)
	}

	// parallax skylines (kept clear of the courier sprite rows)
	for x := 0; x < r.w; x++ {
		far := skyFar[((x+r.n/6)%len(skyFar)+len(skyFar))%len(skyFar)]
		g.Set(x, roadY-5, far, BgHi)
		near := skyNear[((x+r.n/3)%len(skyNear)+len(skyNear))%len(skyNear)]
		g.Set(x, roadY-4, near, Comment)
		// lit windows on the near layer
		if (x*7+r.n/25)%13 == 0 {
			g.Set(x, roadY-4, '▪', Yellow)
		}
	}

	// road
	for x := 0; x < r.w; x++ {
		g.Set(x, roadY-1, '▔', BgHi)
		if (x+r.n/2)%6 < 3 {
			g.Set(x, roadY+1, '╌', Comment)
		}
	}

	// courier: fixed x, world scrolls past; slight bob
	cx := r.w / 4
	bob := 0
	if math.Sin(float64(r.n)*0.35) > 0.7 {
		bob = -1
	}
	wheel := []rune("*+x+")[(r.n/2)%4]
	sy := roadY - 1 + bob
	g.Text(cx+3, sy-2, "__o", Cyan)
	g.Text(cx+1, sy-1, "_`\\<,_", Cyan)
	g.Set(cx, sy, '(', Blue)
	g.Set(cx+1, sy, wheel, Blue)
	g.Set(cx+2, sy, ')', Blue)
	g.Text(cx+3, sy, "/ ", Cyan)
	g.Set(cx+5, sy, '(', Blue)
	g.Set(cx+6, sy, wheel, Blue)
	g.Set(cx+7, sy, ')', Blue)
	// delivery box on the rack
	g.Set(cx+1, sy-2, '▣', Orange)

	// exhaust/dust puffs
	for i := 1; i <= 3; i++ {
		if (r.n/3+i)%4 == 0 {
			g.Set(cx-i*2, sy-(i%2), '°', lerp(Comment, Bg, float64(i)/3))
		}
	}

	// status caption
	prog := float64(r.n%600) / 600
	eta := int(math.Ceil((1 - prog) * 24))
	caption := "out for delivery"
	g.Text(3, 1, caption, Blue)
	barW := r.w / 3
	for i := 0; i < barW; i++ {
		c := BgHi
		ch := '─'
		if float64(i)/float64(barW) < prog {
			c, ch = Green, '━'
		}
		g.Set(3+i, 2, ch, c)
	}
	g.Text(3+barW+2, 2, "eta "+itoa(eta)+" min", Comment)
	return g.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
