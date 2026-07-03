package screens

import (
	"consolestore/internal/tui/theme"
)

// TrackPick is the modal shown when BOTH verticals have a live delivery at
// once (a restaurant order and an Instamart order) and the user hits the
// splash "track order" entry — one tracking page shows one order, so the user
// picks which to open. It is a passive value type: the root model handles keys
// (↑ ↓ to move, Enter to open) and centers the View.
type TrackPick struct {
	labels [2]string // row 0 = current primary order, row 1 = the other one
	focus  int
}

// NewTrackPick builds the modal with a display label per live order
// (e.g. "Blue Tokai · out for delivery", "Instamart · ~12 min").
func NewTrackPick(primary, other string) TrackPick {
	return TrackPick{labels: [2]string{primary, other}}
}

// WithFocus sets the highlighted row (0 or 1). Returns a copy, per the screen
// builder convention.
func (t TrackPick) WithFocus(i int) TrackPick {
	if i == 0 || i == 1 {
		t.focus = i
	}
	return t
}

// Focus returns the highlighted row.
func (t TrackPick) Focus() int { return t.focus }

// View renders the bordered dialog. The caller centers it in the viewport.
func (t TrackPick) View() string {
	lines := []string{
		theme.TextStyle.Render("two deliveries are on their way."),
		"",
		trackPickRow(t.labels[0], t.focus == 0),
		trackPickRow(t.labels[1], t.focus == 1),
	}
	return autoCard("track which order?", lines, "↑ ↓ move   ↵ open   esc cancel")
}

// trackPickRow renders one order row with the selected-row idiom (green
// left-bar + highlight), matching the conflict-modal buttons.
func trackPickRow(label string, focused bool) string {
	if focused {
		return theme.GreenStyle.Render("▌") + theme.SelRowStyle.Render(" "+label+" ")
	}
	return theme.DimStyle.Render("  " + label + " ")
}
