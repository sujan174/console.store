package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"consolestore/internal/catalog"
	"consolestore/internal/tui/theme"
)

// Wizard is the live, multi-step customize flow for items whose add-on groups
// depend on a variant selection (e.g. a pizza Size where each size has its own
// Crust group). Page 0 is the variant (from search_menu); pages 1…N are the
// valid_addons groups Swiggy reports after each variant/add-on add. The root
// drives the page→cart→next-page loop and reads AllSelections for the payload.
//
// It is a passive value type (like Customize): With*/Up/Down/Toggle/AddPage all
// return a copy.
type Wizard struct {
	item      catalog.Item
	pages     []wizPage
	pageIdx   int
	cursor    int
	loading   bool
	errMsg    string
	viewportH int
	// subtotal is the live price of the current full variant selection, probed
	// from the cart (search_menu omits variant prices, and with multiple variant
	// groups — e.g. Crust × Size — only the combination has a price). subPriced
	// is false while a probe is in flight / before the first result.
	subtotal  int
	subPriced bool
	subShown  bool // whether to render the subtotal line at all
}

// wizPage is one step: a set of choice groups plus the user's picks for them.
type wizPage struct {
	groups []catalog.OptionGroup
	picked map[string]map[string]bool // groupID -> choiceID -> on
	rows   []optRow                   // flattened selectable rows for this page
}

func newWizPage(groups []catalog.OptionGroup) wizPage {
	p := wizPage{groups: groups, picked: make(map[string]map[string]bool, len(groups))}
	for gi, g := range groups {
		p.picked[g.ID] = map[string]bool{}
		if g.Min >= 1 && g.Max == 1 {
			// Default-select the first IN-STOCK choice for required single-choice
			// groups; skip out-of-stock choices so we never auto-select an
			// unavailable item (§6: INVALID_ADDON guard).
			for _, ch := range g.Choices {
				if ch.InStock {
					p.picked[g.ID][ch.ID] = true
					break
				}
			}
		}
		for ci := range g.Choices {
			p.rows = append(p.rows, optRow{group: gi, choice: ci})
		}
	}
	return p
}

// NewWizard builds the wizard with page 0 = the variant group(s).
func NewWizard(item catalog.Item, variantGroups []catalog.OptionGroup) Wizard {
	return Wizard{item: item, pages: []wizPage{newWizPage(variantGroups)}}
}

func (w Wizard) Item() catalog.Item { return w.item }
func (w Wizard) PageIndex() int     { return w.pageIdx }
func (w Wizard) Loading() bool      { return w.loading }
func (w Wizard) Err() string        { return w.errMsg }

// WithSubtotal sets the live price of the current variant selection and shows
// the subtotal line. priced is false while the probe is in flight ("pricing…").
func (w Wizard) WithSubtotal(price int, priced bool) Wizard {
	w.subtotal, w.subPriced, w.subShown = price, priced, true
	return w
}

// WithoutSubtotal hides the subtotal line (used when per-choice prices are shown
// or on a non-final variant page).
func (w Wizard) WithoutSubtotal() Wizard { w.subShown = false; return w }

func (w Wizard) WithLoading(b bool) Wizard { w.loading = b; return w }
func (w Wizard) WithErr(s string) Wizard   { w.errMsg = s; return w }
func (w Wizard) WithViewport(h int) Wizard { w.viewportH = h; return w }

func (w Wizard) cur() wizPage { return w.pages[w.pageIdx] }

func (w Wizard) clampCursor() Wizard {
	n := len(w.cur().rows)
	if w.cursor < 0 {
		w.cursor = 0
	}
	if w.cursor >= n {
		w.cursor = n - 1
	}
	if w.cursor < 0 {
		w.cursor = 0
	}
	return w
}

func (w Wizard) Up() Wizard   { w.cursor--; return w.clampCursor() }
func (w Wizard) Down() Wizard { w.cursor++; return w.clampCursor() }

