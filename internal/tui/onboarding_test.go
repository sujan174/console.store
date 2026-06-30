package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

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

// TestOnboardingAutoOpensAfterSplash verifies that WithOnboarding(true) causes
// the help modal to open exactly once after the splash→start transition, with
// onboardingPending=true.
func TestOnboardingAutoOpensAfterSplash(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := New(render.Caps{}, WithOnboarding(true))
	m.w, m.h = 100, 40

	// Before the splash→menu transition: help must NOT be open.
	if m.helpOpen {
		t.Fatal("help must not be open before the splash→start transition")
	}

	m = advanceThroughSplash(m)

	if m.screen != scrMenu {
		t.Fatalf("expected scrMenu after splash transition, got screen %d", m.screen)
	}
	if !m.helpOpen {
		t.Fatal("WithOnboarding(true): help modal must auto-open after the splash→start transition")
	}
	if !m.onboardingPending {
		t.Fatal("WithOnboarding(true): onboardingPending must be true on auto-open")
	}
	if m.wantOnboarding {
		t.Fatal("wantOnboarding must be cleared after auto-open (so it only fires once)")
	}
	if m.helpPage != 0 {
		t.Fatalf("helpPage must be 0 on auto-open, got %d", m.helpPage)
	}
	if m.helpScroll != 0 {
		t.Fatalf("helpScroll must be 0 on auto-open, got %d", m.helpScroll)
	}
}

// TestOnboardingCloseWritesMarker verifies that closing the onboarding modal (esc)
// clears onboardingPending and returns a non-nil cmd (the MarkOnboarded call).
func TestOnboardingCloseWritesMarker(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := New(render.Caps{}, WithOnboarding(true))
	m.w, m.h = 100, 40
	m = advanceThroughSplash(m)

	if !m.helpOpen || !m.onboardingPending {
		t.Fatal("precondition: help must be open and onboardingPending=true")
	}

	// Close via esc.
	u, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = u.(Model)

	if m.helpOpen {
		t.Fatal("esc must close the help modal")
	}
	if m.onboardingPending {
		t.Fatal("onboardingPending must be false after closing the onboarding modal")
	}
	// The MarkOnboarded cmd must be returned (non-nil).
	if cmd == nil {
		t.Fatal("closing the onboarding modal must return a non-nil MarkOnboarded cmd")
	}
}

// TestOnboardingFalseDoesNotAutoOpen verifies that WithOnboarding(false) leaves
// the help modal closed through the same splash→start transition.
func TestOnboardingFalseDoesNotAutoOpen(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := New(render.Caps{}, WithOnboarding(false))
	m.w, m.h = 100, 40
	m = advanceThroughSplash(m)

	if m.helpOpen {
		t.Fatal("WithOnboarding(false): help must NOT auto-open")
	}
	if m.onboardingPending {
		t.Fatal("WithOnboarding(false): onboardingPending must be false")
	}
}

// TestOnboardingNoOptionDoesNotAutoOpen verifies the default (no WithOnboarding
// option) behaves the same as WithOnboarding(false).
func TestOnboardingNoOptionDoesNotAutoOpen(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := New(render.Caps{}) // no WithOnboarding
	m.w, m.h = 100, 40
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

// TestHelpCloseWithoutOnboardingWritesNothing verifies that closing help normally
// (when not in onboarding flow) returns nil cmd and does not set onboardingPending.
func TestHelpCloseWithoutOnboardingWritesNothing(t *testing.T) {
	m := newAtMenu()
	m.w, m.h = 100, 40

	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = u.(Model)
	if !m.helpOpen {
		t.Fatal("? must open help")
	}

	// onboardingPending is false — close must return nil cmd.
	u, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = u.(Model)
	if m.helpOpen {
		t.Fatal("esc must close help")
	}
	if m.onboardingPending {
		t.Fatal("onboardingPending must remain false")
	}
	if cmd != nil {
		t.Fatal("closing non-onboarding help must return nil cmd")
	}
}
