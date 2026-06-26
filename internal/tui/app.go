package tui

import (
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"console.store/internal/broker/api"
	"console.store/internal/catalog"
	"console.store/internal/catalog/mem"
	swiggysnap "console.store/internal/catalog/swiggy"
	"console.store/internal/config"
	"console.store/internal/tui/components"
	"console.store/internal/tui/datasource"
	"console.store/internal/tui/render"
	"console.store/internal/tui/screens"
	"console.store/internal/tui/theme"
)

// dbgTUI logs to the server stderr when CONSOLE_DEBUG_TUI=1 (temporary, for
// diagnosing the live cart-sync path). Never logs to the SSH channel.
func dbgTUI(format string, args ...any) {
	if os.Getenv("CONSOLE_DEBUG_TUI") == "1" {
		log.Printf("TUI-DEBUG "+format, args...)
	}
}

// Bill constants mirror the design (script line 606: toPay = item + 29 − 50).
// NOTE: duplicated in package screens (cart.go) since screens cannot import tui.
const (
	DeliveryFee  = 29
	CouponCode   = "DEVFRIDAY"
	CouponAmount = 50
)

// toPay applies the design's bill: item + delivery − coupon, or 0 when empty.
func toPay(itemTotal int) int {
	if itemTotal <= 0 {
		return 0
	}
	return itemTotal + DeliveryFee - CouponAmount
}

type tickMsg time.Time

// tickInterval drives all animation. 60ms (~16fps) keeps the braille spinner
// liquid without flooding the SSH pipe; frame-derived cadences below are scaled
// to hold their real-time speed.
const tickInterval = 60 * time.Millisecond

// escDoubleWindow is how many frames (~0.7s at the 60ms tick) may separate two
// Esc presses for them to count as a double-Esc "home" gesture.
const escDoubleWindow = 12

func tick() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// spinFrames is the braille spinner (design line 536).
var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type screen int

const (
	scrSplash screen = iota
	scrMenu
	scrRestaurant
	scrCart
	scrAddress
	scrCheckout
	scrConfirm
	scrTracking
	scrInstamart
	scrImCart
)

type Model struct {
	repo    catalog.Repository
	addr    catalog.Address
	section catalog.Section

	screen         screen
	menu           screens.Menu
	rest           screens.Restaurant
	cart           screens.Cart
	addrScreen     screens.Address
	checkout       screens.Checkout
	lines          []screens.CartLine
	cartRestaurant string
	cartSection    catalog.Section // "" for non-snacks; SectionSnacks when snacks cart

	// customize modal: shown when adding an item that has add-ons (Swiggy's
	// "customise" sheet). Owns its own cursor + selection state.
	customizeOpen bool
	customize     screens.Customize

	// wizard: two-step flow for items with BOTH a variant group AND an add-on
	// group. Swiggy's MCP gives no structured variant→add-on link and its
	// valid_addons echoes ALL groups regardless of variant — so after the user
	// picks a size we DISCOVER its required add-on group by trial: send
	// size+candidate to the live cart and keep the one the server accepts (the
	// server is the only authority on the pairing). Names only order the trials.
	wizardOpen         bool
	wizard             screens.Wizard
	wizardRequired     []catalog.OptionGroup            // required (min≥1) add-on groups — trial candidates
	wizardOptional     []catalog.OptionGroup            // optional (min0) add-on groups — ALSO variant-scoped, trialed
	wizardCandidates   []catalog.OptionGroup            // required groups ranked for the chosen variant, being trialed
	wizardTrialPos     int                              // index into wizardCandidates of the in-flight required trial
	wizardPhase        int                              // wzCrust | wzOptional | wzChoosing — what the next CartSyncedMsg means
	wizardBaseSels     []catalog.Selection              // validated base (variant + discovered crust) for optional probes
	wizardCrust        catalog.OptionGroup              // the discovered required group for the chosen variant
	wizardOptIdx       int                              // index into wizardOptional of the in-flight optional trial
	wizardOptValid     []catalog.OptionGroup            // optional groups the server accepted for the chosen variant
	wizardCache        map[string][]catalog.OptionGroup // variationID → discovered add-on page (crust + valid optionals)
	wizardStock        map[string]bool                  // choiceID → in-stock, harvested from the cart's valid_addons (search_menu omits availability)
	wizardSubtotal     int                              // live price of the current variant selection, probed from the cart
	wizardVarGroups    []catalog.OptionGroup            // ordered variant groups (e.g. Crust, then Size) — shown as sequential pages
	wizardVarShown     int                              // number of variant pages shown so far (the rest are scoped on demand)
	wizardScopeIdx     int                              // index into wizardScopeChoices being probed while scoping the next variant group
	wizardScopeChoices []catalog.Choice                 // the next variant group's choices, ordered name-grouped + cheapest-first for early exit
	wizardScopeValid   []catalog.Choice                 // variations the server accepted for the current selection (deduped by name)
	wizardScopeSeen    map[string]bool                  // variation names already resolved during scoping (skip remaining candidates)

	// conflict modal: shown when adding an item from a restaurant other than
	// the one the cart holds (Swiggy allows one restaurant per cart).
	conflictOpen      bool
	conflict          screens.CartConflict
	conflictSel       int                 // focused button: 0 = start new, 1 = keep current
	pendingItem       catalog.Item        // item awaiting the start-new-cart confirmation (or options fetch)
	pendingAddOns     []catalog.AddOn     // its chosen add-ons
	pendingSelections []catalog.Selection // its live variant/addon selections
	pendingPrice      int                 // its resolved unit price
	pendingRest       string              // its restaurant name
	pendingSection    catalog.Section

	inst    screens.Instamart
	imLines []screens.CartLine
	imCart  screens.Cart

	splash       screens.Splash
	decodeStep   int
	splashTick   int    // ticks since the splash was (re)entered; phases the shimmer
	splashPhrase string // Minecraft-style splash line, re-rolled each time we land
	homeSel      int    // selected home-menu item on the splash
	lastEscFrame int    // frame of the previous Esc (for double-Esc home detection)

	track     screens.Tracking
	trackStep int
	trackTick int

	cmdOpen bool
	cmd     screens.CmdBar

	frame int
	w, h  int // terminal size from WindowSizeMsg
	caps  render.Caps

	// live data path (nil/false on the mock default). When live, repo is a
	// catalog/swiggy.Repository backed by snap, filled by datasource Cmds.
	live         bool
	backend      datasource.Backend
	snap         *swiggysnap.Snapshot
	accountID    string
	authorizeURL string
	needsAuth    bool       // set when a load returns datasource.ErrNeedsAuth
	authFlowID   string     // authorize flow id (native gate poll)
	authPoller   AuthPoller // polls callback completion; nil on the mock path
	seeded       bool       // true when catalog/swiggy.Snapshot was pre-seeded from config; skips live init loads

	placingOrder bool     // true while PlaceOrderCmd is in-flight; blocks double-fire
	cartSyncErr  string   // last cart-sync error; shown in status bar (non-fatal)
	orderErr     string   // last order-placement error; shown in status bar
	liveCart     api.Cart // last synced/fetched Swiggy cart (real lines + pricing)
	cartLoaded   bool     // true once the live Swiggy cart is fetched for the cart screen

	// vertical selects the top-level vertical: 0 = Restaurants, 1 = Instamart.
	vertical int
	chips    []config.Category // cuisine chips for the live browse; set from config
	chipIdx  int               // index of the currently active chip (kept for cartPlaceID compat)

	// Rail nav state (live browse only). railActive is the selected rail entry
	// index (RailSearch=0, RailHome=1, categories from index 2). railFocus is
	// true when the left rail column has keyboard focus. searchMode/searchQuery
	// hold the global search state (submit-only — never per-keystroke).
	// searchSubmitted tracks the last query that was actually submitted (via
	// Enter), so we can distinguish "navigate results" from "re-submit" on Enter.
	railActive       int
	railFocus        bool
	searchMode       bool
	searchQuery      string
	searchSubmitted  string // last query submitted to ensureQuery; "" = none
	searchPending    bool   // a search query is in flight (shows "searching…")
	searchCaret      int    // caret position in searchQuery, in runes
	searchCorrected  string // effective query when search spell-corrected; "" otherwise
	searchAtLeftEdge bool   // last key was ← at caret 0 (a second ← exits to the rail)
	catPending       bool   // a category load is in flight (shows "loading…")
	catPendingQuery  string // the category query catPending is waiting on
	restInfoOpen     bool   // restaurant-info modal ('i' on the browse list) is open
	addrOpen         bool   // address switcher modal ('a') is open
	homePending      bool   // Home's "popular near you" load is in flight (shows "loading…")
	usualsLoaded     bool   // true once LoadUsuals has been fired for the current addr
}

func New(caps render.Caps, opts ...Option) Model {
	repo := mem.New()
	section := catalog.SectionCoffee
	m := Model{repo: repo, section: section, screen: scrSplash, caps: caps, lastEscFrame: -escDoubleWindow - 1, railActive: screens.RailHome, railFocus: true}
	for _, o := range opts {
		o(&m)
	}
	// Adopt first address after opts: live path may have seeded the snapshot already
	// (WithLiveBackend swaps m.repo; if the snapshot has addresses, use them).
	if m.addr.ID == "" {
		if addrs := m.repo.Addresses(); len(addrs) > 0 {
			m.addr = addrs[0]
		}
	}
	// Default chips when none were set via WithChips.
	if len(m.chips) == 0 {
		m.chips = config.DefaultCategories()
	}
	m.splash = screens.NewSplash().WithCaps(caps)
	m.splashPhrase = screens.RandomPhrase("")
	// Live+seeded fires the Home load at Init() — mark it pending so the first
	// Home paint shows the "loading…" cue (matching the category pages).
	if m.live && m.seeded && m.addr.ID != "" {
		m.homePending = true
	}
	m.menu = m.buildMenu()
	return m
}

// browsePlaces returns the places to display on the browse screen. In live mode
// it reads from the chip query key; in mock mode it falls back to section places.
func (m Model) browsePlaces() []catalog.Place {
	if m.live {
		if r, ok := m.repo.(*swiggysnap.Repository); ok {
			if len(m.chips) == 0 {
				return nil
			}
			idx := m.chipIdx
			if idx < 0 {
				idx = 0
			}
			if idx >= len(m.chips) {
				idx = len(m.chips) - 1
			}
			return r.PlacesByQuery(m.addr, m.chips[idx].Query)
		}
	}
	return m.repo.Places(m.addr, m.section)
}

// buildMenu constructs the menu screen for the current address + section.
func (m Model) buildMenu() screens.Menu {
	usual, ok := m.repo.Usual(m.addr)
	// Only surface the usual when its item belongs to the section being viewed,
	// so coffee favourites don't bleed into the food/snacks tabs.
	if ok && usual.Item.Section != m.section {
		ok = false
	}
	counts := map[catalog.Section]int{}
	for _, sec := range catalog.MenuSections {
		counts[sec] = len(m.repo.Places(m.addr, sec))
	}

	if m.live && len(m.chips) > 0 {
		// Live mode: rail + two-pane render. Determine the active view's places
		// so NewMenu's rows (and Selected()) map 1:1 to mainPlaces().
		cats := make([]string, len(m.chips))
		for i, c := range m.chips {
			cats[i] = c.Label
		}
		rail := screens.NewRail(cats).WithActive(m.railActive).WithFocus(m.railFocus)

		var viewPlaces []catalog.Place
		var usuals, nearby []catalog.Place
		isSearch := false

		if m.searchMode {
			isSearch = true
			results := m.liveRepo().PlacesByQuery(m.addr, datasource.SearchKey(m.searchQuery))
			viewPlaces = results
		} else if catIdx, isCat := rail.IsCategory(m.railActive); isCat {
			viewPlaces = m.liveRepo().PlacesByQuery(m.addr, m.chips[catIdx].Query)
		} else {
			// Home view: usuals then a populated "popular" list (Swiggy has no
			// list-all, so Home borrows the first cuisine's results).
			usuals = m.liveRepo().PlacesByQuery(m.addr, datasource.UsualsKey)
			nearby = m.liveRepo().PlacesByQuery(m.addr, m.homeNearbyQuery())
			viewPlaces = make([]catalog.Place, 0, len(usuals)+len(nearby))
			viewPlaces = append(viewPlaces, usuals...)
			viewPlaces = append(viewPlaces, nearby...)
		}

		menu := screens.NewMenu(viewPlaces, m.addr, m.section, usual, ok, m.cartChip()).
			WithCounts(counts).
			WithRail(rail).WithRailFocus(m.railFocus).
			WithSectionTabsHidden(true)

		if isSearch {
			results := m.liveRepo().PlacesByQuery(m.addr, datasource.SearchKey(m.searchQuery))
			menu = menu.WithSearchMode(true, m.searchQuery, results, len(results), m.searchPending).
				WithSearchCaret(m.searchCaret).WithSearchCorrected(m.searchCorrected)
		} else if _, isCat := rail.IsCategory(m.railActive); isCat {
			// Category view: use the flat places path (no sections header).
			// viewPlaces already holds catPlaces; WithSections is intentionally NOT
			// called so mainPlaces() falls through to the default flat-list branch
			// and no "nearby" section header is printed. WithLoading shows a cue
			// while the (auto-fired, on-hover) category load is still in flight.
			menu = menu.WithLoading(m.catPending)
		} else {
			menu = menu.WithSections(usuals, nearby).WithLoading(m.homePending)
		}

		return menu
	}

	// Mock path: single-pane section-tab list (unchanged).
	return screens.NewMenu(m.browsePlaces(), m.addr, m.section, usual, ok, m.cartChip()).
		WithCounts(counts)
}

