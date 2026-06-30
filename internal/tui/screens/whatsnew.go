package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"consolestore/internal/tui/theme"
)

// WhatsNew is the "what's new" release-notes modal. It is a passive value type
// mirroring the Help modal chrome: rounded gold border, BrandStyle title,
// paginated by viewport height. The root holds page/scroll state and rebuilds
// it each frame.
type WhatsNew struct {
	version   string
	lines     []string
	page      int
	scroll    int
	viewportH int
}

// NewWhatsNew constructs a WhatsNew for the given version and pre-rendered lines
// (from renderNotesMarkdown). Lines are the full flat list across all pages;
// WhatsNew paginates them by viewport height.
func NewWhatsNew(version string, lines []string) WhatsNew {
	return WhatsNew{version: version, lines: lines}
}

// WithViewport sets the terminal height so the card windows to fit.
func (w WhatsNew) WithViewport(v int) WhatsNew { w.viewportH = v; return w }

// WithPage sets the page (0-indexed, clamped in View).
func (w WhatsNew) WithPage(n int) WhatsNew { w.page = n; return w }

// WithScroll sets the scroll offset within the current page (clamped in View).
func (w WhatsNew) WithScroll(s int) WhatsNew { w.scroll = s; return w }

const whatsnewCardWidth = 60

// innerRows returns how many content rows fit inside the card. The card chrome
// (border, title blank, footer blank) costs ~7 rows. Mirrors help.go's math.
func (w WhatsNew) innerRows() int {
	if w.viewportH <= 0 {
		return len(w.lines)
	}
	if n := w.viewportH - 7; n >= 4 {
		return n
	}
	return 4
}

// PageCount returns the number of pages of content.
func (w WhatsNew) PageCount() int {
	rows := w.innerRows()
	if rows <= 0 || len(w.lines) == 0 {
		return 1
	}
	n := (len(w.lines) + rows - 1) / rows
	if n < 1 {
		return 1
	}
	return n
}

func (w WhatsNew) View() string {
	pageCount := w.PageCount()
	rows := w.innerRows()

	// Clamp page.
	pg := w.page
	if pg < 0 {
		pg = 0
	}
	if pg >= pageCount {
		pg = pageCount - 1
	}

	// Slice out the content for this page.
	start := pg * rows
	end := start + rows
	if end > len(w.lines) {
		end = len(w.lines)
	}
	if start > len(w.lines) {
		start = len(w.lines)
	}
	pageLines := w.lines[start:end]

	// Scroll within the page.
	scroll := w.scroll
	if max := len(pageLines) - rows; scroll > max {
		scroll = max
	}
	if scroll < 0 {
		scroll = 0
	}
	viewEnd := scroll + rows
	if viewEnd > len(pageLines) {
		viewEnd = len(pageLines)
	}
	above := scroll
	below := len(pageLines) - viewEnd

	var inner strings.Builder
	ver := ""
	if w.version != "" {
		ver = theme.FaintStyle.Render("  ·  " + w.version)
	}
	title := theme.BrandStyle.Render("what's new") + ver
	inner.WriteString(title + "\n\n")

	if above > 0 {
		inner.WriteString("  " + theme.FaintStyle.Render(fmt.Sprintf("↑ %d more", above)) + "\n")
	}
	for _, line := range pageLines[scroll:viewEnd] {
		inner.WriteString(line + "\n")
	}
	if below > 0 {
		inner.WriteString("  " + theme.FaintStyle.Render(fmt.Sprintf("↓ %d more", below)) + "\n")
	}

	pageIndicator := theme.FaintStyle.Render(fmt.Sprintf("‹ %d/%d ›", pg+1, pageCount))
	scrollHint := ""
	if above > 0 || below > 0 {
		scrollHint = "   ↑↓ scroll ·"
	}
	foot := pageIndicator + theme.FaintStyle.Render(scrollHint+"   ← → page · esc close")
	inner.WriteString("\n" + foot)

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.Gold)).
		Padding(0, 2).
		Width(whatsnewCardWidth).
		Render(inner.String())
	return card
}

// RenderNotesMarkdown converts a markdown string into styled terminal lines.
// It handles headings (# / ## / ###) → bold gold; bullet lines (- / *) →
// indented bullet; blank → blank; everything else → plain dim text.
// No full markdown engine — only these patterns are needed.
// Exported so tests can exercise the render logic directly.
func RenderNotesMarkdown(md string) []string {
	return renderNotesMarkdown(md)
}

func renderNotesMarkdown(md string) []string {
	raw := strings.Split(md, "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		switch {
		case strings.HasPrefix(line, "### "):
			text := strings.TrimPrefix(line, "### ")
			out = append(out, theme.GoldStyle.Bold(true).Render(text))
		case strings.HasPrefix(line, "## "):
			text := strings.TrimPrefix(line, "## ")
			out = append(out, theme.GoldStyle.Bold(true).Render(text))
		case strings.HasPrefix(line, "# "):
			text := strings.TrimPrefix(line, "# ")
			out = append(out, theme.GoldStyle.Bold(true).Render(text))
		case strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* "):
			text := line[2:]
			out = append(out, "  • "+theme.TextStyle.Render(text))
		case line == "":
			out = append(out, "")
		default:
			out = append(out, theme.DimStyle.Render(line))
		}
	}
	return out
}
