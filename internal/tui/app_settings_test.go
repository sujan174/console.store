package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	swiggysnap "consolestore/internal/catalog/swiggy"
	"consolestore/internal/tui/render"
)

// From the splash, navigating to "settings" + Enter opens the settings modal;
// Enter on Disconnect fires logout, which purges + re-enters the auth gate with
// a fresh authorize URL (re-auth in place).
func TestSplashSettingsDisconnectReauths(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // isolate from real active-order state
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{},
		WithLiveBackend(&liveFake{}, snap, "local", ""),
		WithAuthFlow("", fakePoller{}),
	)
	// homeSel 1 = "settings" in the standard 2-item layout (no active order).
	// settings is the last item; no active order → [go to shop, settings].
	m.homeSel = 1

	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = out.(Model)
	if !m.settingsOpen {
		t.Fatal("settings modal should open from the splash settings item")
	}

	out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // Disconnect Swiggy
	m = out.(Model)
	if m.settingsOpen {
		t.Fatal("settings modal should close on disconnect")
	}
	if cmd == nil {
		t.Fatal("disconnect should fire a Logout command")
	}

	out, _ = m.Update(cmd()) // run Logout → LoggedOutMsg, feed back
	m = out.(Model)
	if !m.needsAuth {
		t.Fatal("after disconnect the app should re-enter the auth gate")
	}
	if m.authorizeURL != "https://authz/y" {
		t.Fatalf("re-auth authorize URL = %q, want https://authz/y", m.authorizeURL)
	}
	if m.authAutoOpen {
		t.Fatal("logout re-auth must NOT auto-open the browser (force Enter)")
	}
}
