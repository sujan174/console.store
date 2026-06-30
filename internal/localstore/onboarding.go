package localstore

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// onboardedPath returns the path to the onboarding marker file.
func onboardedPath() (string, error) {
	p, err := configPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "onboarded"), nil
}

// fileExists reports whether path exists on disk (any kind of file/dir).
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Onboarded reports true if the onboarding marker file exists.
func Onboarded() bool {
	p, err := onboardedPath()
	if err != nil {
		return false
	}
	return fileExists(p)
}

// MarkOnboarded writes the onboarding marker file (mode 0600), creating the
// config directory if necessary. The marker content is a timestamp string;
// existence is what matters.
func MarkOnboarded() error {
	p, err := onboardedPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	content := fmt.Sprintf("onboarded at %s\n", time.Now().UTC().Format(time.RFC3339))
	return os.WriteFile(p, []byte(content), 0o600)
}

// ShouldOnboard returns true when the onboarding help modal should auto-open.
// Decision order:
//  1. CONSOLE_NO_ONBOARDING=1  → false
//  2. CONSOLE_FORCE_ONBOARDING=1 → true
//  3. Onboarded() → false
//  4. Grandfather: if client.json, presets.json, or active-order.json exists →
//     silently mark onboarded and return false (existing user, fresh binary)
//  5. otherwise → true (genuinely fresh install)
func ShouldOnboard() bool {
	if os.Getenv("CONSOLE_NO_ONBOARDING") == "1" {
		return false
	}
	if os.Getenv("CONSOLE_FORCE_ONBOARDING") == "1" {
		return true
	}
	if Onboarded() {
		return false
	}

	// Grandfather check: any prior state files in the config dir?
	clientP, err := configPath()
	if err == nil && fileExists(clientP) {
		_ = MarkOnboarded()
		return false
	}
	presetsP, err := presetsPath()
	if err == nil && fileExists(presetsP) {
		_ = MarkOnboarded()
		return false
	}
	orderP, err := orderPath()
	if err == nil && fileExists(orderP) {
		_ = MarkOnboarded()
		return false
	}

	return true
}