// liveRepo casts the Repository to the swiggy snapshot repo for PlacesByQuery.
// Returns nil when not live; callers guard with m.live.
func (m Model) liveRepo() *swiggysnap.Repository {
	if r, ok := m.repo.(*swiggysnap.Repository); ok {
		return r
	}
	return nil
}

// ensureQuery fires LoadPlacesQuery for query if the snapshot doesn't already
// have results for it. Deduplicates: if the cache is non-empty, no-ops.
func (m Model) ensureQuery(query string) tea.Cmd {
	if r := m.liveRepo(); r != nil {
		if places := r.PlacesByQuery(m.addr, query); len(places) > 0 {
			return nil // already cached
		}
	}
	return datasource.LoadPlacesQuery(m.backend, m.snap, m.addr.ID, query)
}

// searchLoad fires an ad-free global search for query, cached under the search
// key. Deduplicates: if the search cache already holds this query, no-ops.
func (m Model) searchLoad(query string) tea.Cmd {
	if r := m.liveRepo(); r != nil {
		if places := r.PlacesByQuery(m.addr, datasource.SearchKey(query)); len(places) > 0 {
			return nil // already searched
		}
	}
	return datasource.LoadSearch(m.backend, m.snap, m.addr.ID, query)
}

// ensureHomeLoaded fires LoadUsuals + LoadPlacesQuery("") if not already cached.
// Called when the user selects Home on the rail or on initial browse entry.
// syncSearchEntry keeps the search input shown exactly while the rail cursor
// sits on the Search entry — landing on Search opens a fresh input (no Enter),
// leaving it closes the input.
func (m *Model) syncSearchEntry() {
	if m.railActive == screens.RailSearch {
		m.searchMode = true
		m.searchQuery = ""
		m.searchSubmitted = ""
		m.searchCaret = 0
		m.searchCorrected = ""
		m.searchAtLeftEdge = false
	} else {
		m.searchMode = false
	}
}

// searchInsert inserts s into searchQuery at the caret and advances the caret.
func (m *Model) searchInsert(s string) {
	r := []rune(m.searchQuery)
	c := m.searchCaret
	if c < 0 {
		c = 0
	}
	if c > len(r) {
		c = len(r)
	}
	m.searchQuery = string(r[:c]) + s + string(r[c:])
	m.searchCaret = c + len([]rune(s))
}

// loadForRail fires the (deduped) load for the currently-active rail entry, so
// the main pane populates as the user arrows through the rail.
func (m *Model) loadForRail(rail screens.Rail) tea.Cmd {
	m.catPending = false
	switch m.railActive {
	case screens.RailSearch:
		return nil
	case screens.RailHome:
		return m.ensureHomeLoaded()
	default:
		if catIdx, isCat := rail.IsCategory(m.railActive); isCat && catIdx < len(m.chips) {
			q := m.chips[catIdx].Query
			cmd := m.ensureQuery(q)
			// A non-nil cmd means results aren't cached yet — show "loading…".
			m.catPending = cmd != nil
			m.catPendingQuery = q
			return cmd
		}
	}
	return nil
}

// homeNearbyQuery is the keyword used to populate Home's "popular near you"
// list. Swiggy's search_restaurants REQUIRES a query (there is no list-all), so
// Home borrows the first configured cuisine — a real, populated default.
func (m Model) homeNearbyQuery() string {
	if len(m.chips) > 0 {
		return m.chips[0].Query
	}
	return "pizza"
}

