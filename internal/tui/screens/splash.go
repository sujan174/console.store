package screens

import (
	"strings"

	"console.store/internal/tui/render"
	"console.store/internal/tui/theme"
)

// bootLines stream during the splash boot phase (design lines 539-545).
var bootLines = []struct{ Text, Color string }{
	{"guest@laptop ~ % ssh console.store", theme.Text},
	{"  ⊙ resolving console.store … 12.4ms", theme.Dim},
	{"  ⊙ tls handshake … ed25519 ✓", theme.Dim},
	{"  ⊙ auth guest@hsr-layout … ok", theme.Green},
	{"  ⊙ 247 devs online · kitchen warm ☕", theme.Price},
}

// BootLineCount is exported so the router knows when boot is done.
const BootLineCount = 5

// Taglines rotate on the splash (design line 535).
var Taglines = []string{"fetching your grub …", "compiling your cravings …", "warming the kitchen …", "git pull origin coffee …"}

type Splash struct {
	bootStep  int
	spin      string
	tagline   string
	caps      render.Caps
	logoCache string // render.Logo is constant per session; computed once here
}

func NewSplash() Splash { return Splash{} }

// WithCaps sets the terminal capabilities and precomputes the logo. The logo is
// invariant for the session, so caching it here avoids re-rendering (and, on the
// Kitty path, re-encoding a PNG) on every ~60ms animation tick. The cache rides
// through the value-copy WithBoot returns.
func (s Splash) WithCaps(c render.Caps) Splash {
	s.caps = c
	s.logoCache = render.Logo(c, 64)
	return s
}

// WithBoot returns a copy reflecting the current boot step, spinner, tagline.
func (s Splash) WithBoot(step int, spin, tagline string) Splash {
	s.bootStep = step
	s.spin = spin
	s.tagline = tagline
	return s
}

func (s Splash) View() string {
	var b strings.Builder
	b.WriteString("\n\n")
	if s.bootStep < BootLineCount {
		for i := 0; i < s.bootStep && i < len(bootLines); i++ {
			ln := bootLines[i]
			b.WriteString("  " + theme.Fg(ln.Color).Render(ln.Text) + "\n")
		}
		b.WriteString("  " + theme.CursorStyle.Render(s.spin+" establishing session …") + "\n")
		return b.String()
	}
	logo := s.logoCache
	if logo == "" { // defensive: WithCaps not called (e.g. bare NewSplash)
		logo = render.Logo(s.caps, 64)
	}
	for _, l := range strings.Split(strings.TrimRight(logo, "\n"), "\n") {
		b.WriteString("  " + l + "\n")
	}
	b.WriteString("\n")
	b.WriteString("  " + theme.GreenStyle.Render(s.tagline) + " " + theme.PriceStyle.Render(s.spin) + "\n\n")
	b.WriteString("  " + theme.DimStyle.Render("bangalore · coffee, food & snacks · ") +
		theme.GreenStyle.Render("247") + theme.DimStyle.Render(" devs online") + "\n\n")
	b.WriteString("  " + theme.FaintStyle.Render("press any key to connect") + theme.CursorStyle.Render("▋") + "\n")
	return b.String()
}
