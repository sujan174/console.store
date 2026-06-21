package components

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"console.store/internal/tui/theme"
)

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

// stripANSI removes SGR colour codes (used only for display-width maths).
func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

// withBg paints a continuous background behind an already-coloured string
// WITHOUT changing any foreground colours. Naively wrapping styled text in a
// Background tears at each inner reset (which clears the bg); here we re-assert
// the bg immediately after every reset, so the highlight is seamless and every
// element keeps its own colour (price stays green, etc.).
func withBg(s, hex string) string {
	open := bgSeq(hex)
	return open + strings.ReplaceAll(s, "\x1b[0m", "\x1b[0m"+open) + "\x1b[0m"
}

// bgSeq is the truecolor background SGR for a #rrggbb hex.
func bgSeq(hex string) string {
	var r, g, b int
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
}

// Row is one line: Left label, Right meta (eta/price), optional Tag (new), Fav marker.
type Row struct {
	Left     string
	Right    string
	BarGreen bool // green left-bar when in-cart but not the cursor row
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
		if i > 0 {
			b.WriteString("\n") // one blank line between rows (terminal's only gap increment)
		}
		right := r.Right
		// rightGutter keeps the price/ETA column a little off the right edge.
		const rightGutter = 2
		pad := width - rightGutter - lipgloss.Width(r.Left) - lipgloss.Width(right)
		if pad < 1 {
			pad = 1
		}
		body := r.Left + strings.Repeat(" ", pad) + right
		if i == l.Cursor {
			// Subtle selection: a blue ▌ border + > cursor on the selected-row
			// background. The row's own colours are PRESERVED (price stays green
			// etc.) — withBg paints the highlight without recolouring anything.
			cursor := theme.CursorStyle.Render("> ")
			lead := strings.Repeat(" ", margin-1)
			used := (margin - 1) + lipgloss.Width("> ") + lipgloss.Width(stripANSI(body))
			tail := ""
			if rest := FrameWidth() - 1 - used; rest > 0 {
				tail = strings.Repeat(" ", rest)
			}
			inner := theme.CursorStyle.Render("▌") + lead + cursor + body + tail
			b.WriteString(withBg(inner, theme.SelRowBg) + "\n")
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
