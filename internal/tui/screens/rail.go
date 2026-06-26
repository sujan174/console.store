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
	railWidth   = 14 // column width incl. the divider gutter
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

func (r Rail) View() string {
	var b strings.Builder
	for i, e := range r.entries {
		cursor := "  "
		label := theme.CatOffStyle.Render(e)
		if i == r.active {
			if r.focus {
				cursor = theme.CursorStyle.Render("▸ ")
			} else {
				cursor = theme.DimStyle.Render("▸ ")
			}
			label = theme.Fg(theme.Gold).Underline(true).Render(e)
		}
		row := cursor + label
		// pad/truncate to the content width (rail minus the divider gutter)
		row = lipgloss.NewStyle().Width(railWidth - 2).Render(row)
		b.WriteString(row + theme.Fg(theme.Div2).Render(" │") + "\n")
	}
	// pad to height so the divider runs the full pane
	for n := len(r.entries); n < r.height; n++ {
		b.WriteString(lipgloss.NewStyle().Width(railWidth-2).Render("") + theme.Fg(theme.Div2).Render(" │") + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
