package screens

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"console.store/internal/tui/render"
	"console.store/internal/tui/theme"
)

// homeItems are the splash menu choices. Only "go to shop" is live today; the
// list is the seam for future home destinations (orders, the usual, settings).
var homeItems = []string{"go to shop"}

// Indentation: the splash reads as a real terminal session. Prompt lines sit at
// the left gutter; the banner (wordmark + tagline) is inset one step further so
// it stands apart as the login banner. The whole block is centred in the
// viewport by the root view, so these are relative to the block's left edge.
const (
	promptIndent = 2
	bodyIndent   = 4
)

type Splash struct {
	decodeStep int // decode progress (0..render.DecodeSteps)
	frame      int // global animation frame (decode flicker + prompt cursor blink)
	splashTick int // ticks since the splash was (re)entered; phases the shimmer
	sel       int // selected home item (seam for future multi-item home)
	caps      render.Caps
	logoCache string // render.Logo is constant per session; computed once here
}

func NewSplash() Splash { return Splash{} }

// WithCaps sets the terminal capabilities and precomputes the logo. The logo is
// invariant for the session, so caching it here avoids re-rendering (and, on the
// Kitty path, re-encoding a PNG) on every ~60ms animation tick.
func (s Splash) WithCaps(c render.Caps) Splash {
	s.caps = c
	s.logoCache = render.Logo(c, 64)
	return s
}

// WithDecode returns a copy reflecting the current decode step.
func (s Splash) WithDecode(step int) Splash { s.decodeStep = step; return s }

// WithFrame sets the global animation frame (decode flicker + cursor blink).
func (s Splash) WithFrame(f int) Splash { s.frame = f; return s }

// WithSplashTick sets ticks-since-splash-entry. The idle shimmer phases off this
// (minus the decode duration) so its first sweep begins at the left the instant
// the reveal lands — the wipe and the sheen read as one continuous motion.
func (s Splash) WithSplashTick(n int) Splash { s.splashTick = n; return s }

// WithSelection sets the highlighted home item.
func (s Splash) WithSelection(i int) Splash { s.sel = i; return s }

// ItemCount is the number of home items (for cursor bounds in the router).
func ItemCount() int { return len(homeItems) }

// blockWidth is the widest display width across lines.
func blockWidth(lines []string) int {
	w := 0
	for _, l := range lines {
		if x := lipgloss.Width(l); x > w {
			w = x
		}
	}
	return w
}

// padRight right-pads every line to width so the block keeps its internal
// left-alignment once the root centres it as a unit. (lipgloss.Place centres
// each line by its own width, so uniform width is what holds the gutter.)
func padRight(lines []string, width int) {
	for i, l := range lines {
		if d := width - lipgloss.Width(l); d > 0 {
			lines[i] = l + strings.Repeat(" ", d)
		}
	}
}

// sshLine is the top prompt — the command the user just "ran" to arrive here.
func sshLine() string {
	return strings.Repeat(" ", promptIndent) +
		theme.DimStyle.Render("~ % ssh ") + theme.TextStyle.Render("console.store")
}

// tagline is the banner's one-line descriptor, inset under the wordmark.
func tagline() string {
	return strings.Repeat(" ", bodyIndent) + theme.DimStyle.Render("coffee · food · snacks")
}

// prompt is the settled call-to-action: a live shell prompt with a blinking
// block cursor. Enter goes to the shop; q quits.
func (s Splash) prompt() string {
	cur := " "
	if (s.frame/9)%2 == 0 { // ~1s blink, matched to the rest of the app
		cur = "▉"
	}
	return strings.Repeat(" ", promptIndent) +
		theme.BrandStyle.Render("console.store") + " " +
		theme.CursorStyle.Render("▸") + " " +
		theme.DimStyle.Render("press ↵ to enter") +
		theme.CursorStyle.Render(cur) +
		theme.FaintStyle.Render("    ·  q quit")
}

func (s Splash) View() string { return s.view() }

func (s Splash) view() string {
	ind := strings.Repeat(" ", bodyIndent)

	// Decode phase: the wordmark glitch-resolves under the ssh prompt, as if the
	// login banner is materialising. The Kitty graphics path renders a bitmap
	// that can't be glyph-decoded (and would break width math), so it skips
	// straight to the settled banner.
	if s.decodeStep < render.DecodeSteps && !(s.caps.KittyGraphics && render.KittyFlag) {
		art := render.DecodeWordmark(s.caps, s.decodeStep, s.frame)
		artLines := strings.Split(strings.TrimRight(art, "\n"), "\n")
		lines := []string{sshLine(), "", ""}
		for _, l := range artLines {
			lines = append(lines, ind+l)
		}
		lines = append(lines, "", tagline())
		padRight(lines, blockWidth(lines))
		return strings.Join(lines, "\n")
	}

	// The wordmark carries the signature light-sweep shimmer, recomputed each
	// frame. The Kitty PNG path can't be per-column re-tinted (and re-encoding a
	// bitmap every tick would be wasteful), so it uses the cached static bloom.
	var logo string
	if s.caps.KittyGraphics && render.KittyFlag {
		logo = s.logoCache
		if logo == "" { // defensive: WithCaps not called (e.g. bare NewSplash)
			logo = render.Logo(s.caps, 64)
		}
	} else {
		// Phase the sweep so its first pass starts at the left right as the
		// reveal finishes (splashTick == DecodeSteps -> shimmer frame 0).
		sweep := s.splashTick - render.DecodeSteps
		if sweep < 0 {
			sweep = 0
		}
		logo = render.ShimmerWordmark(s.caps, sweep)
	}
	logoLines := strings.Split(strings.TrimRight(logo, "\n"), "\n")
	// Gold ".store" rides the wordmark's baseline — one accent, inline.
	if n := len(logoLines); n > 0 {
		logoLines[n-1] = logoLines[n-1] + "   " + theme.GoldStyle.Render(".store")
	}

	lines := []string{sshLine(), "", ""}
	for _, l := range logoLines {
		lines = append(lines, ind+l)
	}
	lines = append(lines, "", tagline(), "", "", s.prompt())
	padRight(lines, blockWidth(lines))
	return strings.Join(lines, "\n")
}
