package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkg/browser"
)

// browserOpenedMsg reports the result of an auto-open attempt (currently
// advisory only — failure leaves the copyable URL on screen).
type browserOpenedMsg struct{ err error }

// openBrowserCmd opens url in the user's default browser off the UI goroutine.
func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		if url == "" {
			return browserOpenedMsg{}
		}
		return browserOpenedMsg{err: browser.OpenURL(url)}
	}
}
