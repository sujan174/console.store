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
	if !ShouldOnboard() {
		t.Error("fresh dir: ShouldOnboard() should be true")
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
	if ShouldOnboard() {
		t.Error("ShouldOnboard() should be false after MarkOnboarded()")
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

	if ShouldOnboard() {
		t.Error("grandfather (presets.json): ShouldOnboard() should be false")
	}
	if !Onboarded() {
		t.Error("grandfather (presets.json): marker should be written after ShouldOnboard()")
	}
}

func TestShouldOnboard_Grandfather_ClientJSON(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := configDir(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "client.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatalf("WriteFile client.json: %v", err)
	}

	if ShouldOnboard() {
		t.Error("grandfather (client.json): ShouldOnboard() should be false")
	}
	if !Onboarded() {
		t.Error("grandfather (client.json): marker should be written after ShouldOnboard()")
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

	if ShouldOnboard() {
		t.Error("grandfather (active-order.json): ShouldOnboard() should be false")
	}
	if !Onboarded() {
		t.Error("grandfather (active-order.json): marker should be written after ShouldOnboard()")
	}
}

func TestShouldOnboard_NoOnboardingEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CONSOLE_NO_ONBOARDING", "1")

	if ShouldOnboard() {
		t.Error("CONSOLE_NO_ONBOARDING=1: ShouldOnboard() should be false even on fresh dir")
	}
}

func TestShouldOnboard_ForceOnboardingEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CONSOLE_FORCE_ONBOARDING", "1")

	// Mark onboarded first — force should override.
	if err := MarkOnboarded(); err != nil {
		t.Fatalf("MarkOnboarded: %v", err)
	}
	if !ShouldOnboard() {
		t.Error("CONSOLE_FORCE_ONBOARDING=1: ShouldOnboard() should be true even after MarkOnboarded()")
	}
}
