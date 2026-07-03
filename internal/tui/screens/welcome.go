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
const welcomeCmd = "console order dinner"

// Phase-0 animation tick thresholds (60ms/tick). Three beats, kept minimal:
// the command types out at the prompt; the loading pulse "works" beneath it
// for a moment; the ✓ settle line lands. onTick auto-advances to phase 1 at
// animEnd. All thresholds are in ticks.
const (
	typeEvery = 2                                     // one char every ~120ms
	typeDone  = len(welcomeCmd)*typeEvery + typeEvery // command fully typed
	workTick  = typeDone + 5                          // beat, then the pulse "places the order"
	stampTick = workTick + 34                         // ~2s of work → the settle line
	animEnd   = stampTick + 25                        // hold, then hand off to the card
)

// WelcomeAnimEnd is the phase-0 tick at which the food animation is finished and
// the root should auto-advance to the intro card (phase 1).
const WelcomeAnimEnd = animEnd

// welcomeW is the width the phase-0 beats center against (≈ the typed line).
const welcomeW = 24

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

// foodView renders phase 0 — three minimal beats, centered as one block by
// the root:
//
//	~ % console order dinner     (typewriter)
//	· ∙ • ● • ∙ · · ·            (the pulse "places the order")
//	✓ dinner is on its way       (settle, hand off to the card)
func (w Welcome) foodView() string {
	cur := "█"
	if !w.blink() {
		cur = " "
	}

	// Beat 1 — typewriter: reveal one char of the command every typeEvery ticks.
	shown := w.tick / typeEvery
	if shown > len(welcomeCmd) {
		shown = len(welcomeCmd)
	}
	typed := welcomeCmd[:shown]

	prompt := theme.PurpleStyle.Render("~ % ") + theme.BrightStyle.Render(typed)
	committed := w.tick >= typeDone
	if !committed {
		prompt += theme.CursorStyle.Render(cur)
	}

	lines := []string{prompt, ""}

	// Beat 2 — the order "places": the same traveling-light pulse every wait
	// in the app uses, centered under the command. It keeps breathing through
	// the settle so the block never freezes.
	if w.tick >= workTick {
		lines = append(lines, centerTo(pulse(w.frame), welcomeW))
	}

	// Beat 3 — the settle line: order away, welcome in.
	if w.tick >= stampTick {
		lines = append(lines, "",
			centerTo(theme.GreenStyle.Render("✓ ")+theme.BrightStyle.Render("dinner is on its way"), welcomeW))
	}

	return strings.Join(lines, "\n")
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
