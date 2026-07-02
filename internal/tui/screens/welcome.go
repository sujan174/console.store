package screens

import (
	"strings"

	"consolestore/internal/tui/render"
	"consolestore/internal/tui/theme"
)

// DefaultLearnURL is the guide URL shown on the welcome intro card. It is a
// placeholder that will be swapped for the real learn page later.
const DefaultLearnURL = "https://consolestore.in/how-to"

// welcomeCmd is the command the phase-0 typewriter "types" at the shell prompt.
const welcomeCmd = "console order ramen"

// Phase-0 animation tick thresholds (60ms/tick). The command types out one
// character every typeEvery ticks; once typed it holds for a beat, then the
// bowl reveals line-by-line, and the whole animation settles by animEnd — after
// which onTick auto-advances to phase 1. All thresholds are in ticks.
const (
	typeEvery      = 2                                           // one char every ~120ms
	typeDone       = len(welcomeCmd)*typeEvery + typeEvery       // command fully typed
	bowlRevealTick = typeDone + 8                                // ~0.5s pause, then the bowl starts revealing
	bowlPerLine    = 2                                           // ticks between revealed bowl rows
	bowlLines      = 6                                           // == len(bowlArt); kept const so animEnd is const
	animEnd        = bowlRevealTick + bowlLines*bowlPerLine + 20 // hold, then hand off to the card
)

// WelcomeAnimEnd is the phase-0 tick at which the food animation is finished and
// the root should auto-advance to the intro card (phase 1).
const WelcomeAnimEnd = animEnd

