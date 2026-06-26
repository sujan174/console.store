package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"console.store/internal/catalog"
	"console.store/internal/tui/theme"
)

// Customize is the modal shown when adding an item that has add-ons. It has two
// modes:
//
//   - Grouped (live): item.Options carries Swiggy variant/addon GROUPS. Variant
//     groups (and any Max==1 group) are single-choice (radio); multi groups
//     respect Max. Required groups (Min>0) must be satisfied to confirm.
//   - Flat (mock): item.AddOns is a list of independent toggles.
//
// The root routes keys to Up/Down/Toggle and reads the result on confirm.
type Customize struct {
	item catalog.Item

	// flat (mock) mode
	selected []bool // parallel to item.AddOns

	// grouped (live) mode
	groups []catalog.OptionGroup
	picked map[string]map[string]bool // groupID -> choiceID -> on
	rows   []optRow                   // flattened selectable rows

	cursor    int
	viewportH int // terminal height; 0 = no windowing (render all)
}

// WithViewport sets the terminal height so a long option list scrolls within the
// viewport instead of overflowing off the top.
func (c Customize) WithViewport(h int) Customize { c.viewportH = h; return c }

// optRow references one choice within a group, for cursor navigation.
type optRow struct {
	group  int
	choice int
}

// NewCustomize builds the modal. Grouped mode is used when item.Options is set;
// required single-choice groups pre-select their first choice.
func NewCustomize(item catalog.Item) Customize {
	c := Customize{item: item}
	if len(item.Options) == 0 {
		c.selected = make([]bool, len(item.AddOns))
		return c
	}
	c.groups = item.Options
	c.picked = make(map[string]map[string]bool, len(item.Options))
	for gi, g := range item.Options {
		c.picked[g.ID] = map[string]bool{}
		if g.Min >= 1 && g.Max == 1 && len(g.Choices) > 0 {
			c.picked[g.ID][g.Choices[0].ID] = true // sensible default for required single-choice
		}
		for ci := range g.Choices {
			c.rows = append(c.rows, optRow{group: gi, choice: ci})
		}
	}
	return c
}

func (c Customize) grouped() bool { return len(c.groups) > 0 }

// Item returns the item being customised.
func (c Customize) Item() catalog.Item { return c.item }

func (c Customize) clampCursor() Customize {
	n := len(c.item.AddOns)
	if c.grouped() {
		n = len(c.rows)
	}
	if c.cursor < 0 {
		c.cursor = 0
	}
	if c.cursor >= n {
		c.cursor = n - 1
	}
	if c.cursor < 0 {
		c.cursor = 0
	}
	return c
}

func (c Customize) Up() Customize   { c.cursor--; return c.clampCursor() }
func (c Customize) Down() Customize { c.cursor++; return c.clampCursor() }

// Toggle flips the choice/add-on under the cursor. In grouped mode a single-
// choice group (Max==1) behaves like a radio; a multi group respects Max.
func (c Customize) Toggle() Customize {
	if !c.grouped() {
		if c.cursor >= 0 && c.cursor < len(c.selected) {
			c.selected[c.cursor] = !c.selected[c.cursor]
		}
		return c
	}
	if c.cursor < 0 || c.cursor >= len(c.rows) {
		return c
	}
	r := c.rows[c.cursor]
	g := c.groups[r.group]
	ch := g.Choices[r.choice]
	pg := c.picked[g.ID]
	if pg[ch.ID] {
		delete(pg, ch.ID) // turning off — allowed; min enforced at confirm.
		return c
	}
	if g.Max == 1 {
		c.picked[g.ID] = map[string]bool{ch.ID: true} // radio
		return c
	}
	if g.Max > 0 && len(pg) >= g.Max {
		return c // at the group's max — ignore.
	}
	pg[ch.ID] = true
	return c
}

