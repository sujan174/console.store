package screens

import (
	"strings"
	"testing"

	"consolestore/internal/tui/render"
)

// TestWelcomePhase0OrderStory verifies the three beats of phase 0: the typed
// command, the working pulse, and the "on its way" settle before hand-off.
func TestWelcomePhase0OrderStory(t *testing.T) {
	w := NewWelcome(DefaultLearnURL).WithCaps(render.Caps{})

	// Beat 1 — enough ticks to fully type the command; nothing else yet.
	early := w.WithTick(typeDone).WithPhase(0).View()
	if !strings.Contains(early, "console order dinner") {
		t.Fatalf("phase-0 view must show the typed command; got:\n%s", early)
	}
	if strings.Contains(early, "●") {
		t.Fatalf("the pulse must not run before workTick; got:\n%s", early)
	}

	// Beat 2 — the pulse works under the command; no settle line yet.
	working := w.WithTick(workTick + 3).WithPhase(0).View()
	if !strings.Contains(working, "●") {
		t.Fatalf("working beat must show the pulse; got:\n%s", working)
	}
	if strings.Contains(working, "on its way") {
		t.Fatalf("the settle line must not land before stampTick; got:\n%s", working)
	}

	// Beat 3 — the settle line before hand-off; the pulse keeps breathing.
	late := w.WithTick(WelcomeAnimEnd).WithPhase(0).View()
	if !strings.Contains(late, "dinner is on its way") {
		t.Fatalf("phase-0 view must settle on the dispatch line; got:\n%s", late)
	}
	if !strings.Contains(late, "●") {
		t.Fatalf("the pulse must keep breathing through the settle; got:\n%s", late)
	}
}

// TestWelcomePhase1Card verifies the intro card copy, URL, and key hints.
func TestWelcomePhase1Card(t *testing.T) {
	const url = "https://example.test/guide"
	v := NewWelcome(url).WithCaps(render.Caps{}).WithPhase(1).View()

	for _, want := range []string{
		"welcome to consolestore",
		"a terminal-native food shop.",
		"place real orders without",
		"orders run live through Swiggy.",
		"Tokyo Night themed, keyboard-first.",
		"learn how to use it:",
		url,
		"[L]",
		"open guide",
		"[↵]",
		"continue",
	} {
		if !strings.Contains(v, want) {
			t.Fatalf("phase-1 card missing %q; got:\n%s", want, v)
		}
	}

	// The URL must be wrapped in an OSC 8 hyperlink escape.
	if !strings.Contains(v, "\x1b]8;;"+url+"\x1b\\") {
		t.Fatal("phase-1 card must wrap the URL in an OSC 8 hyperlink")
	}
}

// TestWelcomeDefaultURL verifies an empty URL falls back to DefaultLearnURL.
func TestWelcomeDefaultURL(t *testing.T) {
	w := NewWelcome("")
	if w.LearnURL() != DefaultLearnURL {
		t.Fatalf("empty URL must default to %q, got %q", DefaultLearnURL, w.LearnURL())
	}
	if !strings.Contains(w.WithPhase(1).View(), DefaultLearnURL) {
		t.Fatal("phase-1 card must show the default learn URL")
	}
}

// TestWelcomeBuildersReturnCopies verifies the With* builders are value-copy
// (immutable) and don't mutate the receiver.
func TestWelcomeBuildersReturnCopies(t *testing.T) {
	base := NewWelcome(DefaultLearnURL)

	if got := base.WithPhase(1); base.Phase() != 0 || got.Phase() != 1 {
		t.Fatalf("WithPhase must return a copy: base=%d got=%d", base.Phase(), got.Phase())
	}

	tickd := base.WithTick(42)
	if base.tick != 0 || tickd.tick != 42 {
		t.Fatalf("WithTick must return a copy: base=%d got=%d", base.tick, tickd.tick)
	}

	framed := base.WithFrame(7)
	if base.frame != 0 || framed.frame != 7 {
		t.Fatalf("WithFrame must return a copy: base=%d got=%d", base.frame, framed.frame)
	}

	capped := base.WithCaps(render.Caps{Truecolor: true})
	if base.caps.Truecolor || !capped.caps.Truecolor {
		t.Fatal("WithCaps must return a copy")
	}
}
