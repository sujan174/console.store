package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"consolestore/internal/catalog"
	"consolestore/internal/tui/components"
	"consolestore/internal/tui/theme"
	"consolestore/internal/version"
)

// plural renders "1 result" / "N results".
func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// Version is the build tag shown in the brand header, sourced from the stamped
// build identity (internal/version). "dev" on unstamped local builds.
var Version = version.Version

type Menu struct {
	places          []catalog.Place
	address         catalog.Address
	section         catalog.Section
	usual           catalog.Usual
	hasUsual        bool
	cartChip        string
	counts          map[catalog.Section]int
	list            components.List
	searching       bool
	chipLabels      []string
	chipActive      int
	catHeader       string // section header for a category flat list (e.g. "Coffee")
	hideSectionTabs bool
	// two-pane live fields (only active when hasRail is true)
	rail            Rail
	hasRail         bool
	railFocus       bool
	usuals          []catalog.Place
	nearby          []catalog.Place
	hasSections     bool
	loading         bool // an initial load is in flight (list empty → full-pane loader)
	loaded          bool // a load for this view has reached a terminal state at least once
	loadingMore     bool // pagination is still fetching more rows below (foot-of-list spinner)
	searchMode      bool
	searchPending   bool
	searchQuery     string
	searchCaret     int    // caret position in searchQuery, in runes
	searchCorrected string // non-empty when results came from a spell-correction
	results         []catalog.Place
	resultCount     int
	animFrame       int // global tick frame — drives the loading scene
	animHour        int // local hour — flips loaders to late-night copy
}

func NewMenu(places []catalog.Place, addr catalog.Address, section catalog.Section, usual catalog.Usual, hasUsual bool, cartChip string) Menu {
	rows := make([]components.Row, len(places))
	for i, p := range places {
		rows[i] = components.Row{
			Left:  theme.ItemStyle.Render(p.Name),
			Right: theme.EtaStyle.Render(p.ETA),
		}
	}
	return Menu{
		places:   places,
		address:  addr,
		section:  section,
		usual:    usual,
		hasUsual: hasUsual,
		cartChip: cartChip,
		list:     components.List{Rows: rows, Cursor: 0},
	}
}

// Selected returns the place under the cursor (false if the list is empty).
// When a rail is set (live two-pane mode) the cursor maps into mainPlaces()
// so selection crosses usuals/nearby/results seamlessly.
func (m Menu) Selected() (catalog.Place, bool) {
	src := m.places
	if m.hasRail {
		src = m.mainPlaces()
	}
	i := m.list.Cursor
	if m.hasRail {
		// In two-pane mode cursor is a direct index into mainPlaces().
		if i < 0 || i >= len(src) {
			return catalog.Place{}, false
		}
		return src[i], true
	}
	// Mock path: use the list's filtered SelectedIndex.
	i = m.list.SelectedIndex()
	if i < 0 || i >= len(src) {
		return catalog.Place{}, false
	}
	return src[i], true
}

// WithCartTotal returns a copy with an updated cart total, preserving the cursor.
func (m Menu) WithCartChip(s string) Menu { m.cartChip = s; return m }

// ListCursor returns the current cursor position in the list.
func (m Menu) ListCursor() int { return m.list.Cursor }

// WithListCursor sets the list cursor position (used by the root for live rail nav).
func (m Menu) WithListCursor(i int) Menu {
	if i < 0 {
		i = 0
	}
	places := m.mainPlaces()
	if len(places) > 0 && i >= len(places) {
		i = len(places) - 1
	}
	m.list.Cursor = i
	return m
}

// WithMaxRows sets the list viewport height (rows). 0 = show all.
func (m Menu) WithMaxRows(n int) Menu { m.list.MaxRows = n; return m }

// WithCounts sets the per-section place counts shown on the tab bar.
func (m Menu) WithCounts(c map[catalog.Section]int) Menu { m.counts = c; return m }

// WithSectionTabsHidden hides the coffee/food/snacks tab row. Use in live mode
// where the cuisine-chip row replaces it; leave false (default) for mock mode.
func (m Menu) WithSectionTabsHidden(v bool) Menu { m.hideSectionTabs = v; return m }

// WithChips sets the cuisine chip labels and the active (highlighted) chip index.
func (m Menu) WithChips(labels []string, active int) Menu {
	m.chipLabels = labels
	m.chipActive = active
	return m
}

