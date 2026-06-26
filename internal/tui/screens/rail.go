package screens

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"console.store/internal/tui/theme"
)

// Rail is the left navigation column on the live Restaurants browse: a 🔍 Search
// entry, Home, then the cuisine categories. The root maps the active entry to a
// load command. It is a passive value type (With* return copies).
type Rail struct {
	entries []string
	active  int
	focus   bool
	height  int
}

// Fixed rail entry indices. Category entries begin at railCatBase.
const (
	RailSearch  = 0
	RailHome    = 1
	railCatBase = 2
	railWidth   = 19 // text column width (the right divider is drawn separately)
	railInner   = railWidth - 3
)

// NewRail builds the rail entries: Search, Home, then the categories.
func NewRail(categories []string) Rail {
	entries := make([]string, 0, len(categories)+2)
	entries = append(entries, "🔍 Search", "Home")
	entries = append(entries, categories...)
	return Rail{entries: entries, active: RailHome}
}

func (r Rail) WithActive(i int) Rail { r.active = i; return r.clamp() }
func (r Rail) WithFocus(f bool) Rail { r.focus = f; return r }
func (r Rail) WithHeight(h int) Rail { r.height = h; return r }

func (r Rail) clamp() Rail {
	if r.active < 0 {
		r.active = 0
	}
	if r.active >= len(r.entries) {
		r.active = len(r.entries) - 1
	}
	return r
}

func (r Rail) Active() int { return r.active }
func (r Rail) Len() int    { return len(r.entries) }
func (r Rail) Width() int  { return railWidth }
func (r Rail) EntryLabel(i int) string {
	if i < 0 || i >= len(r.entries) {
		return ""
	}
	// strip the icon prefix for the Search entry's logical label
	return strings.TrimPrefix(r.entries[i], "🔍 ")
}

// IsCategory reports whether entry i is a cuisine category (vs Search/Home), and
// returns its 0-based category index.
func (r Rail) IsCategory(i int) (int, bool) {
	if i >= railCatBase && i < len(r.entries) {
		return i - railCatBase, true
	}
	return 0, false
}

// railTrunc shortens a label to fit the inner column, adding an ellipsis.
func railTrunc(s string) string {
	r := []rune(s)
	if len(r) <= railInner {
		return s
	}
	return string(r[:railInner-1]) + "…"
}

func (r Rail) View() string {
	rows := make([]string, 0, len(r.entries)+2)
	rows = append(rows, theme.FaintStyle.Render(" explore"))
	for i, e := range r.entries {
		var row string
		switch {
		case i == r.active && r.focus:
			row = theme.Fg(theme.Gold).Bold(true).Render("▌ " + railTrunc(e))
		case i == r.active:
			row = theme.Fg(theme.Gold).Render("▌ " + railTrunc(e))
		default:
			row = theme.CatOffStyle.Render("  " + railTrunc(e))
		}
		rows = append(rows, row)
		if i == RailSearch {
			rows = append(rows, "") // blank line sets Search apart from the nav list
		}
	}

	block := lipgloss.NewStyle().
		Width(railWidth).
		Padding(0, 1, 0, 0).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(lipgloss.Color(theme.Div2))
	if h := r.height; h > len(rows) {
		block = block.Height(h)
	}
	return block.Render(strings.Join(rows, "\n"))
}
