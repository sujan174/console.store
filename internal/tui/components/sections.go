package components

import (
	"strings"

	"consolestore/internal/catalog"
	"consolestore/internal/tui/theme"
)

// SectionTabs renders "coffee   food   snacks   instamart ↗" with the active
// menu section in gold and the rest dim. Instamart is always shown as a
// cyan jump-link (it is a separate lane, never the "active" tab here).
func SectionTabs(active catalog.Section) string {
	labels := map[catalog.Section]string{
		catalog.SectionCoffee: "coffee",
		catalog.SectionFood:   "food",
		catalog.SectionSnacks: "snacks",
	}
	var parts []string
	for _, s := range catalog.MenuSections {
		if s == active {
			parts = append(parts, theme.CatOnStyle.Render(labels[s]))
		} else {
			parts = append(parts, theme.CatOffStyle.Render(labels[s]))
		}
	}
	parts = append(parts, theme.PriceStyle.Render("instamart ↗"))
	return "  " + strings.Join(parts, "   ") + "\n"
}