func (m *Model) ensureHomeLoaded() tea.Cmd {
	if m.addr.ID == "" {
		return nil // no address yet; AddressesLoadedMsg fires Home loads after adoption
	}
	var cmds []tea.Cmd
	if !m.usualsLoaded {
		m.usualsLoaded = true
		cmds = append(cmds, datasource.LoadUsuals(m.backend, m.snap, m.addr.ID))
	}
	if c := m.ensureQuery(m.homeNearbyQuery()); c != nil {
		m.homePending = true // "popular near you" is fetching
		cmds = append(cmds, c)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

var menuTabs = []catalog.Section{catalog.SectionCoffee, catalog.SectionFood, catalog.SectionSnacks}

func sectionIndex(s catalog.Section) int {
	for i, t := range menuTabs {
		if t == s {
			return i
		}
	}
	return 0
}

// appendOrInc adds item (with add-ons, live selections, resolved price) to lines,
// incrementing the qty if an identical line is already present.
func appendOrInc(lines []screens.CartLine, item catalog.Item, addons []catalog.AddOn, sels []catalog.Selection, price int) []screens.CartLine {
	nl := screens.CartLine{Item: item, Qty: 1, AddOns: addons, Selections: sels, Price: price}
	key := nl.Key()
	for i := range lines {
		if lines[i].Key() == key {
			lines[i].Qty++
			return lines
		}
	}
	return append(lines, nl)
}

// decLastByItem decrements the most recently added line of item id (the last
// matching line), removing it at qty 0. The restaurant inline stepper uses this:
// several customised variants of one item can coexist, and removing the latest
// add is the least surprising undo. Per-variant control lives in the cart.
func decLastByItem(lines []screens.CartLine, id string) []screens.CartLine {
	for i := len(lines) - 1; i >= 0; i-- {
		if lines[i].Item.ID == id {
			lines[i].Qty--
			if lines[i].Qty <= 0 {
				return append(lines[:i], lines[i+1:]...)
			}
			return lines
		}
	}
	return lines
}

// conflictsWithCart reports whether adding from restaurant rest (in section) would
// mix incompatible carts. All SectionSnacks places share one cart; everything else
// is scoped to a single named restaurant.
func (m Model) conflictsWithCart(rest string, section catalog.Section) bool {
	if len(m.lines) == 0 || m.cartRestaurant == "" {
		return false
	}
	if m.cartSection == catalog.SectionSnacks && section == catalog.SectionSnacks {
		return false
	}
	return m.cartRestaurant != rest
}

// startNewCart clears the food cart and seeds it with a single item (and its
// chosen add-ons) from rest — the one-restaurant-per-cart resolution for
// non-snacks; for snacks the section acts as the cart owner.
func (m Model) startNewCart(item catalog.Item, addons []catalog.AddOn, sels []catalog.Selection, price int, rest string, section catalog.Section) Model {
	m.lines = []screens.CartLine{{Item: item, Qty: 1, AddOns: addons, Selections: sels, Price: price}}
	m.cartRestaurant = rest
	m.cartSection = section
	return m
}

// hasVariantGroup / hasAddonGroup classify an item's fetched option groups.
func hasVariantGroup(gs []catalog.OptionGroup) bool {
	for _, g := range gs {
		if g.Variant {
			return true
		}
	}
	return false
}

func hasAddonGroup(gs []catalog.OptionGroup) bool {
	for _, g := range gs {
		if !g.Variant {
			return true
		}
	}
	return false
}

// wizardEligible is true when an item's add-ons may depend on its variant — it
// has BOTH a variant group and an add-on group. Those items must use the
// server-driven wizard (add variant → read valid_addons). Variant-only or
// addon-only items are safe in the single-page Customize sheet.
func wizardEligible(gs []catalog.OptionGroup) bool {
	return hasVariantGroup(gs) && hasAddonGroup(gs)
}

// variantGroups returns just the variant groups (page 0 of the wizard).
func variantGroups(gs []catalog.OptionGroup) []catalog.OptionGroup {
	var out []catalog.OptionGroup
	for _, g := range gs {
		if g.Variant {
			out = append(out, g)
		}
	}
	return out
}

// wizard0 builds a fresh wizard for item it, seeding ONLY the first variant
// group on page 0. Further variant groups (e.g. Size after Crust) are added as
// sequential pages, each scoped to the prior selection (their valid options are
// discovered against the live cart).
func (m Model) wizard0(it catalog.Item, gs []catalog.OptionGroup) screens.Wizard {
	vgs := variantGroups(gs)
	first := vgs
	if len(vgs) > 1 {
		first = vgs[:1]
	}
	return screens.NewWizard(it, first).WithViewport(m.h)
}

// onVariantPage reports whether the wizard's current page is a variant page (vs
// an add-on page). The first wizardVarShown pages are the variant groups.
func (m Model) onVariantPage() bool { return m.wizard.PageIndex() < m.wizardVarShown }

// variantGroupHasPrices reports whether a variant group's choices carry their own
// price (e.g. a Domino's Size where each crust×size combo is priced) — in which
// case per-choice prices are shown and no subtotal probe is needed.
func variantGroupHasPrices(g catalog.OptionGroup) bool {
	for _, ch := range g.Choices {
		if ch.Price > 0 {
			return true
		}
	}
	return false
}

// variantComboKey is the cache key for the current full variant selection (all
// chosen variant choices, in page order).
func (m Model) variantComboKey() string {
	var b strings.Builder
	for _, s := range m.variantSelections() {
		b.WriteString(s.ChoiceID)
		b.WriteByte('|')
	}
	return b.String()
}

// enterVariantPage configures the just-shown variant page: when it is the LAST
// variant group and its choices have no per-choice price (e.g. Onesta Size), it
// probes a subtotal; otherwise it idles (per-choice prices, or a non-final
// variant page like Crust).
func (m Model) enterVariantPage() (Model, tea.Cmd) {
	cur := m.wizardVarGroups[m.wizardVarShown-1]
	last := m.wizardVarShown >= len(m.wizardVarGroups)
	if last && !variantGroupHasPrices(cur) {
		m.wizardPhase = wzPricing
		m.wizard = m.wizard.WithLoading(true).WithSubtotal(0, false)
		return m, m.wizardSubtotalCmd()
	}
	m.wizardPhase = wzPickVariant
	m.wizard = m.wizard.WithLoading(false).WithoutSubtotal()
	return m, nil
}

// beginVariantScope discovers which options of the NEXT variant group are valid
// for the current selection — probing each against the live cart — and shows
// them deduped by name. Realizes the two-layer model (Crust → that crust's Sizes).
func (m Model) beginVariantScope() (Model, tea.Cmd) {
	g := m.wizardVarGroups[m.wizardVarShown]
	m.wizardScopeChoices = orderScopeChoices(g.Choices)
	m.wizardScopeIdx = 0
	m.wizardScopeValid = nil
	m.wizardScopeSeen = map[string]bool{}
	m.wizardPhase = wzVarScope
	m.wizard = m.wizard.WithLoading(true)
	return m, m.scopeVariantCmd()
}

// orderScopeChoices groups a variant group's choices by display name (in
// first-appearance order) and, within each name, orders them cheapest-first.
// Probing in this order lets scoping STOP at a name's first accepted candidate —
// for the common (cheapest/default) outer variant that resolves in one probe per
// name instead of one per matrix row.
func orderScopeChoices(choices []catalog.Choice) []catalog.Choice {
	var order []string
	byName := map[string][]catalog.Choice{}
	for _, c := range choices {
		if _, ok := byName[c.Name]; !ok {
			order = append(order, c.Name)
		}
		byName[c.Name] = append(byName[c.Name], c)
	}
	var out []catalog.Choice
	for _, n := range order {
		cs := byName[n]
		sort.SliceStable(cs, func(i, j int) bool { return cs[i].Price < cs[j].Price })
		out = append(out, cs...)
	}
	return out
}

// scopeVariantCmd probes whether the current candidate of the next variant group
// is valid alongside the already-chosen variant(s).
func (m Model) scopeVariantCmd() tea.Cmd {
	g := m.wizardVarGroups[m.wizardVarShown]
	if m.wizardScopeIdx >= len(m.wizardScopeChoices) {
		return nil
	}
	v := m.wizardScopeChoices[m.wizardScopeIdx]
	sels := append(m.variantSelections(), catalog.Selection{
		GroupID: g.ID, ChoiceID: v.ID, Name: v.Name, Price: v.Price, Variant: true, Absolute: g.Absolute,
	})
	dbgTUI("scopeVariantCmd: probe %q (%d/%d) of %q", v.Name, m.wizardScopeIdx+1, len(m.wizardScopeChoices), g.Name)
	return m.wizardSyncCmd(sels)
}

// liveRestReady reports whether a live cart is usable for the browsed restaurant
// (the wizard mutates the live cart, so it needs a real Swiggy restaurant id).
func (m Model) liveRestReady() bool {
	return m.live && m.rest.PlaceData().SwiggyID != ""
}

// Wizard discovery phases — what the next CartSyncedMsg result means.
const (
	wzPickVariant = iota // idle on a variant page (no probe in flight)
	wzPricing            // probing the current variant selection's subtotal
	wzVarScope           // probing which options of the NEXT variant group are valid for the current selection
	wzCrust              // probing required (crust) candidates for the chosen variant
	wzOptional           // probing optional (toppings/beverage) groups for the variant
	wzChoosing           // on the add-on page; the sync is the confirm
)

// requiredAddonGroups returns the add-on groups that REQUIRE a choice (Min≥1) —
// the mutually-exclusive variant-scoped groups (e.g. Crust Small / Crust Medium)
// that trial-discovery must pair to the chosen variant.
func requiredAddonGroups(gs []catalog.OptionGroup) []catalog.OptionGroup {
	var out []catalog.OptionGroup
	for _, g := range gs {
		if !g.Variant && g.Min >= 1 {
			out = append(out, g)
		}
	}
	return out
}

// optionalAddonGroups returns the non-required add-on groups (Min==0) — toppings,
// beverages — which the server accepts with any variant, so they need no pairing.
func optionalAddonGroups(gs []catalog.OptionGroup) []catalog.OptionGroup {
	var out []catalog.OptionGroup
	for _, g := range gs {
		if !g.Variant && g.Min == 0 {
			out = append(out, g)
		}
	}
	return out
}

// trialOrder ranks required add-on groups for a chosen variant: groups whose
// name contains the variant name (e.g. "Crust Small." for "Small") are tried
// first, the rest keep their order. The name is only a HINT for ordering — the
// live cart's accept/reject is the actual authority. Groups with no in-stock
// choice are dropped (can't be sent).
func trialOrder(required []catalog.OptionGroup, variantName string) []catalog.OptionGroup {
	vn := strings.ToLower(strings.TrimSpace(variantName))
	var match, rest []catalog.OptionGroup
	for _, g := range required {
		if _, ok := firstInStockChoice(g); !ok {
			continue
		}
		if vn != "" && strings.Contains(strings.ToLower(g.Name), vn) {
			match = append(match, g)
		} else {
			rest = append(rest, g)
		}
	}
	return append(match, rest...)
}

// firstInStockChoice returns the first selectable choice in a group.
func firstInStockChoice(g catalog.OptionGroup) (catalog.Choice, bool) {
	for _, ch := range g.Choices {
		if ch.InStock {
			return ch, true
		}
	}
	return catalog.Choice{}, false
}

// wizardTrialCmd probes the current candidate required group: it sends the cart
// = committed lines + the wizard item carrying the chosen variant + this
// candidate's default choice. The server accepts only the group that pairs with
// the variant; the CartSyncedMsg handler keeps that one or tries the next.
func (m Model) wizardTrialCmd() tea.Cmd {
	if m.wizardTrialPos >= len(m.wizardCandidates) {
		return nil
	}
	cand := m.wizardCandidates[m.wizardTrialPos]
	ch, ok := firstInStockChoice(cand)
	if !ok {
		return nil
	}
	sels := append(m.wizard.AllSelections(), catalog.Selection{
		GroupID: cand.ID, ChoiceID: ch.ID, Name: ch.Name, Price: ch.Price,
	})
	dbgTUI("wizardTrialCmd: trial %d/%d group=%q", m.wizardTrialPos+1, len(m.wizardCandidates), cand.Name)
	return m.wizardSyncCmd(sels)
}

// wizardOptionalCmd probes whether the current optional group is valid for the
// chosen variant: it sends the validated base (variant + crust) plus this
// optional group's default choice. The server accepts only same-size optional
// groups (e.g. "Toppings (Medium)" for Medium, not "Toppings (Regular)").
func (m Model) wizardOptionalCmd() tea.Cmd {
	if m.wizardOptIdx >= len(m.wizardOptional) {
		return nil
	}
	g := m.wizardOptional[m.wizardOptIdx]
	ch, ok := firstInStockChoice(g)
	if !ok {
		return nil
	}
	sels := append(append([]catalog.Selection{}, m.wizardBaseSels...),
		catalog.Selection{GroupID: g.ID, ChoiceID: ch.ID, Name: ch.Name, Price: ch.Price})
	dbgTUI("wizardOptionalCmd: probe %d/%d group=%q", m.wizardOptIdx+1, len(m.wizardOptional), g.Name)
	return m.wizardSyncCmd(sels)
}

// beginOptionalDiscovery starts probing the optional add-on groups against the
// validated base (variant + crust). Groups with no in-stock choice are skipped.
// When there are no optional groups, discovery finishes immediately.
func (m Model) beginOptionalDiscovery() (Model, tea.Cmd) {
	m.wizardOptValid = nil
	m.wizardOptIdx = 0
	for m.wizardOptIdx < len(m.wizardOptional) {
		if _, ok := firstInStockChoice(m.wizardOptional[m.wizardOptIdx]); ok {
			break
		}
		m.wizardOptIdx++
	}
	if m.wizardOptIdx >= len(m.wizardOptional) {
		return m.finishDiscovery()
	}
	m.wizardPhase = wzOptional
	m.wizard = m.wizard.WithLoading(true)
	return m, m.wizardOptionalCmd()
}

// finishDiscovery builds the add-on page from the discovered required crust group
// plus the server-accepted optional groups, caches it for the chosen variant, and
// advances the wizard to it.
func (m Model) finishDiscovery() (Model, tea.Cmd) {
	var page []catalog.OptionGroup
	if m.wizardCrust.ID != "" {
		page = append(page, m.wizardCrust)
	}
	page = append(page, m.wizardOptValid...)
	// Stamp authoritative availability from the cart's valid_addons onto each
	// choice (search_menu marks everything in-stock; the cart knows the truth).
	page = applyStock(page, m.wizardStock)
	if key := m.variantComboKey(); key != "" {
		if m.wizardCache == nil {
			m.wizardCache = map[string][]catalog.OptionGroup{}
		}
		m.wizardCache[key] = page
	}
	m.wizardPhase = wzChoosing
	m.wizard = m.wizard.AddPage(page) // AddPage clears loading + advances to the page
	return m, nil
}

// mergeStock records each choice's authoritative in-stock flag from the cart's
// valid_addons (search_menu omits availability, so unavailable add-ons would
// otherwise look selectable and get rejected at order time).
func mergeStock(dst map[string]bool, groups []api.OptionGroup) {
	if dst == nil {
		return
	}
	for _, g := range groups {
		for _, ch := range g.Choices {
			dst[ch.ID] = ch.InStock
		}
	}
}

// applyStock stamps harvested availability onto a page's choices: a choice the
// cart marked out-of-stock is set !InStock (the wizard renders it "sold out" and
// blocks selecting it). Choices absent from the map keep their original flag.
func applyStock(groups []catalog.OptionGroup, stock map[string]bool) []catalog.OptionGroup {
	out := make([]catalog.OptionGroup, len(groups))
	for i, g := range groups {
		g.Choices = append([]catalog.Choice(nil), g.Choices...)
		for j := range g.Choices {
			if in, ok := stock[g.Choices[j].ID]; ok {
				g.Choices[j].InStock = in
			}
		}
		out[i] = g
	}
	return out
}

// wizardSubtotalCmd probes the price of the CURRENT variant selection: it adds
// the chosen variant(s) — plus a default for each required add-on group that
// matches the chosen variant, so the add validates — and reads the item line's
// price back. Works for one variant group (size) or several (size × crust).
func (m Model) wizardSubtotalCmd() tea.Cmd {
	sels := m.variantSelections()
	sels = append(sels, m.requiredDefaultsForVariants(sels)...)
	return m.wizardSyncCmd(sels)
}

// variantSelections returns the currently-picked variant choices (page 0).
func (m Model) variantSelections() []catalog.Selection {
	var out []catalog.Selection
	for _, s := range m.wizard.AllSelections() {
		if s.Variant {
			out = append(out, s)
		}
	}
	return out
}

// requiredDefaultsForVariants returns a default choice for each required add-on
// group that applies to the chosen variant — name-matched to a selected variant
// (so we never send two mutually-exclusive crusts), or the sole required group.
func (m Model) requiredDefaultsForVariants(variantSels []catalog.Selection) []catalog.Selection {
	var out []catalog.Selection
	for _, g := range m.wizardRequired {
		match := len(m.wizardRequired) == 1
		for _, vs := range variantSels {
			if vs.Name != "" && strings.Contains(strings.ToLower(g.Name), strings.ToLower(vs.Name)) {
				match = true
			}
		}
		if !match {
			continue
		}
		gg := applyStock([]catalog.OptionGroup{g}, m.wizardStock)[0]
		if ch, ok := firstInStockChoice(gg); ok {
			out = append(out, catalog.Selection{GroupID: g.ID, ChoiceID: ch.ID, Name: ch.Name, Price: ch.Price})
		}
	}
	return out
}

// itemLinePrice returns the wizard item's per-unit price from a cart response.
func (m Model) itemLinePrice(cart api.Cart) int {
	swID := m.wizard.Item().SwiggyID
	for _, l := range cart.Lines {
		if l.ItemID == swID {
			return l.Price
		}
	}
	return 0
}

// wizardCartCmd confirms the full current selection (variant + chosen add-ons)
// against the live cart. Used on the add-on page's Enter.
func (m Model) wizardCartCmd() tea.Cmd { return m.wizardSyncCmd(m.wizard.AllSelections()) }

// wizardSyncCmd sends the live cart = committed lines + the wizard item carrying
// sels, returning pricing via CartSyncedMsg. The draft is NOT yet in m.lines.
func (m Model) wizardSyncCmd(sels []catalog.Selection) tea.Cmd {
	if !m.live {
		return nil
	}
	pd := m.rest.PlaceData()
	if pd.SwiggyID == "" {
		dbgTUI("wizardSyncCmd: nil (browsed restaurant has no SwiggyID)")
		return nil
	}
	items := m.cartItemsForLines() // committed lines as api.CartItem
	draft := api.CartItem{ItemID: m.wizard.Item().SwiggyID, Quantity: 1}
	for _, s := range sels {
		switch {
		case s.Variant && s.Absolute:
			draft.VariantsV2 = append(draft.VariantsV2, api.CartVariantSel{GroupID: s.GroupID, VariationID: s.ChoiceID})
		case s.Variant:
			draft.VariantsLegacy = append(draft.VariantsLegacy, api.CartVariantSel{GroupID: s.GroupID, VariationID: s.ChoiceID})
		default:
			draft.Addons = append(draft.Addons, api.CartAddonSel{GroupID: s.GroupID, ChoiceID: s.ChoiceID})
		}
	}
	items = append(items, draft)
	return datasource.SyncCart(m.backend, m.snap, m.addr.ID, pd.SwiggyID, pd.Name, items)
}

// cartItemsForLines converts the committed cart lines into api.CartItems (the
// payload shared by liveSyncCart and the wizard's draft send).
func (m Model) cartItemsForLines() []api.CartItem {
	items := make([]api.CartItem, 0, len(m.lines))
	for _, l := range m.lines {
		if l.Item.SwiggyID == "" {
			continue
		}
		ci := api.CartItem{ItemID: l.Item.SwiggyID, Quantity: l.Qty}
		for _, s := range l.Selections {
			switch {
			case s.Variant && s.Absolute:
				ci.VariantsV2 = append(ci.VariantsV2, api.CartVariantSel{GroupID: s.GroupID, VariationID: s.ChoiceID})
			case s.Variant:
				ci.VariantsLegacy = append(ci.VariantsLegacy, api.CartVariantSel{GroupID: s.GroupID, VariationID: s.ChoiceID})
			default:
				ci.Addons = append(ci.Addons, api.CartAddonSel{GroupID: s.GroupID, ChoiceID: s.ChoiceID})
			}
		}
		items = append(items, ci)
	}
	return items
}

// beginAdd starts adding item from rest (section). If the item is customizable it
// opens the customise modal (the add is finished on confirm); otherwise it adds the
// item straight away. Centralising this keeps the restaurant-add, usual-add and
// modal-confirm paths in one place.
func (m Model) beginAdd(item catalog.Item, rest string, section catalog.Section) Model {
	if len(item.AddOns) > 0 {
		m.customize = screens.NewCustomize(item)
		m.customizeOpen = true
		m.pendingRest = rest
		m.pendingSection = section
		return m
	}
	return m.commitAdd(item, nil, nil, 0, rest, section)
}

// commitAdd adds item (with its chosen add-ons) from rest (section) to the cart,
// raising the cart-conflict modal first when the cart belongs to an incompatible
// owner (different restaurant outside the snacks section).
func (m Model) commitAdd(item catalog.Item, addons []catalog.AddOn, sels []catalog.Selection, price int, rest string, section catalog.Section) Model {
	if m.conflictsWithCart(rest, section) {
		m.pendingItem = item
		m.pendingAddOns = addons
		m.pendingSelections = sels
		m.pendingPrice = price
		m.pendingRest = rest
		m.pendingSection = section
		m.conflict = screens.NewCartConflict(m.cartHeader(), rest, item.Name)
		m.conflictSel = 1
		m.conflictOpen = true
		return m
	}
	wasEmpty := len(m.lines) == 0
	m.lines = appendOrInc(m.lines, item, addons, sels, price)
	if wasEmpty {
		m.cartRestaurant = rest
		m.cartSection = section
	}
	return m
}

// addonsFromSelections returns the non-variant selections as flat AddOns for the
// cart-line display (variant selections set the base price instead).
func addonsFromSelections(sels []catalog.Selection) []catalog.AddOn {
	var out []catalog.AddOn
	for _, s := range sels {
		if !s.Variant {
			out = append(out, catalog.AddOn{ID: s.ChoiceID, Name: s.Name, Price: s.Price})
		}
	}
	return out
}

// priceFromSelections computes the per-unit price: a variantsV2 selection SETS
// the base (absolute); legacy variations and add-ons add on.
func priceFromSelections(base int, sels []catalog.Selection) int {
	extra := 0
	for _, s := range sels {
		if s.Absolute {
			base = s.Price
		} else {
			extra += s.Price
		}
	}
	return base + extra
}

// commitAddNoSync appends the configured draft to the local lines WITHOUT firing
// another cart sync — the wizard already synced it to the live cart page by
// page. Conflict was resolved before the wizard opened, so no conflict check.
func (m Model) commitAddNoSync(item catalog.Item, addons []catalog.AddOn, sels []catalog.Selection, price int, rest string, section catalog.Section) Model {
	wasEmpty := len(m.lines) == 0
	m.lines = appendOrInc(m.lines, item, addons, sels, price)
	if wasEmpty {
		m.cartRestaurant = rest
		m.cartSection = section
	}
	return m
}

// refreshAfterAdd re-syncs the chips and the live restaurant stepper after a
// cart change (a no-op for the conflict/customize paths, which haven't committed
// yet). Returns the updated model.
func (m Model) refreshAfterAdd() Model {
	m.menu = m.menu.WithCartChip(m.cartChip())
	if m.screen == scrRestaurant {
		ci := m.rest.CursorIndex()
		info := m.rest.InfoOpen()
		cat := m.rest.ActiveCategory()
		veg := m.rest.VegOnly()
		// WithCategory and WithVegOnly reset cursor to 0; apply WithCursor last.
		m.rest = screens.NewRestaurant(m.rest.PlaceData(), m.qtyMap(), m.cartChip()).
			WithAddr(m.addr).WithInfo(info).
			WithCategory(cat).WithVegOnly(veg).WithCursor(ci)
	}
	return m
}

// restIncSelected adds one unit of the focused dish (Enter / +). A live
// customizable dish first fetches its options; non-customizable / mock dishes go
// straight through beginAdd (which may open the customize or conflict modal).
func (m Model) restIncSelected() (tea.Model, tea.Cmd) {
	it, ok := m.rest.Selected()
	if !ok {
		dbgTUI("add: Selected() returned !ok (no item under cursor)")
		return m, nil
	}
	dbgTUI("add: item=%q swiggyID=%q rest=%q", it.Name, it.SwiggyID, m.rest.PlaceData().Name)
	if m.live && it.Customizable && len(it.Options) == 0 {
		m.pendingItem = it
		m.pendingRest = m.rest.PlaceData().Name
		m.pendingSection = m.rest.PlaceData().Section
		dbgTUI("add: fetching options for %q", it.Name)
		return m, datasource.LoadItemOptions(m.backend, m.addr.ID, m.rest.PlaceData().SwiggyID, it.Name, it.SwiggyID)
	}
	m = m.beginAdd(it, m.rest.PlaceData().Name, m.rest.PlaceData().Section)
	if m.customizeOpen || m.conflictOpen {
		dbgTUI("add: modal opened (customize=%v conflict=%v)", m.customizeOpen, m.conflictOpen)
		return m, nil // a modal will finish the add
	}
	m = m.refreshAfterAdd()
	return m, m.liveCartCmd()
}

// restDecSelected removes one unit of the focused dish (−), releasing the
// restaurant binding when the cart empties.
func (m Model) restDecSelected() (tea.Model, tea.Cmd) {
	it, ok := m.rest.Selected()
	if !ok {
		return m, nil
	}
	m.lines = decLastByItem(m.lines, it.ID)
	if len(m.lines) == 0 {
		m.cartRestaurant = ""
		m.cartSection = ""
	}
	m = m.refreshAfterAdd()
	return m, m.liveCartCmd()
}

// qtyMap returns current cart quantities keyed by item ID.
func (m Model) qtyMap() map[string]int {
	q := map[string]int{}
	for _, l := range m.lines {
		q[l.Item.ID] += l.Qty
	}
	return q
}

// imQtyMap returns current Instamart cart quantities keyed by item ID.
func (m Model) imQtyMap() map[string]int {
	q := map[string]int{}
	for _, l := range m.imLines {
		q[l.Item.ID] += l.Qty
	}
	return q
}

func orderID(lines []screens.CartLine) string {
	sum := 0
	for _, l := range lines {
		for _, r := range l.Item.ID + l.Item.Name {
			sum = (sum*31 + int(r)) & 0xffff
		}
		sum = (sum + l.Qty) & 0xffff
	}
	return fmt.Sprintf("#SW%04X", sum)
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tick()}
	if c := m.liveInitCmds(); c != nil {
		cmds = append(cmds, c)
	}
	if m.needsAuth && m.authorizeURL != "" {
		cmds = append(cmds, openBrowserCmd(m.authorizeURL))
	}
	return tea.Batch(cmds...)
}

// onTick advances time-based screen state; extended by later tasks.
func (m Model) onTick() Model {
	if m.screen == scrSplash {
		// Resolve the decode, then keep ticking so the idle shimmer animates.
		if m.decodeStep < render.DecodeSteps {
			m.decodeStep++
		}
		m.splashTick++
	}
	if m.screen == scrTracking {
		m.trackTick++
		// Advance through all four timeline steps; the final step (== 4) marks the
		// order delivered and reveals the thank-you note.
		if m.trackTick%70 == 0 && m.trackStep < 4 {
			m.trackStep++
		}
	}
	return m
}

// toSplash returns to the splash and replays the decode from the start. It is a
// visual "home" gesture (double-Esc) — cart, address, and section are preserved.
func (m Model) toSplash() Model {
	m.screen = scrSplash
	m.decodeStep = 0
	m.splashTick = 0
	m.homeSel = 0
	m.cmdOpen = false
	// Re-roll the splash phrase so returning home shows a fresh one-liner.
	m.splashPhrase = screens.RandomPhrase(m.splashPhrase)
	return m
}

func (m Model) spin() string { return spinFrames[m.frame%len(spinFrames)] }

// blinkOn reports the on-phase of a ~1s cursor blink.
func (m Model) blinkOn() bool { return (m.frame/9)%2 == 0 }

func (m Model) cartTotal() int {
	t := 0
	for _, l := range m.lines {
		t += l.UnitPrice() * l.Qty
	}
	return t
}

// cartCount is the total quantity of items in the food cart.
func (m Model) cartCount() int {
	n := 0
	for _, l := range m.lines {
		n += l.Qty
	}
	return n
}

// imCartCount is the total quantity of items in the Instamart cart.
func (m Model) imCartCount() int {
	n := 0
	for _, l := range m.imLines {
		n += l.Qty
	}
	return n
}

// cartChipStr formats the cart chip: "🛒 cart empty" or "🛒 cart · <n> · ₹<total>".
func cartChipStr(count, total int) string {
	if count == 0 {
		return "🛒 cart empty"
	}
	return fmt.Sprintf("🛒 cart · %d · ₹%d", count, total)
}

// cartChip / imCartChip are the formatted chips for the food / Instamart carts.
func (m Model) cartChip() string   { return cartChipStr(m.cartCount(), m.cartTotal()) }
func (m Model) imCartChip() string { return cartChipStr(m.imCartCount(), m.imCartTotal()) }

// cartRestaurantServes reports whether the cart's restaurant/section is serviceable
// at addr. For snacks, any non-empty snack catalogue at the new address is enough.
func (m Model) cartRestaurantServes(addr catalog.Address) bool {
	if m.cartRestaurant == "" {
		return true
	}
	if m.cartSection == catalog.SectionSnacks {
		return len(m.repo.Places(addr, catalog.SectionSnacks)) > 0
	}
	for _, section := range catalog.MenuSections {
		for _, p := range m.repo.Places(addr, section) {
			if p.Name == m.cartRestaurant {
				return true
			}
		}
	}
	return false
}

func (m Model) imCartTotal() int {
	t := 0
	for _, l := range m.imLines {
		t += l.UnitPrice() * l.Qty
	}
	return t
}

// InstamartMin is the Instamart minimum order value (₹).
const InstamartMin = 99

func (m Model) cartHeader() string {
	if m.cartRestaurant != "" {
		if m.cartSection == catalog.SectionSnacks {
			return "quick snacks"
		}
		return m.cartRestaurant
	}
	return "your order"
}

// cartPlaceID finds the id of the cart's restaurant by name. In live mode the
// browse places are keyed by chip query (e.g. "pizza"), not by the mock section
// names, so search every chip's results first; fall back to the mock sections.
func (m Model) cartPlaceID() string {
	if m.live {
		if r, ok := m.repo.(*swiggysnap.Repository); ok {
			for _, c := range m.chips {
				for _, p := range r.PlacesByQuery(m.addr, c.Query) {
					if p.Name == m.cartRestaurant {
						return p.ID
					}
				}
			}
		}
	}
	for _, sec := range catalog.MenuSections {
		for _, p := range m.repo.Places(m.addr, sec) {
			if p.Name == m.cartRestaurant {
				return p.ID
			}
		}
	}
	return ""
}

// cartEta returns "~{tail}" of the cart restaurant's ETA, e.g. "35-45 min" -> "~45 min".
func (m Model) cartEta() string {
	if p, ok := m.repo.Menu(m.cartPlaceID()); ok {
		parts := strings.SplitN(p.ETA, "-", 2)
		if len(parts) == 2 {
			return "~" + strings.TrimSpace(parts[1])
		}
		return "~" + p.ETA
	}
	return ""
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tickMsg); ok {
		m.frame++
		m = m.onTick()
		// Native auth gate: poll the loopback callback. When the browser
		// authorize completes, clear the gate and fire the live loads.
		if m.needsAuth && m.authPoller != nil && m.authFlowID != "" && m.authPoller.Authorized(m.authFlowID) {
			m.needsAuth = false
			return m, tea.Batch(tick(), m.liveInitCmds())
		}
		return m, tick()
	}
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.w, m.h = ws.Width, ws.Height
		components.SetFrameWidth(m.w)
		return m, nil
	}
	switch dm := msg.(type) {
	case browserOpenedMsg:
		// Advisory: on failure the copyable URL stays on screen.
		return m, nil
	case datasource.AddressesLoadedMsg:
		if errIsNeedsAuth(dm.Err) {
			m.needsAuth = true
			return m, nil
		}
		if addrs := m.repo.Addresses(); len(addrs) > 0 {
			m.addr = addrs[0]
		}
		m.menu = m.buildMenu()
		if m.live {
			// Address just adopted → load Home (usuals + the popular list) for it.
			// Reset usualsLoaded so the new address's usuals are fetched.
			m.usualsLoaded = false
			cmd := m.ensureHomeLoaded()
			return m, cmd
		}
		return m, nil
	case datasource.PlacesLoadedMsg:
		if errIsNeedsAuth(dm.Err) {
			m.needsAuth = true
			return m, nil
		}
		if m.searchPending && dm.Query == m.searchSubmitted {
			m.searchPending = false          // results for the submitted query landed
			m.searchCorrected = dm.Corrected // "" unless a spelling correction matched
		}
		if m.catPending && dm.Query == m.catPendingQuery {
			m.catPending = false // category results landed
		}
		if m.homePending && dm.Query == m.homeNearbyQuery() {
			m.homePending = false // Home's "popular near you" landed
		}
		m.menu = m.buildMenu()
		return m, nil
	case datasource.MenuLoadedMsg:
		if errIsNeedsAuth(dm.Err) {
			m.needsAuth = true
			return m, nil
		}
		if m.screen == scrRestaurant {
			if p, ok := m.repo.Menu(dm.PlaceID); ok {
				// The menu place carries only Items; its descriptive fields (Name,
				// ID, Section…) come back empty. Preserve the identity from the
				// restaurant we navigated in from (the search result) so the cart
				// can attribute items to the right restaurant — without the Name,
				// cartRestaurant stays "" and the live cart sync never fires.
				prev := m.rest.PlaceData()
				p.Name = prev.Name
				p.ID = prev.ID
				p.SwiggyID = prev.SwiggyID
				p.Section = prev.Section
				p.City = prev.City
				p.ETA = prev.ETA
				p.Rating = prev.Rating
				p.Description = prev.Description
				ci := m.rest.CursorIndex()
				info := m.rest.InfoOpen()
				cat := m.rest.ActiveCategory()
				veg := m.rest.VegOnly()
				m.rest = screens.NewRestaurant(p, m.qtyMap(), m.cartChip()).
					WithAddr(m.addr).WithInfo(info).
					WithCategory(cat).WithVegOnly(veg).WithCursor(ci)
			}
		}
		return m, nil
	case datasource.ItemOptionsLoadedMsg:
		if dm.Err != nil {
			m.cartSyncErr = "options: " + dm.Err.Error()
			return m, nil
		}
		it := m.pendingItem
		it.Options = dm.Groups
		if len(dm.Groups) == 0 {
			// Item flagged customizable but has no real options — add directly.
			m = m.commitAdd(it, nil, nil, 0, m.pendingRest, m.pendingSection)
			if !m.conflictOpen {
				m = m.refreshAfterAdd()
				return m, m.liveCartCmd()
			}
			return m, nil
		}
		if wizardEligible(dm.Groups) && m.liveRestReady() {
			// Variant-dependent add-ons: drive the trial-discovery wizard. Resolve
			// any cart-restaurant conflict first (the wizard mutates the live cart).
			if m.conflictsWithCart(m.pendingRest, m.pendingSection) {
				m.conflict = screens.NewCartConflict(m.cartHeader(), m.pendingRest, it.Name)
				m.conflictSel = 1
				m.conflictOpen = true
				m.pendingItem = it // re-fetch path on "new cart" (handled in conflict resolve)
				return m, nil
			}
			m.wizardRequired = requiredAddonGroups(dm.Groups)
			m.wizardOptional = optionalAddonGroups(dm.Groups)
			m.wizardVarGroups = variantGroups(dm.Groups)
			m.wizardVarShown = 1
			m.wizardCandidates = nil
			m.wizardTrialPos = 0
			m.wizardCache = nil
			m.wizardStock = map[string]bool{}
			m.wizardSubtotal = 0
			m.wizard = m.wizard0(it, dm.Groups)
			m.wizardOpen = true
			// Configure page 0 (the first variant group): per-choice prices or a
			// probed subtotal, depending on the data.
			nm, cmd := m.enterVariantPage()
			return nm, cmd
		}
		m.customize = screens.NewCustomize(it)
		m.customizeOpen = true
		return m, nil
	case datasource.CartSyncedMsg:
		if m.wizardOpen {
			switch m.wizardPhase {
			case wzPickVariant:
				return m, nil // idle; ignore any stray sync result
			case wzPricing:
				// Result of a subtotal probe (the current variant selection). Read
				// the item line's price + harvest availability; the page is now
				// interactive with the live subtotal.
				if dm.Err == nil {
					mergeStock(m.wizardStock, dm.Cart.ValidAddons)
					m.wizardOptional = applyStock(m.wizardOptional, m.wizardStock)
					if p := m.itemLinePrice(dm.Cart); p > 0 {
						m.wizardSubtotal = p
						m.wizard = m.wizard.WithSubtotal(p, true)
					} else {
						m.wizard = m.wizard.WithSubtotal(0, false)
					}
				} else {
					// Couldn't price this combo (e.g. needs a required add-on we
					// didn't include) — leave the subtotal unpriced rather than wrong.
					m.wizard = m.wizard.WithSubtotal(0, false)
				}
				m.wizardPhase = wzPickVariant
				m.wizard = m.wizard.WithLoading(false)
				return m, nil
			case wzVarScope:
				// Result of a next-variant-group probe: if the server accepts this
				// candidate, it's the valid variation for the chosen outer variant —
				// record it and SKIP the remaining same-named candidates (early exit).
				g := m.wizardVarGroups[m.wizardVarShown]
				if dm.Err == nil {
					mergeStock(m.wizardStock, dm.Cart.ValidAddons)
					v := m.wizardScopeChoices[m.wizardScopeIdx]
					if !m.wizardScopeSeen[v.Name] {
						m.wizardScopeSeen[v.Name] = true
						m.wizardScopeValid = append(m.wizardScopeValid, v)
					}
				}
				m.wizardScopeIdx++
				// Skip candidates whose name we already resolved.
				for m.wizardScopeIdx < len(m.wizardScopeChoices) && m.wizardScopeSeen[m.wizardScopeChoices[m.wizardScopeIdx].Name] {
					m.wizardScopeIdx++
				}
				if m.wizardScopeIdx < len(m.wizardScopeChoices) {
					return m, m.scopeVariantCmd()
				}
				// Build the scoped variant page from the accepted options.
				scoped := g
				scoped.Choices = m.wizardScopeValid
				scoped = applyStock([]catalog.OptionGroup{scoped}, m.wizardStock)[0]
				m.wizard = m.wizard.AddPage([]catalog.OptionGroup{scoped})
				m.wizardVarShown++
				nm, cmd := m.enterVariantPage()
				return nm, cmd
			case wzCrust:
				// Result of a size+candidate-crust probe. Accepted → that candidate
				// is the size's required group; rejected → try the next candidate.
				if dm.Err == nil {
					m.liveCart = dm.Cart
					mergeStock(m.wizardStock, dm.Cart.ValidAddons)
					if len(m.wizardCandidates) > 0 {
						m.wizardCrust = m.wizardCandidates[m.wizardTrialPos]
						ch, _ := firstInStockChoice(m.wizardCrust)
						m.wizardBaseSels = append(m.wizard.AllSelections(),
							catalog.Selection{GroupID: m.wizardCrust.ID, ChoiceID: ch.ID, Name: ch.Name, Price: ch.Price})
					} else {
						// Variant-only base probe (item has no required add-on group).
						m.wizardCrust = catalog.OptionGroup{}
						m.wizardBaseSels = m.wizard.AllSelections()
					}
					// Now that availability is known, stamp it onto the optional
					// groups so their probes pick in-stock choices (and all-sold-out
					// groups are skipped entirely).
					m.wizardOptional = applyStock(m.wizardOptional, m.wizardStock)
					nm, cmd := m.beginOptionalDiscovery()
					return nm, cmd
				}
				m.wizardTrialPos++
				if m.wizardTrialPos < len(m.wizardCandidates) {
					return m, m.wizardTrialCmd() // try the next candidate
				}
				m.wizard = m.wizard.WithLoading(false).WithErr("no valid options for this size")
				return m, nil
			case wzOptional:
				// Result of an optional-group probe: keep the group if accepted.
				if dm.Err == nil {
					m.liveCart = dm.Cart
					mergeStock(m.wizardStock, dm.Cart.ValidAddons)
					m.wizardOptValid = append(m.wizardOptValid, m.wizardOptional[m.wizardOptIdx])
				}
				m.wizardOptIdx++
				for m.wizardOptIdx < len(m.wizardOptional) {
					if _, ok := firstInStockChoice(m.wizardOptional[m.wizardOptIdx]); ok {
						break
					}
					m.wizardOptIdx++
				}
				if m.wizardOptIdx < len(m.wizardOptional) {
					return m, m.wizardOptionalCmd() // probe the next optional group
				}
				nm, cmd := m.finishDiscovery()
				return nm, cmd
			default: // wzChoosing — the add-on page confirm sync
				if dm.Err != nil {
					m.wizard = m.wizard.WithLoading(false).WithErr("cart: " + dm.Err.Error())
					return m, nil
				}
				m.liveCart = dm.Cart // real pricing for the bill
				it := m.wizard.Item()
				it.Options = nil
				sels := m.wizard.AllSelections()
				addons := addonsFromSelections(sels)
				price := priceFromSelections(it.Price, sels)
				m.wizardOpen = false
				m = m.commitAddNoSync(it, addons, sels, price, m.pendingRest, m.pendingSection)
				m = m.refreshAfterAdd()
				return m, nil
			}
		}
		if dm.Err != nil {
			m.cartSyncErr = "cart sync: " + dm.Err.Error()
		} else {
			m.cartSyncErr = ""
			m.liveCart = dm.Cart // real Swiggy pricing for an accurate bill
		}
		return m, nil
	case datasource.CartLoadedMsg:
		if dm.Err != nil {
			m.cartSyncErr = "cart: " + dm.Err.Error()
			return m, nil
		}
		m.cartSyncErr = ""
		m.liveCart = dm.Cart
		m.cartLoaded = true
		if m.screen == scrCart {
			m.cart = m.buildCart()
		}
		return m, nil
	case datasource.UsualsLoadedMsg:
		if dm.Err != nil {
			dbgTUI("usuals: %v", dm.Err)
		}
		m.usualsLoaded = true // fetched (success or empty); don't refire for this addr
		m.menu = m.buildMenu()
		return m, nil
	case datasource.OrderPlacedMsg:
		m.placingOrder = false
		if dm.Err != nil {
			m.orderErr = "order failed: " + dm.Err.Error()
			return m, nil
		}
		m.orderErr = ""
		eta := dm.Order.ETA
		if eta == "" {
			eta = "~40 min"
		}
		m.checkout = m.checkout.Placed(dm.Order.ID, eta)
		m.screen = scrConfirm
		m.lines = nil
		m.cartRestaurant = ""
		m.cartSection = ""
		return m, nil
	}
	if k, ok := msg.(tea.KeyMsg); ok {
		// Command palette captures all keys while open, so letters like `q`
		// type into the prompt instead of quitting (design lines 743-751).
		if m.cmdOpen {
			switch k.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.cmdOpen = false
				m.cmd = m.cmd.ClearText()
			case "enter":
				bar, action := m.cmd.Run()
				m.cmd = bar
				switch action {
				case "clear":
					// out already cleared in Run; stay open
				case "close":
					m.cmdOpen = false
				}
			case "backspace":
				m.cmd = m.cmd.Backspace()
			default:
				if k.Type == tea.KeyRunes {
					m.cmd = m.cmd.Append(string(k.Runes))
				}
			}
			return m, nil
		}

		// Authorize gate captures all keys until the user retries or quits.
		if m.needsAuth {
			switch k.String() {
			case "enter":
				return m, openBrowserCmd(m.authorizeURL)
			case "r":
				m.needsAuth = false
				return m, m.liveInitCmds()
			case "ctrl+c":
				return m, tea.Quit
			}
			return m, nil
		}

		// While the conflict modal is open it captures all keys: ← → move focus
		// between "start new" and "keep current", Enter confirms the focused
		// button. esc cancels (cart intact); ctrl+c quits; any other key is a
		// no-op so a stray press can neither dismiss the modal nor wipe the cart.
		// Default focus is "keep current", so a reflexive Enter is always safe.
		if m.addrOpen {
			switch k.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc", "a":
				m.addrOpen = false
				return m, nil
			case "enter":
				m.addrOpen = false
				prev := m.addr.ID
				m.addr = m.addrScreen.Selected()
				if !m.cartRestaurantServes(m.addr) {
					m.lines = nil
					m.cartRestaurant = ""
					m.cartSection = ""
				}
				var cmd tea.Cmd
				if m.live && m.addr.ID != prev {
					// New address → its catalog is keyed separately; reload Home.
					m.usualsLoaded = false
					m.homePending = false
					m.screen = scrMenu
					m.railActive = screens.RailHome
					m.railFocus = true
					m.searchMode = false
					cmd = m.ensureHomeLoaded()
				}
				m.menu = m.buildMenu()
				return m, cmd
			default:
				na, _ := m.addrScreen.Update(msg)
				m.addrScreen = na.(screens.Address)
				return m, nil
			}
		}

		if m.conflictOpen {
			switch k.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "left", "h":
				m.conflictSel = 0
			case "right", "l":
				m.conflictSel = 1
			case "enter":
				var syncCmd tea.Cmd
				if m.conflictSel == 0 { // start new
					if m.live && m.pendingItem.Customizable {
						// Live customizable item: clear the cart, then re-fetch its
						// options so the (possibly wizard) add restarts cleanly.
						m.lines = nil
						m.cartRestaurant = ""
						m.cartSection = ""
						pd := m.rest.PlaceData()
						m.conflictOpen = false
						return m, datasource.LoadItemOptions(m.backend, m.addr.ID, pd.SwiggyID, m.pendingItem.Name, m.pendingItem.SwiggyID)
					}
					m = m.startNewCart(m.pendingItem, m.pendingAddOns, m.pendingSelections, m.pendingPrice, m.pendingRest, m.pendingSection)
					m = m.refreshAfterAdd()
					syncCmd = m.liveCartCmd()
				}
				m.conflictOpen = false
				return m, syncCmd
			case "esc":
				m.conflictOpen = false
			}
			return m, nil
		}

		if m.wizardOpen {
			switch k.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				// Cancel: flush the draft out of the live cart (re-sync the
				// committed lines without it), then close. If nothing was sent
				// yet it's a pure local close.
				m.wizardOpen = false
				return m, m.liveCartCmd()
			case "up", "k":
				m.wizard = m.wizard.Up()
			case "down", "j":
				m.wizard = m.wizard.Down()
			case " ", "space", "left", "right", "h", "l", "x":
				m.wizard = m.wizard.Toggle()
				if m.onVariantPage() {
					// Variant changed → reconfigure this page (re-price if needed).
					nm, cmd := m.enterVariantPage()
					return nm, cmd
				}
			case "enter":
				if m.wizard.Loading() {
					return m, nil
				}
				if !m.wizard.PageValid() {
					return m, nil // required options not yet picked
				}
				if m.onVariantPage() {
					// More variant groups to go (e.g. Size after Crust): scope the
					// next group's valid options for the current selection.
					if m.wizardVarShown < len(m.wizardVarGroups) {
						nm, cmd := m.beginVariantScope()
						return nm, cmd
					}
					// Last variant group chosen → add-on discovery for the full combo.
					if page, ok := m.wizardCache[m.variantComboKey()]; ok {
						m.wizardPhase = wzChoosing
						m.wizard = m.wizard.AddPage(page)
						return m, nil
					}
					m.wizardCandidates = trialOrder(m.wizardRequired, m.wizard.SelectedVariantName())
					m.wizardTrialPos = 0
					m.wizardCrust = catalog.OptionGroup{}
					m.wizardPhase = wzCrust
					m.wizard = m.wizard.WithLoading(true)
					if len(m.wizardCandidates) == 0 {
						// No required add-on group: a variant-only probe establishes
						// the base + harvests availability before optional discovery.
						return m, m.wizardSyncCmd(m.variantSelections())
					}
					return m, m.wizardTrialCmd()
				}
				// Add-on page: confirm the full selection against the cart.
				m.wizard = m.wizard.WithLoading(true)
				return m, m.wizardCartCmd()
			}
			return m, nil
		}

		// The customise modal captures all keys while open: ↑↓ move over add-ons,
		// space/←→ toggle the focused one, Enter adds the item with the chosen set
		// (then the cart-conflict check still applies), esc cancels.
		if m.customizeOpen {
			switch k.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.customizeOpen = false
			case "up", "k":
				m.customize = m.customize.Up()
			case "down", "j":
				m.customize = m.customize.Down()
			case " ", "space", "left", "right", "h", "l", "x":
				m.customize = m.customize.Toggle()
			case "enter":
				if !m.customize.Valid() {
					return m, nil // required options not yet picked
				}
				item := m.customize.Item()
				addons := m.customize.SelectedAddOns()
				sels := m.customize.SelectedOptions()
				price := m.customize.UnitPrice()
				m.customizeOpen = false
				m = m.commitAdd(item, addons, sels, price, m.pendingRest, m.pendingSection)
				if !m.conflictOpen { // committed directly (no restaurant clash)
					m = m.refreshAfterAdd()
					return m, m.liveCartCmd()
				}
			}
			return m, nil
		}

		switch k.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
		// Double-Esc returns to the splash and replays the loading animation. It
		// is a deliberate "home" gesture, recognised only on the menu root where
		// Esc is otherwise a no-op. On every sub-screen Esc means "back one
		// level", so the timer is cleared there — walking back up the stack with
		// repeated Esc must never teleport home. Cart/address are preserved.
		if k.String() == "esc" {
			// Esc closes the restaurant-info modal first (before the rail/home gestures).
			if m.screen == scrMenu && m.restInfoOpen {
				m.restInfoOpen = false
				return m, nil
			}
			if m.screen == scrMenu && !m.menu.Searching() && !m.searchMode {
				// In live rail mode, Esc when rail is focused unfocuses the rail
				// (not a home gesture); in search mode Esc exits search.
				if m.live && m.railFocus {
					m.railFocus = false
					m.menu = m.buildMenu()
					return m, nil
				}
				if m.frame-m.lastEscFrame <= escDoubleWindow {
					m = m.toSplash()
					m.lastEscFrame = -escDoubleWindow - 1
					return m, nil
				}
				m.lastEscFrame = m.frame
				return m, nil
			}
			m.lastEscFrame = -escDoubleWindow - 1
			// fall through to per-screen single-Esc handling
		}
		if m.screen == scrSplash {
			// The decode plays on its own; the user never has to wait for it.
			// Arrows move the cursor; any other key activates the selection and
			// goes straight to the shop — even mid-animation.
			switch k.String() {
			case "up", "k":
				if m.homeSel > 0 {
					m.homeSel--
				}
			case "down", "j":
				if m.homeSel < screens.ItemCount()-1 {
					m.homeSel++
				}
			default:
				m.screen = scrMenu
			}
			return m, nil
		}
		// `:` opens the palette from any in-app screen (design line 760).
		if k.String() == ":" && m.screen != scrSplash {
			m.cmdOpen = true
			m.cmd = screens.NewCmdBar()
			return m, nil
		}
		switch m.screen {
		case scrMenu:
			// Mock path's / filter (runs menu.Update for the single-pane search).
			if m.menu.Searching() {
				nm, cmd := m.menu.Update(msg)
				m.menu = nm.(screens.Menu)
				return m, cmd
			}

			// Restaurant-info modal captures keys while open: i/esc/q close it; ↑/↓
			// browse to the prev/next restaurant (the modal follows). Everything else
			// is swallowed so the list behind it can't act.
			if m.restInfoOpen {
				switch k.String() {
				case "i", "esc", "q":
					m.restInfoOpen = false
				case "up", "k":
					if m.menu.ListCursor() > 0 {
						m.menu = m.menu.WithListCursor(m.menu.ListCursor() - 1)
					}
				case "down", "j":
					m.menu = m.menu.WithListCursor(m.menu.ListCursor() + 1)
				}
				return m, nil
			}

			// Live rail: search mode captures all printable keys + backspace + enter + esc.
			// Printable runes always append to the query. Named nav keys (↑↓ and the
			// j/k aliases when NOT typed as runes) move the result cursor. Enter either
			// submits the query (if it changed since last submit) or opens the selection.
			if m.live && len(m.chips) > 0 && m.searchMode && !m.railFocus {
				// Every key but a second ← clears the "at left edge" latch.
				wasAtEdge := m.searchAtLeftEdge
				m.searchAtLeftEdge = false

				// Printable rune: insert at the caret (j/k are letters here, not nav).
				if k.Type == tea.KeyRunes {
					m.searchInsert(string(k.Runes))
					m.menu = m.buildMenu()
					return m, nil
				}
				// Space arrives as its own key type — insert it for multi-word search.
				if k.Type == tea.KeySpace || k.String() == " " {
					m.searchInsert(" ")
					m.menu = m.buildMenu()
					return m, nil
				}
				switch k.String() {
				case "esc":
					// Esc leaves search for Home — move the rail selection to Home too
					// (otherwise the sidebar keeps Search highlighted) and re-attach
					// focus to the rail.
					m.searchMode = false
					m.searchQuery = ""
					m.searchSubmitted = ""
					m.searchCaret = 0
					m.railActive = screens.RailHome
					m.railFocus = true
					cmd := m.ensureHomeLoaded()
					m.menu = m.buildMenu()
					return m, cmd
				case "left":
					// ← moves the caret left. A SECOND ← already at the leftmost
					// (caret 0) hands control back to the rail (Search stays selected)
					// so ↑/↓ navigate the sidebar.
					if m.searchCaret > 0 {
						m.searchCaret--
						m.menu = m.buildMenu()
						return m, nil
					}
					if wasAtEdge {
						m.railActive = screens.RailSearch
						m.railFocus = true
						m.menu = m.buildMenu()
						return m, nil
					}
					m.searchAtLeftEdge = true
					return m, nil
				case "right":
					if r := []rune(m.searchQuery); m.searchCaret < len(r) {
						m.searchCaret++
					}
					m.menu = m.buildMenu()
					return m, nil
				case "up", "k":
					// Move the result cursor up (only when results are already loaded).
					if m.searchQuery == m.searchSubmitted {
						if m.menu.ListCursor() > 0 {
							m.menu = m.menu.WithListCursor(m.menu.ListCursor() - 1)
						}
					}
					return m, nil
				case "down", "j":
					// Move the result cursor down (only when results are already loaded).
					if m.searchQuery == m.searchSubmitted {
						m.menu = m.menu.WithListCursor(m.menu.ListCursor() + 1)
					}
					return m, nil
				case "enter":
					// If results are already loaded for the current query, Enter opens
					// the selected result — same downstream path as the Home/category list.
					// If the query has changed since the last submit (or nothing submitted
					// yet), Enter submits the query.
					if m.searchQuery != "" && m.searchQuery == m.searchSubmitted {
						// Results loaded — open the selected place.
						if p, ok := m.menu.Selected(); ok {
							m.rest = screens.NewRestaurant(p, m.qtyMap(), m.cartChip()).WithAddr(m.addr)
							m.screen = scrRestaurant
							if p.SwiggyID != "" {
								return m, datasource.LoadMenu(m.backend, m.snap, m.addr.ID, p.SwiggyID)
							}
						}
						return m, nil
					}
					// Submit the query — never per-keystroke.
					var cmd tea.Cmd
					if m.searchQuery != "" {
						m.searchSubmitted = m.searchQuery
						cmd = m.searchLoad(m.searchQuery)
						m.searchPending = cmd != nil // show "searching…" until results land
						m.searchCorrected = ""       // cleared; the load's msg may set it
					}
					m.menu = m.buildMenu()
					return m, cmd
				case "backspace":
					// Delete the rune BEFORE the caret.
					if r := []rune(m.searchQuery); m.searchCaret > 0 && m.searchCaret <= len(r) {
						m.searchQuery = string(r[:m.searchCaret-1]) + string(r[m.searchCaret:])
						m.searchCaret--
					}
					m.menu = m.buildMenu()
					return m, nil
				}
				return m, nil
			}

			// Live rail: rail-focused keys.
			if m.live && len(m.chips) > 0 && m.railFocus {
				cats := make([]string, len(m.chips))
				for i, c := range m.chips {
					cats[i] = c.Label
				}
				rail := screens.NewRail(cats).WithActive(m.railActive)
				// On the Search entry, typing starts searching immediately (no Enter):
				// commit into the input (drop rail focus) and append the character.
				if m.railActive == screens.RailSearch {
					if k.Type == tea.KeyRunes || k.Type == tea.KeySpace {
						m.searchMode = true
						m.railFocus = false
						if k.Type == tea.KeySpace {
							m.searchInsert(" ")
						} else {
							m.searchInsert(string(k.Runes))
						}
						m.menu = m.buildMenu()
						return m, nil
					}
				}
				switch k.String() {
				case "right", "l", "esc":
					m.railFocus = false
					m.menu = m.buildMenu()
					return m, nil
				case "up", "k":
					if m.railActive > 0 {
						m.railActive--
					}
					m.syncSearchEntry()
					cmd := m.loadForRail(rail)
					m.menu = m.buildMenu()
					return m, cmd
				case "down", "j":
					if m.railActive < rail.Len()-1 {
						m.railActive++
					}
					m.syncSearchEntry()
					cmd := m.loadForRail(rail)
					m.menu = m.buildMenu()
					return m, cmd
				case "enter":
					m.railFocus = false
					switch m.railActive {
					case screens.RailSearch:
						m.searchMode = true
						m.searchQuery = ""
						m.searchCaret = 0
					case screens.RailHome:
						m.searchMode = false
						m.menu = m.buildMenu()
						return m, m.ensureHomeLoaded()
					default:
						if _, isCat := rail.IsCategory(m.railActive); isCat {
							m.searchMode = false
							cmd := m.loadForRail(rail)
							m.menu = m.buildMenu()
							return m, cmd
						}
					}
					m.menu = m.buildMenu()
					return m, nil
				case "c":
					m.railFocus = false
					m.menu = m.buildMenu()
					cmd := m.openCartCmd()
					return m, cmd
				case "a":
					m.railFocus = false
					m.addrScreen = screens.NewAddress(m.repo.Addresses(), m.addr.ID)
					m.addrOpen = true
					return m, nil
				case "tab":
					m.railFocus = false
					m.vertical = 1
					m.screen = scrInstamart
					return m, nil
				}
				return m, nil
			}

			// Live rail: main-list mode (not rail-focused, not search).
			// ← focuses the rail.
			if m.live && len(m.chips) > 0 {
				switch k.String() {
				case "left", "h":
					m.railFocus = true
					m.menu = m.buildMenu()
					return m, nil
				case "right", "l":
					// no-op in live main list (was chip nav, replaced by rail)
					return m, nil
				case "up", "k":
					if m.menu.ListCursor() > 0 {
						m.menu = m.menu.WithListCursor(m.menu.ListCursor() - 1)
					}
					return m, nil
				case "down", "j":
					m.menu = m.menu.WithListCursor(m.menu.ListCursor() + 1)
					return m, nil
				case "enter":
					if p, ok := m.menu.Selected(); ok {
						m.rest = screens.NewRestaurant(p, m.qtyMap(), m.cartChip()).WithAddr(m.addr)
						m.screen = scrRestaurant
						if p.SwiggyID != "" {
							return m, datasource.LoadMenu(m.backend, m.snap, m.addr.ID, p.SwiggyID)
						}
					}
					return m, nil
				case "i":
					// Open the restaurant-info modal for the selected place.
					if _, ok := m.menu.Selected(); ok {
						m.restInfoOpen = true
					}
					return m, nil
				case "c":
					cmd := m.openCartCmd()
					return m, cmd
				case "a":
					m.addrScreen = screens.NewAddress(m.repo.Addresses(), m.addr.ID)
					m.addrOpen = true
					return m, nil
				case "tab":
					m.vertical = 1
					m.screen = scrInstamart
					return m, nil
				}
				return m, nil
			}

			// Mock (non-live) path: unchanged section-tab + cursor nav.
			switch k.String() {
			case "enter":
				if p, ok := m.menu.Selected(); ok {
					m.rest = screens.NewRestaurant(p, m.qtyMap(), m.cartChip()).WithAddr(m.addr)
					m.screen = scrRestaurant
				}
				return m, nil
			case "right", "l":
				// Mock mode: non-cyclable section tabs, clamp at the last tab (snacks)
				if i := sectionIndex(m.section); i < len(menuTabs)-1 {
					m.section = menuTabs[i+1]
					m.menu = m.buildMenu()
				}
				return m, nil
			case "left", "h":
				// Mock mode: non-cyclable section tabs, clamp at the first tab (coffee)
				if i := sectionIndex(m.section); i > 0 {
					m.section = menuTabs[i-1]
					m.menu = m.buildMenu()
				}
				return m, nil
			case "1":
				m.section = catalog.SectionCoffee
				m.menu = m.buildMenu()
				return m, nil
			case "2":
				m.section = catalog.SectionFood
				m.menu = m.buildMenu()
				return m, nil
			case "3":
				m.section = catalog.SectionSnacks
				m.menu = m.buildMenu()
				return m, nil
			case "u":
				if usual, ok := m.repo.Usual(m.addr); ok {
					rest := ""
					var sec catalog.Section
					if p, ok := m.repo.Menu(usual.PlaceID); ok {
						rest = p.Name
						sec = p.Section
					}
					m = m.beginAdd(usual.Item, rest, sec)
					if !m.customizeOpen && !m.conflictOpen {
						m.menu = m.menu.WithCartChip(m.cartChip())
					}
				}
				return m, nil
			case "c":
				cmd := m.openCartCmd()
				return m, cmd
			case "a":
				m.addrScreen = screens.NewAddress(m.repo.Addresses(), m.addr.ID)
				m.addrOpen = true
				return m, nil
			case "tab":
				return m, nil
			default:
				nm, cmd := m.menu.Update(msg)
				m.menu = nm.(screens.Menu)
				return m, cmd
			}
		case scrRestaurant:
			if m.rest.Searching() {
				nr, cmd := m.rest.Update(msg)
				m.rest = nr.(screens.Restaurant)
				return m, cmd
			}
			// The item-info modal captures keys while open: i/esc/q close it; ↑/↓
			// browse to the prev/next dish (the modal follows). Everything else is
			// swallowed so the list behind it can't act.
			if m.rest.InfoOpen() {
				switch k.String() {
				case "i", "esc", "q":
					m.rest = m.rest.WithInfo(false)
				case "up", "k", "down", "j":
					nr, _ := m.rest.Update(msg)
					m.rest = nr.(screens.Restaurant)
				}
				return m, nil
			}
			switch k.String() {
			case "esc":
				m.screen = scrMenu
				return m, nil
			case "left", "h", "[":
				// ← / → navigate the top category bar.
				m.rest = m.rest.PrevCategory()
				return m, nil
			case "right", "l", "]":
				m.rest = m.rest.NextCategory()
				return m, nil
			case "enter", "+", "=":
				// Enter / + add the focused dish (opens the customise sheet when
				// needed). ↑/↓ stay free for moving through the list.
				return m.restIncSelected()
			case "-", "_", "backspace", "delete":
				// − / Del / Backspace removes a unit of the focused dish (to zero it
				// leaves the cart).
				return m.restDecSelected()
			case "c":
				cmd := m.openCartCmd()
				return m, cmd
			case "v":
				m.rest = m.rest.WithVegOnly(!m.rest.VegOnly())
				return m, nil
			default:
				nr, cmd := m.rest.Update(msg)
				m.rest = nr.(screens.Restaurant)
				return m, cmd
			}
		case scrCart:
			switch k.String() {
			case "esc":
				m.screen = scrMenu
				return m, nil
			case "enter":
				if len(m.cartScreenLines()) > 0 {
					m.checkout = screens.NewCheckout(m.cartHeader(), m.addr, m.cartScreenLines(), m.cartEta()).WithBill(m.billFromLive())
					m.screen = scrCheckout
					return m, nil
				}
			}
			// When showing the live Swiggy cart, the lines are Swiggy's truth —
			// qty editing would overwrite the local lines (which carry the
			// variant/add-on selections) with the flattened display copy. So the
			// live cart is display-only; ↑↓ still move the cursor. Edit quantities
			// from the restaurant screen (+/−).
			liveDisplay := m.live && m.cartLoaded
			mutated := false
			switch k.String() {
			case "j", "down":
				m.cart = m.cart.Down()
			case "k", "up":
				m.cart = m.cart.Up()
			case "right", "l":
				if !liveDisplay {
					m.cart = m.cart.Right()
					mutated = true
				}
			case "left", "h":
				if !liveDisplay {
					m.cart = m.cart.Left()
					mutated = true
				}
			}
			if liveDisplay {
				return m, nil // display-only; never write Swiggy's lines back
			}
			// keep router's authoritative lines in sync with cart edits
			m.lines = m.cart.Lines()
			// emptying the cart here must also release the restaurant binding —
			// otherwise the stale name lingers on the next cart view (and would
			// wrongly trigger a cart-conflict against a different restaurant).
			if len(m.lines) == 0 {
				m.cartRestaurant = ""
				m.cartSection = ""
			}
			m.menu = m.menu.WithCartChip(m.cartChip())
			if mutated {
				// qty/remove on the cart screen must reach Swiggy too (UpdateCart
				// when items remain, flush when the cart just went empty).
				return m, m.liveCartCmd()
			}
			return m, nil
		case scrCheckout:
			switch k.String() {
			case "esc":
				m.screen = scrCart
				return m, nil
			case "enter":
				if m.live && !m.placingOrder {
					m.placingOrder = true
					m.orderErr = ""
					return m, tea.Sequence(m.liveSyncCart(), datasource.PlaceOrderCmd(m.backend, m.snap, m.addr.ID))
				}
				if !m.live {
					m.checkout = m.checkout.Placed(orderID(m.checkout.Lines()), "~40 min")
					m.screen = scrConfirm
				}
				return m, nil
			}
		case scrConfirm:
			switch k.String() {
			case "enter", "t":
				m.track = screens.NewTracking(m.checkout.Place(), m.addr.Line, m.checkout.OrderID())
				m.screen = scrTracking
				m.trackStep = 1
				m.trackTick = 0
				return m, nil
			case "esc":
				m.lines = nil
				m.imLines = nil
				m.cartRestaurant = ""
				m.cartSection = ""
				m.menu = m.buildMenu()
				m.screen = scrMenu
				return m, nil
			}
		case scrTracking:
			if k.String() == "esc" {
				m.lines = nil
				m.imLines = nil
				m.cartRestaurant = ""
				m.cartSection = ""
				m.screen = scrMenu
				m.menu = m.buildMenu()
				return m, nil
			}
		case scrInstamart:
			switch k.String() {
			case "esc", "tab":
				// esc always returns to Restaurants browse. When live and we entered
				// via vertical toggle, also reset the vertical state.
				m.vertical = 0
				m.screen = scrMenu
				return m, nil
			case "enter", "right", "l":
				it, ok := m.inst.Selected()
				if !ok {
					return m, nil
				}
				m.imLines = appendOrInc(m.imLines, it, nil, nil, 0)
				ci := m.inst.CursorIndex()
				m.inst = screens.NewInstamart(m.repo.InstamartItems(m.addr), m.imQtyMap(), m.imCartChip()).WithCursor(ci)
				return m, nil
			case "left", "h":
				it, ok := m.inst.Selected()
				if !ok {
					return m, nil
				}
				m.imLines = decLastByItem(m.imLines, it.ID)
				ci := m.inst.CursorIndex()
				m.inst = screens.NewInstamart(m.repo.InstamartItems(m.addr), m.imQtyMap(), m.imCartChip()).WithCursor(ci)
				return m, nil
			case "c":
				m.imCart = screens.NewCart("Instamart", m.imLines).WithEta(screens.InstamartETA)
				if m.imCartTotal() < InstamartMin {
					m.imCart = m.imCart.WithMinNotice(fmt.Sprintf("add ₹%d more — ₹%d minimum on Instamart", InstamartMin-m.imCartTotal(), InstamartMin))
				}
				m.screen = scrImCart
				return m, nil
			default:
				ni, cmd := m.inst.Update(msg)
				m.inst = ni.(screens.Instamart)
				return m, cmd
			}
		case scrImCart:
			switch k.String() {
			case "esc":
				m.screen = scrInstamart
				return m, nil
			case "j", "down":
				m.imCart = m.imCart.Down()
			case "k", "up":
				m.imCart = m.imCart.Up()
			case "right", "l":
				m.imCart = m.imCart.Right()
			case "left", "h":
				m.imCart = m.imCart.Left()
			case "enter":
				if m.imCartTotal() >= InstamartMin {
					m.checkout = screens.NewCheckout("Instamart", m.addr, m.imLines, screens.InstamartETA)
					m.screen = scrCheckout
					return m, nil
				}
			}
			m.imLines = m.imCart.Lines()
			return m, nil
		}
	}
	return m, nil
}

