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

// steamFrames animate rising steam above the cup (settled phase). Each frame is
// two stacked lines; cycling them makes the wisps drift up and apart.
var steamFrames = [][]string{
	{" ) (  ", "( )   "},
	{"( )   ", " ) (  "},
	{" ( )  ", "  ) ( "},
	{"  ) ( ", " ( )  "},
}

// homeItems are the splash menu choices. Only "go to shop" is live today; the
// list is the seam for future home destinations (orders, the usual, settings)
// reachable with the down arrow.
var homeItems = []string{"go to shop"}

type Splash struct {
	bootStep  int
	spin      string
	tagline   string
	frame     int // animation frame (steam)
	sel       int // selected home item
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

// WithFrame sets the animation frame (drives the steam cadence).
func (s Splash) WithFrame(f int) Splash { s.frame = f; return s }

// WithSelection sets the highlighted home item.
func (s Splash) WithSelection(i int) Splash { s.sel = i; return s }

// ItemCount is the number of home items (for cursor bounds in the router).
func ItemCount() int { return len(homeItems) }

// coffeeBlock renders the cup: optional drifting steam, then a mug whose liquid
// fills bottom-up to `fill` rows (0-3). Gold throughout — the design's cup hue.
func coffeeBlock(frame, fill int, steam bool) string {
	var lines []string
	if steam {
		s := steamFrames[(frame/4)%len(steamFrames)]
		lines = append(lines,
			theme.FaintStyle.Render("  "+s[0]),
			theme.FaintStyle.Render("  "+s[1]))
	} else {
		lines = append(lines, "", "")
	}
	interior := []string{"      ", "      ", "      "}
	for i := 0; i < fill && i < 3; i++ {
		interior[2-i] = "▓▓▓▓▓▓" // fill from the bottom row up
	}
	lines = append(lines,
		theme.GoldStyle.Render(" ╭──────╮"),
		theme.GoldStyle.Render(" │"+interior[0]+"│"),
		theme.GoldStyle.Render(" │"+interior[1]+"│)"),
		theme.GoldStyle.Render(" │"+interior[2]+"│"),
		theme.GoldStyle.Render(" ╰──────╯"))
	return strings.Join(lines, "\n")
}

// menuBlock renders the home choices; the selected one reads as a button.
func (s Splash) menuBlock() string {
	lines := []string{theme.DimStyle.Render("where to?"), ""}
	for i, it := range homeItems {
		if i == s.sel {
			lines = append(lines, theme.GreenStyle.Bold(true).Render("▸ ")+
				theme.SelRowStyle.Bold(true).Render(" "+it+" "))
		} else {
			lines = append(lines, "  "+theme.DimStyle.Render(it))
		}
	}
	lines = append(lines, "", theme.FaintStyle.Render("↑↓ navigate · ↵ select"))
	return strings.Join(lines, "\n")
}

// centerBlock pads every line of a multi-line block equally so the whole block
// sits centred within `width`.
func centerBlock(block string, width int) string {
	lines := strings.Split(block, "\n")
	max := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > max {
			max = w
		}
	}
	pad := (width - max) / 2
	if pad < 0 {
		pad = 0
	}
	p := strings.Repeat(" ", pad)
	for i := range lines {
		lines[i] = p + lines[i]
	}
	return strings.Join(lines, "\n")
}

func (s Splash) View() string {
	var b strings.Builder
	b.WriteString("\n\n")

	// boot phase: stream the handshake lines while the cup brews beside them.
	if s.bootStep < BootLineCount {
		var stream []string
		for i := 0; i < s.bootStep && i < len(bootLines); i++ {
			ln := bootLines[i]
			stream = append(stream, theme.Fg(ln.Color).Render(ln.Text))
		}
		stream = append(stream, theme.CursorStyle.Render(s.spin+" brewing your session …"))
		fill := s.bootStep
		if fill > 3 {
			fill = 3
		}
		row := lipgloss.JoinHorizontal(lipgloss.Center,
			coffeeBlock(s.frame, fill, false), "   ", strings.Join(stream, "\n"))
		for _, l := range strings.Split(row, "\n") {
			b.WriteString("  " + l + "\n")
		}
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
	// center centres a single (possibly styled) line within the logo's width.
	center := func(s string) string {
		if pad := (logoW - lipgloss.Width(s)) / 2; pad > 0 {
			return strings.Repeat(" ", pad) + s
		}
		return s
	}

	// Settled connect line — a single "✓ connected" handshake above the mark.
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

	// home: a steaming cup on the left, the shop menu (button) on the right.
	home := lipgloss.JoinHorizontal(lipgloss.Center,
		coffeeBlock(s.frame, 3, true), "     ", s.menuBlock())
	for _, l := range strings.Split(centerBlock(home, logoW), "\n") {
		b.WriteString("  " + l + "\n")
	}
	return b.String()
}
