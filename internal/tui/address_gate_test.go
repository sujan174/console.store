package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"consolestore/internal/catalog"
	swiggysnap "consolestore/internal/catalog/swiggy"
	"consolestore/internal/tui/datasource"
	"consolestore/internal/tui/render"
)

// twoAddrSnap returns a snapshot seeded with two addresses.
func twoAddrSnap() (*swiggysnap.Snapshot, []catalog.Address) {
	snap := swiggysnap.NewSnapshot()
	addrs := []catalog.Address{
		{ID: "addr-1", Label: "home", Line: "123 Main St"},
		{ID: "addr-2", Label: "work", Line: "456 Oak Ave"},
	}
	snap.SetAddresses(addrs)
	return snap, addrs
}

// oneAddrSnap returns a snapshot seeded with one address.
func oneAddrSnap() (*swiggysnap.Snapshot, catalog.Address) {
	snap := swiggysnap.NewSnapshot()
	addr := catalog.Address{ID: "addr-1", Label: "home", Line: "123 Main St"}
	snap.SetAddresses([]catalog.Address{addr})
	return snap, addr
}

// liveModelAtMenu returns a live model that has been moved to scrMenu manually
// (simulates the user having clicked through the splash).
func liveModelAtMenu(snap *swiggysnap.Snapshot) Model {
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "acct-1", ""))
	m.w, m.h = 100, 40
	m.screen = scrMenu
	return m
}

// TestAddrGate2AddrsNoOverlay verifies the core gate flow for 2+ addresses:
// after entering scrMenu and receiving AddressesLoadedMsg, the forced picker
// opens and Home is NOT loaded yet (addrGatePending is still true). Then esc
// and 'a' are ignored, and Enter picks the selected address, clears the gate
// flags, and returns a cmd.
func TestAddrGate2AddrsNoOverlay(t *testing.T) {
	snap, addrs := twoAddrSnap()
	m := liveModelAtMenu(snap)

	// Precondition: gate pending.
	if !m.addrGatePending {
		t.Fatal("addrGatePending must be true at launch")
	}

	// Send AddressesLoadedMsg — should open the forced gate.
	u, _ := m.Update(datasource.AddressesLoadedMsg{})
	m = u.(Model)

	if !m.addrOpen {
		t.Fatal("addrOpen must be true after AddressesLoadedMsg with 2 addresses")
	}
	if !m.addrForced {
		t.Fatal("addrForced must be true (this is the entry gate, not the user switcher)")
	}
	// addrGatePending must still be true — gate is open but not yet satisfied.
	if !m.addrGatePending {
		t.Fatal("addrGatePending must remain true while the forced gate is open (not yet picked)")
	}

	// esc must be ignored — gate stays open.
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = u.(Model)
	if !m.addrOpen {
		t.Fatal("esc must NOT close the forced gate (addrOpen must stay true)")
	}
	if !m.addrForced {
		t.Fatal("addrForced must remain true after esc")
	}

	// 'a' must also be ignored.
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = u.(Model)
	if !m.addrOpen {
		t.Fatal("'a' must NOT close the forced gate (addrOpen must stay true)")
	}

	// Enter picks the selected address (cursor=0 → addrs[0]).
	u, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)

	if m.addrOpen {
		t.Fatal("addrOpen must be false after Enter")
	}
	if m.addrForced {
		t.Fatal("addrForced must be false after Enter")
	}
	if m.addrGatePending {
		t.Fatal("addrGatePending must be false after Enter")
	}
	if m.addr.ID != addrs[0].ID {
		t.Fatalf("addr must be set to the selected address %q, got %q", addrs[0].ID, m.addr.ID)
	}
	if cmd == nil {
		t.Fatal("Enter on forced gate must return a non-nil Home-load cmd")
	}
}

// TestAddrGateEscAndAIgnored verifies that esc and 'a' both leave the forced
// gate open (addrOpen and addrForced remain true).
func TestAddrGateEscAndAIgnored(t *testing.T) {
	snap, _ := twoAddrSnap()
	m := liveModelAtMenu(snap)

	u, _ := m.Update(datasource.AddressesLoadedMsg{})
	m = u.(Model)
	if !m.addrOpen || !m.addrForced {
		t.Fatal("precondition: forced gate must be open")
	}

	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyEsc},
		{Type: tea.KeyRunes, Runes: []rune("a")},
		{Type: tea.KeyEsc},
	} {
		u, _ = m.Update(key)
		m = u.(Model)
		if !m.addrOpen {
			t.Fatalf("key %v must not close the forced gate", key)
		}
		if !m.addrForced {
			t.Fatalf("key %v must not clear addrForced", key)
		}
	}
}

// TestAddrGate1AddrAutoUse verifies that a single address is auto-adopted
// without opening the picker, and a Home-load cmd is returned.
func TestAddrGate1AddrAutoUse(t *testing.T) {
	snap, addr := oneAddrSnap()
	m := liveModelAtMenu(snap)

	u, cmd := m.Update(datasource.AddressesLoadedMsg{})
	m = u.(Model)

	if m.addrOpen {
		t.Fatal("addrOpen must be false for a single address (no picker needed)")
	}
	if m.addrForced {
		t.Fatal("addrForced must be false for single address")
	}
	if m.addrGatePending {
		t.Fatal("addrGatePending must be false after single-address auto-use")
	}
	if m.addr.ID != addr.ID {
		t.Fatalf("single address must be auto-adopted: want %q, got %q", addr.ID, m.addr.ID)
	}
	if cmd == nil {
		t.Fatal("single-address gate must return a non-nil Home-load cmd")
	}
}

