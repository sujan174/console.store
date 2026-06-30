package localstore

import (
	"os"
	"path/filepath"
	"testing"
)

// configDir returns the config directory for the current XDG_CONFIG_HOME
// setting, used in tests to write seed files.
func configDir(t *testing.T) string {
	t.Helper()
	p, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	return filepath.Dir(p)
}

func TestShouldOnboard_FreshDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if !ShouldOnboard(false) {
		t.Error("fresh dir: ShouldOnboard(false) should be true")
	}
}

func TestMarkOnboarded_And_Onboarded(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if Onboarded() {
		t.Fatal("Onboarded() should be false before marking")
	}
	if err := MarkOnboarded(); err != nil {
		t.Fatalf("MarkOnboarded: %v", err)
	}
	if !Onboarded() {
		t.Error("Onboarded() should be true after MarkOnboarded()")
	}
	if ShouldOnboard(false) {
		t.Error("ShouldOnboard(false) should be false after MarkOnboarded()")
	}
}

func TestShouldOnboard_Grandfather_Presets(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := configDir(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Write a presets.json to simulate an existing user.
	if err := os.WriteFile(filepath.Join(dir, "presets.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatalf("WriteFile presets.json: %v", err)
	}

	if ShouldOnboard(false) {
		t.Error("grandfather (presets.json): ShouldOnboard(false) should be false")
	}
	if !Onboarded() {
		t.Error("grandfather (presets.json): marker should be written after ShouldOnboard(false)")
	}
}

// Regression: client.json is written by the OAuth DCR on the first launch, BEFORE
// the onboarding check, so it must NOT grandfather — a fresh install (not signed
// in) with only a client.json present must still onboard.
func TestShouldOnboard_ClientJSONDoesNotGrandfather(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := configDir(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "client.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatalf("WriteFile client.json: %v", err)
	}

	if !ShouldOnboard(false) {
		t.Error("client.json alone (not signed in) must NOT suppress onboarding")
	}
	if Onboarded() {
		t.Error("client.json must not cause the marker to be written")
	}
}

// An already-signed-in user (token present) is an existing user: skip onboarding
// and record the marker.
func TestShouldOnboard_SignedInGrandfathers(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if ShouldOnboard(true) {
		t.Error("signed-in user: ShouldOnboard(true) should be false")
	}
	if !Onboarded() {
		t.Error("signed-in grandfather: marker should be written")
	}
}

func TestShouldOnboard_Grandfather_ActiveOrder(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := configDir(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "active-order.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatalf("WriteFile active-order.json: %v", err)
	}

	if ShouldOnboard(false) {
		t.Error("grandfather (active-order.json): ShouldOnboard(false) should be false")
	}
	if !Onboarded() {
		t.Error("grandfather (active-order.json): marker should be written after ShouldOnboard(false)")
	}
}

func TestShouldOnboard_NoOnboardingEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CONSOLE_NO_ONBOARDING", "1")

	if ShouldOnboard(false) {
		t.Error("CONSOLE_NO_ONBOARDING=1: ShouldOnboard(false) should be false even on fresh dir")
	}
}

func TestShouldOnboard_ForceOnboardingEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CONSOLE_FORCE_ONBOARDING", "1")

	// Mark onboarded first — force should override.
	if err := MarkOnboarded(); err != nil {
		t.Fatalf("MarkOnboarded: %v", err)
	}
	if !ShouldOnboard(false) {
		t.Error("CONSOLE_FORCE_ONBOARDING=1: ShouldOnboard(false) should be true even after MarkOnboarded()")
	}
}