// Valid reports whether every required group (Min>0) has at least Min picks.
func (c Customize) Valid() bool {
	if !c.grouped() {
		return true
	}
	for _, g := range c.groups {
		if g.Min > 0 && len(c.picked[g.ID]) < g.Min {
			return false
		}
	}
	return true
}

// SelectedOptions returns the live selections (group/choice ids + variant flag)
// for the cart payload. Empty in flat mode.
func (c Customize) SelectedOptions() []catalog.Selection {
	if !c.grouped() {
		return nil
	}
	var out []catalog.Selection
	for _, g := range c.groups {
		for _, ch := range g.Choices {
			if c.picked[g.ID][ch.ID] {
				out = append(out, catalog.Selection{
					GroupID: g.ID, ChoiceID: ch.ID, Name: ch.Name, Price: ch.Price,
					Variant: g.Variant, Absolute: g.Absolute,
				})
			}
		}
	}
	return out
}

// SelectedAddOns returns the chosen add-ons as flat AddOns — the mock toggles in
// flat mode, or the non-variant selections in grouped mode (for cart-line
// display). Variant selections are excluded (they set the base price instead).
func (c Customize) SelectedAddOns() []catalog.AddOn {
	if !c.grouped() {
		var out []catalog.AddOn
		for i, on := range c.selected {
			if on {
				out = append(out, c.item.AddOns[i])
			}
		}
		return out
	}
	var out []catalog.AddOn
	for _, s := range c.SelectedOptions() {
		if !s.Variant {
			out = append(out, catalog.AddOn{ID: s.ChoiceID, Name: s.Name, Price: s.Price})
		}
	}
	return out
}

// UnitPrice is the per-unit price with the current selection applied. A selected
// variant SETS the price (Swiggy variant prices are absolute); add-ons add on.
func (c Customize) UnitPrice() int {
	if !c.grouped() {
		p := c.item.Price
		for i, on := range c.selected {
			if on {
				p += c.item.AddOns[i].Price
			}
		}
		return p
	}
	base, hasAbs, extra := c.item.Price, false, 0
	for _, s := range c.SelectedOptions() {
		if s.Absolute { // variantsV2 price replaces the base
			base = s.Price
			hasAbs = true
		} else { // legacy variation increment or addon — additive
			extra += s.Price
		}
	}
	if !hasAbs {
		base = c.item.Price
	}
	return base + extra
}

// View renders the bordered dialog. The caller centers it in the viewport.
func (c Customize) View() string {
	if !c.grouped() {
		return c.flatView()
	}
	return c.groupedView()
}

func (c Customize) groupedView() string {
	sub := theme.DimStyle.Render(fmt.Sprintf("₹%d base · pick options", c.item.Price))

	nameW := 0
	for _, g := range c.groups {
		for _, ch := range g.Choices {
			if w := lipgloss.Width(ch.Name); w > nameW {
				nameW = w
			}
		}
	}

	var rows []string
	row := 0
	cursorLine := 0
	for _, g := range c.groups {
		req := ""
		if g.Min > 0 {
			req = theme.FavStyle.Render(" *required")
		} else if g.Max != 1 {
			req = theme.DimStyle.Render(" · optional")
		}
		rows = append(rows, theme.DimStyle.Render("  "+strings.TrimSpace(g.Name))+req)
		for _, ch := range g.Choices {
			on := c.picked[g.ID][ch.ID]
			var box string
			if g.Max == 1 {
				box = theme.DimStyle.Render("( )")
				if on {
					box = theme.GreenStyle.Render("(•)")
				}
			} else {
				box = theme.DimStyle.Render("[ ]")
				if on {
					box = theme.GreenStyle.Render("[x]")
				}
			}
			name := theme.TextStyle.Render(ch.Name)
			price := theme.FaintStyle.Render("free")
			if ch.Price > 0 {
				tag := "+₹" // additive (legacy variation increment or addon)
				if g.Absolute {
					tag = "₹" // variantsV2 price is absolute
				}
				price = theme.GoldStyle.Render(fmt.Sprintf("%s%d", tag, ch.Price))
			}
			cursor := "  "
			if row == c.cursor {
				cursor = theme.CursorStyle.Render("> ")
				cursorLine = len(rows) // index this row will occupy
			}
			gap := strings.Repeat(" ", nameW-lipgloss.Width(ch.Name)+3)
			rows = append(rows, cursor+box+" "+name+gap+price)
			row++
		}
	}
	rows = windowRows(rows, cursorLine, c.viewportH)

	total := justify(
		theme.DimStyle.Render("per item"),
		theme.PriceStyle.Render(fmt.Sprintf("₹%d", c.UnitPrice())),
		nameW+12,
	)

	lines := []string{sub, ""}
	lines = append(lines, rows...)
	lines = append(lines, "", "  "+total)
	if !c.Valid() {
		lines = append(lines, "", theme.FavStyle.Render("  pick required options to add"))
	}
	return autoCard("customise · "+c.item.Name, lines, "↑↓ move   space select   ↵ add   esc cancel")
}

