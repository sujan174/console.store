package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"console.store/internal/tui/render"
	"console.store/internal/tui/theme"
)

// homeItems are the splash menu choices. Only "go to shop" is live today; the
// list is the seam for future home destinations (orders, the usual, settings)
// reachable with the down arrow.
var homeItems = []string{"go to shop"}

// taglines rotate under the button — dev-flex personality for the home screen.
// The "%d devs" line is appended only when a stats provider is wired (see ticker).
var taglines = []string{
	"git pull origin coffee",
	"0 ads · 0 popups · 1 keypress",
	"sudo make me a sandwich",
	"latency: food 28min, regret 0ms",
}

// speckGlyphs are the twinkle characters for the ambient starfield.
var speckGlyphs = []rune{'·', '˙', '‧', '⋆'}

const (
	speckCount   = 28 // candidate specks (some land on content and are skipped)
	canvasMargin = 6  // blank columns added each side of the hero for the starfield
)

type Splash struct {
	decodeStep int // decode progress (0..render.DecodeSteps)
	frame      int // animation frame (glitch shimmer, twinkle, blink, ticker)
	sel        int // selected home item
	caps       render.Caps
	logoCache  string // render.Logo is constant per session; computed once here
	statsFunc  func() (online, orders int)
}

func NewSplash() Splash { return Splash{} }

// WithCaps sets the terminal capabilities and precomputes the logo. The logo is
// invariant for the session, so caching it here avoids re-rendering (and, on the
// Kitty path, re-encoding a PNG) on every ~60ms animation tick. The cache rides
// through the value-copy WithDecode returns.
func (s Splash) WithCaps(c render.Caps) Splash {
	s.caps = c
	s.logoCache = render.Logo(c, 64)
	return s
}

// WithDecode returns a copy reflecting the current decode step.
func (s Splash) WithDecode(step int) Splash { s.decodeStep = step; return s }

// WithFrame sets the animation frame (drives twinkle, blink, ticker rotation).
func (s Splash) WithFrame(f int) Splash { s.frame = f; return s }

// WithSelection sets the highlighted home item.
func (s Splash) WithSelection(i int) Splash { s.sel = i; return s }

// WithStats wires the live-stats provider (used by the dynamic tagline).
func (s Splash) WithStats(f func() (online, orders int)) Splash { s.statsFunc = f; return s }

// ItemCount is the number of home items (for cursor bounds in the router).
func ItemCount() int { return len(homeItems) }

// centerLine left-pads a single (possibly styled) line so it sits centred
// within `width`. ANSI styling is ignored for width (lipgloss.Width strips it).
func centerLine(s string, width int) string {
	if pad := (width - lipgloss.Width(s)) / 2; pad > 0 {
		return strings.Repeat(" ", pad) + s
	}
	return s
}

// homeMenuLines renders the centred home choices: the selected item as a framed
// button, the rest as dim centred lines. width is the canvas width everything
// centres against.
func (s Splash) homeMenuLines(width int) []string {
	var out []string
	for i, it := range homeItems {
		if i == s.sel {
			inner := "  ▸ " + it + "  "
			rule := strings.Repeat("─", lipgloss.Width(inner))
			out = append(out,
				centerLine(theme.GreenStyle.Render("┌"+rule+"┐"), width),
				centerLine(theme.GreenStyle.Render("│")+
					theme.BrightStyle.Bold(true).Render(inner)+
					theme.GreenStyle.Render("│"), width),
				centerLine(theme.GreenStyle.Render("└"+rule+"┘"), width))
		} else {
			out = append(out, centerLine(theme.DimStyle.Render(it), width))
		}
	}
	return out
}

// ticker is the rotating dev-flex tagline line with a blinking cursor.
func (s Splash) ticker() string {
	tls := taglines
	if s.statsFunc != nil {
		online, _ := s.statsFunc()
		if n := online - 1; n >= 0 {
			tls = append(append([]string{}, taglines...),
				fmt.Sprintf("you + %d devs ordering right now", n))
		}
	}
	tl := tls[(s.frame/50)%len(tls)]
	cursor := " "
	if (s.frame/9)%2 == 0 {
		cursor = "█"
	}
	return theme.CursorStyle.Render("› ") + theme.DimStyle.Render(tl) +
		theme.FaintStyle.Render(cursor)
}

// bracketRow renders a full-width row with corner-bracket glyphs at the ends —
// the decorative frame's top (or bottom) edge.
func bracketRow(width int, top bool) string {
	l, r := "┌─", "─┐"
	if !top {
		l, r = "└─", "─┘"
	}
	mid := width - lipgloss.Width(l) - lipgloss.Width(r)
	if mid < 0 {
		mid = 0
	}
	return theme.FaintStyle.Render(l) + strings.Repeat(" ", mid) + theme.FaintStyle.Render(r)
}

