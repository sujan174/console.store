package components

import "consolestore/internal/tui/theme"

// KeyHints renders the dim footer hint line.
func KeyHints(hints string) string {
	return "  " + theme.KeyHintStyle.Render(hints) + "\n"
}