// Toggle flips the choice under the cursor on the current page. Max==1 groups
// behave like a radio; multi groups respect Max (0/<0 = unlimited).
// Toggling ON an out-of-stock choice is silently ignored; turning OFF is always allowed.
func (w Wizard) Toggle() Wizard {
	p := w.cur()
	if w.cursor < 0 || w.cursor >= len(p.rows) {
		return w
	}
	r := p.rows[w.cursor]
	g := p.groups[r.group]
	ch := g.Choices[r.choice]
	pg := p.picked[g.ID]
	if pg[ch.ID] {
		delete(pg, ch.ID) // turning off is always allowed; min enforced at PageValid.
		return w
	}
	// Do not allow selecting an out-of-stock choice (§6: INVALID_ADDON guard).
	if !ch.InStock {
		return w
	}
	if g.Max == 1 {
		p.picked[g.ID] = map[string]bool{ch.ID: true} // radio
		return w
	}
	if g.Max > 0 && len(pg) >= g.Max {
		return w // at this group's max — ignore.
	}
	pg[ch.ID] = true
	return w
}

// PageValid reports whether every required group (Min>0) on the current page has
// at least Min picks.
func (w Wizard) PageValid() bool {
	p := w.cur()
	for _, g := range p.groups {
		if g.Min > 0 && len(p.picked[g.ID]) < g.Min {
			return false
		}
	}
	return true
}

// SeenGroupIDs returns the set of group ids shown on any page so far.
func (w Wizard) SeenGroupIDs() map[string]bool {
	seen := map[string]bool{}
	for _, p := range w.pages {
		for _, g := range p.groups {
			seen[g.ID] = true
		}
	}
	return seen
}

// AllSelections returns the cumulative selections across all pages, in page
// order (variant first), as the cart payload needs them.
func (w Wizard) AllSelections() []catalog.Selection {
	var out []catalog.Selection
	for _, p := range w.pages {
		for _, g := range p.groups {
			for _, ch := range g.Choices {
				if p.picked[g.ID][ch.ID] {
					out = append(out, catalog.Selection{
						GroupID: g.ID, ChoiceID: ch.ID, Name: ch.Name, Price: ch.Price,
						Variant: g.Variant, Absolute: g.Absolute,
					})
				}
			}
		}
	}
	return out
}

// AddPage appends a page of new groups (Swiggy's valid_addons for the current
// selection), advances to it, and clears the loading flag.
func (w Wizard) AddPage(groups []catalog.OptionGroup) Wizard {
	w.pages = append(w.pages, newWizPage(groups))
	w.pageIdx = len(w.pages) - 1
	w.cursor = 0
	w.loading = false
	w.errMsg = ""
	return w
}

// Back moves to the previous page (no-op on page 0). Selections are kept.
func (w Wizard) Back() Wizard {
	if w.pageIdx > 0 {
		w.pageIdx--
		w.cursor = 0
	}
	return w
}

// AtVariantPage reports whether the wizard is on page 0 (the variant/size pick).
func (w Wizard) AtVariantPage() bool { return w.pageIdx == 0 }

// SelectedVariantName returns the name of the chosen variation on the variant
// page (page 0), used to rank which required add-on group to try first during
// server-trial discovery. Empty if nothing is picked.
func (w Wizard) SelectedVariantName() string {
	_, name := w.selectedVariant()
	return name
}

// SelectedVariantID returns the id of the chosen variation — the cache key for a
// variant's discovered add-on groups. Empty if nothing is picked.
func (w Wizard) SelectedVariantID() string {
	id, _ := w.selectedVariant()
	return id
}

func (w Wizard) selectedVariant() (id, name string) {
	if len(w.pages) == 0 {
		return "", ""
	}
	p := w.pages[0]
	for _, g := range p.groups {
		if !g.Variant {
			continue
		}
		for _, ch := range g.Choices {
			if p.picked[g.ID][ch.ID] {
				return ch.ID, ch.Name
			}
		}
	}
	return "", ""
}

