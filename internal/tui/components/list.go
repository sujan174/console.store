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
}

func (l *List) Up() {
	if l.Cursor > 0 {
		l.Cursor--
	}
}

func (l *List) Down() {
	if l.Cursor < len(l.Rows)-1 {
		l.Cursor++
	}
}

func (l List) View() string {
	width := l.Width
	if width == 0 {
		width = 50
	}
	var b strings.Builder
	for i, r := range l.Rows {
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
