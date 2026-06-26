package localstore

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Registration is the cached OAuth client identity + the discovered endpoints
// the binary needs to authorize and refresh. None of it is secret, so it lives
// in a plain 0600 file (the token stays in the keyring).
type Registration struct {
	ClientID              string `json:"client_id"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
}

// configPath returns ~/.config/console-store/client.json, honoring
// XDG_CONFIG_HOME.
func configPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "console-store", "client.json"), nil
}

// LoadRegistration reads the cached registration. ok is false (nil error) when
// the file does not exist yet.
func LoadRegistration() (Registration, bool, error) {
	p, err := configPath()
	if err != nil {
		return Registration{}, false, err
	}
	raw, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return Registration{}, false, nil
	}
	if err != nil {
		return Registration{}, false, err
	}
	var r Registration
	if err := json.Unmarshal(raw, &r); err != nil {
		return Registration{}, false, err
	}
	return r, true, nil
}

// SaveRegistration writes the registration (0600), creating the directory.
func SaveRegistration(r Registration) error {
	p, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return os.WriteFile(p, raw, 0o600)
}
