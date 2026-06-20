package screens

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"console.store/internal/tui/components"
	"console.store/internal/tui/theme"
)

// streak mirrors the design's this.state.streak (script line 712).
const streak = 7

// CmdLine is one rendered output line: text plus a theme color hex.
type CmdLine struct{ Text, Color string }

// CmdBar is the vim-style `:` command palette (design lines 446-457, runCmd 659-736).
// It holds the in-progress text and the prior output lines (last ~9).
type CmdBar struct {
	text string
	out  []CmdLine
}

func NewCmdBar() CmdBar { return CmdBar{} }

func (c CmdBar) Text() string { return c.text }

func (c CmdBar) WithText(s string) CmdBar { c.text = s; return c }

func (c CmdBar) Backspace() CmdBar {
	if c.text != "" {
		c.text = c.text[:len(c.text)-1]
	}
	return c
}

func (c CmdBar) Append(s string) CmdBar { c.text += s; return c }

func (c CmdBar) ClearText() CmdBar { c.text = ""; return c }

// Run executes the current text. Returns the updated bar (with output appended)
// and an action: "" | "instamart" | "clear" | "close".
func (c CmdBar) Run() (CmdBar, string) {
	raw := strings.TrimSpace(c.text)
	cmd := ""
	if fields := strings.Fields(strings.ToLower(raw)); len(fields) > 0 {
		cmd = fields[0]
	}

	// Color hexes mapped to theme consts, matching the design's runCmd palette:
	// G=Green, B=Cursor/blue, A=Gold, D=Dim, T=Text, C=Price/cyan, P=Purple, red=Fav.
	const (
		G = theme.Green
		B = theme.Cursor
		A = theme.Gold
		D = theme.Dim
		T = theme.Text
		C = theme.Price
		P = theme.Purple
		R = theme.Fav
	)

	if cmd == "" {
		c.text = ""
		return c, ""
	}
	if cmd == "clear" || cmd == "cls" {
		c.out = nil
		c.text = ""
		return c, "clear"
	}

	action := ""
	var out []CmdLine
	switch cmd {
	case "help", "?":
		out = []CmdLine{
			{"commands —", A},
			{"  neofetch   system info", D},
			{"  coffee     brew one", D},
			{"  whoami     who you are", D},
			{"  streak     your morning run", D},
			{"  uptime     server uptime", D},
			{"  instamart  fast lane", D},
			{"  sl         choo choo", D},
			{"  sudo …     go on, try it", D},
			{"  vim        enter (you can leave)", D},
			{"  42         the answer", D},
			{"  clear      wipe screen · exit  close", D},
		}
	case "neofetch":
		out = []CmdLine{
			{"      ▟▙       guest@console.store", B},
			{"     ▟██▙      ───────────────────", D},
			{"     ▜██▛      host    bangalore · IN", T},
			{"      ▜▛       uptime  3d 4h · load 0.07", T},
			{"               online  247 devs ☕", T},
			{"               shell   zsh · tokyo night", T},
			{"               fav     cold coffee ×7 🔥", A},
		}
	case "coffee", "brew":
		out = []CmdLine{
			{"☕ brewing a virtual espresso …", A},
			{"done. caffeine +1. ship it.", G},
		}
	case "whoami":
		out = []CmdLine{
			{"guest@hsr-layout", C},
			{"a developer with excellent taste and a 7-day cold coffee streak.", D},
		}
	case "streak":
		out = []CmdLine{
			{"🔥 7-day streak", A},
			{"cold coffee, every morning. don't break the chain.", D},
		}
	case "uptime":
		out = []CmdLine{
			{"up 3 days, 4:12 · 247 devs online · load avg 0.07 0.05 0.01", T},
		}
	case "sudo":
		out = []CmdLine{
			{"guest is not in the sudoers file.", R},
			{"this incident will be reported. (to the barista.)", D},
		}
	case "vim", "vi":
		out = []CmdLine{
			{"entered vim.", T},
			{":q  ✓ you left. unlike real vim, we let you go.", G},
		}
	case "sl":
		out = []CmdLine{
			{"      ____", A},
			{"  ___/ ☕ |__   choo choo", A},
			{" |_o____o_|_/   (you typed sl, didn't you)", D},
		}
	case "42", "answer":
		out = []CmdLine{
			{"the answer to coffee, food and everything.", P},
		}
	case "theme":
		out = []CmdLine{
			{"only one theme worth shipping: tokyo night.", B},
		}
	case "tip":
		out = []CmdLine{
			{"tip: press : anywhere for the command palette.", D},
		}
	case "instamart":
		out = []CmdLine{
			{"opening instamart fast lane …", G},
		}
		action = "instamart"
	case "exit", "quit", ":q":
		out = []CmdLine{
			{"connection to console.store closed.", D},
		}
		action = "close"
	default:
		out = []CmdLine{
			{"command not found: " + cmd + " — try `help`", R},
		}
	}

	echo := CmdLine{": " + raw, theme.Faint}
	c.out = append(c.out, echo)
	c.out = append(c.out, out...)
	// Keep the last window of output. The design slices to 9, but the help list
	// alone is 12 lines (echo + 11); cap at 13 so a single command's output is
	// never truncated mid-block.
	if len(c.out) > 13 {
		c.out = c.out[len(c.out)-13:]
	}
	c.text = ""
	return c, action
}

// View renders the palette body (output lines, prompt with blinking cursor, hint)
// on the PanelCmd background. Width matches the content column.
func (c CmdBar) View(blink bool) string {
	var lines []string
	for _, l := range c.out {
		lines = append(lines, theme.Fg(l.Color).Render(l.Text))
	}

	cursor := " "
	if blink {
		cursor = theme.CursorStyle.Render("▋")
	}
	prompt := theme.PurpleStyle.Render(":") + " " + theme.BrightStyle.Render(c.text) + cursor
	lines = append(lines, prompt)

	hint := theme.FaintStyle.Render("type ") + theme.DimStyle.Render("help") +
		theme.FaintStyle.Render(" · ") + theme.DimStyle.Render("↵") + theme.FaintStyle.Render(" run · ") +
		theme.DimStyle.Render("esc") + theme.FaintStyle.Render(" close")
	lines = append(lines, hint)

	// A top rule separates the palette from the content above; rendering on the
	// terminal's own background avoids the colour-reset banding that an outer
	// Background() over styled lines would cause.
	rule := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Div2)).Render(strings.Repeat("─", components.FrameWidth()))
	indented := make([]string, len(lines))
	for i, l := range lines {
		indented[i] = "  " + l
	}
	return rule + "\n" + strings.Join(indented, "\n")
}
