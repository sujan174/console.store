// Package telemetry sends anonymous, fire-and-forget usage pings (install
// heartbeat + order placed). It depends only on stdlib + internal/version, and
// NEVER touches the keyring, auth token, or any Swiggy data.
package telemetry

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// configDir mirrors the path used for channel.json (see internal/updater and
// internal/localstore) — deliberately duplicated to keep this package free of
// any cross-import into the updater/auth stack.
func configDir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "console-store"), nil
}

func newUUIDv4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

type installFile struct {
	ID string `json:"id"`
}

// InstallID returns a stable anonymous UUIDv4 for this install, creating and
// persisting it on first call. Returns "" if the config dir is unwritable. The
// id is random — NOT derived from or linked to the Swiggy account.
func InstallID() string {
	dir, err := configDir()
	if err != nil {
		return ""
	}
	p := filepath.Join(dir, "install.json")
	if b, err := os.ReadFile(p); err == nil {
		var f installFile
		if json.Unmarshal(b, &f) == nil && f.ID != "" {
			return f.ID
		}
	}
	id, err := newUUIDv4()
	if err != nil {
		return ""
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}
	b, _ := json.Marshal(installFile{ID: id})
	if err := os.WriteFile(p, b, 0o600); err != nil {
		return ""
	}
	return id
}