// statusHints rotate in the status bar (design line 925).
var statusHints = []string{"type : for commands", "247 devs online", "DEVFRIDAY −₹50", "esc esc · home", "ssh consolestore.in"}

// screenLabel maps the current screen to the status-bar label (design line 836).
func (m Model) screenLabel() string {
	switch m.screen {
	case scrMenu:
		return "menu"
	case scrRestaurant:
		return "menu"
	case scrCart:
		return "cart"
	case scrCheckout:
		return "checkout"
	case scrConfirm:
		return "order placed"
	case scrTracking:
		return "tracking"
	case scrAddress:
		return "menu"
	case scrInstamart:
		return "instamart"
	case scrImCart:
		return "cart"
	default:
		return "menu"
	}
}

// statusBar renders the bottom bar for the current frame/screen.
func (m Model) statusBar() string {
	hint := statusHints[(m.frame/27)%len(statusHints)]
	if m.orderErr != "" {
		hint = m.orderErr
	} else if m.cartSyncErr != "" {
		hint = m.cartSyncErr
	}
	return components.StatusBar(m.addr.Line, m.screenLabel(), hint, "12.4", m.blinkOn())
}

// listRows is the list viewport height: the window height minus the screen's
// fixed chrome (header + detail + footer). 0 when the size is unknown (show
// all). This keeps the header and footer on screen no matter how long the list
// or how short the window.
func (m Model) listRows(chrome int) int {
	if m.h == 0 {
		return 0
	}
	if n := m.h - chrome; n >= 3 {
		return n
	}
	return 3
}

