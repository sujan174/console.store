package localstore

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// AddrPref is the address model for the ordering app: a sticky Default (the
// "lock"), and a Last address updated on every placement. No favorites/taste —
// this deliberately replaces the card for the app flow.
type AddrPref struct {
	DefaultAddrID string `json:"defaultAddrId,omitempty"`
	DefaultLabel  string `json:"defaultLabel,omitempty"`
	Locked        bool   `json:"locked,omitempty"`
	LastAddrID    string `json:"lastAddrId,omitempty"`
	LastLabel     string `json:"lastLabel,omitempty"`
	LastUsedUnix  int64  `json:"lastUsedUnix,omitempty"`
}

// Active returns the address the app should open with: the locked default when
// locked, otherwise the last-used, falling back to the default.
func (p AddrPref) Active() (id, label string) {
	if p.Locked && p.DefaultAddrID != "" {
		return p.DefaultAddrID, p.DefaultLabel
	}
	if p.LastAddrID != "" {
		return p.LastAddrID, p.LastLabel
	}
	return p.DefaultAddrID, p.DefaultLabel
}

func (p AddrPref) SetActive(id, label string) AddrPref {
	p.LastAddrID, p.LastLabel = id, label
	return p
}

// SetDefault pins an explicit default and turns the lock on.
func (p AddrPref) SetDefault(id, label string) AddrPref {
	p.DefaultAddrID, p.DefaultLabel, p.Locked = id, label, true
	return p
}

// RecordPlacement always updates Last; the default (when locked) is untouched.
func (p AddrPref) RecordPlacement(id, label string, now int64) AddrPref {
	p.LastAddrID, p.LastLabel, p.LastUsedUnix = id, label, now
	return p
}

func addrPrefPath() (string, error) {
	p, err := configPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "addrpref.json"), nil
}

func LoadAddrPref() (AddrPref, error) {
	p, err := addrPrefPath()
	if err != nil {
		return AddrPref{}, err
	}
	raw, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return AddrPref{}, nil
	}
	if err != nil {
		return AddrPref{}, err
	}
	var ap AddrPref
	if err := json.Unmarshal(raw, &ap); err != nil {
		return AddrPref{}, err
	}
	return ap, nil
}

func SaveAddrPref(ap AddrPref) error {
	p, err := addrPrefPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(ap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, raw, 0o600)
}
