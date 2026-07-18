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
	return writeFileAtomic(p, []byte(content), 0o600)
}

// ShouldOnboard reports whether the first-run onboarding manual should auto-open.
//
// signedIn is whether a Swiggy token is already stored — the authoritative
// "existing user" signal. It is absent on a genuinely fresh install (the token is
// only written after the in-app authorize) and present for any returning user.
//
// Decision order:
//  1. CONSOLE_NO_ONBOARDING=1  → false
//  2. CONSOLE_FORCE_ONBOARDING=1 → true
//  3. Onboarded() (marker present) → false
//  4. Grandfather an existing user — already signed in, OR has presets.json /
//     active-order.json (created only by real prior use) → mark + false.
//  5. otherwise → true (genuinely fresh install)
//
// NOTE: client.json is deliberately NOT a grandfather signal. The OAuth DCR
// registration writes it on the very first launch, BEFORE this check runs, so
// using it would misclassify every fresh install as an existing user and suppress
// onboarding entirely (the bug this fixes).
func ShouldOnboard(signedIn bool) bool {
	if os.Getenv("CONSOLE_NO_ONBOARDING") == "1" {
		return false
	}
	if os.Getenv("CONSOLE_FORCE_ONBOARDING") == "1" {
		return true
	}
	if Onboarded() {
		return false
	}
	if signedIn {
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