// WithCategoryHeader sets the section header shown above a category's flat list
// (so categories read consistently with Home's "popular near you" divider).
func (m Menu) WithCategoryHeader(label string) Menu { m.catHeader = label; return m }

// sectionRule renders a centered "── label ──" divider, matching the Home
// section headers.
func sectionRule(label string) string {
	rule := theme.Fg(theme.Div2).Render(strings.Repeat("─", 20))
	return "  " + rule + " " + theme.DimStyle.Render(label) + " " + rule + "\n"
}

// ChipCount returns the number of cuisine chips.
func (m Menu) ChipCount() int { return len(m.chipLabels) }

// WithRail attaches the left rail to the menu, enabling the two-pane render
// path. When a rail is set, View() renders the two-pane layout; without it
// the existing single-pane (mock) render runs unchanged.
func (m Menu) WithRail(r Rail) Menu { m.rail = r; m.hasRail = true; return m }

// WithRailFocus sets whether the rail column has keyboard focus (the main list
// is focused when false).
func (m Menu) WithRailFocus(f bool) Menu { m.railFocus = f; return m }

// WithSections sets the Home view's usuals + nearby slices. The usuals block
// is omitted from View() entirely when usuals is empty. Clears search mode.
func (m Menu) WithSections(usuals, nearby []catalog.Place) Menu {
	m.usuals = usuals
	m.nearby = nearby
	m.hasSections = true
	m.searchMode = false
	return m
}

// WithSearchMode sets the live search state. When active is true the main pane
// shows the search input, result count, and result list. Clears sections view.
func (m Menu) WithSearchMode(active bool, query string, results []catalog.Place, count int, pending bool) Menu {
	m.searchMode = active
	m.searchQuery = query
	m.results = results
	m.resultCount = count
	m.searchPending = pending
	if active {
		m.hasSections = false
	}
	return m
}

// WithLoading marks the flat (category) list as still loading, so an empty list
// shows a "loading…" cue instead of "no restaurants" while results stream in.
func (m Menu) WithLoading(loading bool) Menu { m.loading = loading; return m }

// WithLoaded marks that a load for this view has completed at least once (ok or
// error). Until then an empty list stays a loader — never a "nothing here"
// flash before the first fetch has even landed.
func (m Menu) WithLoaded(loaded bool) Menu { m.loaded = loaded; return m }

// WithLoadingMore marks that more rows are still streaming in below the ones
// already painted, so a non-empty list shows the foot-of-list spinner.
func (m Menu) WithLoadingMore(more bool) Menu { m.loadingMore = more; return m }

// WithAnim carries the global frame + local hour into the loading scenes.
// Chained at render time by the root (like WithMaxRows).
func (m Menu) WithAnim(frame, hour int) Menu { m.animFrame, m.animHour = frame, hour; return m }

// paneW is the width of the pane a loading scene should center in: the main
// column right of the rail in two-pane mode, the full content width otherwise.
func (m Menu) paneW() int {
	if m.hasRail {
		return components.ContentWidth() - railWidth - 2
	}
	return components.ContentWidth()
}

// WithSearchCaret sets the caret position (in runes) for the search input.
func (m Menu) WithSearchCaret(caret int) Menu { m.searchCaret = caret; return m }

// WithSearchCorrected notes the spelling Swiggy was searched with when the typed
// query found nothing and a correction matched (shown as "showing results for…").
func (m Menu) WithSearchCorrected(s string) Menu { m.searchCorrected = s; return m }

// searchInputLine renders the 🔍 search field with a block caret drawn at the
// caret position, so ←/→ editing is visible mid-string.
func (m Menu) searchInputLine() string {
	r := []rune(m.searchQuery)
	c := m.searchCaret
	if c < 0 {
		c = 0
	}
	if c > len(r) {
		c = len(r)
	}
	before := string(r[:c])
	at := " "
	after := ""
	if c < len(r) {
		at = string(r[c]) // caret sits ON this char (reverse video)
		after = string(r[c+1:])
	}
	caret := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Bg)).
		Background(lipgloss.Color(theme.Cursor)).
		Render(at)
	return theme.CursorStyle.Render("⌕ "+before) + caret + theme.CursorStyle.Render(after)
}

// mainPlaces is the flat, cursor-addressable slice for the active view:
// search → results; Home (hasSections) → usuals then nearby; else → places.
// The components.List rows must be in the same order.
func (m Menu) mainPlaces() []catalog.Place {
	switch {
	case m.searchMode:
		return m.results
	case m.hasSections:
		out := make([]catalog.Place, 0, len(m.usuals)+len(m.nearby))
		out = append(out, m.usuals...)
		out = append(out, m.nearby...)
		return out
	default:
		return m.places
	}
}