func (m Model) View() string {
	if m.needsAuth {
		gate := "  console.store needs to connect to your Swiggy account.\n\n" +
			"  Opening your browser to log in to Swiggy…\n\n" +
			"  If it didn't open, copy this link:\n\n" +
			"     " + m.authorizeURL + "\n\n" +
			"  [ Enter ] open in browser       waiting for authorization…\n"
		if m.w == 0 || m.h == 0 {
			return gate
		}
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, gate)
	}

	// Splash is centered in the viewport (design lines 196-228). We render on
	// the terminal's own background — wrapping the frame in a lipgloss
	// Background tears on inner colour resets (banding), and a dark terminal
	// already provides the #15161f-ish canvas.
	if m.screen == scrSplash {
		sp := m.splash.WithDecode(m.decodeStep).WithFrame(m.frame).WithSplashTick(m.splashTick).
			WithSelection(m.homeSel).WithPhrase(m.splashPhrase).View()
		if m.w == 0 || m.h == 0 {
			return sp
		}
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, sp)
	}

	// The customise / conflict modals take over the viewport, centered. They are
	// blocking, so the context behind them is not needed.
	if m.wizardOpen {
		dialog := m.wizard.WithViewport(m.h).View()
		if m.w == 0 || m.h == 0 {
			return dialog
		}
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, dialog)
	}
	if m.customizeOpen {
		dialog := m.customize.WithViewport(m.h).View()
		if m.w == 0 || m.h == 0 {
			return dialog
		}
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, dialog)
	}
	if m.conflictOpen {
		dialog := m.conflict.WithFocus(m.conflictSel).View()
		if m.w == 0 || m.h == 0 {
			return dialog
		}
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, dialog)
	}
	// Item-info modal ('i' on the restaurant screen) — a centered overlay.
	if m.screen == scrRestaurant && m.rest.InfoOpen() {
		if card := m.rest.InfoView(0); card != "" {
			if m.w == 0 || m.h == 0 {
				return card
			}
			return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, card)
		}
	}
	// Restaurant-info modal ('i' on the browse list) — a centered overlay.
	if m.screen == scrMenu && m.restInfoOpen {
		if p, ok := m.menu.Selected(); ok {
			card := screens.RestaurantInfoCard(p)
			if m.w == 0 || m.h == 0 {
				return card
			}
			return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, card)
		}
	}
	// Address switcher modal ('a') — a centered overlay over the current screen.
	if m.addrOpen {
		card := m.addrScreen.ModalView()
		if m.w == 0 || m.h == 0 {
			return card
		}
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, card)
	}

	var body string
	switch m.screen {
	case scrRestaurant:
		chrome := 14 + screens.BrandHeaderLines
		body = m.rest.WithMaxRows(m.listRows(chrome)).View()
	case scrCart:
		body = m.cart.View()
	case scrCheckout, scrConfirm:
		body = m.checkout.WithPlacing(m.placingOrder).View(m.frame)
	case scrTracking:
		body = m.track.View(m.trackStep, m.frame, m.spin())
	case scrInstamart:
		if m.live {
			// Live vertical: show "coming soon" placeholder until Instamart is built.
			body = "  " + theme.BrandStyle.Render("Instamart") + "\n\n  " +
				theme.DimStyle.Render("groceries in minutes — coming soon") + "\n\n" +
				"  " + theme.FaintStyle.Render("tab · back to Restaurants")
		} else {
			body = m.inst.WithMaxRows(m.listRows(11 + screens.BrandHeaderLines)).View()
		}
	case scrImCart:
		body = m.imCart.View()
	default: // scrMenu
		body = m.menu.WithMaxRows(m.listRows(13 + screens.BrandHeaderLines)).View()
	}

	// The footer — the screen's hint line + optional command palette + status
	// bar — is pinned to the bottom. The hint is the screen's last rendered
	// line; lift it out so it sits WITH the status bar instead of floating
	// after the content with a large void between them.
	content, hint := splitHint(body)

	// Centered brand logo at the top of every post-landing screen, with a gap
	// below it (the splash has its own big wordmark, so it is excluded above).
	content = screens.BrandBanner(components.FrameWidth()) + content

	footer := ""
	if hint != "" {
		footer += hint + "\n\n"
	}
	if m.cmdOpen {
		footer += m.cmd.View(m.blinkOn()) + "\n"
	}
	footer += m.statusBar()

	if m.w == 0 || m.h == 0 {
		return content + "\n" + footer
	}
	gap := m.h - lipgloss.Height(content) - lipgloss.Height(footer)
	if gap < 1 {
		gap = 1
	}
	return content + strings.Repeat("\n", gap) + footer
}