// starfield returns the lit specks for this frame as row -> col -> styled glyph.
// Positions are fixed per speck (deterministic from the index); each twinkles on
// its own phase so they blink asynchronously. Purely a function of frame.
func starfield(rows, cols, frame int) map[int]map[int]string {
	out := map[int]map[int]string{}
	if rows <= 0 || cols <= 0 {
		return out
	}
	for k := 0; k < speckCount; k++ {
		h := (k * 2654435761) & 0x7fffffff
		x := h % cols
		y := (h / cols) % rows
		phase := k*7 + 3
		t := frame/8 + phase
		if t%4 != 0 { // lit ~1/4 of the time → gentle twinkle
			continue
		}
		g := speckGlyphs[t%len(speckGlyphs)]
		if out[y] == nil {
			out[y] = map[int]string{}
		}
		out[y][x] = theme.FaintStyle.Render(string(g))
	}
	return out
}

// overlay composites specks onto a line, replacing only blank cells that sit
// outside any active ANSI style span (i.e. raw padding) so it never disturbs the
// wordmark, menu, brackets, or ticker. Tracks the display column past ANSI codes.
func overlay(line string, specks map[int]string) string {
	if len(specks) == 0 {
		return line
	}
	runes := []rune(line)
	var out strings.Builder
	col := 0
	styleOpen := false
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\x1b' { // SGR escape: copy through, track open/reset
			j := i
			for j < len(runes) && runes[j] != 'm' {
				j++
			}
			end := min(j+1, len(runes))
			seq := string(runes[i:end])
			out.WriteString(seq)
			styleOpen = seq != "\x1b[0m"
			i = j
			continue
		}
		if r == ' ' && !styleOpen {
			if g, ok := specks[col]; ok {
				out.WriteString(g)
				col++
				continue
			}
		}
		out.WriteString(string(r))
		col++
	}
	return out.String()
}

func (s Splash) View() string { return s.view() }

func (s Splash) view() string {
	// decode phase: glitch-resolve the wordmark, subtitle beneath. The Kitty
	// graphics path renders a non-text bitmap that can't be glyph-decoded (and
	// would break width math here), so it skips straight to the settled view.
	if s.decodeStep < render.DecodeSteps && !(s.caps.KittyGraphics && render.KittyFlag) {
		var b strings.Builder
		b.WriteString("\n\n")
		art := render.DecodeWordmark(s.caps, s.decodeStep, s.frame)
		artLines := strings.Split(strings.TrimRight(art, "\n"), "\n")
		w := 0
		for _, l := range artLines {
			if x := lipgloss.Width(l); x > w {
				w = x
			}
		}
		for _, l := range artLines {
			b.WriteString("  " + l + "\n")
		}
		b.WriteString("\n")
		sub := theme.DimStyle.Render("coffee · food · snacks")
		pad := (w - lipgloss.Width(sub)) / 2
		if pad < 0 {
			pad = 0
		}
		b.WriteString("  " + strings.Repeat(" ", pad) + sub + "\n")
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
	canvasW := logoW + 2*canvasMargin

	// Build the settled home as a list of canvas-width lines. Everything centres
	// on canvasW so the whole block centres as a unit (the root View places it).
	var lines []string
	addC := func(s string) { lines = append(lines, centerLine(s, canvasW)) }

	lines = append(lines, bracketRow(canvasW, true), "")

	conn := theme.DimStyle.Render("guest@blr ~ % ssh console.store   ") +
		theme.GreenStyle.Render("✓ connected") + theme.FaintStyle.Render(" · 14ms")
	addC(conn)
	lines = append(lines, "")

	for _, l := range logoLines {
		addC(l)
	}
	// gold ".store" suffix, right-aligned under the wordmark (design accent).
	storeLine := strings.Repeat(" ", logoW-lipgloss.Width(".store")) + theme.GoldStyle.Render(".store")
	addC(storeLine)
	lines = append(lines, "")

	addC(theme.FaintStyle.Render(strings.Repeat("─", 30)))
	addC(theme.DimStyle.Render("coffee · food · snacks"))
	lines = append(lines, "")

	lines = append(lines, s.homeMenuLines(canvasW)...)
	lines = append(lines, "")

	addC(s.ticker())
	addC(theme.FaintStyle.Render("↑↓ navigate  ·  ↵ select"))
	lines = append(lines, "", bracketRow(canvasW, false))

	// Normalise to canvas width FIRST so blank lines and short rows have real
	// space cells for specks to land in; then twinkle over the blanks.
	for i, l := range lines {
		if d := canvasW - lipgloss.Width(l); d > 0 {
			lines[i] = l + strings.Repeat(" ", d)
		}
	}
	field := starfield(len(lines), canvasW, s.frame)
	for y := range lines {
		if sp, ok := field[y]; ok {
			lines[y] = overlay(lines[y], sp)
		}
	}
	return strings.Join(lines, "\n")
}
