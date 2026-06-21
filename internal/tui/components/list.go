package components

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"console.store/internal/tui/theme"
)

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

// stripANSI removes SGR colour codes so a string can be re-styled cleanly.
// Wrapping already-coloured text in a Background leaves gaps where the inner
// resets fire; the selected row is uniformly bright anyway (design line 845).
func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

// Row is one line: Left label, Right meta (eta/price), optional Tag (new), Fav marker.
type Row struct {
	Left     string
	Right    string
	Tag      string // "new" -> green
	Fav      bool   // -> red ♥
	BarGreen bool   // green left-bar when in-cart but not the cursor row
}

// List is a single-column selectable list with a > cursor and highlighted row.
type List struct {
	Rows   []Row
	Cursor int
	Width  int // total render width; 0 -> 50
	filter string
}

// SetFilter sets the case-insensitive substring filter and clamps the cursor.
func (l *List) SetFilter(q string) {
	l.filter = strings.ToLower(strings.TrimSpace(q))
	if l.Cursor >= len(l.VisibleRows()) {
		l.Cursor = 0
	}
}

// Filter returns the current filter string.
func (l *List) Filter() string { return l.filter }

// VisibleRows returns rows matching the filter (all rows if empty).
func (l List) VisibleRows() []Row {
	if l.filter == "" {
		return l.Rows
	}
	var out []Row
	for _, r := range l.Rows {
		if strings.Contains(strings.ToLower(r.Left), l.filter) {
			out = append(out, r)
		}
	}
	return out
}

// SelectedIndex returns the index into Rows of the currently selected visible row.
func (l List) SelectedIndex() int {
	vis := l.VisibleRows()
	if len(vis) == 0 {
		return -1
	}
	sel := vis[l.Cursor]
	for i, r := range l.Rows {
		if r == sel {
			return i
		}
	}
	return -1
}

func (l *List) Up() {
	if l.Cursor > 0 {
		l.Cursor--
	}
}

func (l *List) Down() {
	if l.Cursor < len(l.VisibleRows())-1 {
		l.Cursor++
	}
}

func (l List) View() string {
	// Text area width: explicit override, else the dynamic content width.
	width := l.Width
	if width == 0 {
		width = ContentWidth()
	}
	var b strings.Builder
	for i, r := range l.VisibleRows() {
		right := r.Right
		if r.Tag != "" {
			right += "  " + theme.NewStyle.Render(r.Tag)
		}
		if r.Fav {
			right += "  " + theme.FavStyle.Render("♥")
		}
		pad := width - lipgloss.Width(r.Left) - lipgloss.Width(right)
		if pad < 1 {
			pad = 1
		}
		body := r.Left + strings.Repeat(" ", pad) + right
		if i == l.Cursor {
			// Full-bleed selected row: blue ▌ border at col 0, then a blue >
			// cursor and uniformly-bright text on the selected-row background.
			// Every piece is background-aware so the highlight is one continuous
			// strip with no colour-reset banding.
			selBg := lipgloss.Color(theme.SelRowBg)
			chevron := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Cursor)).Background(selBg).Render("> ")
			bright := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Bright)).Background(selBg)
			lead := bright.Render(strings.Repeat(" ", margin-1))
			text := bright.Render(stripANSI(body))
			// pad the remainder of the row with the selected-row background
			used := margin - 1 + lipgloss.Width("> ") + lipgloss.Width(stripANSI(body))
			tail := ""
			if rest := FrameWidth() - 1 - used; rest > 0 {
				tail = bright.Render(strings.Repeat(" ", rest))
			}
			b.WriteString(theme.CursorStyle.Render("▌") + lead + chevron + text + tail + "\n")
		} else {
			// idle row: a chevron slot keeps names aligned with the selected row.
			lead := strings.Repeat(" ", margin)
			if r.BarGreen {
				lead = theme.GreenStyle.Render("▌") + strings.Repeat(" ", margin-1)
			}
			b.WriteString(lead + theme.FaintStyle.Render("  ") + theme.ItemStyle.Render(body) + "\n")
		}
	}
	return b.String()
}
