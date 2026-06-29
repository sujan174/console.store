package screens

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"consolestore/internal/tui/components"
	"consolestore/internal/tui/theme"
)

// streak mirrors the design's this.state.streak (script line 712).
const streak = 7

// CmdLine is one rendered output line: text plus a theme color hex.
type CmdLine struct{ Text, Color string }

// CmdBar is the vim-style `:` command palette (design lines 446-457, runCmd 659-736).
// It holds the in-progress text, a caret position (in runes) for mid-string
// editing, and the prior output lines (last ~9).
type CmdBar struct {
	text  string
	caret int // caret position in text, in runes
	out   []CmdLine
}

func NewCmdBar() CmdBar { return CmdBar{} }

func (c CmdBar) Text() string { return c.text }

func (c CmdBar) WithText(s string) CmdBar {
	c.text = s
	c.caret = len([]rune(s))
	return c
}

// clamp keeps the caret within [0, len(runes)] — makes a stale caret left over
// from a prior Run (which clears text) harmless before the next edit.
func (c CmdBar) clamp() CmdBar {
	n := len([]rune(c.text))
	if c.caret < 0 {
		c.caret = 0
	}
	if c.caret > n {
		c.caret = n
	}
	return c
}

// Insert adds s at the caret and advances the caret past it.
func (c CmdBar) Insert(s string) CmdBar {
	c = c.clamp()
	r := []rune(c.text)
	ins := []rune(s)
	out := append(append(append([]rune{}, r[:c.caret]...), ins...), r[c.caret:]...)
	c.text = string(out)
	c.caret += len(ins)
	return c
}

// Append adds s at the end and moves the caret to the end (caret-agnostic helper).
func (c CmdBar) Append(s string) CmdBar {
	c.text += s
	c.caret = len([]rune(c.text))
	return c
}

// Backspace deletes the rune before the caret.
func (c CmdBar) Backspace() CmdBar {
	c = c.clamp()
	if c.caret == 0 {
		return c
	}
	r := []rune(c.text)
	c.text = string(r[:c.caret-1]) + string(r[c.caret:])
	c.caret--
	return c
}

// Delete removes the rune at the caret (forward delete).
func (c CmdBar) Delete() CmdBar {
	c = c.clamp()
	r := []rune(c.text)
	if c.caret >= len(r) {
		return c
	}
	c.text = string(r[:c.caret]) + string(r[c.caret+1:])
	return c
}

// Left moves the caret one rune left.
func (c CmdBar) Left() CmdBar {
	c = c.clamp()
	if c.caret > 0 {
		c.caret--
	}
	return c
}

// Right moves the caret one rune right.
func (c CmdBar) Right() CmdBar {
	c = c.clamp()
	if c.caret < len([]rune(c.text)) {
		c.caret++
	}
	return c
}

// Home / End jump the caret to the start / end of the text.
func (c CmdBar) Home() CmdBar { c.caret = 0; return c }
func (c CmdBar) End() CmdBar  { c.caret = len([]rune(c.text)); return c }

func (c CmdBar) ClearText() CmdBar { c.text = ""; c.caret = 0; return c }

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
			{"  sl         choo choo", D},
			{"  sudo …     go on, try it", D},
			{"  vim        enter (you can leave)", D},
			{"  42         the answer", D},
			{"  clear      wipe screen · exit  close", D},
		}
	case "neofetch":
		out = []CmdLine{
			{"      ▟▙       guest@consolestore.in", B},
			{"     ▟██▙      ─────────────────────", D},
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
	case "alias":
		// Side-effecting command: the root performs capture/list/rm and appends
		// the result lines via AppendOut. Echo the command, emit no body here.
		echo := CmdLine{": " + raw, theme.Faint}
		c.out = append(c.out, echo)
		if len(c.out) > 13 {
			c.out = c.out[len(c.out)-13:]
		}
		c.text = ""
		// Return the args after "alias" (preserve original case for names).
		rest := strings.TrimSpace(raw[len("alias"):])
		return c, "alias " + rest
	case "exit", "quit", ":q":
		out = []CmdLine{
			{"connection to consolestore.in closed.", D},
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

// AppendOut appends output lines (used by the root for side-effecting commands
// like `alias`, whose result is computed outside the bar) and keeps the window.
func (c CmdBar) AppendOut(lines []CmdLine) CmdBar {
	c.out = append(c.out, lines...)
	if len(c.out) > 13 {
		c.out = c.out[len(c.out)-13:]
	}
	return c
}

// input renders the in-progress text with a block caret drawn AT the caret
// position (reverse video on the rune under it, a trailing block at the end),
// so ←/→ editing is visible mid-string — matching the search field. When blink
// is off the caret cell shows the plain glyph so the text never strobe-hides.
func (c CmdBar) input(blink bool) string {
	c = c.clamp()
	r := []rune(c.text)
	before := string(r[:c.caret])
	at := " "
	after := ""
	if c.caret < len(r) {
		at = string(r[c.caret])
		after = string(r[c.caret+1:])
	}
	caret := at
	if blink {
		caret = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.Bg)).
			Background(lipgloss.Color(theme.Cursor)).
			Render(at)
	}
	return theme.BrightStyle.Render(before) + caret + theme.BrightStyle.Render(after)
}

// View renders the palette body (output lines, prompt with blinking cursor, hint)
// on the PanelCmd background. Width matches the content column.
func (c CmdBar) View(blink bool) string {
	var lines []string
	for _, l := range c.out {
		lines = append(lines, theme.Fg(l.Color).Render(l.Text))
	}

	prompt := theme.PurpleStyle.Render(":") + " " + c.input(blink)
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