// splitHint separates a screen body into its content and its trailing footer
// hint (the last non-empty line). Trailing blank padding is trimmed from the
// content so the over-padding below a list doesn't survive.
func splitHint(body string) (content, hint string) {
	lines := strings.Split(body, "\n")
	last := len(lines) - 1
	for last >= 0 && strings.TrimSpace(lines[last]) == "" {
		last--
	}
	if last < 0 {
		return body, ""
	}
	hint = lines[last]
	c := lines[:last]
	for len(c) > 0 && strings.TrimSpace(c[len(c)-1]) == "" {
		c = c[:len(c)-1]
	}
	return strings.Join(c, "\n"), hint
}

func errIsNeedsAuth(err error) bool {
	return err != nil && errors.Is(err, datasource.ErrNeedsAuth)
}

// liveSyncCart assembles the current food cart and dispatches a SyncCart Cmd
// to keep Swiggy's cart in sync. No-op when not live, cart is empty, or the
// restaurant has no SwiggyID (e.g. if menu hasn't loaded yet). Items without
// a SwiggyID are skipped — they can't be referenced by Swiggy.
// cartScreenLines are the lines to DISPLAY on the cart/checkout screens. In live
// mode, once the Swiggy cart is fetched, they come straight from Swiggy (the
// source of truth: real names, per-unit prices, and any pre-existing items) —
// not the local in-memory approximation. Falls back to the local lines in mock
// mode or before the fetch returns.
func (m Model) cartScreenLines() []screens.CartLine {
	if m.live && m.cartLoaded {
		out := make([]screens.CartLine, 0, len(m.liveCart.Lines))
		for _, l := range m.liveCart.Lines {
			out = append(out, screens.CartLine{
				Item:  catalog.Item{ID: l.ItemID, SwiggyID: l.ItemID, Name: l.Name, Price: l.Price},
				Qty:   l.Quantity,
				Price: l.Price, // Swiggy final per-unit price (incl. variant + add-ons)
			})
		}
		return out
	}
	return m.lines
}

