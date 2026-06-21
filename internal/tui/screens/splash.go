package screens

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

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
	logoLines := strings.Split(strings.TrimRight(logo, "\n"), "\n")
	logoW := 0
	for _, l := range logoLines {
		if x := lipgloss.Width(l); x > logoW {
			logoW = x
		}
	}
	// center centres a (possibly styled) line within the logo's width so every
	// element stacks under the wordmark; the whole block is then centred in the
	// viewport by the root View.
	center := func(s string) string {
		if pad := (logoW - lipgloss.Width(s)) / 2; pad > 0 {
			return strings.Repeat(" ", pad) + s
		}
		return s
	}

	// Settled connect line (one line, not the streamed boot) — design: a single
	// "✓ connected" handshake above the mark.
	conn := theme.DimStyle.Render("guest@blr ~ % ssh console.store   ") +
		theme.GreenStyle.Render("✓ connected") + theme.FaintStyle.Render(" · 14ms")
	b.WriteString("  " + center(conn) + "\n\n")

	for _, l := range logoLines {
		b.WriteString("  " + l + "\n")
	}
	// gold ".store" suffix, right-aligned under the wordmark (design accent).
	if storePad := logoW - lipgloss.Width(".store"); storePad > 0 {
		b.WriteString("  " + strings.Repeat(" ", storePad) + theme.GoldStyle.Render(".store") + "\n")
	}
	b.WriteString("\n")
	b.WriteString("  " + center(theme.DimStyle.Render("coffee · food · snacks")) + "\n\n")
	b.WriteString("  " + center(theme.FaintStyle.Render("press any key to connect")+theme.CursorStyle.Render(" ▋")) + "\n")
	return b.String()
}