// bowlArt is the ramen bowl, revealed line-by-line during phase 0. Steam wisps
// above it (rendered separately, animated) are NOT part of this slice.
var bowlArt = []string{
	`     .-"""""""""""-.`,
	`    /  ~~~~~~~~~~~  \`,
	`   |   ( noodles )   |====`,
	`    \  ~~~~~~~~~~~  /`,
	`     '-._______.-'`,
	`        \_____/`,
}

// keep bowlLines in sync with bowlArt (const needed for animEnd).
var _ = [1]struct{}{}[len(bowlArt)-bowlLines]

// Welcome is the first-run onboarding screen: a short ramen food animation
// (phase 0) that gives way to a welcome/intro card (phase 1). It is a passive
// value type driven by the root model, mirroring Splash.
type Welcome struct {
	caps     render.Caps
	frame    int // global animation frame (cursor blink + steam)
	tick     int // ticks since welcome entry (phase-0 animation progress)
	phase    int // 0 = food animation, 1 = intro card
	learnURL string
	w, h     int // viewport (the root Places this centered; kept for parity)
}

// NewWelcome builds the welcome screen with the given guide URL.
func NewWelcome(url string) Welcome {
	if url == "" {
		url = DefaultLearnURL
	}
	return Welcome{learnURL: url}
}

// WithCaps sets the terminal capabilities.
func (w Welcome) WithCaps(c render.Caps) Welcome { w.caps = c; return w }

// WithFrame sets the global animation frame (cursor blink + steam phase).
func (w Welcome) WithFrame(f int) Welcome { w.frame = f; return w }

// WithTick sets ticks-since-entry, which drives the phase-0 animation.
func (w Welcome) WithTick(n int) Welcome { w.tick = n; return w }

// WithPhase sets the phase (0 = animation, 1 = intro card).
func (w Welcome) WithPhase(p int) Welcome { w.phase = p; return w }

// WithViewport records the viewport size (kept for parity with Splash; the root
// centers the block, so this is currently advisory).
func (w Welcome) WithViewport(width, height int) Welcome { w.w, w.h = width, height; return w }

// Phase reports the current phase.
func (w Welcome) Phase() int { return w.phase }

// LearnURL returns the guide URL.
func (w Welcome) LearnURL() string { return w.learnURL }

func (w Welcome) View() string {
	if w.phase == 0 {
		return w.foodView()
	}
	return w.cardView()
}

// blink reports whether the block cursor is in its "on" phase this frame.
func (w Welcome) blink() bool { return (w.frame/8)%2 == 0 }

// foodView renders phase 0: a shell prompt typing out `console order ramen`,
// then a steaming ramen bowl that reveals line-by-line.
func (w Welcome) foodView() string {
	cur := "█"
	if !w.blink() {
		cur = " "
	}

	// Typewriter: reveal one char of the command every typeEvery ticks.
	shown := w.tick / typeEvery
	if shown > len(welcomeCmd) {
		shown = len(welcomeCmd)
	}
	typed := welcomeCmd[:shown]

	prompt := theme.PurpleStyle.Render("~ % ") + theme.BrightStyle.Render(typed)
	// While typing, the cursor trails the text; once committed it drops away and
	// the bowl takes over below.
	committed := w.tick >= typeDone
	if !committed {
		prompt += theme.CursorStyle.Render(cur)
	}

	lines := []string{prompt, ""}

	if committed {
		// Steam wisps rise above the bowl, animated off frame. Two alternating
		// rows of offset glyphs read as wisps drifting upward.
		lines = append(lines, w.steam()...)

		// Bowl reveals line-by-line after the pause.
		revealed := 0
		if w.tick >= bowlRevealTick {
			revealed = (w.tick - bowlRevealTick) / bowlPerLine
		}
		if revealed > len(bowlArt) {
			revealed = len(bowlArt)
		}
		for i := 0; i < revealed; i++ {
			lines = append(lines, w.styleBowl(bowlArt[i]))
		}
	}

	return strings.Join(lines, "\n")
}

// steam renders two rows of rising steam wisps above the bowl. The glyphs shift
// horizontally with the frame so the wisps appear to drift upward.
func (w Welcome) steam() []string {
	off := (w.frame / 6) % 3
	pad := strings.Repeat(" ", 8+off)
	pad2 := strings.Repeat(" ", 7+((off+1)%3))
	top := pad + theme.DimStyle.Render(")   )   )")
	bot := pad2 + theme.DimStyle.Render("(   (   (")
	// Alternate which wisp row leads so the steam looks like it's rising.
	if (w.frame/12)%2 == 0 {
		return []string{top, bot}
	}
	return []string{bot, top}
}

// styleBowl colors a single bowl row: noodles in gold, ~ broth waves in green,
// the bowl body in the default text tone.
func (w Welcome) styleBowl(line string) string {
	switch {
	case strings.Contains(line, "noodles"):
		// Split so "noodles" reads gold against the text-toned rim.
		i := strings.Index(line, "noodles")
		return theme.TextStyle.Render(line[:i]) +
			theme.GoldStyle.Render("noodles") +
			theme.TextStyle.Render(line[i+len("noodles"):])
	case strings.Contains(line, "~"):
		return theme.GreenStyle.Render(line)
	default:
		return theme.TextStyle.Render(line)
	}
}

// cardView renders phase 1: the welcome/intro card.
func (w Welcome) cardView() string {
	url := w.learnURL
	if url == "" {
		url = DefaultLearnURL
	}
	// OSC 8 hyperlink: clickable in capable terminals, raw URL as the fallback
	// text. Colored links-back blue.
	link := theme.CursorStyle.Render(osc8(url, url))

	lines := []string{
		theme.BrandStyle.Render("welcome to consolestore"),
		"",
		theme.TextStyle.Render("a terminal-native food shop. no browser, no app —"),
		theme.TextStyle.Render("browse, build a cart, and place real orders without"),
		theme.TextStyle.Render("leaving your shell. orders run live through Swiggy."),
		theme.TextStyle.Render("Tokyo Night themed, keyboard-first."),
		"",
		theme.DimStyle.Render("learn how to use it:"),
		link,
		"",
		theme.HintKeyStyle.Render("[L]") + theme.DimStyle.Render(" open guide     ") +
			theme.HintKeyStyle.Render("[↵]") + theme.DimStyle.Render(" continue"),
	}
	return strings.Join(lines, "\n")
}

// osc8 wraps text in an OSC 8 hyperlink escape so capable terminals make it
// clickable, with text as the visible fallback.
func osc8(url, text string) string {
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}
