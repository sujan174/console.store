package screens

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"console.store/internal/tui/render"
	"console.store/internal/tui/theme"
)

// HomeItems returns the ordered list of home menu items depending on whether an
// order is currently live. When hasOrder is true, a "track order" entry is
// inserted between "go to shop" (index 0) and "settings" (index 2).
func HomeItems(hasOrder bool) []string {
	if hasOrder {
		return []string{"go to shop", "track order", "settings"}
	}
	return []string{"go to shop", "settings"}
}

// IsSettings reports whether home item i is the settings entry, given the
// current layout: 2-item (no active order, settings = 1) or 3-item (active
// order, settings = 2).
func IsSettings(i int, hasOrder bool) bool {
	items := HomeItems(hasOrder)
	return i >= 0 && i < len(items) && items[i] == "settings"
}

// IsTrack reports whether home item i is the "track order" entry (index 1 in the
// 3-item layout). Always false when hasOrder is false.
func IsTrack(i int, hasOrder bool) bool {
	if !hasOrder {
		return false
	}
	items := HomeItems(true)
	return i >= 0 && i < len(items) && items[i] == "track order"
}

// Indentation: the splash reads as a real terminal session. Prompt lines sit at
// the left gutter; the banner (wordmark + tagline) is inset one step further so
// it stands apart as the login banner. The whole block is centred in the
// viewport by the root view, so these are relative to the block's left edge.
const (
	promptIndent = 2
	bodyIndent   = 4
)

