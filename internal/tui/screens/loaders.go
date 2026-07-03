package screens

// Loading scenes — the in-between moments. Every network wait renders ONE
// consistent, minimal animation: a light traveling across a row of dots
// (size + color falloff reads as motion blur), with a single rotating line
// of copy beneath it, both centered in the pane that's waiting. No
// figurative ASCII — just clean geometry on the Tokyo Night ramp, driven by
// the app's one global 60ms tick.
//
// Late at night (23:00–04:59) the copy swaps to gentle go-to-sleep taunts —
// same cadence, different personality. Pickers are deterministic in
// (frame, hour) so tests can pin them.

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"consolestore/internal/tui/theme"
)

// copyEvery is how many ticks each rotating copy line holds (~1.6s at 60ms) —
// slow enough to read, fast enough that a long load doesn't feel stuck.
const copyEvery = 27

// IsLateNight reports whether hour (0–23) falls in the go-to-sleep window.
func IsLateNight(hour int) bool { return hour >= 23 || hour < 5 }

// Rotating copy sets. Day sets sell the wait; night sets tease the dev.
var (
	foodLines = []string{
		"warming the tandoor…",
		"asking the chef what's good…",
		"plating the menu…",
		"tasting for salt…",
	}
	foodNightLines = []string{
		"the kitchen's awake. you shouldn't be…",
		"midnight cravings detected…",
		"carbs now, bugs tomorrow…",
		"order it, eat it, then sleep. deal?",
	}
	imLines = []string{
		"sprinting the aisles…",
		"bagging your basket…",
		"racing the 10-minute promise…",
		"checking the back shelf…",
	}
	imNightLines = []string{
		"late-night grocery run — zero judgment…",
		"restocking your willpower…",
		"snacks incoming. then bed. promise?",
	}
	cartLines = []string{
		"fetching your cart from swiggy…",
		"counting what you picked…",
	}
)

// loadingLine picks the rotating copy: day/night set by hour, line by frame.
func loadingLine(day, night []string, frame, hour int) string {
	set := day
	if IsLateNight(hour) && len(night) > 0 {
		set = night
	}
	return set[(frame/copyEvery)%len(set)]
}

// pulse renders the traveling-light row: pulseN dots, one bright peak
// ping-ponging end to end, neighbours falling off in glyph size and tone —
// terminal motion blur. One step every 2 ticks (~8 sweeps a minute).
const pulseN = 9

func pulse(frame int) string {
	// Triangle wave over 2(n-1) steps: 0,1,…,n-1,n-2,…,1 — the ping-pong.
	step := (frame / 2) % (2 * (pulseN - 1))
	peak := step
	if peak >= pulseN {
		peak = 2*(pulseN-1) - step
	}
	cells := make([]string, pulseN)
	for i := range cells {
		d := i - peak
		if d < 0 {
			d = -d
		}
		switch d {
		case 0:
			cells[i] = theme.GoldStyle.Render("●")
		case 1:
			cells[i] = theme.TextStyle.Render("•")
		case 2:
			cells[i] = theme.DimStyle.Render("∙")
		default:
			cells[i] = theme.FaintStyle.Render("·")
		}
	}
	return strings.Join(cells, " ")
}

// centerTo left-pads s so its display width sits centered in w.
func centerTo(s string, w int) string {
	if pad := (w - lipgloss.Width(s)) / 2; pad > 0 {
		return strings.Repeat(" ", pad) + s
	}
	return s
}

// LoadingScene renders the standard wait state: the pulse over one line of
// rotating copy, horizontally centered in w columns and vertically centered
// in budget rows (0 = no vertical centering). Every loading state in the app
// goes through here so the waits all speak with one voice.
func LoadingScene(frame, hour, w, budget int, day, night []string) string {
	block := []string{
		centerTo(pulse(frame), w),
		"",
		centerTo(theme.DimStyle.Render(loadingLine(day, night, frame, hour)), w),
	}
	var b strings.Builder
	if above := (budget - len(block)) / 2; above > 0 {
		b.WriteString(strings.Repeat("\n", above))
	}
	b.WriteString(strings.Join(block, "\n"))
	b.WriteString("\n")
	return b.String()
}

// FoodLoading / IMLoading are the two verticals' scenes: same animation,
// their own copy. w is the waiting pane's width, budget its row count.
func FoodLoading(frame, hour, w, budget int) string {
	return LoadingScene(frame, hour, w, budget, foodLines, foodNightLines)
}

func IMLoading(frame, hour, w, budget int) string {
	return LoadingScene(frame, hour, w, budget, imLines, imNightLines)
}

// CartLoading renders the checkout's cart-fetch state: shown instead of
// "your cart is empty" while the live cart is still in flight, so an empty
// flash never lies about a cart that's about to arrive.
func CartLoading(frame, hour, w int) string {
	var b strings.Builder
	b.WriteString("\n\n")
	b.WriteString(LoadingScene(frame, hour, w, 0, cartLines, nil))
	if IsLateNight(hour) {
		b.WriteString("\n" + centerTo(theme.FaintStyle.Render("(it's late — order and log off, yeah?)"), w) + "\n")
	}
	b.WriteString("\n")
	return b.String()
}

// NightHint returns a status-bar go-to-sleep taunt for the given frame, or ""
// outside the late-night window. Rotates on the same cadence as statusHints.
func NightHint(frame, hour int) string {
	if !IsLateNight(hour) {
		return ""
	}
	h := hour
	if h == 0 {
		h = 12
	}
	lines := []string{
		fmt.Sprintf("it's %d am — ship, eat, sleep", h),
		"the terminal never sleeps. you should",
		"one order, then bed ♥",
		"git commit, then goodnight",
	}
	if hour == 23 {
		lines[0] = "it's 11 pm — ship, eat, sleep"
	}
	return lines[(frame/27)%len(lines)]
}