// View renders the current page: title, step indicator, the page's groups
// (radios for single-choice, checkboxes for multi), a loading/error line, and
// contextual hints (next on intermediate pages, add on the last). The caller
// centers it in the viewport.
func (w Wizard) View() string {
	p := w.cur()
	step := theme.DimStyle.Render(fmt.Sprintf("step %d of %d+ · pick options", w.pageIdx+1, len(w.pages)))

	nameW := 0
	for _, g := range p.groups {
		for _, ch := range g.Choices {
			if wd := lipgloss.Width(ch.Name); wd > nameW {
				nameW = wd
			}
		}
	}

	var rows []string
	row := 0
	cursorLine := 0
	for _, g := range p.groups {
		req := ""
		if g.Min > 0 {
			req = theme.FavStyle.Render(" *required")
		} else if g.Max != 1 {
			req = theme.DimStyle.Render(" · optional")
		}
		rows = append(rows, theme.DimStyle.Render("  "+strings.TrimSpace(g.Name))+req)
		for _, ch := range g.Choices {
			on := p.picked[g.ID][ch.ID]
			var box string
			if !ch.InStock {
				// Out-of-stock: render the box as unavailable (dimmed, never filled).
				if g.Max == 1 {
					box = theme.DimStyle.Render("( )")
				} else {
					box = theme.DimStyle.Render("[ ]")
				}
			} else if g.Max == 1 {
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
			var name, price string
			if !ch.InStock {
				name = theme.DimStyle.Render(ch.Name)
				price = theme.DimStyle.Render("sold out")
			} else {
				name = theme.TextStyle.Render(ch.Name)
				switch {
				case g.Absolute && ch.Price > 0:
					// A priced variant (the full combo price for that choice).
					price = theme.GoldStyle.Render(fmt.Sprintf("₹%d", ch.Price))
				case g.Absolute:
					// A variant whose price Swiggy's menu API omits — shown via the
					// subtotal line instead (set by the root when it probed it).
					price = ""
				case ch.Price > 0:
					price = theme.GoldStyle.Render(fmt.Sprintf("+₹%d", ch.Price))
				default:
					price = theme.FaintStyle.Render("free") // genuinely-free add-on
				}
			}
			cursor := "  "
			if row == w.cursor {
				cursor = theme.CursorStyle.Render("> ")
				cursorLine = len(rows)
			}
			gap := strings.Repeat(" ", nameW-lipgloss.Width(ch.Name)+3)
			rows = append(rows, cursor+box+" "+name+gap+price)
			row++
		}
	}
	rows = windowRows(rows, cursorLine, w.viewportH)

	var status string
	switch {
	case w.loading:
		status = theme.DimStyle.Render("  updating…")
	case w.errMsg != "":
		status = theme.FavStyle.Render("  " + w.errMsg)
	}

	lines := []string{step, ""}
	lines = append(lines, rows...)
	// Subtotal — the live price of the current variant selection (probed), shown
	// only when per-choice prices aren't available for this page.
	if w.subShown {
		if w.subPriced {
			lines = append(lines, "", theme.DimStyle.Render("  subtotal  ")+theme.PriceStyle.Render(fmt.Sprintf("₹%d", w.subtotal)))
		} else {
			lines = append(lines, "", theme.DimStyle.Render("  subtotal  pricing…"))
		}
	}
	if status != "" {
		lines = append(lines, "", status)
	}
	// Intermediate pages advance (next); the page is "last" only after the root
	// has confirmed via the cart that no more groups follow. When the required
	// group is empty, a red note calls it out (the footer keeps the key hints).
	if !w.PageValid() {
		lines = append(lines, "", theme.FavStyle.Render("  pick required options"))
	}
	return autoCard("customise · "+w.item.Name, lines, "↑↓ move   space select   ↵ next   esc cancel")
}
