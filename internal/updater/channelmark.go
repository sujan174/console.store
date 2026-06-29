package updater

import (
	"encoding/json"
	"os"
	"path/filepath"

	"consolestore/internal/version"
)

// Mark records the channel the user opted into and (for alpha) their access
// code, so self-update keeps tracking the right channel across launches.
type Mark struct {
	Channel   string `json:"channel"`
	AlphaCode string `json:"alpha_code,omitempty"`
}

func markPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "console-store", "channel.json"), nil
}

// LoadMark returns the saved mark, or the build-default channel when absent.
func LoadMark() Mark {
	def := Mark{Channel: version.Channel}
	p, err := markPath()
	if err != nil {
		return def
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return def
	}
	var m Mark
	if err := json.Unmarshal(b, &m); err != nil || m.Channel == "" {
		return def
	}
	return m
}

func SaveMark(m Mark) error {
	p, err := markPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}
