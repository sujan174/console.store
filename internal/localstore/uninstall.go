package localstore

import "os"

// ConfigDir returns the per-user config directory
// (~/.config/console-store, honoring XDG_CONFIG_HOME) that holds all of the
// binary's non-keyring state: client.json, channel.json, onboarded,
// last-seen-version, presets.json, active-order.json, card, taste,
// agents-sync.hash, and the file-token fallback.
func ConfigDir() (string, error) {
	return baseConfigDir()
}

// RemoveAllData deletes the entire config directory and everything under it.
// It resolves the path exactly as configPath() does, so tests that isolate the
// directory via XDG_CONFIG_HOME are honored. Removing a non-existent directory
// is not an error.
func RemoveAllData() error {
	dir, err := baseConfigDir()
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}