func (c Customize) flatView() string {
	sub := theme.DimStyle.Render(fmt.Sprintf("₹%d base · pick your add-ons", c.item.Price))

	nameW := 0
	for _, a := range c.item.AddOns {
		if w := lipgloss.Width(a.Name); w > nameW {
			nameW = w
		}
	}

	rows := make([]string, 0, len(c.item.AddOns))
	for i, a := range c.item.AddOns {
		box := theme.DimStyle.Render("[ ]")
		if c.selected[i] {
			box = theme.GreenStyle.Render("[x]")
		}
		name := theme.TextStyle.Render(a.Name)
		price := theme.FaintStyle.Render("free")
		if a.Price > 0 {
			price = theme.GoldStyle.Render(fmt.Sprintf("+₹%d", a.Price))
		}
		gap := strings.Repeat(" ", nameW-lipgloss.Width(a.Name)+3)
		cursor := "  "
		if i == c.cursor {
			cursor = theme.CursorStyle.Render("> ")
		}
		rows = append(rows, cursor+box+" "+name+gap+price)
	}

	total := justify(
		theme.DimStyle.Render("per item"),
		theme.PriceStyle.Render(fmt.Sprintf("₹%d", c.UnitPrice())),
		nameW+10,
	)
	lines := []string{sub, ""}
	lines = append(lines, rows...)
	lines = append(lines, "", "  "+total)
	return autoCard("customise · "+c.item.Name, lines, "↑↓ move   space toggle   ↵ add   esc cancel")
}

// windowRows scrolls a long option list to fit the viewport, keeping the cursor
// visible and marking hidden rows above/below. Returns rows unchanged when the
// height is unknown (0) or everything already fits.
func windowRows(rows []string, cursorLine, viewportH int) []string {
	const chrome = 12 // title+sub+blanks+total+hint+border+padding
	if viewportH <= 0 {
		return rows
	}
	budget := viewportH - chrome
	if budget < 3 {
		budget = 3
	}
	if len(rows) <= budget {
		return rows
	}
	content := budget - 2 // reserve two lines for the up/down markers
	if content < 1 {
		content = 1
	}
	start := cursorLine - content/2
	if start < 0 {
		start = 0
	}
	if start+content > len(rows) {
		start = len(rows) - content
	}
	if start < 0 {
		start = 0
	}
	end := start + content
	if end > len(rows) {
		end = len(rows)
	}
	out := make([]string, 0, content+2)
	if start > 0 {
		out = append(out, theme.DimStyle.Render(fmt.Sprintf("  ↑ %d more above", start)))
	} else {
		out = append(out, "")
	}
	out = append(out, rows[start:end]...)
	if end < len(rows) {
		out = append(out, theme.DimStyle.Render(fmt.Sprintf("  ↓ %d more below", len(rows)-end)))
	} else {
		out = append(out, "")
	}
	return out
}