// buildCart constructs the cart screen from the display lines + live bill.
func (m Model) buildCart() screens.Cart {
	return screens.NewCart(m.cartHeader(), m.cartScreenLines()).
		WithEta(m.cartEta()).WithBill(m.billFromLive()).WithLiveSync(m.live, m.cartSyncErr)
}

// openCartCmd opens the cart screen and, in live mode, fetches the real Swiggy
// cart so the display reflects exactly what Place Order will charge.
func (m *Model) openCartCmd() tea.Cmd {
	m.cartLoaded = false
	m.cart = m.buildCart()
	m.screen = scrCart
	if !m.live {
		return nil
	}
	rest := m.cartRestaurant
	if rest == "" {
		rest = m.rest.PlaceData().Name
	}
	if rest == "" {
		return nil
	}
	return datasource.LoadCart(m.backend, m.addr.ID, rest)
}

// billFromLive returns Swiggy's real pricing breakdown for the cart/checkout
// bill. In mock mode (or before any sync), Live is false and screens fall back
// to the design's delivery/coupon math.
func (m Model) billFromLive() screens.Bill {
	if !m.live || m.liveCart.Total == 0 {
		return screens.Bill{}
	}
	return screens.Bill{
		ItemTotal: m.liveCart.ItemTotal,
		Delivery:  m.liveCart.Delivery,
		Taxes:     m.liveCart.Taxes,
		ToPay:     m.liveCart.Total,
		Live:      true,
	}
}

