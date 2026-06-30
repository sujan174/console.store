package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"consolestore/internal/tui/theme"
)

// Help is the in-app help & docs modal (opened with ? / H, or :help). It is a
// passive value type: the root holds the scroll offset and terminal height and
// rebuilds it each frame. The content is a fixed reference — a short intro plus
// the real keybindings for every screen and the alias/preset workflow. It
// windows to the viewport height (scrollable) so it never overflows.
type Help struct {
	scroll    int
	viewportH int
}

func NewHelp() Help { return Help{} }

// WithViewport sets the terminal height so the card windows to fit.
func (h Help) WithViewport(v int) Help { h.viewportH = v; return h }

// WithScroll sets the scroll offset (clamped in View).
func (h Help) WithScroll(s int) Help { h.scroll = s; return h }

const helpCardWidth = 60

// helpContent is the full reference, pre-styled, one entry per line. Keys are
// the ACTUAL bindings from the root's key router — keep them in sync with it.
func helpContent() []string {
	sec := func(s string) string { return theme.GoldStyle.Bold(true).Render(s) }
	txt := func(s string) string { return theme.DimStyle.Render(s) }
	kv := func(key, desc string) string {
		pad := 12 - lipgloss.Width(key)
		if pad < 1 {
			pad = 1
		}
		return "  " + theme.Fg(theme.Cursor).Render(key) + strings.Repeat(" ", pad) + theme.TextStyle.Render(desc)
	}
	blank := ""

	return []string{
		txt("order real food from your terminal, powered by Swiggy."),
		txt("browse → cart → checkout, then pay the rider (COD)."),
		txt("orders are real & can't be cancelled here — call Swiggy."),
		blank,
		sec("move & select"),
		kv("↑ ↓ k j", "move the cursor / list"),
		kv("← → h l", "switch column · category · quantity"),
		kv("↵", "select · confirm · add"),
		kv("esc", "back a step      esc esc   jump home"),
		kv("tab", "switch Restaurants ⟷ Instamart"),
		kv("ctrl-c", "quit"),
		blank,
		sec("browse restaurants"),
		kv("/", "search restaurants"),
		kv("i", "restaurant info       c   open cart"),
		kv("a", "change delivery address"),
		blank,
		sec("inside a restaurant"),
		kv("↵ +", "add the dish          −   remove one"),
		kv("← →", "change category       /   search dishes"),
		kv("v", "veg only              i   dish info"),
		kv("c", "open cart             esc back"),
		blank,
		sec("cart & checkout"),
		kv("↑ ↓", "pick a line"),
		kv("← → + −", "change quantity       ⌫   remove the line"),
		kv("↵", "place the order (cash on delivery)"),
		blank,
		sec("tracking"),
		kv("d", "dismiss a delivered order   esc  back"),
		blank,
		sec("command palette   —   press :"),
		kv(":alias set", "save the current cart as an order"),
		kv(":alias list", "show your saved orders"),
		kv(":alias rm", "remove a saved order  ·  <name> [n]"),
		kv(":help", "open this help screen"),
		blank,
		sec("aliases  →  reorder from your shell"),
		txt("  console order <name>   place a saved order (asks first)"),
		txt("  console status         your live order + ETA"),
		txt("  an alias is a saved cart: a restaurant, a delivery"),
		txt("  address, and its items — reorder in one command."),
	}
}

// innerRows is how many content rows fit inside the card for the viewport. The
// card chrome (border, title, footer, blanks) is ~7 rows. 0/unknown → show all.
func (h Help) innerRows() int {
	if h.viewportH <= 0 {
		return len(helpContent())
	}
	if n := h.viewportH - 7; n >= 4 {
		return n
	}
	return 4
}

// HelpMaxScroll is the largest valid scroll offset for the given height, so the
// root can clamp its stored offset.
func HelpMaxScroll(viewportH int) int {
	h := Help{viewportH: viewportH}
	if d := len(helpContent()) - h.innerRows(); d > 0 {
		return d
	}
	return 0
}

func (h Help) View() string {
	content := helpContent()
	rows := h.innerRows()

	// Window the content around the scroll offset, reserving a line for each
	// ↑/↓ "more" indicator that is off-screen.
	scroll := h.scroll
	if max := len(content) - rows; scroll > max {
		scroll = max
	}
	if scroll < 0 {
		scroll = 0
	}
	end := scroll + rows
	if end > len(content) {
		end = len(content)
	}
	above, below := scroll, len(content)-end

	var inner strings.Builder
	title := theme.BrandStyle.Render("help") + theme.FaintStyle.Render("  ·  consolestore")
	inner.WriteString(title + "\n\n")
	if above > 0 {
		inner.WriteString("  " + theme.FaintStyle.Render(fmt.Sprintf("↑ %d more", above)) + "\n")
	}
	for _, line := range content[scroll:end] {
		inner.WriteString(line + "\n")
	}
	if below > 0 {
		inner.WriteString("  " + theme.FaintStyle.Render(fmt.Sprintf("↓ %d more", below)) + "\n")
	}
	foot := theme.FaintStyle.Render("↑↓ scroll")
	if above == 0 && below == 0 {
		foot = theme.FaintStyle.Render("")
	}
	inner.WriteString("\n" + foot + theme.FaintStyle.Render("   ·   esc / ? close"))

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.Gold)).
		Padding(0, 2).
		Width(helpCardWidth).
		Render(inner.String())
	return card
}