// placeRow renders one restaurant row, matching the in-restaurant dish list as
// the standard: the selected row (when the main pane has focus) gets a blue ▌
// border + "> " cursor + bright white name on the highlighted selected-row
// background. When focus is on the rail the cursor row is shown only faintly (a
// dim · marker, no highlight) so there is exactly one active cursor on screen.
func (m Menu) placeRow(p catalog.Place, selected bool) string {
	w := components.ContentWidth() - railWidth - 5
	if w < 16 {
		w = 16
	}

	// meta = rating★ + ETA, RIGHT-aligned to the row edge. Shown ONLY in search
	// results; on Home/category the focused restaurant's stats live in the detail
	// strip above the list, so the rows stay clean (just names). The compact
	// per-row form is unspaced ("4.7★") to stay distinct from the spaced strip
	// form ("4.7 ★") above.
	meta := ""
	if m.searchMode {
		rating := ""
		if p.Rating > 0 {
			rating = theme.Fg(theme.Gold).Render(fmt.Sprintf("%.1f★", p.Rating))
		}
		eta := ""
		if p.ETA != "" {
			eta = theme.EtaStyle.Render(p.ETA)
		}
		meta = rating
		if rating != "" && eta != "" {
			meta += "  " + eta
		} else {
			meta += eta
		}
	}
	metaW := lipgloss.Width(meta)

	const leadW = 4 // every lead ("▌ > ", "  · ", "    ") is 4 cells wide
	name := railTrunc2(p.Name, w-leadW-metaW-1)

	var lead, styledName string
	switch {
	case selected && !m.railFocus:
		lead = theme.CursorStyle.Render("▌ > ")
		styledName = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Bold(true).Render(name)
	case selected:
		lead = theme.FaintStyle.Render("  · ")
		styledName = theme.TextStyle.Render(name)
	default:
		lead = "    "
		styledName = theme.ItemStyle.Render(name)
	}

	pad := w - leadW - lipgloss.Width(name) - metaW
	if pad < 1 {
		pad = 1
	}
	content := lead + styledName + strings.Repeat(" ", pad) + meta
	if selected && !m.railFocus {
		return lipgloss.NewStyle().Background(lipgloss.Color(theme.SelRowBg)).Render(content)
	}
	return content
}