type Splash struct {
	decodeStep int    // decode progress (0..render.DecodeSteps)
	frame      int    // global animation frame (decode flicker + prompt cursor blink)
	splashTick int    // ticks since the splash was (re)entered; phases the shimmer
	sel        int    // selected home item (seam for future multi-item home)
	phrase     string // Minecraft-style splash line shown by the wordmark
	orderLabel string // non-empty when an order is live (e.g. "Blue Tokai · ~12 min")
	caps       render.Caps
	logoCache  string // render.Logo is constant per session; computed once here
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

// WithPhrase sets the Minecraft-style splash line shown by the wordmark.
func (s Splash) WithPhrase(p string) Splash { s.phrase = p; return s }

// WithOrder sets a live-order label (e.g. "Blue Tokai · ~12 min"). An empty
// label means no active order — the splash renders the standard 2-item layout.
func (s Splash) WithOrder(label string) Splash { s.orderLabel = label; return s }

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

// phraseBox renders the Minecraft-style splash line inside a dotted border whose
// bright "sparks" march around the perimeter (driven by frame), right-aligned to
// the wordmark's right edge so it hugs the top-right of CONSOLE without widening
// the centred block. Returns the three fully-indented lines, or nil when there
// is no phrase.
func phraseBox(phrase string, frame, wmW int) []string {
	if phrase == "" {
		return nil
	}
	w := lipgloss.Width(phrase) + 4 // text + a space of padding + a border cell each side

	dim := theme.Fg("#8a6d3b")               // muted gold — the resting dots
	bright := theme.Fg("#f9d99a").Bold(true) // the travelling sparks

	const gap = 4     // one bright spark every `gap` cells
	step := frame / 3 // advance ~every 180ms so the sparks march, not strobe

	// cell renders the border glyph at clockwise perimeter index i: a bright
	// star on the marching beat, otherwise a dim dot.
	cell := func(i int) string {
		if ((i+step)%gap+gap)%gap == 0 {
			return bright.Render("*")
		}
		return dim.Render("·")
	}

	// Clockwise perimeter indices: top 0..w-1, right side = w, bottom (right to
	// left) = w+1..2w, left side = 2w+1.
	var top, bot strings.Builder
	for c := 0; c < w; c++ {
		top.WriteString(cell(c))
		bot.WriteString(cell(2*w - c))
	}
	mid := cell(2*w+1) + " " + theme.Fg(theme.Gold).Bold(true).Render(phrase) + " " + cell(w)

	pad := wmW - w
	if pad < 0 {
		pad = 0
	}
	lead := strings.Repeat(" ", bodyIndent+pad)
	return []string{lead + top.String(), lead + mid, lead + bot.String()}
}

// storeBlock renders the gold "STORE" block-art (with sweeping shimmer),
// right-aligned to consoleW so it sits flush under CONSOLE's right edge.
func storeBlock(caps render.Caps, frame, consoleW int) []string {
	rows := strings.Split(strings.TrimRight(render.ShimmerStore(caps, frame), "\n"), "\n")
	sw := 0
	for _, r := range rows {
		if w := lipgloss.Width(r); w > sw {
			sw = w
		}
	}
	pad := consoleW - sw
	if pad < 0 {
		pad = 0
	}
	lead := strings.Repeat(" ", pad)
	for i, r := range rows {
		rows[i] = lead + r
	}
	return rows
}

// letterspace inserts a single space between runes, giving short labels a
// larger, more deliberate feel in the fixed terminal grid.
func letterspace(s string) string {
	parts := make([]string, 0, len(s))
	for _, r := range s {
		parts = append(parts, string(r))
	}
	return strings.Join(parts, " ")
}

// sshLine is the top prompt — the command the user just "ran" to arrive here.
func sshLine() string {
	return strings.Repeat(" ", promptIndent) +
		theme.DimStyle.Render("~ % ssh ") + theme.TextStyle.Render("consolestore.in")
}

// tagline is the banner's one-line descriptor, inset under the wordmark.
func tagline() string {
	return strings.Repeat(" ", bodyIndent) + theme.DimStyle.Render("coffee · food · quick snacks")
}

// prompt is the settled call-to-action: a live shell prompt with a blinking
// block cursor. Enter goes to the shop; q quits.
// When orderLabel is non-empty a gold "track order · {label}" row is inserted
// between start and settings (sel 0 = start, sel 1 = track, sel 2 = settings).
// splashBtn renders a home-menu item as a blue "button" that matches the
// menu/restaurant selected-row look: a blue ▌ bar + selected-row background +
// bright text when focused; a dim, aligned label otherwise.
func splashBtn(label string, focused bool) string {
	if focused {
		return theme.CursorStyle.Render("▌") + theme.SelRowStyle.Render(" "+label+" ")
	}
	// Two-cell lead keeps idle labels aligned under the focused button's label.
	return "  " + theme.DimStyle.Render(label)
}

func (s Splash) prompt() string {
	ind := strings.Repeat(" ", promptIndent)

	// All home items are left-aligned blue buttons at the same column.
	// line 1 — start the shop (+ a faint quit hint).
	start := ind + splashBtn("press ↵ to enter", s.sel == 0) +
		theme.FaintStyle.Render("    ·  q quit")

	// Settings index depends on whether the track row is present.
	settingsSel := 1
	if s.orderLabel != "" {
		settingsSel = 2
	}

	lines := []string{start}

	// line 2 (optional) — track order (same blue button family).
	if s.orderLabel != "" {
		lines = append(lines, ind+splashBtn("track order · "+s.orderLabel, s.sel == 1))
	}

	// final line — settings.
	lines = append(lines, ind+splashBtn("settings", s.sel == settingsSel))

	return strings.Join(lines, "\n")
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
		// STORE rides along under CONSOLE so the banner height is stable through
		// the reveal (no jump when it settles).
		for _, l := range storeBlock(s.caps, s.frame, blockWidth(artLines)) {
			lines = append(lines, ind+l)
		}
		lines = append(lines, "", tagline())
		padRight(lines, blockWidth(lines))
		return strings.Join(lines, "\n")
	}

	// The wordmark carries the signature light-sweep shimmer, recomputed each
	// frame. The Kitty PNG path can't be per-column re-tinted (and re-encoding a
	// bitmap every tick would be wasteful), so it uses the cached static bloom.
	// Phase the sweep so its first pass starts at the left right as the reveal
	// finishes (splashTick == DecodeSteps -> shimmer frame 0). Shared by CONSOLE
	// and the STORE block below so their sheens travel in sync.
	sweep := s.splashTick - render.DecodeSteps
	if sweep < 0 {
		sweep = 0
	}
	var logo string
	if s.caps.KittyGraphics && render.KittyFlag {
		logo = s.logoCache
		if logo == "" { // defensive: WithCaps not called (e.g. bare NewSplash)
			logo = render.Logo(s.caps, 64)
		}
	} else {
		logo = render.ShimmerWordmark(s.caps, sweep)
	}
	logoLines := strings.Split(strings.TrimRight(logo, "\n"), "\n")
	// Gold "STORE" block-art sits under CONSOLE (right-aligned), with the same
	// sweeping shimmer — the full brand reads CONSOLESTORE.
	logoLines = append(logoLines, storeBlock(s.caps, sweep, blockWidth(logoLines))...)

	// The bordered splash phrase hugs the top-right of the wordmark
	// (Minecraft-style), appearing once the logo has settled. A blank line below
	// it keeps it clear of CONSOLE.
	box := phraseBox(s.phrase, s.frame, blockWidth(logoLines))

	lines := []string{sshLine(), ""}
	if len(box) > 0 {
		lines = append(lines, box...)
	}
	lines = append(lines, "")
	for _, l := range logoLines {
		lines = append(lines, ind+l)
	}
	lines = append(lines, "", tagline(), "", "", s.prompt())
	padRight(lines, blockWidth(lines))
	return strings.Join(lines, "\n")
}
