package localstore

import (
	"os"
	"path/filepath"
	"strings"
)

// lastVersionPath returns the path to the last-seen-version file.
func lastVersionPath() (string, error) {
	p, err := configPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "last-version"), nil
}

// LastSeenVersion reads the last-seen version from disk. Returns "" when the
// file is absent or unreadable.
func LastSeenVersion() string {
	p, err := lastVersionPath()
	if err != nil {
		return ""
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

// SetLastSeenVersion writes v to the last-version file (mode 0600), creating
// the config directory (mode 0700) if necessary.
func SetLastSeenVersion(v string) error {
	p, err := lastVersionPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return writeFileAtomic(p, []byte(v), 0o600)
}