// TestAddrGateOverlayFirst verifies the ordering requirement:
// with onboarding armed (WithOnboarding(true)), entering the shop opens help
// (not the addr gate); closing help opens the forced addr gate.
func TestAddrGateOverlayFirst(t *testing.T) {
	snap, _ := twoAddrSnap()
	m := New(render.Caps{},
		WithLiveBackend(&liveFake{}, snap, "acct-1", ""),
		WithOnboarding(true),
	)
	m.w, m.h = 100, 40

	// Simulate AddressesLoadedMsg arriving before the splash transition.
	u, _ := m.Update(datasource.AddressesLoadedMsg{})
	m = u.(Model)
	// Still on splash — gate must not open yet.
	if m.addrOpen {
		t.Fatal("gate must not open while still on splash")
	}

	// Advance through splash to scrMenu.
	m = advanceThroughSplash(m)

	if m.screen != scrMenu {
		t.Fatalf("expected scrMenu, got %d", m.screen)
	}
	// Onboarding overlay opens; addr gate must NOT open yet.
	if !m.helpOpen {
		t.Fatal("onboarding help must open on splash→menu with WithOnboarding(true)")
	}
	if m.addrOpen {
		t.Fatal("addr gate must NOT open while help is open")
	}
	if !m.addrGatePending {
		t.Fatal("addrGatePending must still be true while onboarding is open")
	}

	// Close the onboarding overlay — gate must open now.
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = u.(Model)

	if m.helpOpen {
		t.Fatal("esc must close the help modal")
	}
	if !m.addrOpen {
		t.Fatal("addr gate must open after onboarding overlay closes")
	}
	if !m.addrForced {
		t.Fatal("addrForced must be true: the gate opened from the overlay-close path")
	}
}

// TestAddrGateSessionNoPending verifies that once addrGatePending is false,
// opening and closing help (or the 'a' switcher) does NOT re-open the forced gate.
func TestAddrGateSessionNoPending(t *testing.T) {
	snap, _ := twoAddrSnap()
	m := liveModelAtMenu(snap)

	// Pick the address through the gate.
	u, _ := m.Update(datasource.AddressesLoadedMsg{})
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)

	if m.addrGatePending {
		t.Fatal("precondition: addrGatePending must be false after pick")
	}

	// Open and close help — gate must NOT re-open.
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = u.(Model)
	if !m.helpOpen {
		t.Fatal("? must open help")
	}
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = u.(Model)

	if m.addrOpen {
		t.Fatal("addr gate must NOT re-open after session pick (addrGatePending is false)")
	}
	if m.addrForced {
		t.Fatal("addrForced must remain false")
	}
}

// TestAddrGateSplashTransitionNoOverlay verifies that when no overlay is armed,
// the splash→menu transition calls maybeOpenAddrGate directly: if addresses
// are already loaded, the gate opens immediately.
func TestAddrGateSplashTransitionNoOverlay(t *testing.T) {
	snap, _ := twoAddrSnap()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "acct-1", ""))
	m.w, m.h = 100, 40

	// Deliver addresses before the splash transition.
	u, _ := m.Update(datasource.AddressesLoadedMsg{})
	m = u.(Model)
	if m.addrOpen {
		t.Fatal("gate must not open while on splash (scrSplash, not scrMenu)")
	}

	// Now advance through the splash — no overlay armed, so gate opens on transition.
	m = advanceThroughSplash(m)

	if m.screen != scrMenu {
		t.Fatalf("expected scrMenu, got %d", m.screen)
	}
	if !m.addrOpen {
		t.Fatal("forced gate must open at splash→menu transition when no overlay is armed")
	}
	if !m.addrForced {
		t.Fatal("addrForced must be true")
	}
}

// TestAddrGateWhatsnewOverlayFirst mirrors TestAddrGateOverlayFirst but for
// the what's-new modal: the gate must wait until the notes modal closes.
func TestAddrGateWhatsnewOverlayFirst(t *testing.T) {
	snap, _ := twoAddrSnap()
	m := New(render.Caps{},
		WithLiveBackend(&liveFake{}, snap, "acct-1", ""),
		WithReleaseNotes("v1.0.0", "stable", ""),
	)
	m.w, m.h = 100, 40

	// Arm notesReady (would normally come from a network fetch).
	m.notesReady = true

	// Simulate AddressesLoadedMsg arriving early.
	u, _ := m.Update(datasource.AddressesLoadedMsg{})
	m = u.(Model)

	// Advance through splash — what's-new opens.
	m = advanceThroughSplash(m)

	if !m.whatsnewOpen {
		t.Fatal("whatsnewOpen must be true after splash→menu with notesReady")
	}
	if m.addrOpen {
		t.Fatal("addr gate must NOT open while what's-new is open")
	}

	// Close the what's-new modal — gate must open.
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = u.(Model)

	if m.whatsnewOpen {
		t.Fatal("esc must close the what's-new modal")
	}
	if !m.addrOpen {
		t.Fatal("addr gate must open after what's-new overlay closes")
	}
	if !m.addrForced {
		t.Fatal("addrForced must be true")
	}
}
