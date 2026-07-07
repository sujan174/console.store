package main

// Boot: typewriter shell session with spinners resolving to statuses.
// Integration target: splash boot sequence / first-run flow.
type Boot struct {
	w, h, n int
	total   int
}

type bootLine struct {
	prefix  string
	prefixC RGB
	text    string
	typeSpd int // frames per typed char (0 = instant)
	spin    int // frames of trailing spinner
	status  string
	statusC RGB
}

var bootScript = []bootLine{
	{"~ % ", Green, "console order ramen", 3, 0, "", Fg},
	{"  » ", Comment, "resolving preset 'ramen'", 0, 22, "ok", Green},
	{"  » ", Comment, "pushing cart → swiggy", 0, 30, "ok", Green},
	{"  » ", Comment, "pulling live bill", 0, 26, "₹412", Yellow},
	{"  » ", Comment, "confirm order?", 0, 34, "y", Cyan},
	{"  » ", Comment, "placing order", 0, 40, "ok", Green},
	{"", Fg, "", 0, 8, "", Fg},
	{"  ✓ ", Green, "order placed · eta 24 min", 0, 0, "", Fg},
}

const bootHold = 90

func (b *Boot) Name() string    { return "boot" }
func (b *Boot) Tagline() string { return "typewriter shell session (splash)" }
func (b *Boot) Step(n int)      { b.n = n }

func (b *Boot) Init(w, h int) {
	b.w, b.h = w, h
	b.total = bootHold
	for _, l := range bootScript {
		b.total += l.typeSpd*len([]rune(l.text)) + l.spin
	}
}

var bootSpin = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

func (b *Boot) View() string {
	g := NewGrid(b.w, b.h)
	x0 := 4
	y := 2
	rem := b.n % b.total
	for _, l := range bootScript {
		runes := []rune(l.text)
		typeDur := l.typeSpd * len(runes)

		g.Text(x0, y, l.prefix, l.prefixC)
		tx := x0 + len([]rune(l.prefix))

		if l.typeSpd > 0 && rem < typeDur {
			// mid-type: partial text + block cursor
			k := rem / l.typeSpd
			g.Text(tx, y, string(runes[:k]), Fg)
			g.Set(tx+k, y, '▉', Fg)
			return g.String()
		}
		g.Text(tx, y, l.text, Fg)
		rem -= typeDur

		if rem < l.spin {
			// spinning
			g.Set(tx+len(runes)+1, y, bootSpin[(b.n/2)%len(bootSpin)], Cyan)
			return g.String()
		}
		rem -= l.spin
		if l.status != "" {
			g.Text(tx+len(runes)+1, y, l.status, l.statusC)
		}
		y++
	}
	// done — blinking prompt below
	g.Text(x0, y+1, "~ % ", Green)
	if (b.n/9)%2 == 0 {
		g.Set(x0+4, y+1, '▉', Fg)
	}
	return g.String()
}