// railTrunc2 shortens s to at most max cells, adding an ellipsis. max<1 → "".
func railTrunc2(s string, max int) string {
	if max < 1 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

// windowRange computes the visible [start,end) slice of n items that keeps the
// cursor on screen within `budget` rows, reserving a line for each ↑/↓ "N more"
// indicator that is off-screen. Mirrors components.List's viewport math so the
// two-pane browse list scrolls identically to every other list in the app.
// budget <= 0 (size unknown) shows everything.
func windowRange(cursor, n, budget int) (start, end, above, below int) {
	start, end = 0, n
	if budget > 0 && n > budget {
		start = cursor - budget/2
		if start < 0 {
			start = 0
		}
		if start > n-budget {
			start = n - budget
		}
		end = start + budget
		above, below = start, n-end
		if above > 0 {
			start++
			above = start
		}
		if below > 0 {
			end--
			below = n - end
		}
		if cursor < start {
			start = cursor
		}
		if cursor >= end {
			end = cursor + 1
		}
	}
	return start, end, above, below
}

// mainSectionLabel returns the section-header label for the place at flat index i
// in the current browse view (Home: usuals/nearby; category: catHeader; else "").
func (m Menu) mainSectionLabel(i int) string {
	if m.hasSections {
		if len(m.usuals) > 0 && i < len(m.usuals) {
			return "your usuals"
		}
		return "popular near you"
	}
	return m.catHeader
}

// sectionHeaderCount is how many section headers the current view shows, so the
// row window can reserve their lines and the page fits the viewport.
func (m Menu) sectionHeaderCount() int {
	if m.hasSections {
		if len(m.usuals) > 0 {
			return 2
		}
		return 1
	}
	if m.catHeader != "" {
		return 1
	}
	return 0
}

// browseRows renders the windowed main-pane restaurant rows with section headers
// and ↑/↓ "N more" indicators, capping visible rows to `budget` so the page never
// overflows the viewport (overflow scrolls the brand header off the top). Spans
// usuals + nearby (Home) or a flat category list as one cursor list.
func (m Menu) browseRows(budget int) string {
	places := m.mainPlaces()
	if len(places) == 0 {
		// Empty list: keep the loader up until a fetch has actually completed, so
		// "no restaurants" never flashes in the corner before the first load lands.
		// Once loaded-and-still-empty, the note sits centered where the loader was.
		if m.loading || !m.loaded {
			return FoodLoading(m.animFrame, m.animHour, m.paneW(), budget)
		}
		return CenteredNote("no restaurants deliver here right now", m.paneW(), budget)
	}
	rowBudget := budget
	if rowBudget > 0 {
		if rowBudget -= m.sectionHeaderCount(); rowBudget < 3 {
			rowBudget = 3
		}
	}
	start, end, above, below := windowRange(m.list.Cursor, len(places), rowBudget)

	var b strings.Builder
	if above > 0 {
		b.WriteString("  " + theme.FaintStyle.Render(fmt.Sprintf("↑ %d more", above)) + "\n")
	}
	prev := ""
	for i := start; i < end; i++ {
		if lbl := m.mainSectionLabel(i); lbl != "" && lbl != prev {
			b.WriteString(sectionRule(lbl))
			prev = lbl
		}
		b.WriteString(m.placeRow(places[i], i == m.list.Cursor) + "\n")
	}
	if below > 0 {
		// More rows exist below the viewport — scroll cue, not a fetch state.
		b.WriteString("  " + theme.FaintStyle.Render(fmt.Sprintf("↓ %d more", below)) + "\n")
	} else if m.loadingMore {
		// At the foot of everything painted, but more pages are still streaming in.
		b.WriteString(LoadingMore(m.animFrame, m.paneW()))
	} else if m.loaded {
		// Nothing more coming — mark the end so the last page never looks cut off.
		b.WriteString(ListEnd(m.paneW()))
	}
	return b.String()
}

// twoPaneView renders the rail + main pane layout used in live mode.
// focusedDetail renders the highlighted restaurant's stats (★rating · ETA ·
// city · offer) as a strip above the list — the master/detail pattern that keeps
// per-row clutter off the browse list. "" when there's nothing to show.
func (m Menu) focusedDetail() string {
	places := m.mainPlaces()
	if len(places) == 0 {
		return ""
	}
	c := m.list.Cursor
	if c < 0 || c >= len(places) {
		c = 0
	}
	p := places[c]
	dot := theme.FaintStyle.Render("  ·  ")
	out := theme.BrightStyle.Bold(true).Render(p.Name)
	stats := ""
	add := func(s string) {
		if stats != "" {
			stats += dot
		}
		stats += s
	}
	if p.Rating > 0 {
		add(theme.GoldStyle.Render(fmt.Sprintf("%.1f ★", p.Rating)))
	}
	if p.ETA != "" {
		add(theme.DimStyle.Render(p.ETA))
	}
	if p.City != "" {
		add(theme.DimStyle.Render(p.City))
	}
	if p.Offer != "" {
		add(theme.GoldStyle.Render(p.Offer))
	}
	if stats != "" {
		out += "   " + stats
	}
	return "  " + out
}

// keycapHint renders a "<key> label" affordance: the key in a subtle grey cap,
// the label dim. Shared by the store switcher and the focused detail strip so
// their two right-aligned hints look like one family.
func keycapHint(key, label string) string {
	// Key glyph reads brighter than its label (Dim > Faint), no highlight.
	return theme.DimStyle.Render(key) + theme.FaintStyle.Render(" "+label)
}

// verticalTabs renders the top-level store switcher (Food ⟷ Instamart) — a
// full-width row above the rail/main split, shared by both live verticals so
// their switchers can never drift apart. active is 0 for Food, 1 for
// Instamart; the active side renders as a solid gold pill, the other dim.
// Both verticals are live now — neither side carries a "soon" tag.
func verticalTabs(active int) string {
	w := components.ContentWidth()
	pill := func(label string) string {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.Bg)).
			Background(lipgloss.Color(theme.Gold)).
			Bold(true).Render(" " + label + " ")
	}
	dim := func(label string) string { return theme.CatOffStyle.Render(label) }

	var food, inst string
	if active == 1 {
		food = dim("Food")
		inst = pill("INSTAMART")
	} else {
		food = pill("FOOD")
		inst = dim("Instamart")
	}

	hint := keycapHint("tab", "switch")
	left := "  " + food + "    " + inst
	gap := w - lipgloss.Width(left) - lipgloss.Width(hint) - 2
	if gap < 2 {
		gap = 2
	}
	return left + strings.Repeat(" ", gap) + hint + "\n\n"
}

