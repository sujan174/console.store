package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"consolestore/internal/localstore"
	"consolestore/internal/tui/render"
	"consolestore/internal/tui/screens"
)

// advanceThroughSplash drives the model from scrSplash to scrMenu by ticking
// through the decode animation and then sending enter to activate "go to shop".
func advanceThroughSplash(m Model) Model {
	// Tick until decode is complete.
	for m.decodeStep < render.DecodeSteps {
		u, _ := m.Update(tickMsg(time.Now()))
		m = u.(Model)
	}
	// Enter activates the highlighted home item (defaults to "go to shop").
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	return m
}

// drainCmd runs a tea.Cmd to completion (single-message cmds only) so its side
// effects (e.g. MarkOnboarded's disk write) happen synchronously in the test.
func drainCmd(cmd tea.Cmd) {
	if cmd != nil {
		_ = cmd()
	}
}

// TestOnboardingStartsOnWelcome verifies that WithOnboarding(true) starts the
// session on the welcome screen (phase 0, the food animation) — NOT the splash,
// and never auto-opens the help modal.
func TestOnboardingStartsOnWelcome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := New(render.Caps{}, WithOnboarding(true))
	m.w, m.h = 100, 40

	if m.screen != scrWelcome {
		t.Fatalf("WithOnboarding(true): must start on scrWelcome, got screen %d", m.screen)
	}
	if m.welcome.Phase() != 0 {
		t.Fatalf("welcome must start in phase 0 (animation), got %d", m.welcome.Phase())
	}
	if m.helpOpen {
		t.Fatal("onboarding must NOT auto-open the help modal")
	}
	if !m.wantOnboarding {
		t.Fatal("wantOnboarding must be set while onboarding is in progress")
	}
}

// TestOnboardingTickAdvancesToCard verifies the food animation auto-advances to
// the intro card (phase 1) once enough ticks elapse, without ever opening help.
func TestOnboardingTickAdvancesToCard(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := New(render.Caps{}, WithOnboarding(true))
	m.w, m.h = 100, 40

	for i := 0; i < screens.WelcomeAnimEnd+2; i++ {
		u, _ := m.Update(tickMsg(time.Now()))
		m = u.(Model)
	}

	if m.welcome.Phase() != 1 {
		t.Fatalf("welcome must reach phase 1 after WelcomeAnimEnd ticks, got %d", m.welcome.Phase())
	}
	if m.screen != scrWelcome {
		t.Fatalf("still on the welcome screen until Enter, got screen %d", m.screen)
	}
	if m.helpOpen {
		t.Fatal("help must NOT open during the onboarding animation")
	}
}

// TestOnboardingKeySkipsToCard verifies pressing any key during phase 0 skips
// the animation straight to the intro card (phase 1).
func TestOnboardingKeySkipsToCard(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := New(render.Caps{}, WithOnboarding(true))
	m.w, m.h = 100, 40

	if m.welcome.Phase() != 0 {
		t.Fatal("precondition: welcome starts in phase 0")
	}

	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = u.(Model)

	if m.welcome.Phase() != 1 {
		t.Fatalf("any key in phase 0 must skip to phase 1, got %d", m.welcome.Phase())
	}
	if m.screen != scrWelcome {
		t.Fatalf("skipping must stay on the welcome screen, got screen %d", m.screen)
	}
	if m.helpOpen {
		t.Fatal("the skip key must NOT open help")
	}
}

// TestOnboardingEnterWritesMarkerAndGoesToSplash verifies pressing Enter on the
// intro card transitions to the splash, writes the first-run marker, and clears
// wantOnboarding — with no help modal opened anywhere in the flow.
func TestOnboardingEnterWritesMarkerAndGoesToSplash(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if localstore.Onboarded() {
		t.Fatal("precondition: fresh config must not be onboarded")
	}

	m := New(render.Caps{}, WithOnboarding(true))
	m.w, m.h = 100, 40

	// Skip the animation to the card, then press Enter.
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = u.(Model)
	if m.welcome.Phase() != 1 {
		t.Fatal("precondition: on the intro card")
	}

	u, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)

	if m.screen != scrSplash {
		t.Fatalf("Enter on the intro card must go to scrSplash, got screen %d", m.screen)
	}
	if m.wantOnboarding {
		t.Fatal("wantOnboarding must be cleared after dismissing onboarding")
	}
	if m.helpOpen {
		t.Fatal("dismissing onboarding must NOT open the help modal")
	}
	if m.decodeStep != 0 || m.splashTick != 0 {
		t.Fatal("splash boot must be reset so the wordmark plays fresh")
	}
	if cmd == nil {
		t.Fatal("dismissing onboarding must return the MarkOnboarded cmd")
	}
	drainCmd(cmd)
	if !localstore.Onboarded() {
		t.Fatal("the onboarding marker must be written to disk after Enter")
	}
}