// liveCartCmd syncs the Swiggy cart after any local cart mutation: an UpdateCart
// when items remain, or a flush when the cart just went empty (UpdateCart can't
// express an empty cart — it requires a restaurant id).
func (m Model) liveCartCmd() tea.Cmd {
	if !m.live {
		return nil
	}
	if len(m.lines) == 0 {
		return datasource.ClearCartCmd(m.backend)
	}
	return m.liveSyncCart()
}

func (m Model) liveSyncCart() tea.Cmd {
	if !m.live || len(m.lines) == 0 {
		dbgTUI("liveSyncCart: nil (live=%v lines=%d)", m.live, len(m.lines))
		return nil
	}
	pid := m.cartPlaceID()
	p, ok := m.repo.Menu(pid)
	if !ok || p.SwiggyID == "" {
		dbgTUI("liveSyncCart: nil (cartRestaurant=%q cartPlaceID=%q menuFound=%v swiggyID=%q)", m.cartRestaurant, pid, ok, p.SwiggyID)
		return nil
	}
	items := m.cartItemsForLines()
	if len(items) == 0 {
		dbgTUI("liveSyncCart: nil (no items with SwiggyID; lines=%d)", len(m.lines))
		return nil
	}
	dbgTUI("liveSyncCart: SYNC restaurant=%q swiggyRest=%q items=%d", m.cartRestaurant, p.SwiggyID, len(items))
	return datasource.SyncCart(m.backend, m.snap, m.addr.ID, p.SwiggyID, m.cartRestaurant, items)
}
