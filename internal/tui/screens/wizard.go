package screens

import (
	"console.store/internal/catalog"
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
		if g.Min >= 1 && g.Max == 1 && len(g.Choices) > 0 {
			p.picked[g.ID][g.Choices[0].ID] = true // default for required single-choice
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
		delete(pg, ch.ID) // turning off is allowed; min enforced at PageValid.
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
