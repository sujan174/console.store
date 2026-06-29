package screens

import (
	"consolestore/internal/tui/theme"
)

// AuthGate is the connect-your-Swiggy-account screen shown when there is no
// token yet. It renders as a Tokyo Night modal card (matching the settings /
// conflict modals) with the authorize link and a live "waiting" spinner.
type AuthGate struct {
	url     string
	opening bool // browser is auto-opening (true) vs press-Enter to open (false)
	frame   int
}

func NewAuthGate(url string, opening bool) AuthGate {
	return AuthGate{url: url, opening: opening}
}

// WithFrame sets the animation frame (drives the waiting spinner).
func (a AuthGate) WithFrame(f int) AuthGate { a.frame = f; return a }

var authSpin = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (a AuthGate) View() string {
	spin := authSpin[(a.frame/3)%len(authSpin)]

	body := []string{
		theme.TextStyle.Render("console.store places real orders through your"),
		theme.TextStyle.Render("Swiggy account. Connect it once to get going."),
		"",
	}
	if a.opening {
		body = append(body,
			theme.GreenStyle.Render("✦ ")+theme.BrightStyle.Render("opening your browser to sign in…"),
			theme.DimStyle.Render("  didn't open? copy the link below:"),
		)
	} else {
		body = append(body,
			theme.CursorStyle.Render("↵ ")+theme.BrightStyle.Render("open your browser to sign in"),
			theme.DimStyle.Render("  or copy the link below:"),
		)
	}
	body = append(body,
		"",
		theme.Fg(theme.Price).Render(a.url),
		"",
		theme.GreenStyle.Render(spin)+theme.DimStyle.Render("  waiting for authorization…"),
	)

	footer := "↵ open browser   ·   esc quit"
	if a.opening {
		footer = "waiting…   ·   esc quit"
	}
	return autoCard("connect to swiggy", body, footer)
}