func (m Menu) twoPaneView() string {
	// budget = rows available for the main list (set by the root via WithMaxRows).
	// 0 = size unknown → show all.
	// budget = rows available for the main list (set by the root via WithMaxRows).
	// 0 = size unknown → show all.
	budget := m.list.MaxRows

	var main strings.Builder

	switch {
	case m.searchMode:
		main.WriteString(m.searchInputLine() + "\n")
		switch {
		case m.searchPending:
			// A query is in flight (search paginates, so it can take a moment).
			main.WriteString(theme.GoldStyle.Render("searching…") + "\n")
		case m.searchQuery == "":
			main.WriteString(theme.DimStyle.Render("type to search restaurants, ↵ to search") + "\n")
		case len(m.results) == 0:
			main.WriteString(theme.DimStyle.Render(fmt.Sprintf(`no restaurants for "%s"`, m.searchQuery)) + "\n")
		case m.searchCorrected != "":
			main.WriteString(theme.DimStyle.Render("showing results for ") +
				theme.GoldStyle.Render(`"`+m.searchCorrected+`"`) + "\n\n")
		default:
			main.WriteString(theme.DimStyle.Render(plural(m.resultCount, "result", "results")) + "\n\n")
		}
		if !m.searchPending && len(m.results) > 0 {
			// Window the results too — the input + status line above eat ~2 rows.
			rb := budget
			if rb > 0 {
				if rb -= 2; rb < 3 {
					rb = 3
				}
			}
			start, end, above, below := windowRange(m.list.Cursor, len(m.results), rb)
			if above > 0 {
				main.WriteString("  " + theme.FaintStyle.Render(fmt.Sprintf("↑ %d more", above)) + "\n")
			}
			for i := start; i < end; i++ {
				main.WriteString(m.placeRow(m.results[i], i == m.list.Cursor) + "\n")
			}
			if below > 0 {
				main.WriteString("  " + theme.FaintStyle.Render(fmt.Sprintf("↓ %d more", below)) + "\n")
			}
		}
	default:
		// Non-search browse: the focused restaurant's stats (rating · ETA ·
		// location) sit in a detail strip above the list, so the rows stay clean.
		// The detail strip eats one row from the list budget.
		rb := budget
		if d := m.focusedDetail(); d != "" {
			main.WriteString(d + "\n\n")
			if rb > 0 {
				if rb -= 2; rb < 3 {
					rb = 3
				}
			}
		}
		main.WriteString(m.browseRows(rb))
	}

	mainStr := main.String()

	// Size the rail to the main pane's ACTUAL height so the divider ends with the
	// list — no blank rows padded under either column. Floor at the rail's own
	// entry count so every category stays visible when the list is short.
	railH := lipgloss.Height(mainStr)
	if railH < m.rail.Len()+1 {
		railH = m.rail.Len() + 1
	}
	left := m.rail.WithFocus(m.railFocus).WithActive(m.rail.Active()).WithHeight(railH).View()

	// No extra indent before the main pane — each row already leads with a
	// 4-cell cursor column, which is enough gap from the rail divider.
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, mainStr)
	// Trailing key-affordance line — floats after a blank so the root's
	// splitHint lifts it to sit with the status bar (same pattern as the
	// restaurant screen). The mock single-pane path renders its own below.
	hint := components.Hint("↑↓", "move", "↵", "open", "/", "search", "i", "info", "c", "cart") +
		"   " + theme.PurpleStyle.Render(":") + " " + theme.FaintStyle.Render("cmd")
	// The store switcher (Food ⟷ Instamart) sits above the rail/main split.
	return verticalTabs(0) + body + "\n\n" + hint
}

func (m Menu) Init() tea.Cmd { return nil }

func (m Menu) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.searching {
		switch k.String() {
		case "esc":
			m.searching = false
			m.list.SetFilter("")
		case "enter":
			m.searching = false
		case "backspace":
			f := m.list.Filter()
			if f != "" {
				m.list.SetFilter(f[:len(f)-1])
			}
		default:
			if k.Type == tea.KeyRunes {
				m.list.SetFilter(m.list.Filter() + string(k.Runes))
			}
		}
		return m, nil
	}
	switch k.String() {
	case "/":
		m.searching = true
	case "j", "down":
		m.list.Down()
	case "k", "up":
		m.list.Up()
	}
	return m, nil
}

