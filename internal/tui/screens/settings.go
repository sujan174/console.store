package screens

import "console.store/internal/tui/theme"

// settingsItems are the rows in the Settings modal. Only Disconnect today; the
// slice is the seam for future settings.
var settingsItems = []string{"Disconnect Swiggy"}

// SettingsItemCount is the number of selectable settings rows.
func SettingsItemCount() int { return len(settingsItems) }

// Settings is the start-screen settings modal, themed like the other modal
// cards (gold-bordered autoCard). It is a passive value type; the root drives
// selection and the chosen action.
type Settings struct {
	sel       int
	connected bool // false in mock mode → Disconnect reads as "(not connected)"
}

// NewSettings builds the modal. connected reflects whether a live Swiggy session
// is active (false on the mock path), which gates the Disconnect action.
func NewSettings(connected bool) Settings { return Settings{connected: connected} }

// WithSelection sets (and clamps) the highlighted row.
func (s Settings) WithSelection(i int) Settings {
	s.sel = i
	if s.sel < 0 {
		s.sel = 0
	}
	if s.sel >= len(settingsItems) {
		s.sel = len(settingsItems) - 1
	}
	return s
}

func (s Settings) Selection() int { return s.sel }

// SelectedAction returns the logical action for the highlighted row ("disconnect"
// for the Disconnect Swiggy row), or "" if it cannot be acted on (not connected).
func (s Settings) SelectedAction() string {
	if s.sel == 0 && s.connected {
		return "disconnect"
	}
	return ""
}

// ModalView renders the settings modal as a centered gold-bordered card,
// matching the address / item-info modals: the selected row gets the blue ▌ bar
// + a bright line, the others read as plain items.
func (s Settings) ModalView() string {
	lines := make([]string, len(settingsItems))
	for i, it := range settingsItems {
		label := it
		if it == "Disconnect Swiggy" && !s.connected {
			label += "  " + theme.FaintStyle.Render("(not connected)")
		}
		if i == s.sel {
			lines[i] = theme.CursorStyle.Render("▌ ") + theme.BrightStyle.Render(label)
		} else {
			lines[i] = "  " + theme.ItemStyle.Render(label)
		}
	}
	return autoCard("settings", lines, "↑↓ move   ↵ select   esc close")
}
