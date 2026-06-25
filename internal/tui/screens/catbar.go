package screens

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"console.store/internal/tui/theme"
)

// windowedBar renders a horizontal nav row (categories, cuisine chips) as a
// window centred on the active item so a long list stays navigable: the active
// item is always visible, with ‹ / › markers when items are hidden off either
// side. budget is the character width available; sepText joins items.
func windowedBar(items []string, active, budget int, sepText string) string {
	if len(items) == 0 {
		return ""
	}
	if active < 0 {
		active = 0
	}
	if active >= len(items) {
		active = len(items) - 1
	}

	sepW := lipgloss.Width(sepText)
	const markW = 2           // "‹ " / " ›"
	avail := budget - 2*markW // reserve room for both overflow markers
	if w := lipgloss.Width(items[active]); avail < w {
		avail = w // always show the active item, even if it alone overflows
	}

	// Grow a window outward from the active item, alternating sides, while it
	// still fits the available width.
	lo, hi := active, active
	cur := lipgloss.Width(items[active])
	for {
		grew := false
		if hi+1 < len(items) {
			if wd := sepW + lipgloss.Width(items[hi+1]); cur+wd <= avail {
				hi++
				cur += wd
				grew = true
			}
		}
		if lo-1 >= 0 {
			if wd := sepW + lipgloss.Width(items[lo-1]); cur+wd <= avail {
				lo--
				cur += wd
				grew = true
			}
		}
		if !grew {
			break
		}
	}

	sep := theme.Fg(theme.Div2).Render(sepText)
	parts := make([]string, 0, hi-lo+1)
	for i := lo; i <= hi; i++ {
		if i == active {
			parts = append(parts, theme.Fg(theme.Gold).Underline(true).Render(items[i]))
		} else {
			parts = append(parts, theme.CatOffStyle.Render(items[i]))
		}
	}
	bar := strings.Join(parts, sep)
	if lo > 0 {
		bar = theme.FaintStyle.Render("‹ ") + bar
	}
	if hi < len(items)-1 {
		bar = bar + theme.FaintStyle.Render(" ›")
	}
	return bar
}