// TestOnboardingFalseStartsOnSplash verifies WithOnboarding(false) leaves the
// session on the splash (returning-user behaviour), never on the welcome screen.
func TestOnboardingFalseStartsOnSplash(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := New(render.Caps{}, WithOnboarding(false))
	m.w, m.h = 100, 40

	if m.screen != scrSplash {
		t.Fatalf("WithOnboarding(false): must start on scrSplash, got screen %d", m.screen)
	}
	m = advanceThroughSplash(m)
	if m.helpOpen {
		t.Fatal("WithOnboarding(false): help must NOT auto-open")
	}
}

// TestReturningUserStartsOnSplashNoHelp verifies the default (no WithOnboarding)
// returning-user path: starts on the splash and advancing through it never opens
// help.
func TestReturningUserStartsOnSplashNoHelp(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := New(render.Caps{}) // no WithOnboarding
	m.w, m.h = 100, 40

	if m.screen != scrSplash {
		t.Fatalf("returning user must start on scrSplash, got screen %d", m.screen)
	}
	m = advanceThroughSplash(m)
	if m.helpOpen {
		t.Fatal("no WithOnboarding: help must NOT auto-open")
	}
}

// TestHelpPageNavigation verifies left/right (and h/l) page navigation and
// clamping at both ends of the paginated help modal.
func TestHelpPageNavigation(t *testing.T) {
	m := newAtMenu()
	m.w, m.h = 100, 40

	// Open help.
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = u.(Model)
	if !m.helpOpen {
		t.Fatal("? must open help modal")
	}
	if m.helpPage != 0 {
		t.Fatalf("help opens on page 0, got %d", m.helpPage)
	}

	// right → page 1
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = u.(Model)
	if m.helpPage != 1 {
		t.Fatalf("right must advance to page 1, got %d", m.helpPage)
	}
	if m.helpScroll != 0 {
		t.Fatalf("right must reset helpScroll to 0, got %d", m.helpScroll)
	}

	// left → page 0
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = u.(Model)
	if m.helpPage != 0 {
		t.Fatalf("left must return to page 0, got %d", m.helpPage)
	}

	// left again — should clamp at 0
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = u.(Model)
	if m.helpPage != 0 {
		t.Fatalf("left at page 0 must clamp to 0, got %d", m.helpPage)
	}

	// l key → page 1
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	m = u.(Model)
	if m.helpPage != 1 {
		t.Fatalf("l must advance to page 1, got %d", m.helpPage)
	}

	// h key → page 0
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	m = u.(Model)
	if m.helpPage != 0 {
		t.Fatalf("h must return to page 0, got %d", m.helpPage)
	}

	// Advance to the last page and verify right clamps.
	last := screens.HelpPageCount() - 1
	m.helpPage = last
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = u.(Model)
	if m.helpPage != last {
		t.Fatalf("right at last page (%d) must clamp, got %d", last, m.helpPage)
	}
}

// TestHelpNumberKeyJumpsToPage verifies that keys 1–5 jump directly to the
// corresponding page (0-indexed).
func TestHelpNumberKeyJumpsToPage(t *testing.T) {
	m := newAtMenu()
	m.w, m.h = 100, 40

	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = u.(Model)
	if !m.helpOpen {
		t.Fatal("? must open help")
	}

	// Key "3" → page 2 (0-indexed)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	m = u.(Model)
	if m.helpPage != 2 {
		t.Fatalf("key '3' must jump to page 2, got %d", m.helpPage)
	}

	// Key "1" → page 0
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	m = u.(Model)
	if m.helpPage != 0 {
		t.Fatalf("key '1' must jump to page 0, got %d", m.helpPage)
	}
}

// TestHelpCloseWritesNothing verifies that closing a manually-opened help modal
// returns a nil cmd (onboarding no longer uses the help modal at all).
func TestHelpCloseWritesNothing(t *testing.T) {
	m := newAtMenu()
	m.w, m.h = 100, 40

	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = u.(Model)
	if !m.helpOpen {
		t.Fatal("? must open help")
	}

	u, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = u.(Model)
	if m.helpOpen {
		t.Fatal("esc must close help")
	}
	if cmd != nil {
		t.Fatal("closing help must return nil cmd")
	}
}
