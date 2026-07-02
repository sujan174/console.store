package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	swiggysnap "consolestore/internal/catalog/swiggy"
	"consolestore/internal/localstore"
	"consolestore/internal/tui/render"
)

// TestFirstRunWelcomeBeforeAuthGate is the regression test for the fresh-install
// bug where the pending-auth gate (needsAuth) short-circuited both View and key
// routing, hiding the welcome walkthrough and auto-opening the browser at launch.
//
// A real fresh install sets BOTH WithOnboarding (screen=scrWelcome) and
// WithPendingAuth (needsAuth=true). The welcome screen must own the viewport
// first; the browser must NOT auto-open; and only after the walkthrough is
// dismissed does the connect gate appear, where an explicit Enter opens the
// browser.
func TestFirstRunWelcomeBeforeAuthGate(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{},
		WithLiveBackend(&liveFake{}, snap, "local", "https://authz/x"),
		WithPendingAuth(),
		WithOnboarding(true),
	)
	m.w, m.h = 80, 24

	if m.screen != scrWelcome {
		t.Fatalf("fresh install must start on the welcome screen, got screen %d", m.screen)
	}
	if !m.needsAuth {
		t.Fatal("precondition: needsAuth should be true on a fresh install")
	}

	// While scrWelcome is active the connect gate must NOT pre-empt the view.
	if v := m.View(); strings.Contains(v, "connect swiggy") {
		t.Fatalf("welcome screen must own the viewport, not the connect gate\n%s", v)
	}

	// Any key skips the animation to the intro card; the card is shown, not the gate.
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = out.(Model)
	if v := m.View(); !strings.Contains(v, "welcome to consolestore") || strings.Contains(v, "connect swiggy") {
		t.Fatalf("expected the intro card, not the connect gate\n%s", v)
	}

	// Enter on the card dismisses onboarding → the connect gate becomes the start
	// screen (needsAuth still true), and the first-run marker is written.
	out, markerCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = out.(Model)
	if markerCmd != nil {
		markerCmd() // runs localstore.MarkOnboarded()
	}
	if m.screen != scrSplash {
		t.Fatalf("dismissing the card should drop to the splash, got screen %d", m.screen)
	}
	if !m.needsAuth {
		t.Fatal("connect gate should still be pending after the walkthrough")
	}
	m.decodeStep = render.DecodeSteps // settle the boot banner so the button renders
	if v := m.View(); !strings.Contains(v, "connect swiggy") {
		t.Fatalf("after the walkthrough the connect gate must be the start screen\n%s", v)
	}

	// Only NOW does Enter open the browser.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on the connect gate should open the browser")
	}
	if _, ok := cmd().(browserOpenedMsg); !ok {
		t.Fatal("Enter on the connect gate should fire the browser-open command")
	}

	// The walkthrough was completed, so the first-run marker must be written.
	if !localstore.Onboarded() {
		t.Fatal("dismissing the welcome card should write the onboarding marker")
	}
}
