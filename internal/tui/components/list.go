package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"console.store/internal/tui/theme"
)

// Row is one line: Left label, Right meta (eta/price), optional Tag (new), Fav marker.
type Row struct {
	Left  string
	Right string
	Tag   string // "new" -> green
	Fav   bool   // -> red ♥
}

// List is a single-column selectable list with a ❯ cursor and highlighted row.
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
	width := l.Width
	if width == 0 {
		width = 50
	}
	var b strings.Builder
	for i, r := range l.VisibleRows() {
		// build "  ❯ Left ........ Right tag ♥"
		left := r.Left
		right := r.Right
		if r.Tag != "" {
			right = right + "  " + theme.NewStyle.Render(r.Tag)
		}
		if r.Fav {
			right = right + "  " + theme.FavStyle.Render("♥")
		}
		// pad between left and right
		pad := width - lipgloss.Width(r.Left) - lipgloss.Width(right)
		if pad < 1 {
			pad = 1
		}
		body := fmt.Sprintf("%s%s%s", left, strings.Repeat(" ", pad), right)

		if i == l.Cursor {
			cur := theme.CursorStyle.Render("❯")
			// pad body so the highlight bar fills the full column width
			bodyWidth := lipgloss.Width(body)
			if bodyWidth < width {
				body = body + strings.Repeat(" ", width-bodyWidth)
			}
			line := theme.SelRowStyle.Render(" " + body + " ")
			b.WriteString("  " + cur + " " + line + "\n")
		} else {
			b.WriteString("  " + theme.FaintStyle.Render("·") + " " + theme.ItemStyle.Render(body) + "\n")
		}
	}
	return b.String()
}
