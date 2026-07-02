package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"consolestore/internal/localstore"
	"consolestore/internal/tui/datasource"
	"consolestore/internal/tui/render"
)

// TestWhatsNewAutoOpensAfterSplash verifies that WithReleaseNotes + a success
// ReleaseNotesMsg causes the what's-new modal to open at the splash→scrMenu
// transition (and does not open the help / onboarding modal).
func TestWhatsNewAutoOpensAfterSplash(t *testing.T) {
	m := New(render.Caps{}, WithReleaseNotes("v1.2.3", "stable", ""))
	m.w, m.h = 100, 40

	// Inject a successful ReleaseNotesMsg directly (no real network).
	u, _ := m.Update(datasource.ReleaseNotesMsg{Markdown: "# Hi\n- thing"})
	m = u.(Model)

	if !m.notesReady {
		t.Fatal("notesReady must be true after a success ReleaseNotesMsg")
	}
	if m.whatsnewOpen {
		t.Fatal("whatsnewOpen must not be true before splash→menu transition")
	}

	// Drive through the splash.
	m = advanceThroughSplash(m)

	if m.screen != scrMenu {
		t.Fatalf("expected scrMenu, got %d", m.screen)
	}
	if !m.whatsnewOpen {
		t.Fatal("whatsnewOpen must be true after splash→scrMenu when notesReady")
	}
	if m.helpOpen {
		t.Fatal("helpOpen must be false when what's-new takes precedence")
	}
	if m.notesReady {
		t.Fatal("notesReady must be cleared after auto-open")
	}
	if m.whatsnewPage != 0 {
		t.Fatalf("whatsnewPage must be 0 on auto-open, got %d", m.whatsnewPage)
	}
	if m.whatsnewScroll != 0 {
		t.Fatalf("whatsnewScroll must be 0 on auto-open, got %d", m.whatsnewScroll)
	}
}

// TestWhatsNewCloseAdvancesLastSeenVersion verifies that closing the what's-new
// modal (esc) sets whatsnewOpen=false and returns a cmd that writes LastSeenVersion.
func TestWhatsNewCloseAdvancesLastSeenVersion(t *testing.T) {
	const ver = "v1.2.3"

	m := New(render.Caps{}, WithReleaseNotes(ver, "stable", ""))
	m.w, m.h = 100, 40

	u, _ := m.Update(datasource.ReleaseNotesMsg{Markdown: "# Hi\n- thing"})
	m = u.(Model)
	m = advanceThroughSplash(m)

	if !m.whatsnewOpen {
		t.Fatal("precondition: whatsnewOpen must be true")
	}

	// Close via esc.
	u, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = u.(Model)

	if m.whatsnewOpen {
		t.Fatal("esc must close the what's-new modal")
	}
	if cmd == nil {
		t.Fatal("closing must return a non-nil cmd (to advance LastSeenVersion)")
	}
	// Execute the cmd (writes to the temp XDG dir set up by TestMain).
	cmd()

	got := localstore.LastSeenVersion()
	if got != ver {
		t.Errorf("LastSeenVersion() = %q, want %q", got, ver)
	}
}

// TestWhatsNewNotFoundAdvancesVersionNoModal verifies that ReleaseNotesMsg with
// NotFound=true advances LastSeenVersion but does NOT open the modal.
func TestWhatsNewNotFoundAdvancesVersionNoModal(t *testing.T) {
	const ver = "v1.2.3"

	m := New(render.Caps{}, WithReleaseNotes(ver, "stable", ""))
	m.w, m.h = 100, 40

	u, cmd := m.Update(datasource.ReleaseNotesMsg{NotFound: true})
	m = u.(Model)

	if m.notesReady {
		t.Fatal("notesReady must not be set on NotFound")
	}
	if m.whatsnewOpen {
		t.Fatal("whatsnewOpen must not be set on NotFound")
	}
	if cmd == nil {
		t.Fatal("NotFound must return a non-nil cmd (to advance LastSeenVersion)")
	}
	// Execute cmd to write the version.
	cmd()

	got := localstore.LastSeenVersion()
	if got != ver {
		t.Errorf("LastSeenVersion() = %q, want %q", got, ver)
	}

	// Advance through splash — modal must still not open.
	m = advanceThroughSplash(m)
	if m.whatsnewOpen {
		t.Fatal("whatsnewOpen must remain false after NotFound → splash transition")
	}
}

// TestWhatsNewErrorDoesNotAdvanceVersion verifies that ReleaseNotesMsg with
// Err set does nothing: no modal, no LastSeenVersion advance.
func TestWhatsNewErrorDoesNotAdvanceVersion(t *testing.T) {
	// Use a fresh isolated config dir so prior tests don't pollute LastSeenVersion.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	const ver = "v1.2.3"

	m := New(render.Caps{}, WithReleaseNotes(ver, "stable", ""))
	m.w, m.h = 100, 40

	u, cmd := m.Update(datasource.ReleaseNotesMsg{Err: errors.New("timeout")})
	m = u.(Model)

	if m.notesReady {
		t.Fatal("notesReady must not be set on Err")
	}
	if m.whatsnewOpen {
		t.Fatal("whatsnewOpen must not be set on Err")
	}
	if cmd != nil {
		// Execute it just in case, but we don't expect one.
		cmd()
	}

	got := localstore.LastSeenVersion()
	if got != "" {
		t.Errorf("LastSeenVersion() must remain empty on error, got %q", got)
	}
}

// TestWhatsNewPageNav verifies that right/left keys page through the modal and
// clamp at both ends.
func TestWhatsNewPageNav(t *testing.T) {
	// Use enough lines to guarantee multiple pages in a small viewport.
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = "content"
	}
	// Build a model with the what's-new modal already open and lines set.
	m := New(render.Caps{})
	m.w, m.h = 100, 12 // small viewport → multiple pages
	m.whatsnewOpen = true
	m.whatsnewLines = lines
	m.notesVersion = "v1.0.0"

	// right → advance page
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = u.(Model)
	if m.whatsnewPage != 1 {
		t.Fatalf("right must advance to page 1, got %d", m.whatsnewPage)
	}
	if m.whatsnewScroll != 0 {
		t.Fatalf("right must reset whatsnewScroll to 0, got %d", m.whatsnewScroll)
	}

	// left → back to page 0
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = u.(Model)
	if m.whatsnewPage != 0 {
		t.Fatalf("left must return to page 0, got %d", m.whatsnewPage)
	}

	// left again — should clamp at 0
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = u.(Model)
	if m.whatsnewPage != 0 {
		t.Fatalf("left at page 0 must clamp to 0, got %d", m.whatsnewPage)
	}
}

// TestWhatsNewOnboardingTakesPrecedence verifies that onboarding takes over the
// start of the session: with WithOnboarding(true) the session begins on the
// welcome screen (not the splash, not help, not what's-new). The what's-new
// modal never opens while onboarding owns the screen.
func TestWhatsNewOnboardingTakesPrecedence(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := New(render.Caps{}, WithOnboarding(true), WithReleaseNotes("v1.0.0", "stable", ""))
	m.w, m.h = 100, 40

	// Manually arm notesReady (would normally come via ReleaseNotesMsg).
	m.notesReady = true

	// Onboarding owns the session start: welcome screen, no help, no what's-new.
	if m.screen != scrWelcome {
		t.Fatalf("expected scrWelcome at session start, got %d", m.screen)
	}
	if m.helpOpen {
		t.Fatal("onboarding must NOT auto-open help")
	}
	if m.whatsnewOpen {
		t.Fatal("what's-new must NOT open while onboarding owns the screen")
	}
}