// Searching reports whether the menu is in search-input mode.
func (m Menu) Searching() bool { return m.searching }

// justify spreads left and right across width with the gap padded by spaces.
func justify(left, right string, width int) string {
	pad := width - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + right
}

func (m Menu) View() string {
	// Two-pane live render: rail column + sectioned main pane.
	// This branch is ONLY taken when a rail has been attached via WithRail().
	// The mock single-pane path below runs byte-for-byte as before.
	if m.hasRail {
		return m.twoPaneView()
	}

	var b strings.Builder
	w := components.ContentWidth()

	// row 1: deliver to ⊕ <addr> · <label> ⌄  (the brand logo is rendered as a
	// centered banner above every screen by the root, so it isn't repeated here).
	deliver := theme.DimStyle.Render("deliver to ") + theme.CursorStyle.Render("⊕ ") +
		theme.BrightStyle.Render(m.address.Line) + theme.DimStyle.Render(" · "+m.address.Label) +
		theme.FaintStyle.Render(" ⌄")
	b.WriteString("  " + justify("", deliver, w) + "\n")

	b.WriteString("\n")

	// tab bar with per-section counts + cart chip (suppressed in live mode where
	// the cuisine-chip row below replaces it):
	//   coffee 4 │ food 5 │ quick snacks 5            🛒 cart empty
	cartStyle := theme.CartStyle
	if strings.Contains(m.cartChip, "empty") {
		cartStyle = theme.DimStyle
	}
	if !m.hideSectionTabs {
		labels := map[catalog.Section]string{
			catalog.SectionCoffee: "coffee",
			catalog.SectionFood:   "food",
			catalog.SectionSnacks: "quick snacks",
		}
		var tabs []string
		for _, s := range catalog.MenuSections {
			cnt := theme.DimStyle.Render(fmt.Sprintf(" %d", m.counts[s]))
			if s == m.section {
				tabs = append(tabs, theme.Fg(theme.Gold).Underline(true).Render(labels[s])+cnt)
			} else {
				tabs = append(tabs, theme.CatOffStyle.Render(labels[s])+cnt)
			}
		}
		sep := theme.Fg(theme.Div2).Render(" │ ")
		b.WriteString("  " + justify(strings.Join(tabs, sep), cartStyle.Render(m.cartChip), w) + "\n")
	} else {
		// In live mode the section tabs are hidden; still render the cart chip
		// right-aligned so it stays visible on the browse screen.
		b.WriteString("  " + justify("", cartStyle.Render(m.cartChip), w) + "\n")
	}

	b.WriteString("\n")

	// vertical-toggle row + cuisine chip row — rendered only in live mode (when
	// chips have been set via WithChips). Mock path (no chips) is unchanged.
	if len(m.chipLabels) > 0 {
		// vertical toggle: Restaurants (active) · Instamart soon   tab ·
		vertSep := theme.Fg(theme.Div2).Render("  ·  ")
		activeV := theme.Fg(theme.Gold).Underline(true).Render("Restaurants")
		inactiveV := theme.CatOffStyle.Render("Instamart") + "  " + theme.FaintStyle.Render("soon")
		tabHint := theme.FaintStyle.Render("tab ·")
		b.WriteString("  " + activeV + vertSep + inactiveV + "   " + tabHint + "\n")

		// cuisine chip row — scrolls horizontally (windowed around the active chip
		// with ‹ / › markers) so a long chip list never truncates.
		b.WriteString("  " + windowedBar(m.chipLabels, m.chipActive, components.ContentWidth()-4, " │ ") + "\n")
		b.WriteString("\n")
	}

	// search prompt (when active)
	if m.searching || m.list.Filter() != "" {
		b.WriteString("  " + theme.CursorStyle.Render("/"+m.list.Filter()) + "\n")
	}

	if len(m.places) == 0 && !m.hasUsual {
		b.WriteString("  " + theme.DimStyle.Render("no curated spots deliver here right now") + "\n")
	} else {
		b.WriteString(m.list.View())
	}

	b.WriteString("\n\n\n") // padding below the list

	hint := components.Hint("↑↓", "move", "←→", "category", "↵", "open", "i", "info", "a", "address", "c", "cart") +
		"   " + theme.PurpleStyle.Render(":") + " " + theme.FaintStyle.Render("cmd")
	b.WriteString(hint)

	return b.String()
}
