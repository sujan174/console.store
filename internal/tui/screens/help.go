package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"consolestore/internal/tui/theme"
)

// Help is the in-app help & docs modal (opened with ? / H, or :help). It is a
// passive value type: the root holds the page, scroll offset, and terminal height
// and rebuilds it each frame. The content is organized into discrete pages.
// It windows to the viewport height (scrollable per-page) so it never overflows.
type Help struct {
	page      int
	scroll    int
	viewportH int
}

func NewHelp() Help { return Help{} }

// WithViewport sets the terminal height so the card windows to fit.
func (h Help) WithViewport(v int) Help { h.viewportH = v; return h }

// WithScroll sets the scroll offset (clamped in View).
func (h Help) WithScroll(s int) Help { h.scroll = s; return h }

// WithPage sets the page (0-indexed, clamped in View).
func (h Help) WithPage(n int) Help { h.page = n; return h }

const helpCardWidth = 60

// helpers for content formatting.
func helpSec(s string) string { return theme.GoldStyle.Bold(true).Render(s) }
func helpTxt(s string) string { return theme.DimStyle.Render(s) }
func helpKv(key, desc string) string {
	pad := 12 - lipgloss.Width(key)
	if pad < 1 {
		pad = 1
	}
	return "  " + theme.Fg(theme.Cursor).Render(key) + strings.Repeat(" ", pad) + theme.TextStyle.Render(desc)
}

// helpPages returns all pages of help content. Each page is a []string of lines.
// Page indices are 0-based. HelpPageCount() returns the number of pages.
func helpPages() [][]string {
	blank := ""

	// Page 0: Welcome / contents
	page0 := []string{
		helpTxt("order real food from your terminal, powered by Swiggy."),
		blank,
		helpSec("safety"),
		helpTxt("  orders are real — can't cancel here, call Swiggy."),
		helpTxt("  pay the rider on delivery (cash on delivery / COD)."),
		blank,
		helpSec("quick keys"),
		helpKv("esc", "back a step      esc esc  jump home"),
		helpKv("← →", "next / prev page"),
		blank,
		helpSec("pages"),
		helpTxt("  2  move & select"),
		helpTxt("  3  browse & inside a restaurant"),
		helpTxt("  4  cart, checkout & tracking"),
		helpTxt("  5  aliases & the shell"),
	}

	// Page 1: Move & select
	page1 := []string{
		helpSec("move & select"),
		blank,
		helpKv("↑ ↓ k j", "move the cursor / list"),
		helpKv("← → h l", "switch column · category · quantity"),
		helpKv("↵", "select · confirm · add"),
		helpKv("esc", "back a step      esc esc   jump home"),
		helpKv("tab", "switch Restaurants ⟷ Instamart"),
		helpKv("ctrl-c", "quit"),
	}

	// Page 2: Browse & inside a restaurant
	page2 := []string{
		helpSec("browse restaurants"),
		blank,
		helpKv("/", "search restaurants"),
		helpKv("i", "restaurant info       c   open cart"),
		helpKv("a", "change delivery address"),
		blank,
		helpSec("inside a restaurant"),
		blank,
		helpKv("↵ +", "add the dish          −   remove one"),
		helpKv("← →", "change category       /   search dishes"),
		helpKv("v", "veg only              i   dish info"),
		helpKv("c", "open cart             esc back"),
	}

	// Page 3: Cart, checkout & tracking
	page3 := []string{
		helpSec("cart & checkout"),
		blank,
		helpKv("↑ ↓", "pick a line"),
		helpKv("← → + −", "change quantity       ⌫   remove the line"),
		helpKv("↵", "place the order (cash on delivery)"),
		blank,
		helpSec("tracking"),
		blank,
		helpKv("d", "dismiss a delivered order"),
		helpKv("esc", "back"),
	}

	// Page 4: Aliases & the shell
	page4 := []string{
		helpSec("command palette   —   press :"),
		blank,
		helpKv(":alias set", "save the current cart as an order"),
		helpKv(":alias list", "show your saved orders"),
		helpKv(":alias rm", "remove a saved order  ·  <name> [n]"),
		helpKv(":help", "open this help screen"),
		blank,
		helpSec("aliases  →  reorder from your shell"),
		blank,
		helpTxt("  an alias is a saved cart: a restaurant, a delivery"),
		helpTxt("  address, and its items — reorder in one command."),
		blank,
		helpTxt("  console order <name>   place a saved order"),
		helpTxt("  console status         live order + ETA"),
	}

	return [][]string{page0, page1, page2, page3, page4}
}

// HelpPageCount returns the total number of help pages.
func HelpPageCount() int {
	return len(helpPages())
}

// innerRows is how many content rows fit inside the card for the viewport. The
// card chrome (border, title, footer, blanks) is ~7 rows. 0/unknown → show all.
func (h Help) innerRowsForPage(page int) int {
	pages := helpPages()
	if page < 0 {
		page = 0
	}
	if page >= len(pages) {
		page = len(pages) - 1
	}
	contentLen := len(pages[page])
	if h.viewportH <= 0 {
		return contentLen
	}
	if n := h.viewportH - 7; n >= 4 {
		return n
	}
	return 4
}

// HelpMaxScroll is the largest valid scroll offset for the given height, so the
// root can clamp its stored offset. It clamps against page 0 (the first page)
// for backwards compatibility — the root should use per-page scroll clamping
// via WithPage + View's internal clamping.
func HelpMaxScroll(viewportH int) int {
	h := Help{viewportH: viewportH}
	pages := helpPages()
	if len(pages) == 0 {
		return 0
	}
	contentLen := len(pages[0])
	rows := h.innerRowsForPage(0)
	if d := contentLen - rows; d > 0 {
		return d
	}
	return 0
}

func (h Help) View() string {
	pages := helpPages()
	pageCount := len(pages)

	// Clamp page.
	pg := h.page
	if pg < 0 {
		pg = 0
	}
	if pg >= pageCount {
		pg = pageCount - 1
	}

	content := pages[pg]
	rows := h.innerRowsForPage(pg)

	// Window the content around the scroll offset.
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

	// Footer: page indicator + controls.
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
		Width(helpCardWidth).
		Render(inner.String())
	return card
}
