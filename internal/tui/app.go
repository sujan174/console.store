package tui

import (
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"consolestore/internal/broker/api"
	"consolestore/internal/catalog"
	"consolestore/internal/catalog/mem"
	swiggysnap "consolestore/internal/catalog/swiggy"
	"consolestore/internal/config"
	"consolestore/internal/localstore"
	"consolestore/internal/tui/components"
	"consolestore/internal/tui/datasource"
	"consolestore/internal/tui/render"
	"consolestore/internal/tui/screens"
	"consolestore/internal/tui/theme"
)

// dbgTUI logs to stderr (redirect with CONSOLE_DEBUG_LOG) when CONSOLE_DEBUG_TUI=1,
// for diagnosing the live cart-sync path. The TUI alt-screen otherwise hides stderr.
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
// liquid without flooding the terminal; frame-derived cadences below are scaled
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
	scrWelcome // first-run onboarding: food animation + intro card (appended last so no other iota shifts)
)

type Model struct {
	repo    catalog.Repository
	addr    catalog.Address
	section catalog.Section

	screen         screen
	menu           screens.Menu
	rest           screens.Restaurant
	addrScreen     screens.Address
	checkout       screens.Checkout
	lines          []screens.CartLine
	cartRestaurant string
	cartSection    catalog.Section // "" for non-snacks; SectionSnacks when snacks cart
	// cartForeign marks a cart seeded at launch from a pre-existing Swiggy cart
	// whose owning restaurant we could not identify (Swiggy returned items but no
	// restaurant name). While true, ANY add conflicts — we can't prove the new
	// item belongs to the same place, so we must prompt to replace rather than
	// silently mix restaurants. Cleared the moment a real in-app cart is started.
	cartForeign bool

	// confirmed* is the last Swiggy-CONFIRMED cart state (after a successful
	// pull/load/sync). Cart mutations are optimistic; if the resulting sync FAILS
	// the cart rolls back to this snapshot and surfaces an error, so the local cart
	// is always a faithful replica of the real Swiggy cart (no silent divergence).
	confirmedLines      []screens.CartLine
	confirmedRestaurant string
	confirmedSection    catalog.Section
	confirmedForeign    bool
	cartConfirmed       bool // true once a baseline has been captured (vs zero-value)

	// unavailableItems holds the Swiggy menu-item ids the live cart reports as out
	// of stock. An item can look in-stock on the menu yet be unavailable once
	// added; Swiggy still prices it in the bill but the order can't be placed. We
	// flag the line and block checkout. Keyed by SwiggyID (== cart menu_item_id).
	unavailableItems map[string]bool

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

	// order-confirm modal: shown on ↵ in checkout before an order actually
	// fires. Default focus is "yes" so a reflexive Enter still places the
	// order — this is a safety pause, not an extra hoop.
	orderConfirmOpen bool
	orderConfirmSel  int // focused button: 0 = yes (default), 1 = no

	// progressive catalog streaming. Generation counters stamp every page
	// Cmd; a page landing with a stale gen belongs to a dead stream (the
	// user navigated away or restarted the load) and is dropped — which also
	// kills the chain, since the next page is only ever fired from the
	// current page's handler.
	menuGen         int
	menuLoadingMore bool // pages still arriving for the open restaurant
	menuPartial     bool // stream died mid-way; menu on screen is incomplete
	menuStaged      bool // stream fills the staging area behind a disk-cache seed
	placesGen       int
	// seededQueries marks place-list snapshot entries that came from the
	// disk cache (not a live fetch) — ensureQuery must still fire a live
	// refresh for them exactly once.
	seededQueries map[string]bool
	// deferredLaunch holds the below-the-fold launch loads (usuals, cart
	// pull, active-order check) until the first visible list paints, so the
	// serialized rate-limiter slots go to what the user is looking at.
	deferredLaunch []tea.Cmd

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
	imCart  screens.Cart // legacy scrImCart render target; scrImCart is no longer navigated to

	// Instamart live-data state (mirrors the food cart's confirmed/sync fields,
	// kept separate since the two verticals never share a cart).
	imQuery        string     // "" = your-usuals go-to list; non-empty = last submitted search
	imCartPulled   bool       // PullIMCart has fired once this session (re-armed on address change)
	imPending      bool       // an IMProducts load is in flight (shows "loading…")
	imLiveCart     api.IMCart // last synced/fetched Instamart cart (real lines + pricing)
	imCartSyncErr  string     // last Instamart cart-sync error; shown on the IM checkout
	imOrderErr     string     // last Instamart order-placement error
	imCartMutating bool       // true while an IM reduce/delete sync is in flight (freezes input)
	imCartRebuilt  bool       // true after a one-shot cart-expired auto-rebuild, to avoid retry loops

	// imConfirmed* is the last Instamart-cart-CONFIRMED state (mirrors
	// confirmedLines/cartConfirmed for food) — the rollback target for a failed sync.
	imConfirmedLines []screens.CartLine
	imCartConfirmed  bool

	// Instamart search (mirrors scrMenu's search fields, but submit-only — no
	// live-typing filter; the rail's Search entry starts it the same way Food's does).
	imSearchMode  bool
	imSearchQuery string
	imSearchCaret int
	// imSearchSubmitted is the last query actually submitted via search (not a
	// rail category). While imQuery == imSearchSubmitted the results carry a
	// persistent search chip, and `/` re-opens the editor PRE-FILLED with it so
	// a search can always be edited instead of retyped.
	imSearchSubmitted string

	// Instamart cart-sync debounce (mirrors cartSyncPending/cartSyncFrame).
	imCartSyncPending bool
	imCartSyncFrame   int

	// imConfirmPending gates the order-confirm modal on a fresh sync: when the
	// user hits Enter to place while the server cart lags the local lines (a
	// debounced qty bump not yet flushed), we sync FIRST and open the modal only
	// once IMCartSyncedMsg lands — so the confirmed total is always Swiggy's
	// authoritative bill for exactly the lines being ordered, never a stale one.
	imConfirmPending bool

	// Instamart rail nav state — mirrors railActive/railFocus/railSettlePending/
	// railSettleFrame exactly (own fields since Food and Instamart rails are
	// independent columns with independent cursors). Index 0 = Search, 1 =
	// Usuals (the go-to list, Food's Home equivalent), 2+ = categories.
	imRailActive        int
	imRailFocus         bool
	imRailSettlePending bool
	imRailSettleFrame   int
	// imLoadedQueries marks queries that have received a LIVE IMProducts load
	// this session (mirrors seededQueries/ensureQuery's live-loaded dedupe, but
	// simpler: Instamart has no disk cache — see the design note on
	// ensureIMQuery). Revisiting a loaded query renders instantly from the
	// snapshot with no re-fetch.
	imLoadedQueries map[string]bool
	// imChips is the fixed, curated Instamart rail category set (no
	// config.json override — unlike food's m.chips).
	imChips []config.Category

	// checkoutVertical routes the merged checkout page + confirmPlaceOrder: 0 =
	// food (default), 1 = Instamart. Screens/keys that mutate "the cart" on
	// scrCheckout must branch on this instead of assuming food.
	checkoutVertical int

	splash       screens.Splash
	welcome      screens.Welcome // first-run onboarding screen (holds its own phase)
	welcomeTick  int             // ticks since welcome entry; drives the food animation
	decodeStep   int
	splashTick   int    // ticks since the splash was (re)entered; phases the shimmer
	splashPhrase string // Minecraft-style splash line, re-rolled each time we land
	homeSel      int    // selected home-menu item on the splash
	lastEscFrame int    // frame of the previous Esc (for double-Esc home detection)

	track          screens.Tracking
	trackTick      int
	confirmTick    int                    // ticks since scrConfirm was entered; auto-advances to scrTracking
	nowUnix        int64                  // updated each tick — passed to tracking View for live elapsed
	activeOrder    localstore.ActiveOrder // last placed order (persisted)
	hasActiveOrder bool                   // true when activeOrder is set and not yet delivered/cleared

	// altOrder is the OTHER vertical's live delivery when a restaurant order
	// and an Instamart order are in flight at the same time. Session-discovered
	// (never persisted — active-order.json holds only the primary); the splash
	// "track order" entry becomes a picker while it is set, and the primary's
	// delivery promotes it.
	altOrder      localstore.ActiveOrder
	hasAltOrder   bool
	trackPickOpen bool
	trackPick     screens.TrackPick

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
	authClient   AuthClient // polls callback completion + starts re-auth; nil on the mock path
	seeded       bool       // true when catalog/swiggy.Snapshot was pre-seeded from config; skips live init loads

	placingOrder bool     // true while PlaceOrderCmd is in-flight; blocks double-fire
	cartSyncErr  string   // last cart-sync error; shown in status bar (non-fatal)
	orderErr     string   // last order-placement error; shown in status bar
	liveCart     api.Cart // last synced/fetched Swiggy cart (real lines + pricing)
	cartLoaded   bool     // true once the live Swiggy cart is fetched for the cart screen
	cartMutating bool     // true while a reduce/delete cart sync is in flight (freezes input)

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
	// Rail-load debounce: arrowing through rail categories arms a pending load
	// instead of firing one per step, so fast-scrolling the rail doesn't spray a
	// search_restaurants per category. onTick fires the load once the cursor
	// settles (railSettleFrames). Enter still loads immediately.
	railSettlePending bool
	railSettleFrame   int
	// Cart-sync debounce: rapid qty +/− mutations update the cart optimistically
	// and arm a pending sync instead of firing an update_food_cart per keystroke,
	// so mashing +/− can't spray write-tool calls past Swiggy's rate limit. onTick
	// fires ONE sync once the keys settle (cartSettleFrames). SyncCart sends the
	// full cart snapshot (idempotent SET), so collapsing many edits into one
	// trailing sync is correct.
	cartSyncPending bool
	cartSyncFrame   int
	restInfoOpen    bool // restaurant-info modal ('i' on the browse list) is open
	addrOpen        bool // address switcher modal ('a') is open
	addrForced      bool // true while addrOpen is the forced entry gate (non-dismissible)
	addrGatePending bool // true from launch until the forced pick is satisfied
	addressesLoaded bool // true once AddressesLoadedMsg has been handled
	settingsOpen    bool // settings modal (from the splash) is open
	settingsSel     int  // selected row in the settings modal
	helpOpen        bool // help & controls modal (? / H / :help) is open
	helpScroll      int  // scroll offset within the help modal
	helpPage        int  // current page in the paginated help modal (0-indexed)
	wantOnboarding  bool // set by WithOnboarding(true); starts the session on the welcome screen

	// what's-new modal: shows release notes after an update.
	whatsnewOpen   bool     // true while the what's-new modal is showing
	whatsnewPage   int      // current page in the paginated modal (0-indexed)
	whatsnewScroll int      // scroll offset within the current page
	whatsnewLines  []string // pre-rendered lines from renderNotesMarkdown

	// release-notes fetch params (set by WithReleaseNotes).
	notesVersion string // the version whose notes to fetch and display
	notesChannel string // channel (stable / beta / alpha)
	notesCode    string // alpha access code (may be "")
	wantNotes    bool   // true when WithReleaseNotes was called
	notesReady   bool   // true once notes are fetched and waiting for splash→menu transition

	homePending  bool // Home's "popular near you" load is in flight (shows "loading…")
	usualsLoaded bool // true once LoadUsuals has been fired for the current addr
}

func New(caps render.Caps, opts ...Option) Model {
	repo := mem.New()
	section := catalog.SectionCoffee
	m := Model{repo: repo, section: section, screen: scrSplash, caps: caps, lastEscFrame: -escDoubleWindow - 1, railActive: screens.RailHome, railFocus: true, imRailActive: screens.RailHome, addrGatePending: true, seededQueries: map[string]bool{}, imLoadedQueries: map[string]bool{}}
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
	m.imChips = config.DefaultIMCategories() // fixed set, no config.json override
	m.splash = screens.NewSplash().WithCaps(caps)
	m.welcome = screens.NewWelcome(screens.DefaultLearnURL).WithCaps(caps)
	m.splashPhrase = screens.RandomPhrase("")
	// Live+seeded fires the Home load at Init() — mark it pending so the first
	// Home paint shows the "loading…" cue (matching the category pages).
	if m.live && m.seeded && m.addr.ID != "" {
		m.homePending = true
	}
	m.menu = m.buildMenu()
	m.nowUnix = time.Now().Unix()
	// Load any persisted active order (crash-resume + splash liveness).
	if ao, ok, err := localstore.LoadActiveOrder(); err == nil && ok {
		now := m.nowUnix
		expiry := ao.PlacedAt + int64(ao.ETAHiMin)*60 + 1800
		if now < expiry {
			m.activeOrder = ao
			m.hasActiveOrder = true
			m.splash = m.splash.WithOrder(fmt.Sprintf("%s · ~%d min", ao.Restaurant, ao.ETAHiMin))
		} else {
			// Expired — quietly clear it.
			_ = localstore.ClearActiveOrder()
		}
	}
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
		} else if catIdx, isCat := rail.IsCategory(m.railActive); isCat {
			// Category view: flat places path with a section header (the category
			// name) so it reads consistently with Home's "popular near you" divider.
			menu = menu.WithLoading(m.catPending)
			if catIdx < len(m.chips) {
				menu = menu.WithCategoryHeader(m.chips[catIdx].Label)
			}
		} else {
			menu = menu.WithSections(usuals, nearby).WithLoading(m.homePending)
		}

		return menu
	}

	// Mock path: single-pane section-tab list (unchanged).
	return screens.NewMenu(m.browsePlaces(), m.addr, m.section, usual, ok, m.cartChip()).
		WithCounts(counts)
}

// refreshMenu rebuilds the menu screen from fresh snapshot data while keeping
// the user's place in the list. Async loads land whenever they land — usuals
// finishing after launch, another category's debounced fetch, a streamed page
// — and rebuilding via buildMenu() alone resets the list cursor to 0, yanking
// a mid-scroll user back to the top. Data refreshes go through here; genuine
// view switches (rail move, search enter, address adoption) keep calling
// buildMenu() directly, where the reset-to-top is the correct behavior.
// WithListCursor clamps, so a shrunken list can't strand the cursor.
func (m *Model) refreshMenu() {
	cur := m.menu.ListCursor()
	m.menu = m.buildMenu().WithListCursor(cur)
}

// liveRepo casts the Repository to the swiggy snapshot repo for PlacesByQuery.
// Returns nil when not live; callers guard with m.live.
func (m Model) liveRepo() *swiggysnap.Repository {
	if r, ok := m.repo.(*swiggysnap.Repository); ok {
		return r
	}
	return nil
}

// ensureQuery starts a progressive places load for query if the snapshot
// doesn't already hold LIVE results for it. Disk-seeded entries (see
// seedPlacesFromCache) still get one live refresh — the stream's first page
// replaces the seed in place. Deduplicates: a live-loaded cache no-ops.
func (m *Model) ensureQuery(query string) tea.Cmd {
	if r := m.liveRepo(); r != nil {
		if places := r.PlacesByQuery(m.addr, query); len(places) > 0 && !m.seededQueries[query] {
			return nil // already live-loaded
		}
	}
	return m.startPlacesLoad(query)
}

// startPlacesLoad begins a fresh progressive restaurant-list stream for
// query: seed the visible list from the disk cache when available (instant
// paint), then fire page 1 — whose merge replaces the seed with live data.
// Bumping placesGen kills any previous stream's chain.
func (m *Model) startPlacesLoad(query string) tea.Cmd {
	m.placesGen++
	m.seedPlacesFromCache(query)
	return datasource.LoadPlacesPage(m.backend, m.snap, m.addr.ID, query, 0, 1, m.placesGen)
}

// seedPlacesFromCache paints the last-known restaurant list for (addr, query)
// from disk while the live fetch runs — stale-while-revalidate. Only seeds an
// empty snapshot slot, and marks the query seeded so ensureQuery still
// refreshes it.
func (m *Model) seedPlacesFromCache(query string) {
	r := m.liveRepo()
	if r == nil || m.addr.ID == "" || len(r.PlacesByQuery(m.addr, query)) > 0 {
		return
	}
	cached, ok := localstore.LoadCachedPlaces(m.addr.ID, query)
	if !ok {
		return
	}
	places := make([]catalog.Place, 0, len(cached))
	for _, p := range cached {
		places = append(places, catalog.Place{
			ID: p.ID, SwiggyID: p.ID, Name: p.Name, City: p.City,
			Section: catalog.SectionCoffee, ETA: p.ETA, Rating: p.Rating,
			Description: p.Description, Offer: p.Offer,
		})
	}
	m.snap.SetPlaces(m.addr.ID, query, places)
	m.seededQueries[query] = true
}

// placesListEmpty reports whether the snapshot has nothing to paint for query
// — the "show a loading label" test (a disk seed counts as content).
func (m Model) placesListEmpty(query string) bool {
	r := m.liveRepo()
	return r == nil || len(r.PlacesByQuery(m.addr, query)) == 0
}

// savePlacesCache persists the live list for (addr, query) so the next launch
// paints it instantly. Conversion happens synchronously (snapshot reads are
// mutex-guarded). The write is a few KB — synchronous keeps it race-free with tests/exit.
func (m Model) savePlacesCache(query string) {
	r := m.liveRepo()
	if r == nil || m.addr.ID == "" {
		return
	}
	places := r.PlacesByQuery(m.addr, query)
	if len(places) == 0 {
		return
	}
	cached := make([]localstore.CachedPlace, 0, len(places))
	for _, p := range places {
		cached = append(cached, localstore.CachedPlace{
			ID: p.SwiggyID, Name: p.Name, City: p.City,
			ETA: p.ETA, Rating: p.Rating, Description: p.Description, Offer: p.Offer,
		})
	}
	localstore.SaveCachedPlaces(m.addr.ID, query, cached)
}

// startMenuLoad begins a fresh progressive menu stream for the restaurant the
// user just opened. When the disk cache has a copy, the full cached menu
// paints instantly and the live stream fills the snapshot's STAGING area,
// promoted atomically at Done (the visible menu never shrinks mid-refresh).
// Without a cache hit, pages render progressively as they land. Bumping
// menuGen kills any previous stream's chain.
func (m *Model) startMenuLoad(swiggyID string) tea.Cmd {
	m.menuGen++
	m.menuPartial = false
	m.menuLoadingMore = true
	m.menuStaged = false
	if cached, ok := localstore.LoadCachedMenu(swiggyID); ok {
		items := make([]catalog.Item, 0, len(cached))
		for _, it := range cached {
			items = append(items, catalog.Item{
				ID: it.ID, SwiggyID: it.ID, Name: it.Name, Price: it.Price,
				Veg: it.Veg, Desc: it.Desc, Rating: it.Rating,
				Customizable: it.Customizable, Category: it.Category,
				OutOfStock: it.OutOfStock,
			})
		}
		m.snap.SetMenu(catalog.Place{ID: swiggyID, SwiggyID: swiggyID, Items: items})
		m.menuStaged = true
		m.applyMenuFromRepo(swiggyID) // instant paint from disk
	} else {
		m.rest = m.rest.WithLoading(true, false)
	}
	return datasource.LoadMenuPage(m.backend, m.snap, m.addr.ID, swiggyID, 1, m.menuGen, m.menuStaged)
}

// applyMenuFromRepo rebuilds the restaurant screen from the snapshot's menu
// for placeID, preserving the identity fields and cursor/filter state of the
// screen the user is on. The menu place carries only Items; its descriptive
// fields come back empty, so identity is re-stamped from the previous screen
// (without the Name, cartRestaurant stays "" and the live cart sync never
// fires).
func (m *Model) applyMenuFromRepo(placeID string) {
	p, ok := m.repo.Menu(placeID)
	if !ok {
		m.rest = m.rest.WithLoading(m.menuLoadingMore, m.menuPartial)
		return
	}
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
		WithCategory(cat).WithVegOnly(veg).WithCursor(ci).
		WithLoading(m.menuLoadingMore, m.menuPartial)
}

// saveMenuCache persists the completed menu for placeID (a few KB —
// synchronous keeps it race-free with tests/exit).
func (m Model) saveMenuCache(placeID string) {
	p, ok := m.repo.Menu(placeID)
	if !ok || len(p.Items) == 0 {
		return
	}
	cached := make([]localstore.CachedMenuItem, 0, len(p.Items))
	for _, it := range p.Items {
		cached = append(cached, localstore.CachedMenuItem{
			ID: it.SwiggyID, Name: it.Name, Price: it.Price,
			Veg: it.Veg, Desc: it.Desc, Rating: it.Rating,
			Customizable: it.Customizable, Category: it.Category,
			OutOfStock: it.OutOfStock,
		})
	}
	localstore.SaveCachedMenu(placeID, cached)
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

// imSearchInsert inserts s into imSearchQuery at the caret and advances the caret.
func (m *Model) imSearchInsert(s string) {
	r := []rune(m.imSearchQuery)
	c := m.imSearchCaret
	if c < 0 {
		c = 0
	}
	if c > len(r) {
		c = len(r)
	}
	m.imSearchQuery = string(r[:c]) + s + string(r[c:])
	m.imSearchCaret = c + len([]rune(s))
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
			// Show "loading…" only when there's nothing on screen — a disk
			// seed paints the cached list while the live refresh streams in.
			m.catPending = cmd != nil && m.placesListEmpty(q)
			m.catPendingQuery = q
			return cmd
		}
	}
	return nil
}

// railSettleFrames is how long (in 60ms ticks ≈ 0.3s) the rail cursor must rest
// on an entry before its load fires — collapses fast scrolling into one request.
const railSettleFrames = 5

// armRailLoad defers the active rail entry's load until the cursor settles, so
// arrowing through categories doesn't fire a search_restaurants per step.
func (m *Model) armRailLoad() {
	m.railSettlePending = true
	m.railSettleFrame = m.frame
}

// railFromChips rebuilds the rail descriptor (for IsCategory mapping) from the
// current cuisine chips.
func (m Model) railFromChips() screens.Rail {
	cats := make([]string, len(m.chips))
	for i, c := range m.chips {
		cats[i] = c.Label
	}
	return screens.NewRail(cats).WithActive(m.railActive)
}

// settledRailLoad fires the debounced rail load once the cursor has rested long
// enough (and is still on the focused rail), then re-renders so a "loading…" cue
// shows. Clears the pending flag either way.
func (m *Model) settledRailLoad() tea.Cmd {
	if !m.railSettlePending || m.frame-m.railSettleFrame < railSettleFrames {
		return nil
	}
	m.railSettlePending = false
	if !(m.screen == scrMenu && m.live && m.railFocus) {
		return nil // user opened a category / left the rail — don't load
	}
	cmd := m.loadForRail(m.railFromChips())
	m.menu = m.buildMenu()
	return cmd
}

// ensureIMQuery starts an IMProducts load for query if it hasn't been
// live-loaded yet this session (imLoadedQueries); a query already loaded this
// session renders from the snapshot with no re-fetch. On the first load of a
// query it seeds the last-known list from disk (SeedIMFromCache) for an instant
// paint while the live fetch streams over it — mirroring ensureQuery's
// once-per-query semantics AND the food places/menu disk cache.
func (m *Model) ensureIMQuery(query string) tea.Cmd {
	if m.imLoadedQueries[query] {
		m.imPending = false // already have live data in the snapshot this session
		return nil
	}
	m.imLoadedQueries[query] = true
	// Paint the last-known list from disk instantly; the live fetch streams over
	// it (stale-while-revalidate). Only show "loading…" when nothing is cached.
	m.imPending = !datasource.SeedIMFromCache(m.snap, m.addr.ID, query)
	return datasource.LoadIMProducts(m.backend, m.snap, m.addr.ID, query)
}

// loadForIMRail fires the (deduped) load for the currently-active IM rail
// entry — mirrors loadForRail exactly, with RailHome mapped to the go-to
// ("Usuals") list instead of the food Home sections.
func (m *Model) loadForIMRail(rail screens.Rail) tea.Cmd {
	// Leaving Search for Usuals or a category ends the search context — drop the
	// submitted query so its chip clears and `/` opens fresh.
	m.imSearchSubmitted = ""
	switch m.imRailActive {
	case screens.RailSearch:
		return nil
	case screens.RailHome:
		m.imQuery = ""
		return m.ensureIMQuery("") // owns m.imPending (cache-seed aware)
	default:
		if catIdx, isCat := rail.IsCategory(m.imRailActive); isCat && catIdx < len(m.imChips) {
			q := m.imChips[catIdx].Query
			m.imQuery = q
			return m.ensureIMQuery(q) // owns m.imPending (cache-seed aware)
		}
	}
	return nil
}

// armIMRailLoad defers the active IM rail entry's load until the cursor
// settles — mirrors armRailLoad.
func (m *Model) armIMRailLoad() {
	m.imRailSettlePending = true
	m.imRailSettleFrame = m.frame
}

// settledIMRailLoad fires the debounced IM rail load once the cursor has
// rested long enough (and is still on the focused rail) — mirrors
// settledRailLoad exactly.
func (m *Model) settledIMRailLoad() tea.Cmd {
	if !m.imRailSettlePending || m.frame-m.imRailSettleFrame < railSettleFrames {
		return nil
	}
	m.imRailSettlePending = false
	if !(m.screen == scrInstamart && m.imRailFocus) {
		return nil // user opened a category / left the rail — don't load
	}
	cmd := m.loadForIMRail(m.imRail())
	m.inst = m.buildInstamart()
	return cmd
}

// syncIMSearchEntry mirrors syncSearchEntry: landing the IM rail cursor on
// Search opens a fresh input; leaving it closes the input.
func (m *Model) syncIMSearchEntry() {
	if m.imRailActive == screens.RailSearch {
		m.imSearchMode = true
		m.imSearchQuery = ""
		m.imSearchCaret = 0
	} else {
		m.imSearchMode = false
	}
}

// cartSettleFrames is how long (in 60ms ticks ≈ 360ms) the cart waits after the
// last qty edit before firing a single live sync — long enough to collapse a
// burst of +/− mashes into one update_food_cart.
const cartSettleFrames = 6

// armCartSync defers the cart's live sync until qty edits settle, so mashing
// +/− collapses to a single update_food_cart instead of one write per keystroke
// (which would spray calls past Swiggy's 30/min write-tool cap).
func (m *Model) armCartSync() {
	m.cartSyncPending = true
	m.cartSyncFrame = m.frame
}

// settledCartSync fires the debounced cart sync once the qty edits have rested
// long enough, then clears the pending flag. It skips firing when a freeze-path
// reduce is already in flight (cartMutating) — that path serializes and
// reconciles on its own. liveCartCmd is a no-op in mock mode.
func (m *Model) settledCartSync() tea.Cmd {
	if !m.cartSyncPending || m.frame-m.cartSyncFrame < cartSettleFrames {
		return nil
	}
	m.cartSyncPending = false
	if m.cartMutating {
		return nil
	}
	return m.liveCartCmd()
}

// armIMCartSync defers the Instamart cart's live sync until qty edits settle —
// same collapse-to-one-write rationale as armCartSync, kept on its own
// pending/frame pair since the two carts never share a debounce window.
func (m *Model) armIMCartSync() {
	m.imCartSyncPending = true
	m.imCartSyncFrame = m.frame
}

// settledIMCartSync fires the debounced Instamart cart sync once qty edits
// have rested long enough, then clears the pending flag. Mirrors
// settledCartSync; imCartMutating freezes input on the checkout reduce/delete
// path, which serializes and reconciles on its own.
func (m *Model) settledIMCartSync() tea.Cmd {
	if !m.imCartSyncPending || m.frame-m.imCartSyncFrame < cartSettleFrames {
		return nil
	}
	m.imCartSyncPending = false
	if m.imCartMutating {
		return nil
	}
	return m.imLiveCartCmd()
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
		// Only show the loading label when there's nothing to paint — a disk
		// seed already fills the list while the live refresh streams in.
		m.homePending = m.placesListEmpty(m.homeNearbyQuery())
		cmds = append(cmds, c)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// loadHomeForCurrentAddr fires the Home loads for the currently-adopted
// address, PRIORITIZED: the visible "popular near you" list gets the first
// rate-limiter slots; usuals, the launch cart pull, and the active-order
// check — none of which the user sees on first paint — are deferred until the
// list's first page lands (flushed by the PlacesPageLoadedMsg handler). This
// turns "blank list while five serialized calls queue" into "list in one
// call's latency". Callers must have already set m.addr.
func (m *Model) loadHomeForCurrentAddr() tea.Cmd {
	m.usualsLoaded = true // fired below (deferred or batched), never via ensureHomeLoaded
	rest := []tea.Cmd{
		datasource.LoadUsuals(m.backend, m.snap, m.addr.ID),
		datasource.PullCart(m.backend, m.addr.ID),
		m.activeOrderCheckCmd(),
	}
	first := m.ensureQuery(m.homeNearbyQuery())
	if first == nil {
		// List already live-cached — nothing to prioritize over.
		return tea.Batch(rest...)
	}
	m.homePending = m.placesListEmpty(m.homeNearbyQuery())
	m.deferredLaunch = rest
	return first
}

// maybeOpenAddrGate opens the forced address gate when all conditions hold:
//   - addrGatePending (gate not yet satisfied this session)
//   - we are on scrMenu (past the splash)
//   - addresses have been loaded (AddressesLoadedMsg received)
//   - no entry overlay is open (helpOpen or whatsnewOpen would obscure the gate)
//
// By address count:
//   - 0  → clears the pending flag and returns nil (leave empty-state as-is).
//   - 1  → auto-selects the single address and loads Home; no picker.
//   - 2+ → builds the address-picker modal in forced mode; waits for Enter.
func (m *Model) maybeOpenAddrGate() tea.Cmd {
	if !m.addrGatePending || m.screen != scrMenu || !m.addressesLoaded || m.helpOpen || m.whatsnewOpen {
		return nil
	}
	addrs := m.repo.Addresses()
	switch len(addrs) {
	case 0:
		m.addrGatePending = false
		return nil
	case 1:
		m.addr = addrs[0]
		m.addrGatePending = false
		return m.loadHomeForCurrentAddr()
	default:
		m.addrScreen = screens.NewAddress(addrs, "").WithForced(true)
		m.addrOpen = true
		m.addrForced = true
		return nil
	}
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
	if len(m.lines) == 0 {
		return false
	}
	// Seeded foreign cart with no identifiable restaurant: we can't verify the
	// add belongs to the same place, so always prompt to replace. (Without this,
	// an empty cartRestaurant fell through to "no conflict" and the local cart
	// silently mixed two restaurants while nothing reached Swiggy.)
	if m.cartForeign && m.cartRestaurant == "" {
		return true
	}
	if m.cartRestaurant == "" {
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
	m.cartForeign = false // a real in-app cart now owns the lines
	return m
}

// seedCartFromLive populates the local cart from a Swiggy cart pulled at launch
// (one built earlier on the Swiggy app/website). The lines are display+identity
// stubs (real menu_item_id + name + price); cartRestaurant is set to the cart's
// owning restaurant so a later add from a different restaurant raises the
// keep/override conflict modal. Pricing shown comes from m.liveCart.
func (m Model) seedCartFromLive(c api.Cart) Model {
	var lines []screens.CartLine
	for _, l := range c.Lines {
		it := catalog.Item{ID: l.ItemID, SwiggyID: l.ItemID, Name: l.Name, Price: l.Price, Section: catalog.SectionFood}
		lines = append(lines, screens.CartLine{Item: it, Qty: l.Quantity, Price: l.Price})
	}
	m.lines = lines
	m.cartRestaurant = c.Restaurant
	m.cartSection = catalog.SectionFood
	// Mark as foreign only when we couldn't name the restaurant; a named cart
	// conflicts normally on a different-name add.
	m.cartForeign = c.Restaurant == ""
	m.menu = m.menu.WithCartChip(m.cartChip())
	// The pulled cart IS what Swiggy holds → it is the confirmed baseline.
	return m.commitCartConfirmed()
}

// applyCartAvailability records which cart items Swiggy reports as out of stock
// (from the authoritative cart response), so the checkout can flag those lines
// and block the order. An empty/all-available cart clears the set.
func (m Model) applyCartAvailability(c api.Cart) Model {
	var bad map[string]bool
	for _, l := range c.Lines {
		if !l.Available {
			if bad == nil {
				bad = map[string]bool{}
			}
			bad[l.ItemID] = true
		}
	}
	m.unavailableItems = bad
	return m
}

// hasUnavailableLine reports whether any cart line is an item Swiggy flagged out
// of stock — checkout is blocked while true.
func (m Model) hasUnavailableLine() bool {
	for _, l := range m.lines {
		if m.unavailableItems[l.Item.SwiggyID] {
			return true
		}
	}
	return false
}

// hasUnavailableIMLine reports whether any Instamart cart line is flagged out
// of stock (Unavailable is stamped directly on the line by the IMCartSyncedMsg
// handler — the IM cart has no separate id-set like unavailableItems).
func (m Model) hasUnavailableIMLine() bool {
	for _, l := range m.imLines {
		if l.Unavailable {
			return true
		}
	}
	return false
}

// cloneCartLines returns a copy with its own backing array so a confirmed
// snapshot is not mutated when later cart edits change qty in place
// (appendOrInc/decLastByItem mutate lines[i].Qty). The per-line AddOns/Selections
// slices are never mutated in place, so sharing their headers is safe.
func cloneCartLines(in []screens.CartLine) []screens.CartLine {
	if len(in) == 0 {
		return nil
	}
	return append([]screens.CartLine(nil), in...)
}

// commitCartConfirmed records the current cart as the last Swiggy-confirmed
// state — the rollback target for a future failed sync. Called after every
// successful pull/load/sync.
func (m Model) commitCartConfirmed() Model {
	m.confirmedLines = cloneCartLines(m.lines)
	m.confirmedRestaurant = m.cartRestaurant
	m.confirmedSection = m.cartSection
	m.confirmedForeign = m.cartForeign
	m.cartConfirmed = true
	return m
}

// rollbackCart restores the cart to the last confirmed state after a failed
// sync, then rebuilds the affected views so the screen reflects the revert. The
// local cart never shows an item Swiggy rejected.
func (m Model) rollbackCart() Model {
	if !m.cartConfirmed {
		// No baseline yet (first mutation from empty): the safe revert is an empty
		// cart, since nothing was ever confirmed.
		m.lines = nil
		m.cartRestaurant = ""
		m.cartSection = ""
		m.cartForeign = false
	} else {
		m.lines = cloneCartLines(m.confirmedLines)
		m.cartRestaurant = m.confirmedRestaurant
		m.cartSection = m.confirmedSection
		m.cartForeign = m.confirmedForeign
	}
	m.menu = m.menu.WithCartChip(m.cartChip())
	if m.screen == scrRestaurant {
		m = m.refreshAfterAdd()
	}
	if m.screen == scrCheckout && m.checkoutVertical == 0 {
		m.checkout = m.buildCheckout()
	}
	return m
}

// commitIMCartConfirmed records the current Instamart cart as the last
// Swiggy-confirmed state — the rollback target for a future failed sync.
// Mirrors commitCartConfirmed.
func (m Model) commitIMCartConfirmed() Model {
	m.imConfirmedLines = cloneCartLines(m.imLines)
	m.imCartConfirmed = true
	return m
}

// rollbackIMCart restores the Instamart cart to the last confirmed state
// after a failed sync, then rebuilds the affected views. Mirrors rollbackCart.
func (m Model) rollbackIMCart() Model {
	if !m.imCartConfirmed {
		m.imLines = nil
	} else {
		m.imLines = cloneCartLines(m.imConfirmedLines)
	}
	if m.screen == scrInstamart {
		m = m.refreshInstamart()
	}
	if m.screen == scrCheckout && m.checkoutVertical == 1 {
		m.checkout = m.buildIMCheckout()
	}
	return m
}

// clearPlacedCarts empties both verticals' carts and their live/sync state
// after an order is placed and the confirm/tracking flow moves on — an
// order-placed event always empties WHICHEVER vertical's cart it came from,
// but both are reset defensively so a stray leftover from the other vertical
// (e.g. a foreign-cart seed that never got placed) can't survive into the
// next browse session looking like an active draft.
func (m Model) clearPlacedCarts() Model {
	m.lines = nil
	m.cartRestaurant = ""
	m.cartSection = ""
	m.imLines = nil
	m.imLiveCart = api.IMCart{}
	m.imCartPulled = false
	m.imCartSyncErr = ""
	m.imOrderErr = ""
	m.imCartConfirmed = false
	m.checkoutVertical = 0
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
		m.cartForeign = false // a real in-app add now owns the cart
	}
	return m
}

// imCommitAdd adds an Instamart product (with its chosen pack-size variant, if
// any) to the Instamart cart. Instamart carts bind to the address, not a
// restaurant — there is no cart-owner concept, so unlike commitAdd this never
// raises the conflict modal. The chosen variant's Selection.ChoiceID IS the
// spinId Swiggy expects on the wire; it REPLACES the item's default
// SwiggyID/Price so the line syncs the exact pack size picked, and the
// Selections are kept on the line so checkout can show the pack size.
func (m Model) imCommitAdd(item catalog.Item, sels []catalog.Selection, price int) Model {
	for _, s := range sels {
		if s.Variant {
			item.SwiggyID = s.ChoiceID
			if s.Name != "" {
				item.Name = item.Name + " (" + s.Name + ")"
			}
		}
	}
	m.imLines = appendOrInc(m.imLines, item, nil, sels, price)
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
		m.cartForeign = false // a real in-app add now owns the cart
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
	if it.OutOfStock {
		// Swiggy would reject the add — say so instead of failing silently.
		m.cartSyncErr = "“" + it.Name + "” is sold out"
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
	m.armCartSync() // debounced: mashing + collapses to one update_food_cart
	return m, nil
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
	m.armCartSync() // debounced: mashing − collapses to one update_food_cart
	return m, nil
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

// imRail builds the Instamart rail descriptor from imChips — mirrors
// railFromChips, with "Usuals" in the Home slot (the your-go-to-items list).
func (m Model) imRail() screens.Rail {
	cats := make([]string, len(m.imChips))
	for i, c := range m.imChips {
		cats[i] = c.Label
	}
	return screens.NewRailLabeled("Usuals", cats).WithActive(m.imRailActive)
}

// buildInstamart constructs the full two-pane Instamart screen (rail + main
// product list) for the current imQuery/search state — mirrors buildMenu.
// Live-only; Instamart has no mock single-pane path (it's always live once
// entered, per the live-vertical rollout).
func (m Model) buildInstamart() screens.Instamart {
	rail := m.imRail().WithFocus(m.imRailFocus)
	items := m.imBrowseItems()
	inst := screens.NewInstamart(items, m.imQtyMap(), m.imCartChip()).
		WithRail(rail).WithRailFocus(m.imRailFocus).
		WithLoading(m.imPending)
	if m.imSearchMode {
		inst = inst.WithSearch(m.imSearchQuery, m.imSearchCaret, true)
	} else if m.imSearchSubmitted != "" && m.imQuery == m.imSearchSubmitted {
		// Editor closed but the list is a search's results — show the persistent
		// "⌕ query · / edit" chip so the search doesn't silently vanish.
		inst = inst.WithSubmittedSearch(m.imSearchSubmitted)
	}
	return inst
}

// refreshInstamart rebuilds the Instamart screen from fresh snapshot data
// while keeping the user's place in the list — mirrors refreshMenu (a page
// landing must not yank a mid-scroll user back to the top). Genuine view
// switches (rail move, search submit, esc back to the go-to list) rebuild via
// buildInstamart directly, where the reset-to-top is correct.
func (m Model) refreshInstamart() Model {
	cur := m.inst.CursorIndex()
	m.inst = m.buildInstamart().WithListCursor(cur)
	return m
}

// imBrowseItems returns the product list for the CURRENT browse view. Live
// reads are query-scoped through the snapshot (a raced slow search write can
// never surface under the go-to view); mock falls back to the repository.
func (m Model) imBrowseItems() []catalog.Item {
	if m.live && m.snap != nil {
		return m.snap.InstamartFor(m.addr.ID, m.imQuery)
	}
	return m.repo.InstamartItems(m.addr)
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
	// The browser is never auto-opened: the user connects by pressing Enter on the
	// "connect swiggy" gate (on first run, only after the welcome walkthrough).
	// In live mode with a valid address, check active orders to refresh liveness.
	if m.live && m.hasActiveOrder && m.backend != nil && m.addr.ID != "" {
		cmds = append(cmds, datasource.LoadActiveOrdersCmd(m.backend, m.addr.ID))
	}
	// Fire the release-notes fetch in the background (result handled in Update).
	// Do not open the modal yet — it opens at the splash→scrMenu transition.
	if m.wantNotes {
		cmds = append(cmds, datasource.FetchReleaseNotesCmd(m.notesChannel, m.notesVersion, m.notesCode))
	}
	return tea.Batch(cmds...)
}

// onTick advances time-based screen state; extended by later tasks.
func (m Model) onTick() (Model, tea.Cmd) {
	m.nowUnix = time.Now().Unix()
	if m.screen == scrSplash {
		// Resolve the decode, then keep ticking so the idle shimmer animates.
		if m.decodeStep < render.DecodeSteps {
			m.decodeStep++
		}
		m.splashTick++
	}
	if m.screen == scrWelcome && m.welcome.Phase() == 0 {
		// Advance the food animation; auto-hand off to the intro card once it ends.
		t := m.welcomeTick + 1
		m.welcomeTick = t
		m.welcome = m.welcome.WithTick(t)
		if t >= screens.WelcomeAnimEnd {
			m.welcome = m.welcome.WithPhase(1)
		}
	}
	if m.screen == scrConfirm {
		m.confirmTick++
		if m.confirmTick >= 42 && m.hasActiveOrder {
			m.screen = scrTracking
			m.trackTick = 0
			if m.backend != nil {
				return m, m.trackingPollCmd()
			}
		}
	}
	if m.screen == scrTracking {
		m.trackTick++
		// Re-poll every ~500 ticks (~30s at 60ms) so the ETA stays synced with
		// Swiggy. Keyed on the order id (not hasActiveOrder) so polling keeps
		// refreshing while the screen is open, even if the delivery heuristic has
		// cleared the active-order flag.
		if m.trackTick%500 == 0 && m.activeOrder.OrderID != "" && m.backend != nil {
			return m, m.trackingPollCmd()
		}
	}
	// Fire the debounced rail-category load once the cursor has settled.
	if cmd := m.settledRailLoad(); cmd != nil {
		return m, cmd
	}
	// Fire the debounced Instamart rail-category load once the cursor has settled.
	if cmd := m.settledIMRailLoad(); cmd != nil {
		return m, cmd
	}
	// Fire the debounced cart sync once qty edits have settled.
	if cmd := m.settledCartSync(); cmd != nil {
		return m, cmd
	}
	// Fire the debounced Instamart cart sync once qty edits have settled.
	if cmd := m.settledIMCartSync(); cmd != nil {
		return m, cmd
	}
	return m, nil
}

// trackingPollCmd fires the right tracking poll for the active order's
// vertical. Instamart's track_order needs lat/lng (harvested from
// IMOrders/get_orders — get_addresses omits coordinates); until we have them,
// fetch the active-orders list instead, which also refreshes status/ETA.
func (m Model) trackingPollCmd() tea.Cmd {
	if m.activeOrder.Vertical == "instamart" {
		if m.activeOrder.Lat != 0 || m.activeOrder.Lng != 0 {
			return datasource.PollIMTrackingCmd(m.backend, m.activeOrder.OrderID, m.activeOrder.Lat, m.activeOrder.Lng)
		}
		return datasource.LoadIMActiveOrdersCmd(m.backend)
	}
	return datasource.PollTrackingCmd(m.backend, m.activeOrder.OrderID)
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

// activeOrderCheckCmd re-checks the account for a currently-live order so the
// Start Screen's delivery-status (track order) button reflects reality. Fired
// on every Start Screen entry (launch + double-Esc home). Returns nil when the
// call can't be made yet (mock mode, no backend, or no resolved address); the
// ActiveOrdersLoadedMsg handler both DISCOVERS a new live order and refreshes a
// known one.
func (m Model) activeOrderCheckCmd() tea.Cmd {
	if !m.live || m.backend == nil || m.addr.ID == "" {
		return nil
	}
	return tea.Batch(
		datasource.LoadActiveOrdersCmd(m.backend, m.addr.ID),
		datasource.LoadIMActiveOrdersCmd(m.backend),
	)
}

// splashOrderLabel builds the splash track-order button label: a real live ETA
// from track_food_order when we have one ("Starbucks · 11 mins"); else a short
// friendly status ("Starbucks · outside now" once the rider arrives — Swiggy
// reports ETA "N/A" then); else the initial placement estimate.
func splashOrderLabel(restaurant, status, liveETA string, etaHi int) string {
	if e := strings.TrimSpace(liveETA); e != "" && !strings.EqualFold(e, "N/A") {
		return fmt.Sprintf("%s · %s", restaurant, e)
	}
	if short := screens.ShortStatus(status); short != "" {
		return fmt.Sprintf("%s · %s", restaurant, short)
	}
	return fmt.Sprintf("%s · ~%d min", restaurant, etaHi)
}

// splashOrder sets the splash track-order button label, appending a "+1 more"
// marker while a second delivery is live (the entry opens a picker then).
func (m Model) splashOrder(label string) Model {
	if m.hasAltOrder {
		label += " · +1 more"
	}
	m.splash = m.splash.WithOrder(label)
	return m
}

// refreshSplashOrderLabel re-renders the splash button from the primary
// order's stored estimate (the tracking polls overwrite it with live data).
func (m Model) refreshSplashOrderLabel() Model {
	if !m.hasActiveOrder {
		m.splash = m.splash.WithOrder("")
		return m
	}
	return m.splashOrder(splashOrderLabel(m.activeOrder.Restaurant, "", "", m.activeOrder.ETAHiMin))
}

// promoteAltOrder makes the alt order the primary after the primary's
// delivery cleared the slot, so the second live delivery keeps its splash
// button instead of vanishing with the first one. Returns the poll Cmd for
// the promoted order (nil when there is nothing to promote).
func (m Model) promoteAltOrder() (Model, tea.Cmd) {
	if !m.hasAltOrder {
		return m, nil
	}
	m.activeOrder = m.altOrder
	m.altOrder = localstore.ActiveOrder{}
	m.hasAltOrder = false
	m.hasActiveOrder = true
	_ = localstore.SaveActiveOrder(m.activeOrder)
	m = m.refreshSplashOrderLabel()
	var cmd tea.Cmd
	if m.backend != nil {
		cmd = m.trackingPollCmd()
	}
	return m, cmd
}

// swapTrackedOrder exchanges the primary and alt orders (both stay live) and
// rebuilds the tracking page for the new primary. Persistence follows the
// primary — active-order.json always holds the order the user is watching.
func (m Model) swapTrackedOrder() (Model, tea.Cmd) {
	if !m.hasAltOrder {
		return m, nil
	}
	m.activeOrder, m.altOrder = m.altOrder, m.activeOrder
	_ = localstore.SaveActiveOrder(m.activeOrder)
	m.track = screens.NewTracking(
		m.activeOrder.Restaurant, m.activeOrder.AddrLine, m.activeOrder.OrderID,
		m.activeOrder.PlacedAt, m.activeOrder.ETALoMin, m.activeOrder.ETAHiMin,
	).WithAlt(true)
	m.trackTick = 0
	m.screen = scrTracking
	m = m.refreshSplashOrderLabel()
	var cmd tea.Cmd
	if m.backend != nil {
		cmd = m.trackingPollCmd()
	}
	return m, cmd
}

// trackPickLabel is one row of the two-orders picker: place + rough ETA.
func trackPickLabel(o localstore.ActiveOrder) string {
	return splashOrderLabel(o.Restaurant, "", "", o.ETAHiMin)
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
// In live mode, once Swiggy's real cart is known the chip shows the true line
// count + grand total (item subtotal + delivery + taxes) so every page agrees
// with what Place Order will charge. Before any live cart exists it falls back
// to the local optimistic lines (instant feedback on the first add).
func (m Model) cartChip() string {
	if m.live && len(m.liveCart.Lines) > 0 {
		n := 0
		for _, l := range m.liveCart.Lines {
			n += l.Quantity
		}
		return cartChipStr(n, m.liveCart.Total)
	}
	return cartChipStr(m.cartCount(), m.cartTotal())
}
func (m Model) imCartChip() string {
	// Prefer Swiggy's confirmed cart (accurate count + fee-inclusive total) —
	// but ONLY when it matches the current local lines. Right after an add or
	// delete the server cart lags the user's intent for the length of the
	// debounce + round-trip; showing it THEN would flash the old contents (e.g.
	// a pre-existing/seeded item) and hide what was just added — the "new items
	// didn't show, bill was the old item" bug. While a change is still settling,
	// fall back to the optimistic local lines so the indicator moves instantly,
	// then converges to the server total when the sync lands.
	if m.live && len(m.imLiveCart.Lines) > 0 && m.imLiveMatchesLines() {
		n := 0
		for _, l := range m.imLiveCart.Lines {
			n += l.Quantity
		}
		return cartChipStr(n, m.imLiveCart.Total)
	}
	return cartChipStr(m.imCartCount(), m.imCartTotal())
}

// imLiveMatchesLines reports whether Swiggy's last-confirmed cart (imLiveCart)
// holds exactly the syncable local lines (same spinIds, same quantities). When
// it does, the server cart is in sync and its fee-inclusive total is safe to
// show; when it doesn't, a mutation is still in flight (or was dropped) and the
// server view is stale. Mirrors imItemsForLines' spinId selection so a line
// without a resolved SwiggyID (never syncable) can't cause a false mismatch.
func (m Model) imLiveMatchesLines() bool {
	local := map[string]int{}
	for _, l := range m.imLines {
		if l.Item.SwiggyID == "" {
			continue
		}
		local[l.Item.SwiggyID] += l.Qty
	}
	server := map[string]int{}
	for _, l := range m.imLiveCart.Lines {
		server[l.SpinID] += l.Quantity
	}
	if len(local) != len(server) {
		return false
	}
	for spin, qty := range local {
		if server[spin] != qty {
			return false
		}
	}
	return true
}

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
	if m.cartForeign {
		return "your existing Swiggy cart"
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
		var tickCmd tea.Cmd
		m, tickCmd = m.onTick()
		// Native auth gate: poll the loopback callback. When the browser
		// authorize completes, clear the gate and fire the live loads.
		if m.needsAuth && m.authClient != nil && m.authFlowID != "" && m.authClient.Authorized(m.authFlowID) {
			m.needsAuth = false
			return m, tea.Batch(tick(), m.liveInitCmds())
		}
		return m, tea.Batch(tick(), tickCmd)
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
		m.addressesLoaded = true
		m.menu = m.buildMenu()
		if m.addrGatePending {
			// Gate is still pending: do NOT auto-pick addrs[0] or load Home.
			// maybeOpenAddrGate will open the picker (or auto-use the single address)
			// once we are on scrMenu with no overlay open.
			return m, m.maybeOpenAddrGate()
		}
		// Gate already satisfied (defensive / non-live paths): keep old behavior.
		if addrs := m.repo.Addresses(); len(addrs) > 0 {
			m.addr = addrs[0]
		}
		if m.live {
			// Address just adopted → load Home for it: the visible list first,
			// usuals + launch cart pull + active-order check deferred behind
			// its first page (see loadHomeForCurrentAddr).
			return m, m.loadHomeForCurrentAddr()
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
		m.refreshMenu() // data refresh — keep the user's scroll position
		return m, nil
	case datasource.PlacesPageLoadedMsg:
		if errIsNeedsAuth(dm.Err) {
			m.needsAuth = true
			return m, nil
		}
		// Whatever happened, release any deferred launch loads (usuals, cart
		// pull, active-order check): the visible list either painted or
		// failed, and holding the rest hostage helps nobody.
		var cmds []tea.Cmd
		if len(m.deferredLaunch) > 0 {
			cmds = append(cmds, m.deferredLaunch...)
			m.deferredLaunch = nil
		}
		if dm.Gen != m.placesGen {
			return m, tea.Batch(cmds...) // dead stream — drop the page, chain dies
		}
		if m.catPending && dm.Query == m.catPendingQuery {
			m.catPending = false
		}
		if m.homePending && dm.Query == m.homeNearbyQuery() {
			m.homePending = false
		}
		if dm.Err == nil && dm.Page == 1 {
			delete(m.seededQueries, dm.Query) // live data replaced the disk seed
		}
		m.refreshMenu() // data refresh — keep the user's scroll position
		if dm.Err != nil {
			return m, tea.Batch(cmds...) // keep whatever pages already painted
		}
		// Continue the chain under the same cap as the old barrier loop
		// (~12 restaurants / 2 pages) so streaming never increases call volume.
		count := 0
		if r := m.liveRepo(); r != nil {
			count = len(r.PlacesByQuery(m.addr, dm.Query))
		}
		if !dm.Done && dm.Page < 2 && count < 12 {
			cmds = append(cmds, datasource.LoadPlacesPage(m.backend, m.snap, m.addr.ID, dm.Query, dm.NextOffset, dm.Page+1, m.placesGen))
		} else {
			m.savePlacesCache(dm.Query) // list settled — persist for instant next launch
		}
		return m, tea.Batch(cmds...)
	case datasource.MenuPageLoadedMsg:
		if errIsNeedsAuth(dm.Err) {
			m.needsAuth = true
			return m, nil
		}
		if dm.Gen != m.menuGen {
			return m, nil // dead stream (restarted load) — drop, chain dies
		}
		if m.screen != scrRestaurant {
			// User left the restaurant — stop the stream. Discard any staged
			// refresh so a later visit starts clean.
			m.snap.DropStagedMenu(dm.PlaceID)
			return m, nil
		}
		if cur := m.rest.PlaceData().SwiggyID; cur == "" || dm.PlaceID != cur {
			m.snap.DropStagedMenu(dm.PlaceID)
			return m, nil // different restaurant open — same stale-guard as MenuLoadedMsg
		}
		if dm.Err != nil {
			// Partial menu beats none: keep the pages that landed, flag the gap.
			// With a cache seed the full (stale) menu is on screen — also flag it.
			m.menuLoadingMore = false
			m.menuPartial = dm.Page > 1 || m.menuStaged
			m.snap.DropStagedMenu(dm.PlaceID)
			m.menuStaged = false
			m.applyMenuFromRepo(dm.PlaceID)
			return m, nil
		}
		if dm.Done {
			m.menuLoadingMore = false
			if m.menuStaged {
				m.snap.PromoteStagedMenu(dm.PlaceID) // atomic swap: seed → fresh menu
				m.menuStaged = false
			}
			m.applyMenuFromRepo(dm.PlaceID)
			m.saveMenuCache(dm.PlaceID)
			return m, nil
		}
		if !m.menuStaged {
			m.applyMenuFromRepo(dm.PlaceID) // progressive paint, page by page
		}
		return m, datasource.LoadMenuPage(m.backend, m.snap, m.addr.ID, dm.PlaceID, dm.Page+1, m.menuGen, m.menuStaged)
	case datasource.MenuLoadedMsg:
		if errIsNeedsAuth(dm.Err) {
			m.needsAuth = true
			return m, nil
		}
		if m.screen == scrRestaurant {
			// Drop stale loads: a menu that finished AFTER the user opened a
			// different restaurant must not overwrite the current one. Without
			// this guard, a slow load for A lands while B is open and merges A's
			// items onto B's identity — the cross-restaurant cart Swiggy rejects.
			if cur := m.rest.PlaceData().SwiggyID; cur != "" && dm.PlaceID != cur {
				return m, nil
			}
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
		// Drop a stale options response: if the user tapped a different
		// customizable item before this landed, m.pendingItem has moved on and
		// applying these groups would open the picker for the wrong item. (The
		// start-fresh re-fetch re-fires with the same pendingItem, so it passes.)
		if dm.ItemID != "" && dm.ItemID != m.pendingItem.SwiggyID {
			return m, nil
		}
		if dm.Err != nil {
			m.cartSyncErr = "options: " + dm.Err.Error()
			return m, nil
		}
		it := m.pendingItem
		it.Options = dm.Groups
		if len(dm.Groups) == 0 {
			// Item flagged customizable but has no real options — add directly
			// (commitAdd raises the conflict modal itself when needed).
			m = m.commitAdd(it, nil, nil, 0, m.pendingRest, m.pendingSection)
			if !m.conflictOpen {
				m = m.refreshAfterAdd()
				return m, m.liveCartCmd()
			}
			return m, nil
		}
		// A customizable item is about to show a picker (wizard or customize sheet)
		// that mutates the live cart. Resolve a cart-restaurant conflict BEFORE the
		// picker — otherwise the user picks a variant, THEN gets asked to replace
		// the cart, and "start fresh" re-fetches and re-opens the picker (a
		// confusing double-customize). On "start fresh" the cart is cleared and the
		// options re-fetched, so this check passes and the picker opens once.
		if m.conflictsWithCart(m.pendingRest, m.pendingSection) {
			m.conflict = screens.NewCartConflict(m.cartHeader(), m.pendingRest, it.Name)
			m.conflictSel = 1
			m.conflictOpen = true
			m.pendingItem = it // re-fetch path on "start fresh" (handled in conflict resolve)
			return m, nil
		}
		if wizardEligible(dm.Groups) && m.liveRestReady() {
			// Variant-dependent add-ons: drive the trial-discovery wizard.
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
				// The wizard synced this item to the live cart page by page and the
				// final confirm succeeded → record the new baseline.
				m = m.commitCartConfirmed()
				return m, nil
			}
		}
		if dm.Err != nil {
			// The optimistic change did NOT reach Swiggy. Roll the local cart back
			// to the last confirmed state and tell the user — never leave a phantom
			// item that isn't really in their Swiggy cart.
			m.cartMutating = false
			m = m.rollbackCart()
			m.cartSyncErr = "⚠ cart change didn't go through — reverted. try again"
			return m, nil
		}
		m.cartSyncErr = ""
		m.liveCart = dm.Cart // real Swiggy pricing for an accurate bill
		m = m.applyCartAvailability(dm.Cart)
		m = m.commitCartConfirmed()
		m.cartMutating = false // confirmed — unfreeze
		// Vertical guard: a food sync landing while the INSTAMART checkout is
		// on screen must not clobber it with the food cart's view.
		if m.screen == scrCheckout && m.checkoutVertical == 0 {
			m.checkout = m.buildCheckout()
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
		m = m.applyCartAvailability(dm.Cart)
		// A successful load confirms the current lines as the Swiggy baseline.
		m = m.commitCartConfirmed()
		if m.screen == scrCheckout && m.checkoutVertical == 0 {
			m.checkout = m.buildCheckout()
		}
		return m, nil
	case datasource.CartPulledMsg:
		// Launch-time account cart. Swallow errors quietly (don't nag at startup),
		// and only seed when the local cart is empty so we never clobber a cart the
		// user is already building in this session.
		if dm.Err != nil {
			dbgTUI("cart pull: %v", dm.Err)
			return m, nil
		}
		m.liveCart = dm.Cart
		m.cartLoaded = true
		m = m.applyCartAvailability(dm.Cart)
		if len(m.lines) == 0 && m.cartRestaurant == "" && len(dm.Cart.Lines) > 0 {
			m = m.seedCartFromLive(dm.Cart)
		}
		return m, nil
	case datasource.IMProductsLoadedMsg:
		if errIsNeedsAuth(dm.Err) {
			m.needsAuth = true
			return m, nil
		}
		if dm.Query != m.imQuery {
			return m, nil // stale — a newer search/go-to load has since been submitted
		}
		m.imPending = false
		if dm.Err != nil {
			// Non-fatal: the go-to list legitimately errors on no order history
			// ("Failed to fetch Your Go To Items…") — treat any load error the
			// same way rather than nagging the status bar on a routine empty case.
			m.inst = m.inst.WithLoading(false)
			return m, nil
		}
		m = m.refreshInstamart()
		m.inst = m.inst.WithLoading(false)
		return m, nil
	case datasource.IMCartPulledMsg:
		// Launch-time account cart. Swallow errors quietly (don't nag at startup),
		// and only seed when the local IM cart is empty so we never clobber a cart
		// the user is already building in this session.
		if dm.Err != nil {
			dbgTUI("im cart pull: %v", dm.Err)
			return m, nil
		}
		m.imLiveCart = dm.Cart
		if len(m.imLines) == 0 && len(dm.Cart.Lines) > 0 {
			var lines []screens.CartLine
			for _, l := range dm.Cart.Lines {
				// Seeded ids are prefixed "im-<spin>" so they never collide with a
				// browse product's own catalog id — the steppers on the browse list
				// won't reflect a seeded line until the item is re-added there, which
				// is fine: the NEXT sync replaces the server cart wholesale with
				// whatever the local lines are, seeded or not.
				it := catalog.Item{ID: "im-" + l.SpinID, SwiggyID: l.SpinID, Name: l.Name, Price: l.Price, Section: catalog.SectionInstamart}
				lines = append(lines, screens.CartLine{Item: it, Qty: l.Quantity, Price: l.Price, Unavailable: !l.Available})
			}
			m.imLines = lines
			m = m.commitIMCartConfirmed()
		}
		return m, nil
	case datasource.IMCartSyncedMsg:
		if dm.Err != nil {
			// The optimistic change did NOT reach Swiggy. Map the common failures to
			// a friendly message; roll back to the last confirmed state so the local
			// cart never shows an item Swiggy rejected.
			m.imCartMutating = false
			m.imConfirmPending = false // a failed sync must not open the confirm modal
			switch {
			case strings.Contains(dm.Err.Error(), "store is currently unavailable or closed"):
				m = m.rollbackIMCart()
				m.imCartSyncErr = "store closed right now — try again later"
			case strings.Contains(dm.Err.Error(), "Cart not found") || strings.Contains(dm.Err.Error(), "CART_EXPIRED"):
				if !m.imCartRebuilt {
					// One-shot auto-rebuild: resend the current lines against a fresh
					// cart before surfacing anything to the user. Keep the input freeze
					// held for the retry — the sync that just failed may have been a
					// frozen reduce/delete, and unfreezing mid-rebuild would let edits
					// race the in-flight replacement.
					m.imCartRebuilt = true
					m.imCartMutating = m.live
					return m, m.imLiveCartCmd()
				}
				m = m.rollbackIMCart()
				m.imCartSyncErr = dm.Err.Error()
			default:
				m = m.rollbackIMCart()
				m.imCartSyncErr = "⚠ cart change didn't go through — reverted. try again"
			}
			if m.screen == scrCheckout && m.checkoutVertical == 1 {
				m.checkout = m.buildIMCheckout()
			}
			return m, nil
		}
		m.imCartSyncErr = ""
		m.imCartRebuilt = false
		m.imLiveCart = dm.Cart
		// Mark line availability straight on the lines (the IM cart has no
		// separate id-set like unavailableItems — spinIds are line-scoped, not
		// shared across lines the way a food menu_item_id can repeat).
		for i := range m.imLines {
			for _, l := range dm.Cart.Lines {
				if l.SpinID == m.imLines[i].Item.SwiggyID {
					m.imLines[i].Unavailable = !l.Available
				}
			}
		}
		m = m.commitIMCartConfirmed()
		m.imCartMutating = false // confirmed — unfreeze
		if m.screen == scrCheckout && m.checkoutVertical == 1 {
			m.checkout = m.buildIMCheckout()
		}
		if m.screen == scrInstamart {
			// Repaint so the cart chip picks up Swiggy's confirmed count/total.
			m = m.refreshInstamart()
		}
		if m.imConfirmPending {
			// The pre-confirm flush landed: the bill now reflects exactly these
			// lines. Open the order-confirm modal on the fresh, authoritative total.
			m.imConfirmPending = false
			if !m.placingOrder && len(m.imLines) > 0 && m.screen == scrCheckout && m.checkoutVertical == 1 {
				m.orderConfirmOpen = true
				m.orderConfirmSel = 0
			}
		}
		return m, nil
	case datasource.IMCartLoadedMsg:
		if dm.Err != nil {
			m.imCartSyncErr = "cart: " + dm.Err.Error()
			return m, nil
		}
		m.imCartSyncErr = ""
		m.imLiveCart = dm.Cart
		for i := range m.imLines {
			for _, l := range dm.Cart.Lines {
				if l.SpinID == m.imLines[i].Item.SwiggyID {
					m.imLines[i].Unavailable = !l.Available
				}
			}
		}
		// A successful load confirms the current lines as the Swiggy baseline.
		m = m.commitIMCartConfirmed()
		if m.screen == scrCheckout && m.checkoutVertical == 1 {
			m.checkout = m.buildIMCheckout()
		}
		return m, nil
	case datasource.UsualsLoadedMsg:
		if dm.Err != nil {
			dbgTUI("usuals: %v", dm.Err)
		}
		m.usualsLoaded = true // fetched (success or empty); don't refire for this addr
		m.refreshMenu()       // data refresh — keep the user's scroll position
		return m, nil
	case datasource.LoggedOutMsg:
		// Token purged → drop the cart/session state and re-authorize in place:
		// start a fresh OAuth flow and show the gate (which auto-opens the browser
		// and polls for completion). dm.Err is logged; the gate shows regardless.
		if dm.Err != nil {
			dbgTUI("logout: %v", dm.Err)
		}
		m.lines = nil
		m.cartRestaurant = ""
		m.cartSection = ""
		m.liveCart = api.Cart{}
		m.cartLoaded = false
		m = m.clearPlacedCarts() // both verticals' carts belong to the dead session
		m.altOrder = localstore.ActiveOrder{}
		m.hasAltOrder = false
		m.trackPickOpen = false
		m.screen = scrSplash
		// The browser is never auto-opened on the gate — the user reconnects by
		// pressing Enter. (If still logged into Swiggy, an auto-open would silently
		// re-consent and make the disconnect pointless.)
		if m.authClient != nil {
			fid, url, err := m.authClient.StartAuth(m.accountID)
			if err == nil {
				m.authFlowID = fid
				m.authorizeURL = url
				m.needsAuth = true
				return m, nil
			}
			dbgTUI("logout: re-auth start failed: %v", err)
		}
		m.needsAuth = true // no client (mock) → just show the gate
		return m, nil
	case datasource.OrderPlacedMsg:
		m.placingOrder = false
		if dm.Err != nil {
			// Surface the real Swiggy rejection on the checkout page, not just the
			// status bar — the order did NOT go through.
			m.orderErr = "order failed: " + dm.Err.Error()
			if m.screen == scrCheckout && m.checkoutVertical == 0 {
				m.checkout = m.buildCheckout()
			}
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
		// Persist the active order for crash-resume + splash liveness.
		etaLo, etaHi := localstore.ParseETAMinutes(eta)
		placedAt := time.Now().Unix()
		restaurant := dm.Order.Restaurant
		if restaurant == "" {
			restaurant = m.checkout.Place()
		}
		ao := localstore.ActiveOrder{
			OrderID:    dm.Order.ID,
			Restaurant: restaurant,
			AddrLine:   m.addr.Line,
			ETALoMin:   etaLo,
			ETAHiMin:   etaHi,
			Total:      dm.Order.Total,
			PlacedAt:   placedAt,
		}
		_ = localstore.SaveActiveOrder(ao)
		// Accrete the taste card's default address from app orders. The TUI only
		// has the restaurant NAME here (not a Swiggy id), so restaurantID is left
		// empty — bumpFavorite skips, and we never key a favorite by a name.
		_ = localstore.RecordOrder(m.addr.ID, m.addr.Label, "", restaurant, placedAt)
		if m.hasActiveOrder && m.activeOrder.Vertical == "instamart" {
			// The other vertical's delivery is still live — keep it as the alt
			// instead of silently dropping it from the splash/tracking.
			m.altOrder = m.activeOrder
			m.hasAltOrder = true
		}
		m.activeOrder = ao
		m.hasActiveOrder = true
		m.confirmTick = 0
		m.track = screens.NewTracking(restaurant, m.addr.Line, dm.Order.ID, placedAt, etaLo, etaHi)
		m = m.splashOrder(splashOrderLabel(restaurant, "", "", etaHi))
		return m, nil
	case datasource.TrackingPolledMsg:
		if dm.Err != nil {
			// Non-fatal: keep the current tracking view as-is.
			return m, nil
		}
		m.track = m.track.WithLive(dm.Tracking.Status, dm.Tracking.ETA)
		// Check for delivery: active==false AND (elapsed >5min OR status == delivered).
		if !dm.Tracking.Active && m.hasActiveOrder {
			elapsedSec := m.nowUnix - m.activeOrder.PlacedAt
			_, delivered, _ := screens.StageFromStatus(dm.Tracking.Status)
			if delivered || elapsedSec > 90 {
				_ = localstore.ClearActiveOrder()
				m.hasActiveOrder = false
				m.splash = m.splash.WithOrder("")
				// A second live delivery takes over the slot instead of vanishing
				// with the finished one.
				if pm, cmd := m.promoteAltOrder(); pm.hasActiveOrder {
					return pm, cmd
				}
				m.homeSel = clampIdx(m.homeSel, len(screens.HomeItems(false)))
			}
		}
		// Keep the splash track-order button's ETA in sync with the live ETA.
		if m.hasActiveOrder {
			m = m.splashOrder(splashOrderLabel(m.activeOrder.Restaurant, dm.Tracking.Status, dm.Tracking.ETA, m.activeOrder.ETAHiMin))
		}
		return m, nil
	case datasource.ActiveOrdersLoadedMsg:
		if dm.Err != nil {
			return m, nil
		}
		if m.hasActiveOrder && m.activeOrder.Vertical == "instamart" {
			// An Instamart order owns the active-order slot — a live FOOD order
			// found alongside it becomes the alt order (the splash "track order"
			// entry turns into a picker; the food discovery never overrides).
			for _, o := range dm.Orders {
				if _, delivered, _ := screens.StageFromStatus(o.Status); delivered {
					continue
				}
				etaLo, etaHi := localstore.ParseETAMinutes(o.ETA)
				m.altOrder = localstore.ActiveOrder{
					OrderID: o.ID, Restaurant: o.Restaurant, AddrLine: m.addr.Line,
					ETALoMin: etaLo, ETAHiMin: etaHi, Total: o.Total, PlacedAt: time.Now().Unix(),
				}
				m.hasAltOrder = true
				m = m.refreshSplashOrderLabel()
				return m, nil
			}
			// The scan found NO live food order — a previously-set FOOD alt is
			// stale (delivered/cancelled) and must not survive to be promoted or
			// picked later.
			if m.hasAltOrder && m.altOrder.Vertical != "instamart" {
				m.altOrder = localstore.ActiveOrder{}
				m.hasAltOrder = false
				m = m.refreshSplashOrderLabel()
			}
			return m, nil
		}
		if !m.hasActiveOrder {
			// Discovery: the Start Screen check found a live order we didn't know
			// about (placed on the Swiggy app, or after a fresh launch). Surface
			// the delivery-status button. Swiggy's order list carries no items,
			// but id/restaurant/total/ETA are enough to track.
			for _, o := range dm.Orders {
				if _, delivered, _ := screens.StageFromStatus(o.Status); delivered {
					continue // ignore already-delivered orders
				}
				etaLo, etaHi := localstore.ParseETAMinutes(o.ETA)
				ao := localstore.ActiveOrder{
					OrderID:    o.ID,
					Restaurant: o.Restaurant,
					AddrLine:   m.addr.Line,
					ETALoMin:   etaLo,
					ETAHiMin:   etaHi,
					Total:      o.Total,
					PlacedAt:   time.Now().Unix(),
				}
				_ = localstore.SaveActiveOrder(ao)
				m.activeOrder = ao
				m.hasActiveOrder = true
				m = m.splashOrder(splashOrderLabel(o.Restaurant, o.Status, "", etaHi))
				// Pull the live ETA now so the splash button shows it, not just the
				// initial estimate (TrackingPolledMsg updates the label).
				return m, datasource.PollTrackingCmd(m.backend, ao.OrderID)
			}
			return m, nil
		}
		// Check whether our saved order is still in the active list.
		found := false
		for _, o := range dm.Orders {
			if o.ID == m.activeOrder.OrderID {
				found = true
				break
			}
		}
		elapsedSec := m.nowUnix - m.activeOrder.PlacedAt
		if !found && elapsedSec > 90 {
			_ = localstore.ClearActiveOrder()
			m.hasActiveOrder = false
			m.splash = m.splash.WithOrder("")
			if pm, cmd := m.promoteAltOrder(); pm.hasActiveOrder {
				return pm, cmd
			}
			m.homeSel = clampIdx(m.homeSel, len(screens.HomeItems(false)))
			return m, nil
		}
		if found {
			// Order still live — refresh the splash label and pull the live ETA so
			// the button stays in sync (TrackingPolledMsg applies the live ETA).
			m = m.splashOrder(splashOrderLabel(m.activeOrder.Restaurant, "", "", m.activeOrder.ETAHiMin))
			return m, datasource.PollTrackingCmd(m.backend, m.activeOrder.OrderID)
		}
		return m, nil
	case datasource.IMOrderPlacedMsg:
		m.placingOrder = false
		if dm.Err != nil {
			// Surface the real Swiggy rejection on the checkout page, not just the
			// status bar — the order did NOT go through.
			m.imOrderErr = "order failed: " + dm.Err.Error()
			if m.screen == scrCheckout && m.checkoutVertical == 1 {
				m.checkout = m.buildIMCheckout()
			}
			return m, nil
		}
		m.imOrderErr = ""
		eta := dm.Order.ETA
		if eta == "" {
			eta = "10-20 mins"
		}
		m.checkout = m.checkout.Placed(dm.Order.ID, eta)
		m.screen = scrConfirm
		// Capture the total before the cart is cleared (Order.Total may be unset
		// on some responses — the last synced IMCart is the fallback source).
		total := dm.Order.Total
		if total == 0 {
			total = m.imLiveCart.Total
		}
		// The cart's selectedAddressDetails is the ONLY source of the delivery
		// coordinates track_order requires (get_addresses/get_orders omit them,
		// harvested 2026-07-03) — capture them before the cart state is cleared.
		lat, lng := m.imLiveCart.AddrLat, m.imLiveCart.AddrLng
		m.imLines = nil
		m.imLiveCart = api.IMCart{}
		// The empty cart is the new confirmed baseline: a later failed sync must
		// roll back to EMPTY, never resurrect the just-placed lines.
		m = m.commitIMCartConfirmed()
		// Persist the active order for crash-resume + splash liveness.
		etaLo, etaHi := localstore.ParseETAMinutes(eta)
		placedAt := time.Now().Unix()
		ao := localstore.ActiveOrder{
			OrderID:    dm.Order.ID,
			Restaurant: "Instamart",
			AddrLine:   m.addr.Line,
			ETALoMin:   etaLo,
			ETAHiMin:   etaHi,
			Total:      total,
			PlacedAt:   placedAt,
			Vertical:   "instamart",
			Lat:        lat,
			Lng:        lng,
		}
		_ = localstore.SaveActiveOrder(ao)
		_ = localstore.RecordOrder(m.addr.ID, m.addr.Label, "", "Instamart", placedAt)
		if m.hasActiveOrder && m.activeOrder.Vertical != "instamart" {
			// The other vertical's delivery is still live — keep it as the alt
			// instead of silently dropping it from the splash/tracking.
			m.altOrder = m.activeOrder
			m.hasAltOrder = true
		}
		m.activeOrder = ao
		m.hasActiveOrder = true
		m.confirmTick = 0
		m.track = screens.NewTracking("Instamart", m.addr.Line, dm.Order.ID, placedAt, etaLo, etaHi)
		m = m.splashOrder(splashOrderLabel("Instamart", "", "", etaHi))
		// Force-clear the server cart after placement: checkout normally consumes
		// it, but leftovers have been seen live lingering in the Swiggy app cart —
		// clear_cart is idempotent ("Cart not found" maps to success), so this is
		// a safe belt-and-braces flush. Also harvest lat/lng for tracking —
		// track_order requires coordinates that only get_orders (IMOrders)
		// carries; get_addresses omits them.
		return m, tea.Batch(datasource.ClearIMCartCmd(m.backend), datasource.LoadIMActiveOrdersCmd(m.backend))
	case datasource.IMActiveOrdersLoadedMsg:
		if dm.Err != nil {
			return m, nil
		}
		if m.hasActiveOrder && m.activeOrder.Vertical == "instamart" {
			// Refresh: find our order (by id, or adopt the first non-delivered one
			// when we don't have an id yet) to harvest status/ETA.
			found := false
			for _, o := range dm.Orders {
				if m.activeOrder.OrderID != "" && o.ID != m.activeOrder.OrderID {
					continue
				}
				if m.activeOrder.OrderID == "" {
					if _, delivered, _ := screens.StageFromStatus(o.Status); delivered {
						continue
					}
					m.activeOrder.OrderID = o.ID
				}
				found = true
				// The live get_orders payload carries NO coordinates — never let
				// its zeros clobber the good ones persisted from the cart at
				// placement time.
				if o.Lat != 0 || o.Lng != 0 {
					m.activeOrder.Lat = o.Lat
					m.activeOrder.Lng = o.Lng
				}
				_ = localstore.SaveActiveOrder(m.activeOrder)
				m.track = m.track.WithLive(o.Status, o.ETA).WithDetail(o.Detail)
				m = m.splashOrder(splashOrderLabel("Instamart", o.Status, o.ETA, m.activeOrder.ETAHiMin))
				break
			}
			// Delivered orders drop out of the active list immediately. A
			// coords-less order polls THROUGH this handler (track_order needs
			// coordinates), so this is its only clear path — mirror the food
			// branch's not-found clear + promote the alt.
			if !found && m.activeOrder.OrderID != "" && m.nowUnix-m.activeOrder.PlacedAt > 90 {
				_ = localstore.ClearActiveOrder()
				m.hasActiveOrder = false
				m.splash = m.splash.WithOrder("")
				if pm, cmd := m.promoteAltOrder(); pm.hasActiveOrder {
					return pm, cmd
				}
				m.homeSel = clampIdx(m.homeSel, len(screens.HomeItems(false)))
			}
			return m, nil
		}
		if m.hasActiveOrder {
			// A FOOD order owns the active-order slot — a live Instamart order
			// found alongside it becomes the alt order for the splash picker.
			for _, o := range dm.Orders {
				if _, delivered, _ := screens.StageFromStatus(o.Status); delivered {
					continue
				}
				etaLo, etaHi := localstore.ParseETAMinutes(o.ETA)
				m.altOrder = localstore.ActiveOrder{
					OrderID: o.ID, Restaurant: "Instamart", AddrLine: m.addr.Line,
					ETALoMin: etaLo, ETAHiMin: etaHi, Total: o.Total, PlacedAt: time.Now().Unix(),
					Vertical: "instamart", Lat: o.Lat, Lng: o.Lng,
				}
				m.hasAltOrder = true
				m = m.refreshSplashOrderLabel()
				return m, nil
			}
			// No live Instamart order left — drop a stale IM alt so it can't be
			// promoted or picked after its delivery.
			if m.hasAltOrder && m.altOrder.Vertical == "instamart" {
				m.altOrder = localstore.ActiveOrder{}
				m.hasAltOrder = false
				m = m.refreshSplashOrderLabel()
			}
			return m, nil
		}
		if !m.hasActiveOrder {
			// Discovery: the Start Screen check found a live Instamart order we
			// didn't know about (placed on the Swiggy app, or after a fresh launch).
			for _, o := range dm.Orders {
				if _, delivered, _ := screens.StageFromStatus(o.Status); delivered {
					continue // ignore already-delivered orders
				}
				etaLo, etaHi := localstore.ParseETAMinutes(o.ETA)
				ao := localstore.ActiveOrder{
					OrderID:    o.ID,
					Restaurant: "Instamart",
					AddrLine:   m.addr.Line,
					ETALoMin:   etaLo,
					ETAHiMin:   etaHi,
					Total:      o.Total,
					PlacedAt:   time.Now().Unix(),
					Vertical:   "instamart",
					Lat:        o.Lat,
					Lng:        o.Lng,
				}
				_ = localstore.SaveActiveOrder(ao)
				m.activeOrder = ao
				m.hasActiveOrder = true
				m = m.splashOrder(splashOrderLabel("Instamart", o.Status, "", etaHi))
				return m, nil
			}
		}
		return m, nil
	case datasource.IMTrackingPolledMsg:
		if dm.Err != nil {
			// Non-fatal: keep the current tracking view as-is.
			return m, nil
		}
		m.track = m.track.WithLive(dm.Tracking.Status, dm.Tracking.ETA).WithDetail(dm.Tracking.Detail)
		// Check for delivery: active==false AND (elapsed >5min OR status == delivered).
		if !dm.Tracking.Active && m.hasActiveOrder {
			elapsedSec := m.nowUnix - m.activeOrder.PlacedAt
			_, delivered, _ := screens.StageFromStatus(dm.Tracking.Status)
			if delivered || elapsedSec > 90 {
				_ = localstore.ClearActiveOrder()
				m.hasActiveOrder = false
				m.splash = m.splash.WithOrder("")
				// A second live delivery takes over the slot instead of vanishing
				// with the finished one.
				if pm, cmd := m.promoteAltOrder(); pm.hasActiveOrder {
					return pm, cmd
				}
				m.homeSel = clampIdx(m.homeSel, len(screens.HomeItems(false)))
			}
		}
		// Keep the splash track-order button's ETA in sync with the live ETA.
		if m.hasActiveOrder {
			m = m.splashOrder(splashOrderLabel(m.activeOrder.Restaurant, dm.Tracking.Status, dm.Tracking.ETA, m.activeOrder.ETAHiMin))
		}
		return m, nil
	case datasource.ReleaseNotesMsg:
		switch {
		case dm.Err != nil:
			// Network/server error: do nothing. LastSeenVersion is NOT advanced so
			// the notes will be retried on next launch.
		case dm.NotFound:
			// No notes for this version (404): silently advance LastSeenVersion.
			nv := m.notesVersion
			return m, func() tea.Msg {
				_ = localstore.SetLastSeenVersion(nv)
				return nil
			}
		case dm.Markdown != "":
			// Notes received: pre-render and arm the auto-open at splash→scrMenu.
			m.whatsnewLines = screens.RenderNotesMarkdown(dm.Markdown)
			m.notesReady = true
		}
		return m, nil
	}
	if k, ok := msg.(tea.KeyMsg); ok {
		// Help modal captures keys while open: scroll with ↑/↓ (or j/k), close on
		// esc / q / ? / H / enter. It overlays whatever screen is behind it.
		if m.helpOpen {
			switch k.String() {
			case "up", "k":
				if m.helpScroll > 0 {
					m.helpScroll--
				}
			case "down", "j":
				if m.helpScroll < screens.HelpMaxScroll(m.h) {
					m.helpScroll++
				}
			case "left", "h":
				if m.helpPage > 0 {
					m.helpPage--
					m.helpScroll = 0
				}
			case "right", "l":
				if m.helpPage < screens.HelpPageCount()-1 {
					m.helpPage++
					m.helpScroll = 0
				}
			case "1", "2", "3", "4", "5":
				pg := int(k.Runes[0] - '1')
				if pg < 0 {
					pg = 0
				}
				if pg >= screens.HelpPageCount() {
					pg = screens.HelpPageCount() - 1
				}
				m.helpPage = pg
				m.helpScroll = 0
			case "esc", "q", "?", "H", "enter", " ":
				m.helpOpen = false
				if m.addrGatePending {
					return m, m.maybeOpenAddrGate()
				}
			}
			return m, nil
		}
		// What's-new modal: captures all keys while open. Mirrors the help
		// block above. On close, advance LastSeenVersion so notes show once.
		if m.whatsnewOpen {
			wn := screens.NewWhatsNew(m.notesVersion, m.whatsnewLines).WithViewport(m.h)
			pageCount := wn.PageCount()
			switch k.String() {
			case "up", "k":
				if m.whatsnewScroll > 0 {
					m.whatsnewScroll--
				}
			case "down", "j":
				if m.whatsnewScroll < screens.HelpMaxScroll(m.h) {
					m.whatsnewScroll++
				}
			case "left", "h":
				if m.whatsnewPage > 0 {
					m.whatsnewPage--
					m.whatsnewScroll = 0
				}
			case "right", "l":
				if m.whatsnewPage < pageCount-1 {
					m.whatsnewPage++
					m.whatsnewScroll = 0
				}
			case "1", "2", "3", "4", "5", "6", "7", "8", "9":
				pg := int(k.Runes[0] - '1')
				if pg < 0 {
					pg = 0
				}
				if pg >= pageCount {
					pg = pageCount - 1
				}
				m.whatsnewPage = pg
				m.whatsnewScroll = 0
			case "esc", "q", "?", "enter", " ":
				m.whatsnewOpen = false
				nv := m.notesVersion
				versionCmd := func() tea.Msg {
					_ = localstore.SetLastSeenVersion(nv)
					return nil
				}
				if m.addrGatePending {
					return m, tea.Batch(versionCmd, m.maybeOpenAddrGate())
				}
				return m, versionCmd
			}
			return m, nil
		}
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
				switch {
				case action == "clear":
					// out already cleared in Run; stay open
				case action == "close":
					m.cmdOpen = false
				case action == "help":
					// :help opens the full help & controls modal.
					m.cmdOpen = false
					m.cmd = m.cmd.ClearText()
					m.helpOpen = true
					m.helpPage = 0
					m.helpScroll = 0
				case strings.HasPrefix(action, "alias"):
					rest := strings.TrimSpace(strings.TrimPrefix(action, "alias"))
					m.cmd = m.cmd.AppendOut(m.runAliasCommand(rest))
				}
			case "backspace":
				m.cmd = m.cmd.Backspace()
			case "delete":
				m.cmd = m.cmd.Delete()
			case "left":
				m.cmd = m.cmd.Left()
			case "right":
				m.cmd = m.cmd.Right()
			case "home", "ctrl+a":
				m.cmd = m.cmd.Home()
			case "end", "ctrl+e":
				m.cmd = m.cmd.End()
			default:
				// Space arrives as its own key type (tea.KeySpace), not KeyRunes —
				// handle it explicitly so multi-word commands (`alias set x`) work.
				switch {
				case k.Type == tea.KeySpace:
					m.cmd = m.cmd.Insert(" ")
				case k.Type == tea.KeyRunes:
					m.cmd = m.cmd.Insert(string(k.Runes))
				}
			}
			return m, nil
		}

		// Authorize gate captures all keys until the user retries or quits — but
		// not during the first-run welcome walkthrough, which owns the screen until
		// it hands off to the connect gate.
		if m.needsAuth && m.screen != scrWelcome {
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
			if m.addrForced {
				// Forced entry gate: non-dismissible. Only ctrl+c quits, enter picks,
				// esc/a are ignored, everything else moves the cursor.
				switch k.String() {
				case "ctrl+c":
					return m, tea.Quit
				case "esc", "a":
					// Hard gate — do nothing; keep modal open.
					return m, nil
				case "enter":
					m.addr = m.addrScreen.Selected()
					m.addrOpen = false
					m.addrForced = false
					m.addrGatePending = false
					if !m.cartRestaurantServes(m.addr) {
						m.lines = nil
						m.cartRestaurant = ""
						m.cartSection = ""
					}
					m.imCartPulled = false                // address changed — Instamart cart binds to address, re-pull next entry
					m.imLoadedQueries = map[string]bool{} // dedupe is per-address: the snapshot is keyed addr+query
					m.screen = scrMenu
					m.railActive = screens.RailHome
					m.railFocus = true
					m.searchMode = false
					m.homePending = false
					m.menu = m.buildMenu()
					return m, m.loadHomeForCurrentAddr()
				default:
					na, _ := m.addrScreen.Update(msg)
					m.addrScreen = na.(screens.Address)
					return m, nil
				}
			}
			// Normal (dismissible) address switcher.
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
					m.imCartPulled = false                // Instamart cart binds to address, re-pull next entry
					m.imLoadedQueries = map[string]bool{} // dedupe is per-address: the snapshot is keyed addr+query
					if m.screen == scrInstamart {
						// Switched from inside Instamart — stay there: refetch the go-to
						// list + re-pull the cart for the new address.
						m.imSearchMode = false
						m.menu = m.buildMenu()
						return m, m.imEnterCmd()
					}
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

		// While the order-confirm modal is open it captures all keys: ← → (or
		// y/n) move focus between "yes" and "no", Enter confirms the focused
		// button. esc cancels (no order placed); ctrl+c quits. Default focus is
		// "yes", so a reflexive Enter places the order — same as before this
		// modal existed, just with one extra keypress to reach it.
		if m.trackPickOpen {
			switch k.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "up", "k", "down", "j":
				m.trackPick = m.trackPick.WithFocus(1 - m.trackPick.Focus())
			case "esc":
				m.trackPickOpen = false
			case "enter":
				m.trackPickOpen = false
				if m.trackPick.Focus() == 1 {
					// Open the OTHER order: it becomes the primary (persisted),
					// the current primary becomes the alt.
					return m.swapTrackedOrder()
				}
				m.track = screens.NewTracking(
					m.activeOrder.Restaurant, m.activeOrder.AddrLine, m.activeOrder.OrderID,
					m.activeOrder.PlacedAt, m.activeOrder.ETALoMin, m.activeOrder.ETAHiMin,
				).WithAlt(true)
				m.trackTick = 0
				m.screen = scrTracking
				var pollCmd tea.Cmd
				if m.backend != nil {
					pollCmd = m.trackingPollCmd()
				}
				return m, pollCmd
			}
			return m, nil
		}

		if m.orderConfirmOpen {
			switch k.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "left", "h", "right", "l":
				m.orderConfirmSel = 1 - m.orderConfirmSel
			case "y":
				m.orderConfirmSel = 0
				return m.confirmPlaceOrder()
			case "n":
				m.orderConfirmOpen = false
			case "enter":
				if m.orderConfirmSel == 0 {
					return m.confirmPlaceOrder()
				}
				m.orderConfirmOpen = false
			case "esc":
				m.orderConfirmOpen = false
			}
			return m, nil
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
						m.cartForeign = false
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
				if m.pendingSection == catalog.SectionInstamart {
					// Instamart: the chosen variant's ChoiceID IS the spinId. No
					// restaurant/cart-owner concept — multi-store carts are allowed,
					// so there is no conflict modal to raise.
					m = m.imCommitAdd(item, sels, price)
					m = m.refreshInstamart()
					if m.live {
						m.armIMCartSync()
					}
					return m, nil
				}
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
		// Double-Esc (two quick taps) returns to the splash and replays the loading
		// animation — a deliberate "home" gesture recognised from ANY screen.
		// Checked before the per-screen single-Esc back, so two quick taps always
		// teleport home; a single Esc still steps back one level. Cart/address
		// are preserved. (Splash handles its own keys below.)
		if k.String() == "esc" && m.screen != scrSplash {
			// Esc closes the restaurant-info modal first (the browse-list 'i' card).
			if m.screen == scrMenu && m.restInfoOpen {
				m.restInfoOpen = false
				return m, nil
			}
			if m.frame-m.lastEscFrame <= escDoubleWindow {
				m = m.toSplash()
				m.lastEscFrame = -escDoubleWindow - 1
				return m, m.activeOrderCheckCmd()
			}
			m.lastEscFrame = m.frame // first Esc: arm the double-tap window

			// Browse root: a single Esc unfocuses the live rail but never navigates
			// away (the second tap homes). While searching, fall through so the
			// search handler exits search.
			if m.screen == scrMenu && !m.menu.Searching() && !m.searchMode {
				if m.live && m.railFocus {
					m.railFocus = false
					m.menu = m.buildMenu()
				}
				return m, nil
			}
			// fall through to per-screen single-Esc (restaurant/checkout/tracking/…)
		}
		// First-run onboarding: the welcome screen owns all keys (handled before
		// the global ?/H trigger so keys never leak into help or other handlers).
		if m.screen == scrWelcome {
			if m.welcome.Phase() == 0 {
				// Any key skips the animation straight to the intro card.
				m.welcome = m.welcome.WithPhase(1)
				return m, nil
			}
			// Phase 1 (intro card).
			switch k.String() {
			case "enter", " ":
				// Dismiss onboarding: write the first-run marker and drop into the
				// normal splash with a fresh wordmark boot.
				markerCmd := func() tea.Msg {
					_ = localstore.MarkOnboarded()
					return nil
				}
				m.screen = scrSplash
				m.decodeStep = 0
				m.splashTick = 0
				m.wantOnboarding = false
				return m, markerCmd
			case "l", "L":
				url := m.welcome.LearnURL()
				if url == "" {
					url = screens.DefaultLearnURL
				}
				return m, openBrowserCmd(url)
			}
			// Any other key on the card is ignored.
			return m, nil
		}
		// ? or H opens the help & controls modal from anywhere — except while
		// typing (search / palette) or with a blocking modal already up, where the
		// key means something else or would be swallowed.
		if (k.String() == "?" || k.String() == "H") && m.helpTriggerable() {
			m.helpOpen = true
			m.helpPage = 0
			m.helpScroll = 0
			return m, nil
		}
		if m.screen == scrSplash {
			// Settings modal (opened from the splash) captures keys while open.
			if m.settingsOpen {
				switch k.String() {
				case "up", "k":
					if m.settingsSel > 0 {
						m.settingsSel--
					}
				case "down", "j":
					if m.settingsSel < screens.SettingsItemCount()-1 {
						m.settingsSel++
					}
				case "esc", "q":
					m.settingsOpen = false
				case "enter", " ":
					st := screens.NewSettings(m.live && m.backend != nil).WithSelection(m.settingsSel)
					m.settingsOpen = false
					if st.SelectedAction() == "disconnect" {
						return m, datasource.Logout(m.backend)
					}
				}
				return m, nil
			}
			// The decode plays on its own; the user never has to wait for it.
			// Arrows move the cursor; any other key activates the highlighted item:
			// "go to shop" enters the shop, "settings" opens the settings modal.
			switch k.String() {
			case "up", "k":
				if m.homeSel > 0 {
					m.homeSel--
				}
			case "down", "j":
				if m.homeSel < len(screens.HomeItems(m.hasActiveOrder))-1 {
					m.homeSel++
				}
			default:
				if screens.IsSettings(m.homeSel, m.hasActiveOrder) {
					m.settingsOpen = true
					m.settingsSel = 0
				} else if screens.IsTrack(m.homeSel, m.hasActiveOrder) {
					if m.hasAltOrder {
						// Two deliveries live at once (food + Instamart) — one
						// tracking page shows one order, so ask which to open.
						m.trackPick = screens.NewTrackPick(
							trackPickLabel(m.activeOrder), trackPickLabel(m.altOrder))
						m.trackPickOpen = true
						return m, nil
					}
					// Restore tracking screen from persisted active order.
					m.track = screens.NewTracking(
						m.activeOrder.Restaurant,
						m.activeOrder.AddrLine,
						m.activeOrder.OrderID,
						m.activeOrder.PlacedAt,
						m.activeOrder.ETALoMin,
						m.activeOrder.ETAHiMin,
					)
					m.screen = scrTracking
					var pollCmd tea.Cmd
					if m.backend != nil {
						pollCmd = m.trackingPollCmd()
					}
					return m, pollCmd
				} else {
					m.screen = scrMenu
					if m.notesReady {
						// What's-new auto-open: mutually exclusive with onboarding.
						m.whatsnewOpen = true
						m.whatsnewPage = 0
						m.whatsnewScroll = 0
						m.notesReady = false
						// Gate waits until the overlay closes.
					} else {
						// No entry overlay — open the gate now (or wait for addresses).
						return m, m.maybeOpenAddrGate()
					}
				}
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

			// `/` jumps straight into search from anywhere on the browse screen.
			if m.screen == scrMenu && m.live && len(m.chips) > 0 && !m.searchMode && k.String() == "/" {
				m.railActive = screens.RailSearch
				m.railFocus = false
				m.syncSearchEntry() // searchMode=true, fresh empty input
				m.menu = m.buildMenu()
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
								return m, m.startMenuLoad(p.SwiggyID)
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
				case "delete":
					// Forward-delete: remove the rune AT the caret; caret stays put.
					if r := []rune(m.searchQuery); m.searchCaret < len(r) {
						m.searchQuery = string(r[:m.searchCaret]) + string(r[m.searchCaret+1:])
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
					m.armRailLoad() // debounced — onTick loads once the cursor settles
					m.menu = m.buildMenu()
					return m, nil
				case "down", "j":
					if m.railActive < rail.Len()-1 {
						m.railActive++
					}
					m.syncSearchEntry()
					m.armRailLoad() // debounced — onTick loads once the cursor settles
					m.menu = m.buildMenu()
					return m, nil
				case "enter":
					m.railSettlePending = false // explicit pick → load now, not on settle
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
					return m, m.imEnterCmd()
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
							return m, m.startMenuLoad(p.SwiggyID)
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
					return m, m.imEnterCmd()
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
				// Tiered esc: a committed dish search clears first (undo the
				// search, stay on the menu); a second esc leaves to discovery.
				if m.rest.Filter() != "" {
					m.rest = m.rest.ClearSearch()
					return m, nil
				}
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
		case scrCheckout:
			if m.checkoutVertical == 1 {
				// Freeze: while a reduce/delete sync is in flight, ignore all keys
				// until IMCartSyncedMsg clears imCartMutating (race guard).
				if m.imCartMutating {
					return m, nil
				}
				switch k.String() {
				case "esc":
					m.checkoutVertical = 0
					m.screen = scrInstamart
					return m, nil
				case "up", "k":
					m.checkout = m.checkout.Up()
					return m, nil
				case "down", "j":
					m.checkout = m.checkout.Down()
					return m, nil
				case "+", "=", "right", "l":
					// Optimistic increment of the focused line — no freeze.
					i := m.checkout.Cursor()
					if i >= 0 && i < len(m.imLines) {
						m.imOrderErr = "" // editing clears a stale "can't order" message
						m.imLines[i].Qty++
						m.menu = m.menu.WithCartChip(m.cartChip())
						m.checkout = m.buildIMCheckout()
						m.armIMCartSync() // debounced: mashing + collapses to one update_cart
						return m, nil
					}
					return m, nil
				case "-", "_", "left", "h":
					// Reduce (remove at qty 1) — freeze until confirmed.
					i := m.checkout.Cursor()
					if i < 0 || i >= len(m.imLines) {
						return m, nil
					}
					if m.imLines[i].Qty <= 1 {
						m.imLines = append(m.imLines[:i], m.imLines[i+1:]...)
					} else {
						m.imLines[i].Qty--
					}
					return m.afterIMCheckoutReduce()
				case "delete", "backspace":
					// Remove the whole line — freeze until confirmed.
					i := m.checkout.Cursor()
					if i < 0 || i >= len(m.imLines) {
						return m, nil
					}
					m.imLines = append(m.imLines[:i], m.imLines[i+1:]...)
					return m.afterIMCheckoutReduce()
				case "enter":
					if len(m.imLines) == 0 {
						return m, nil // nothing to order — empty cart
					}
					if m.live && m.hasUnavailableIMLine() {
						// Swiggy would reject the order — say which and don't fire it.
						m.imOrderErr = "can't place order — remove the sold-out item(s) first"
						m.checkout = m.buildIMCheckout()
						return m, nil
					}
					if m.live && m.checkout.OverCap() {
						// Swiggy's MCP beta rejects carts ≥ ₹1000. The page already shows
						// the bordered cap notice + a disabled bar; refuse to fire.
						return m, nil
					}
					if m.live {
						if total := m.imCartTotal(); total < InstamartMin {
							m.imOrderErr = fmt.Sprintf("₹99 minimum on instamart — add ₹%d more", InstamartMin-total)
							m.checkout = m.buildIMCheckout()
							return m, nil
						}
					}
					if m.placingOrder {
						return m, nil
					}
					// Final-value guarantee: if the server cart doesn't yet match the
					// local lines (a debounced + bump still settling), flush it now and
					// open the confirm modal only when the sync returns — so the total
					// the user approves is Swiggy's real bill for these exact lines.
					if m.live && !m.imLiveMatchesLines() {
						m.imConfirmPending = true
						m.imCartSyncPending = false // the explicit sync below supersedes the debounce
						m.imCartMutating = true     // freeze input until the sync lands (then the modal opens)
						return m, m.imLiveCartCmd()
					}
					m.orderConfirmOpen = true
					m.orderConfirmSel = 0 // default "yes" — a reflexive ↵ still places the order
					return m, nil
				}
				return m, nil
			}
			// Freeze: while a reduce/delete sync is in flight, ignore all keys
			// until CartSyncedMsg clears cartMutating (race guard).
			if m.cartMutating {
				return m, nil
			}
			switch k.String() {
			case "esc":
				m.screen = scrMenu
				return m, nil
			case "up", "k":
				m.checkout = m.checkout.Up()
				return m, nil
			case "down", "j":
				m.checkout = m.checkout.Down()
				return m, nil
			case "+", "=", "right", "l":
				// Optimistic increment of the focused line — no freeze.
				i := m.checkout.Cursor()
				if i >= 0 && i < len(m.lines) {
					m.orderErr = "" // editing clears a stale "can't order" message
					m.lines[i].Qty++
					m.menu = m.menu.WithCartChip(m.cartChip())
					m.checkout = m.buildCheckout()
					m.armCartSync() // debounced: mashing + collapses to one update_food_cart
					return m, nil
				}
				return m, nil
			case "-", "_", "left", "h":
				// Reduce (remove at qty 1) — freeze until confirmed.
				i := m.checkout.Cursor()
				if i < 0 || i >= len(m.lines) {
					return m, nil
				}
				if m.lines[i].Qty <= 1 {
					m.lines = append(m.lines[:i], m.lines[i+1:]...)
				} else {
					m.lines[i].Qty--
				}
				return m.afterCheckoutReduce()
			case "delete", "backspace":
				// Remove the whole line — freeze until confirmed.
				i := m.checkout.Cursor()
				if i < 0 || i >= len(m.lines) {
					return m, nil
				}
				m.lines = append(m.lines[:i], m.lines[i+1:]...)
				return m.afterCheckoutReduce()
			case "enter":
				if len(m.lines) == 0 {
					return m, nil // nothing to order — empty cart
				}
				if m.live && m.hasUnavailableLine() {
					// Swiggy would reject the order — say which and don't fire it.
					m.orderErr = "can't place order — remove the sold-out item(s) first"
					m.checkout = m.buildCheckout()
					return m, nil
				}
				if m.live && m.checkout.OverCap() {
					// Swiggy's MCP beta rejects carts ≥ ₹1000. The page already shows
					// the bordered cap notice + a disabled bar; refuse to fire.
					return m, nil
				}
				if !m.placingOrder {
					m.orderConfirmOpen = true
					m.orderConfirmSel = 0 // default "yes" — a reflexive ↵ still places the order
				}
				return m, nil
			}
		case scrConfirm:
			// Any key advances immediately to tracking (esc included — there's
			// no going back to "unplace" an order).
			m.screen = scrTracking
			m.trackTick = 0
			m = m.clearPlacedCarts()
			var pollCmd tea.Cmd
			if m.hasActiveOrder && m.backend != nil {
				pollCmd = m.trackingPollCmd()
			}
			return m, pollCmd
		case scrTracking:
			switch k.String() {
			case "esc":
				m = m.clearPlacedCarts()
				m.screen = scrMenu
				m.menu = m.buildMenu()
				return m, nil
			case "o":
				// Switch the page to the OTHER live delivery (food ⟷ instamart).
				if m.hasAltOrder {
					return m.swapTrackedOrder()
				}
				return m, nil
			case "d":
				// Dismiss delivered order: clear persistence + go to menu. A second
				// live delivery (the alt) takes over the slot instead of vanishing.
				_ = localstore.ClearActiveOrder()
				m.hasActiveOrder = false
				m.splash = m.splash.WithOrder("")
				var promoteCmd tea.Cmd
				m, promoteCmd = m.promoteAltOrder()
				m = m.clearPlacedCarts()
				m.screen = scrMenu
				m.menu = m.buildMenu()
				return m, promoteCmd
			}
		case scrInstamart:
			// `/` jumps straight into search from anywhere on the browse screen —
			// mirrors scrMenu's global shortcut. Re-opening after a search
			// PRE-FILLS the editor with the last submitted query (caret at end) so
			// it can be edited rather than retyped.
			if m.live && !m.imSearchMode && k.String() == "/" {
				m.imRailActive = screens.RailSearch
				m.imRailFocus = false
				m.imSearchMode = true
				m.imSearchQuery = m.imSearchSubmitted
				m.imSearchCaret = len([]rune(m.imSearchQuery))
				m.inst = m.buildInstamart()
				return m, nil
			}

			// Live search mode captures all printable keys + backspace/enter/esc,
			// submit-only (no per-keystroke filtering) — mirrors scrMenu's search
			// box.
			if m.live && m.imSearchMode && !m.imRailFocus {
				if k.Type == tea.KeyRunes {
					m.imSearchInsert(string(k.Runes))
					m.inst = m.inst.WithSearch(m.imSearchQuery, m.imSearchCaret, true)
					return m, nil
				}
				if k.Type == tea.KeySpace || k.String() == " " {
					m.imSearchInsert(" ")
					m.inst = m.inst.WithSearch(m.imSearchQuery, m.imSearchCaret, true)
					return m, nil
				}
				switch k.String() {
				case "esc":
					// Esc leaves search for Usuals — move the rail selection there too
					// (otherwise the rail keeps Search highlighted) and re-attach focus.
					m.imSearchMode = false
					m.imSearchQuery = ""
					m.imSearchCaret = 0
					m.imSearchSubmitted = ""
					m.imQuery = ""
					m.imRailActive = screens.RailHome
					m.imRailFocus = true
					cmd := m.ensureIMQuery("")
					m.imPending = cmd != nil
					m.inst = m.buildInstamart()
					return m, cmd
				case "left":
					if m.imSearchCaret > 0 {
						m.imSearchCaret--
						m.inst = m.inst.WithSearch(m.imSearchQuery, m.imSearchCaret, true)
					}
					return m, nil
				case "right":
					if r := []rune(m.imSearchQuery); m.imSearchCaret < len(r) {
						m.imSearchCaret++
					}
					m.inst = m.inst.WithSearch(m.imSearchQuery, m.imSearchCaret, true)
					return m, nil
				case "up", "k":
					if m.inst.CursorIndex() > 0 {
						m.inst = m.inst.WithListCursor(m.inst.CursorIndex() - 1)
					}
					return m, nil
				case "down", "j":
					m.inst = m.inst.WithListCursor(m.inst.CursorIndex() + 1)
					return m, nil
				case "backspace":
					if r := []rune(m.imSearchQuery); m.imSearchCaret > 0 && m.imSearchCaret <= len(r) {
						m.imSearchQuery = string(r[:m.imSearchCaret-1]) + string(r[m.imSearchCaret:])
						m.imSearchCaret--
					}
					m.inst = m.inst.WithSearch(m.imSearchQuery, m.imSearchCaret, true)
					return m, nil
				case "delete":
					if r := []rune(m.imSearchQuery); m.imSearchCaret < len(r) {
						m.imSearchQuery = string(r[:m.imSearchCaret]) + string(r[m.imSearchCaret+1:])
					}
					m.inst = m.inst.WithSearch(m.imSearchQuery, m.imSearchCaret, true)
					return m, nil
				case "enter":
					if m.imSearchQuery == "" {
						return m, nil
					}
					// Submit: close the editor so the result list takes focus (↑↓
					// move, ↵/→ add, ← remove all work on products) but remember the
					// query so a persistent chip shows and `/` re-opens it to edit.
					m.imQuery = m.imSearchQuery
					m.imSearchSubmitted = m.imSearchQuery
					m.imSearchMode = false
					m.imLoadedQueries[m.imQuery] = true // search submit always fetches live
					// Seed the same query from disk for an instant re-search paint;
					// the live fetch streams over it.
					m.imPending = !datasource.SeedIMFromCache(m.snap, m.addr.ID, m.imQuery)
					m.inst = m.buildInstamart().WithLoading(m.imPending)
					return m, datasource.LoadIMProducts(m.backend, m.snap, m.addr.ID, m.imQuery)
				}
				return m, nil
			}

			// Rail-focused keys — mirrors scrMenu's rail-focused block exactly.
			if m.live && m.imRailFocus {
				rail := m.imRail()
				// On the Search entry, typing starts searching immediately (no Enter).
				if m.imRailActive == screens.RailSearch {
					if k.Type == tea.KeyRunes || k.Type == tea.KeySpace {
						m.imSearchMode = true
						m.imRailFocus = false
						if k.Type == tea.KeySpace {
							m.imSearchInsert(" ")
						} else {
							m.imSearchInsert(string(k.Runes))
						}
						m.inst = m.buildInstamart()
						return m, nil
					}
				}
				switch k.String() {
				case "right", "l", "enter":
					m.imRailSettlePending = false // explicit pick → load now, not on settle
					m.imRailFocus = false
					switch m.imRailActive {
					case screens.RailSearch:
						m.imSearchMode = true
						m.imSearchQuery = ""
						m.imSearchCaret = 0
					case screens.RailHome:
						m.imSearchMode = false
						cmd := m.loadForIMRail(rail)
						m.inst = m.buildInstamart()
						return m, cmd
					default:
						if _, isCat := rail.IsCategory(m.imRailActive); isCat {
							m.imSearchMode = false
							cmd := m.loadForIMRail(rail)
							m.inst = m.buildInstamart()
							return m, cmd
						}
					}
					m.inst = m.buildInstamart()
					return m, nil
				case "esc":
					m.imRailFocus = false
					m.inst = m.buildInstamart()
					return m, nil
				case "up", "k":
					if m.imRailActive > 0 {
						m.imRailActive--
					}
					m.syncIMSearchEntry()
					m.armIMRailLoad() // debounced — onTick loads once the cursor settles
					m.inst = m.buildInstamart()
					return m, nil
				case "down", "j":
					if m.imRailActive < rail.Len()-1 {
						m.imRailActive++
					}
					m.syncIMSearchEntry()
					m.armIMRailLoad() // debounced — onTick loads once the cursor settles
					m.inst = m.buildInstamart()
					return m, nil
				case "c":
					m.imRailFocus = false
					m.inst = m.buildInstamart()
					return m, m.openIMCheckoutCmd()
				case "a":
					m.imRailFocus = false
					m.addrScreen = screens.NewAddress(m.repo.Addresses(), m.addr.ID)
					m.addrOpen = true
					return m, nil
				case "tab":
					m.imRailFocus = false
					m.vertical = 0
					m.screen = scrMenu
					return m, nil
				}
				return m, nil
			}

			// Main-list mode (not rail-focused, not search). ← focuses the rail.
			switch k.String() {
			case "esc":
				// esc always returns to Restaurants browse. When live and we entered
				// via vertical toggle, also reset the vertical state.
				m.vertical = 0
				m.screen = scrMenu
				return m, nil
			case "tab":
				m.vertical = 0
				m.screen = scrMenu
				return m, nil
			case "left", "h":
				if m.live {
					// Live: ← at the list column returns focus to the rail.
					m.imRailFocus = true
					m.inst = m.buildInstamart()
					return m, nil
				}
				// Mock (no rail): ← removes a unit, matching the pre-rail behavior.
				it, ok := m.inst.Selected()
				if !ok {
					return m, nil
				}
				m.imLines = decLastByItem(m.imLines, it.ID)
				m = m.refreshInstamart()
				return m, nil
			case "a":
				// Address switcher, same as the restaurant browse. The Instamart
				// cart binds to the address, so the switch path re-pulls it.
				m.addrScreen = screens.NewAddress(m.repo.Addresses(), m.addr.ID)
				m.addrOpen = true
				return m, nil
			case "enter", "right", "l":
				it, ok := m.inst.Selected()
				if !ok {
					return m, nil
				}
				if it.OutOfStock {
					// Swiggy would reject the add — say so instead of failing silently.
					m.imCartSyncErr = "“" + it.Name + "” is sold out"
					return m, nil
				}
				if m.live && it.Customizable {
					// Options are pre-synthesized locally (toIMItems) — no network
					// round-trip needed to open the picker.
					m.pendingItem = it
					m.pendingRest = "Instamart"
					m.pendingSection = catalog.SectionInstamart
					m.customize = screens.NewCustomize(it)
					m.customizeOpen = true
					return m, nil
				}
				m.imLines = appendOrInc(m.imLines, it, nil, nil, 0)
				m = m.refreshInstamart()
				if m.live {
					m.armIMCartSync()
				}
				return m, nil
			case "-", "_", "backspace":
				it, ok := m.inst.Selected()
				if !ok {
					return m, nil
				}
				m.imLines = decLastByItem(m.imLines, it.ID)
				m = m.refreshInstamart()
				if m.live {
					m.armIMCartSync()
				}
				return m, nil
			case "c":
				if m.live {
					return m, m.openIMCheckoutCmd()
				}
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
			// Legacy mock-only cart page — live nav goes straight from scrInstamart
			// to the merged scrCheckout (see the 'c' case above). Kept only so a
			// mock-mode session (which still routes here) compiles/behaves as before.
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
var statusHints = []string{"type : for commands", "247 devs online", "DEVFRIDAY −₹50", "esc esc · home", "consolestore.in"}

// screenLabel maps the current screen to the status-bar label (design line 836).
func (m Model) screenLabel() string {
	switch m.screen {
	case scrMenu:
		return "menu"
	case scrRestaurant:
		return "menu"
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

// screenKeybinds returns the real key affordances for the two main screens, so
// the status bar reads as a command line (power-tool feel) instead of rotating
// marketing copy. "" falls back to the flavor rotation on other screens.
func (m Model) screenKeybinds() string {
	switch m.screen {
	case scrMenu:
		if m.searchMode {
			return "↑↓ move · ↵ open · esc home · : cmd"
		}
		return "↑↓ move · ↵ open · / search · : cmd · ? help"
	case scrRestaurant:
		return "↑↓ move · ↵/+ add · − remove · c cart · : cmd · ? help"
	case scrCheckout:
		return "↑↓ pick · + − qty · ⌫ remove · ↵ place · : cmd · ? help"
	case scrInstamart:
		if m.imSearchMode {
			return "↑↓ move · ↵ open · esc usuals · : cmd"
		}
		if m.imRailFocus {
			return "↑↓ move · ↵ open · / search · : cmd · ? help"
		}
		return "↑↓ move · ↵/+ add · − remove · ← rail · c cart · esc back"
	default:
		return "? help"
	}
}

// statusBar renders the bottom bar for the current frame/screen.
func (m Model) statusBar() string {
	hint := statusHints[(m.frame/27)%len(statusHints)]
	if kb := m.screenKeybinds(); kb != "" {
		hint = kb // real keybinds on the screens users live on
	}
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

// helpTriggerable reports whether ? / H should open the help modal: not while a
// blocking modal is already up, and not while typing into a search box or the
// command palette (where the key is real input).
func (m Model) helpTriggerable() bool {
	if m.helpOpen || m.cmdOpen || m.settingsOpen || m.addrOpen || m.conflictOpen || m.customizeOpen || m.wizardOpen || m.restInfoOpen || m.orderConfirmOpen {
		return false
	}
	if m.searchMode || m.menu.Searching() {
		return false
	}
	if m.screen == scrRestaurant && (m.rest.Searching() || m.rest.InfoOpen()) {
		return false
	}
	return true
}

func (m Model) View() string {
	// The connect gate is the start screen once the welcome walkthrough is done;
	// while scrWelcome is active it owns the viewport (its View branch is below),
	// so the gate must not pre-empt it here.
	if m.needsAuth && m.screen != scrWelcome {
		// The login gate IS the start screen — same boot banner, but the home menu
		// is a single "connect swiggy" button (↵ opens the browser to authorize).
		gate := m.splash.WithDecode(m.decodeStep).WithFrame(m.frame).WithSplashTick(m.splashTick).
			WithPhrase(m.splashPhrase).WithConnect().View()
		if m.w == 0 || m.h == 0 {
			return gate
		}
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, gate)
	}

	// Help & controls modal (? / H / :help) — a centered, scrollable overlay over
	// any screen.
	if m.helpOpen {
		card := screens.NewHelp().WithViewport(m.h).WithPage(m.helpPage).WithScroll(m.helpScroll).View()
		if m.w == 0 || m.h == 0 {
			return card
		}
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, card)
	}

	// What's-new modal — a centered overlay showing release notes for the
	// current version, displayed once after an update.
	if m.whatsnewOpen {
		card := screens.NewWhatsNew(m.notesVersion, m.whatsnewLines).
			WithViewport(m.h).WithPage(m.whatsnewPage).WithScroll(m.whatsnewScroll).View()
		if m.w == 0 || m.h == 0 {
			return card
		}
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, card)
	}

	// Settings modal (opened from the splash) — a centered overlay matching the
	// other modal cards.
	if m.settingsOpen {
		card := screens.NewSettings(m.live && m.backend != nil).WithSelection(m.settingsSel).ModalView()
		if m.w == 0 || m.h == 0 {
			return card
		}
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, card)
	}

	// Splash is centered in the viewport (design lines 196-228). We render on
	// the terminal's own background — wrapping the frame in a lipgloss
	// Background tears on inner colour resets (banding), and a dark terminal
	// already provides the #15161f-ish canvas.
	if m.screen == scrWelcome {
		v := m.welcome.WithCaps(m.caps).WithFrame(m.frame).WithTick(m.welcomeTick).View()
		if m.w == 0 || m.h == 0 {
			return v
		}
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, v)
	}

	// Two-deliveries picker — opened from the splash "track order" entry, so it
	// must render before the splash's own early return.
	if m.trackPickOpen {
		dialog := m.trackPick.View()
		if m.w == 0 || m.h == 0 {
			return dialog
		}
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, dialog)
	}

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
	if m.orderConfirmOpen {
		dialog := screens.NewOrderConfirm(m.checkout.Place(), m.checkout.PayAmount()).
			WithAddress(m.addr.Line).WithFocus(m.orderConfirmSel).View()
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
		// chrome above the list: name + meta + category bar + their blanks + the
		// footer; the list windows to the rest so it fills the viewport (the old
		// 14 left a ~5-row blank gap above the footer).
		chrome := 10 + screens.BrandHeaderLines
		body = m.rest.WithMaxRows(m.listRows(chrome)).View()
	case scrCheckout, scrConfirm:
		body = m.checkout.WithPlacing(m.placingOrder).WithViewport(m.h).View(m.frame)
	case scrTracking:
		body = m.track.WithViewport(m.h).WithAlt(m.hasAltOrder).View(m.nowUnix, m.frame, m.spin())
	case scrInstamart:
		// Two-pane live layout mirrors scrMenu's chrome exactly (store switcher +
		// footer); the mock single-pane path (no rail attached) uses its own
		// fixed header instead, so this budget only matters when hasRail is true.
		body = m.inst.WithMaxRows(m.listRows(8 + screens.BrandHeaderLines)).View()
	case scrImCart:
		body = m.imCart.View()
	default: // scrMenu
		// chrome above the list: store switcher (2) + detail strip + a section
		// header + the footer; the list windows to whatever rows remain so the
		// brand header stays pinned and the page fills the viewport (no big gap).
		body = m.menu.WithMaxRows(m.listRows(8 + screens.BrandHeaderLines)).View()
	}

	// The footer — the screen's hint line + optional command palette + status
	// bar — is pinned to the bottom. The hint is the screen's last rendered
	// line; lift it out so it sits WITH the status bar instead of floating
	// after the content with a large void between them.
	content, hint := splitHint(body)

	// Centered brand logo at the top of every post-landing screen, with a gap
	// below it (the splash has its own big wordmark, so it is excluded above).
	// The cart chip follows the vertical on screen: Instamart pages (browse,
	// legacy cart, IM checkout/confirm) show the Instamart cart, not food's.
	chip := m.cartChip()
	if m.screen == scrInstamart || m.screen == scrImCart ||
		((m.screen == scrCheckout || m.screen == scrConfirm) && m.checkoutVertical == 1) {
		chip = m.imCartChip()
	}
	content = screens.BrandBanner(components.FrameWidth(), m.frame, m.addr.Line, m.addr.Label, chip) + content

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
	// Only lift the last line as a hint when it FLOATS after a void (a blank line
	// precedes it). A contiguous list — like the two-pane browse, which ends with
	// a restaurant row — must NOT have its final row yanked to the bottom.
	if last == 0 || strings.TrimSpace(lines[last-1]) != "" {
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
// cartScreenLines returns the authoritative lines to DISPLAY on the checkout
// screen. The merged checkout drives the list from m.lines — which carry the
// add-on/variant selections — so the bill (from liveCart via billFromLive) and
// the item list are always in sync without the flattened Swiggy copy overwriting
// local selections.
func (m Model) cartScreenLines() []screens.CartLine {
	if len(m.unavailableItems) == 0 {
		return m.lines
	}
	// Stamp the display-only Unavailable flag without mutating the canonical lines.
	out := make([]screens.CartLine, len(m.lines))
	copy(out, m.lines)
	for i := range out {
		if m.unavailableItems[out[i].Item.SwiggyID] {
			out[i].Unavailable = true
		}
	}
	return out
}

// afterCheckoutReduce finalizes a reduce/delete on the checkout: releases the
// restaurant binding if the cart is now empty, sets the freeze, rebuilds the
// page, and fires the cart sync (UpdateCart, or flush when empty).
func (m Model) afterCheckoutReduce() (tea.Model, tea.Cmd) {
	m.orderErr = "" // editing the cart clears a stale "can't order" message
	if len(m.lines) == 0 {
		m.cartRestaurant = ""
		m.cartSection = ""
	}
	cmd := m.liveCartCmd()
	// Freeze input only when a real sync is in flight. In mock mode
	// liveCartCmd() is nil — no CartSyncedMsg would ever arrive to clear the
	// freeze, so freezing here would hard-lock the screen.
	m.cartMutating = m.live && cmd != nil
	m.menu = m.menu.WithCartChip(m.cartChip())
	m.checkout = m.buildCheckout()
	return m, cmd
}

// afterIMCheckoutReduce finalizes a reduce/delete on the Instamart checkout.
// Mirrors afterCheckoutReduce; unlike food's cart (bound to a restaurant, so
// emptying it releases the binding), Instamart's cart is bound to the address
// and there is no restaurant field to clear.
func (m Model) afterIMCheckoutReduce() (tea.Model, tea.Cmd) {
	m.imOrderErr = "" // editing the cart clears a stale "can't order" message
	cmd := m.imLiveCartCmd()
	// Freeze input only when a real sync is in flight. In mock mode
	// imLiveCartCmd() is nil — no IMCartSyncedMsg would ever arrive to clear
	// the freeze, so freezing here would hard-lock the screen.
	m.imCartMutating = m.live && cmd != nil
	m.menu = m.menu.WithCartChip(m.cartChip())
	m.checkout = m.buildIMCheckout()
	return m, cmd
}

// confirmPlaceOrder fires the actual order placement — the "yes" branch of
// the order-confirm modal. This is the same logic checkout's ↵ ran directly
// before the confirm modal existed.
func (m Model) confirmPlaceOrder() (tea.Model, tea.Cmd) {
	m.orderConfirmOpen = false
	if m.checkoutVertical == 1 {
		if m.live && !m.placingOrder {
			m.placingOrder = true
			m.imOrderErr = ""
			m.imCartSyncPending = false // the Sequence syncs final lines; no trailing debounce after placing
			return m, tea.Sequence(m.imLiveCartCmd(), datasource.PlaceIMOrderCmd(m.backend, m.addr.ID))
		}
		if !m.live {
			oid := orderID(m.checkout.Lines())
			m.checkout = m.checkout.Placed(oid, "~40 min")
			m.screen = scrConfirm
			m.track = screens.NewTracking("Instamart", m.addr.Line, oid, 0, 0, 0)
		}
		return m, nil
	}
	if m.live && !m.placingOrder {
		m.placingOrder = true
		m.orderErr = ""
		m.cartSyncPending = false // the Sequence syncs final lines; no trailing debounce after placing
		return m, tea.Sequence(m.liveSyncCart(), datasource.PlaceOrderCmd(m.backend, m.snap, m.addr.ID))
	}
	if !m.live {
		oid := orderID(m.checkout.Lines())
		m.checkout = m.checkout.Placed(oid, "~40 min")
		m.screen = scrConfirm
		m.track = screens.NewTracking(m.checkout.Place(), m.addr.Line, oid, 0, 0, 0)
	}
	return m, nil
}

// clampIdx clamps a cursor index into [0, n-1], or 0 when n==0.
func clampIdx(i, n int) int {
	if n == 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	if i < 0 {
		return 0
	}
	return i
}

// buildCheckout assembles the merged checkout screen. The live item list is
// driven by the authoritative m.lines (carries add-on/variant selections);
// liveCart feeds only the bill via billFromLive().
func (m Model) buildCheckout() screens.Checkout {
	return screens.NewCheckout(m.cartHeader(), m.addr, m.cartScreenLines(), m.cartEta()).
		WithBill(m.billFromLive()).
		WithLiveSync(m.live, m.cartSyncErr).
		WithOrderErr(m.orderErr).
		WithMutating(m.cartMutating).
		WithCursor(clampIdx(m.checkout.Cursor(), len(m.cartScreenLines())))
}

// buildIMCheckout assembles the merged checkout screen for the Instamart
// cart. Mirrors buildCheckout: the live item list is driven by the
// authoritative m.imLines; imLiveCart feeds only the bill via imBillFromLive().
func (m Model) buildIMCheckout() screens.Checkout {
	return screens.NewCheckout("Instamart", m.addr, m.imLines, screens.InstamartETA).
		WithBill(m.imBillFromLive()).
		WithLiveSync(m.live, m.imCartSyncErr).
		WithOrderErr(m.imOrderErr).
		WithMutating(m.imCartMutating).
		WithCursor(clampIdx(m.checkout.Cursor(), len(m.imLines)))
}

// openCartCmd opens the merged checkout screen and, in live mode, fetches the
// real Swiggy cart so the bill reflects exactly what Place Order will charge.
func (m *Model) openCartCmd() tea.Cmd {
	m.cartLoaded = false
	m.checkoutVertical = 0 // food
	m.checkout = m.buildCheckout()
	m.screen = scrCheckout
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

// openIMCheckoutCmd opens the merged checkout screen for the Instamart cart
// and, in live mode, fetches the real Instamart cart so the bill reflects
// exactly what Place Order will charge. Mirrors openCartCmd; Instamart's cart
// binds to the ADDRESS (not a restaurant), so there is no restaurant guard.
func (m *Model) openIMCheckoutCmd() tea.Cmd {
	m.checkoutVertical = 1 // instamart
	m.checkout = m.buildIMCheckout()
	m.screen = scrCheckout
	if !m.live {
		return nil
	}
	return datasource.LoadIMCart(m.backend)
}

// imEnterCmd fires the Instamart entry loads: the go-to ("your usuals") list
// always refreshes (cheap, address-scoped), and the account's Instamart cart
// pulls once per session (re-armed on an address change, mirroring the food
// cart pull) so a pre-existing Swiggy-app cart seeds the local lines.
func (m *Model) imEnterCmd() tea.Cmd {
	if !m.live {
		return nil
	}
	m.imQuery = ""
	m.imRailActive = screens.RailHome // lands on Usuals, matching Food's landing-on-Home
	m.imRailFocus = true
	m.imSearchMode = false
	m.imSearchSubmitted = ""
	// The go-to list always refreshes on entry (cheap, address-scoped) even if
	// already loaded this session — re-marking it loaded is a no-op for
	// ensureIMQuery's dedupe on the NEXT revisit.
	m.imLoadedQueries[""] = true
	// Disk-seed the go-to list so entry paints instantly on a relaunch; the live
	// refresh streams over it. "loading…" only when nothing is cached.
	m.imPending = !datasource.SeedIMFromCache(m.snap, m.addr.ID, "")
	m.inst = m.buildInstamart() // paint rail + cached/loading list synchronously; the live go-to list fills in when the load lands
	cmds := []tea.Cmd{datasource.LoadIMProducts(m.backend, m.snap, m.addr.ID, m.imQuery)}
	if !m.imCartPulled {
		m.imCartPulled = true
		cmds = append(cmds, datasource.PullIMCart(m.backend))
	}
	return tea.Batch(cmds...)
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

// imBillFromLive returns Swiggy's real Instamart pricing breakdown for the
// checkout bill. Instamart's handling fee is folded into the taxes&charges
// row (screens.Bill has no separate handling field) so the checkout page
// renders identically to food's — a design constraint, not a data loss:
// imLiveCart.Handling is still available for anyone auditing the raw split.
func (m Model) imBillFromLive() screens.Bill {
	if !m.live || m.imLiveCart.Total == 0 {
		return screens.Bill{}
	}
	return screens.Bill{
		ItemTotal: m.imLiveCart.ItemTotal,
		Delivery:  m.imLiveCart.Delivery,
		Taxes:     m.imLiveCart.Handling + m.imLiveCart.Taxes,
		ToPay:     m.imLiveCart.Total,
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

// imItemsForLines converts the committed Instamart cart lines into
// api.IMCartItems (the sync payload). Mirrors cartItemsForLines; lines
// without a resolved spinId (SwiggyID) are skipped — they can't be
// referenced by Swiggy.
func (m Model) imItemsForLines() []api.IMCartItem {
	items := make([]api.IMCartItem, 0, len(m.imLines))
	for _, l := range m.imLines {
		if l.Item.SwiggyID == "" {
			continue
		}
		items = append(items, api.IMCartItem{SpinID: l.Item.SwiggyID, Quantity: l.Qty})
	}
	return items
}

// imLiveCartCmd syncs the Instamart cart after any local cart mutation: an
// IMUpdateCart when items remain, or a flush when the cart just went empty.
// Mirrors liveCartCmd.
func (m Model) imLiveCartCmd() tea.Cmd {
	if !m.live {
		return nil
	}
	if len(m.imLines) == 0 {
		return datasource.ClearIMCartCmd(m.backend)
	}
	items := m.imItemsForLines()
	if len(items) == 0 {
		return nil
	}
	return datasource.SyncIMCart(m.backend, m.addr.ID, items)
}

func (m Model) liveSyncCart() tea.Cmd {
	if !m.live || len(m.lines) == 0 {
		dbgTUI("liveSyncCart: nil (live=%v lines=%d)", m.live, len(m.lines))
		return nil
	}
	swiggyID := m.cartRestaurantSwiggyID()
	if swiggyID == "" {
		dbgTUI("liveSyncCart: nil (cartRestaurant=%q — no SwiggyID resolved)", m.cartRestaurant)
		return nil
	}
	items := m.cartItemsForLines()
	if len(items) == 0 {
		dbgTUI("liveSyncCart: nil (no items with SwiggyID; lines=%d)", len(m.lines))
		return nil
	}
	dbgTUI("liveSyncCart: SYNC restaurant=%q swiggyRest=%q items=%d", m.cartRestaurant, swiggyID, len(items))
	return datasource.SyncCart(m.backend, m.snap, m.addr.ID, swiggyID, m.cartRestaurant, items)
}

// cartRestaurantSwiggyID resolves the live restaurant id the cart syncs against.
// It prefers the currently-open restaurant (which always has a SwiggyID after a
// fresh add/override, and is reachable even when the place came from a search and
// isn't in any cuisine-chip list), then falls back to the place-list lookup.
// Without the open-restaurant fallback, a cart whose restaurant wasn't in a chip
// query resolved to "" and the sync silently no-oped.
func (m Model) cartRestaurantSwiggyID() string {
	if p := m.rest.PlaceData(); p.Name != "" && p.Name == m.cartRestaurant && p.SwiggyID != "" {
		return p.SwiggyID
	}
	if p, ok := m.repo.Menu(m.cartPlaceID()); ok && p.SwiggyID != "" {
		return p.SwiggyID
	}
	return ""
}

// runAliasCommand executes `:alias set|list|rm …` and returns palette output
// lines. Preset CREATION captures the current food cart; list/rm manage the
// store. Presets are bound to the cart's restaurant + the current address.
func (m *Model) runAliasCommand(rest string) []screens.CmdLine {
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return []screens.CmdLine{{Text: "alias: set <name> | list | rm <name> [n]", Color: theme.Dim}}
	}
	switch fields[0] {
	case "set":
		if len(fields) < 2 {
			return []screens.CmdLine{{Text: "usage: alias set <name>", Color: theme.Fav}}
		}
		if m.screen == scrInstamart || (m.screen == scrCheckout && m.checkoutVertical == 1) {
			return m.imAliasSet(fields[1])
		}
		return m.aliasSet(fields[1])
	case "list", "ls":
		return aliasListLines()
	case "rm", "remove":
		if len(fields) < 2 {
			return []screens.CmdLine{{Text: "usage: alias rm <name> [n]", Color: theme.Fav}}
		}
		idx := -1 // -1 means no explicit index given
		if len(fields) >= 3 {
			if n, err := strconv.Atoi(fields[2]); err == nil && n >= 1 {
				idx = n - 1
			}
		}
		return aliasRmLines(fields[1], idx)
	default:
		return []screens.CmdLine{{Text: "alias: set <name> | list | rm <name> [n]", Color: theme.Dim}}
	}
}

func (m *Model) aliasSet(name string) []screens.CmdLine {
	if localstore.ReservedPresetName(name) {
		return []screens.CmdLine{{Text: fmt.Sprintf("alias: %q is reserved", name), Color: theme.Fav}}
	}
	if len(m.lines) == 0 {
		return []screens.CmdLine{{Text: "alias: cart is empty — add items first", Color: theme.Fav}}
	}
	if m.cartForeign || m.cartRestaurant == "" {
		return []screens.CmdLine{{Text: "alias: open a restaurant and build a cart first", Color: theme.Fav}}
	}
	restID := m.cartRestaurantSwiggyID()
	var plines []localstore.PresetLine
	for _, l := range m.lines {
		if l.Item.SwiggyID == "" {
			continue
		}
		pl := localstore.PresetLine{ItemID: l.Item.SwiggyID, Name: l.Item.Name, Qty: l.Qty}
		for _, s := range l.Selections {
			pl.Sels = append(pl.Sels, localstore.PresetSel{
				GroupID: s.GroupID, ChoiceID: s.ChoiceID, Variant: s.Variant, Absolute: s.Absolute, Name: s.Name,
			})
		}
		plines = append(plines, pl)
	}
	if len(plines) == 0 {
		return []screens.CmdLine{{Text: "alias: no live items to save (mock cart?)", Color: theme.Fav}}
	}
	ps, err := localstore.LoadPresets()
	if err != nil {
		return []screens.CmdLine{{Text: "alias: " + err.Error(), Color: theme.Fav}}
	}
	if err := ps.Add(localstore.Preset{
		Name: name, AddrID: m.addr.ID, AddrLine: m.addr.Line,
		RestaurantID: restID, RestaurantName: m.cartRestaurant,
		Lines: plines, CreatedAt: time.Now().Unix(),
	}); err != nil {
		return []screens.CmdLine{{Text: "alias: " + err.Error(), Color: theme.Fav}}
	}
	if err := localstore.SavePresets(ps); err != nil {
		return []screens.CmdLine{{Text: "alias: " + err.Error(), Color: theme.Fav}}
	}
	return []screens.CmdLine{
		{Text: fmt.Sprintf("saved preset %q — %d item(s) from %s", name, len(plines), m.cartRestaurant), Color: theme.Green},
		{Text: fmt.Sprintf("run it from your shell: console order %s", name), Color: theme.Dim},
	}
}

// imAliasSet saves the current Instamart cart as a named preset. Mirrors
// aliasSet; Instamart presets have no restaurant/selections concept — lines
// carry only the spinId + qty (PresetLine.Sels stays empty).
func (m *Model) imAliasSet(name string) []screens.CmdLine {
	if localstore.ReservedPresetName(name) {
		return []screens.CmdLine{{Text: fmt.Sprintf("alias: %q is reserved", name), Color: theme.Fav}}
	}
	if len(m.imLines) == 0 {
		return []screens.CmdLine{{Text: "alias: instamart cart is empty — add items first", Color: theme.Fav}}
	}
	var plines []localstore.PresetLine
	for _, l := range m.imLines {
		if l.Item.SwiggyID == "" {
			continue
		}
		plines = append(plines, localstore.PresetLine{ItemID: l.Item.SwiggyID, Name: l.Item.Name, Qty: l.Qty})
	}
	if len(plines) == 0 {
		return []screens.CmdLine{{Text: "alias: no live items to save (mock cart?)", Color: theme.Fav}}
	}
	ps, err := localstore.LoadPresets()
	if err != nil {
		return []screens.CmdLine{{Text: "alias: " + err.Error(), Color: theme.Fav}}
	}
	if err := ps.Add(localstore.Preset{
		Name: name, AddrID: m.addr.ID, AddrLine: m.addr.Line,
		RestaurantName: "Instamart", Vertical: "instamart",
		Lines: plines, CreatedAt: time.Now().Unix(),
	}); err != nil {
		return []screens.CmdLine{{Text: "alias: " + err.Error(), Color: theme.Fav}}
	}
	if err := localstore.SavePresets(ps); err != nil {
		return []screens.CmdLine{{Text: "alias: " + err.Error(), Color: theme.Fav}}
	}
	return []screens.CmdLine{
		{Text: fmt.Sprintf("saved preset %q — %d item(s) from Instamart", name, len(plines)), Color: theme.Green},
		{Text: fmt.Sprintf("run it from your shell: console order %s", name), Color: theme.Dim},
	}
}

func aliasListLines() []screens.CmdLine {
	ps, err := localstore.LoadPresets()
	if err != nil {
		return []screens.CmdLine{{Text: "alias: " + err.Error(), Color: theme.Fav}}
	}
	if len(ps.Items) == 0 {
		return []screens.CmdLine{{Text: "no presets yet", Color: theme.Dim}}
	}
	var out []screens.CmdLine
	seen := map[string]bool{}
	for _, p := range ps.Items {
		key := strings.ToLower(p.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		group := ps.ByName(p.Name)
		out = append(out, screens.CmdLine{Text: fmt.Sprintf("%s (%d)", p.Name, len(group)), Color: theme.Gold})
		for i, g := range group {
			out = append(out, screens.CmdLine{Text: fmt.Sprintf("  %d) %s", i+1, g.RestaurantName), Color: theme.Dim})
		}
	}
	return out
}

func aliasRmLines(name string, idx int) []screens.CmdLine {
	ps, err := localstore.LoadPresets()
	if err != nil {
		return []screens.CmdLine{{Text: "alias: " + err.Error(), Color: theme.Fav}}
	}
	matches := ps.ByName(name)
	if len(matches) == 0 {
		return []screens.CmdLine{{Text: fmt.Sprintf("alias: no preset named %q", name), Color: theme.Fav}}
	}
	if idx < 0 {
		// No explicit index given.
		if len(matches) == 1 {
			idx = 0
		} else {
			// Ambiguous: refuse and list them.
			out := []screens.CmdLine{
				{Text: fmt.Sprintf("%d presets named %q — use: alias rm %s <n>", len(matches), name, name), Color: theme.Fav},
			}
			for i, p := range matches {
				out = append(out, screens.CmdLine{Text: fmt.Sprintf("  %d) %s", i+1, p.RestaurantName), Color: theme.Dim})
			}
			return out
		}
	}
	ok, _ := ps.Remove(name, idx)
	if !ok {
		return []screens.CmdLine{{Text: fmt.Sprintf("alias: no preset %q #%d", name, idx+1), Color: theme.Fav}}
	}
	if err := localstore.SavePresets(ps); err != nil {
		return []screens.CmdLine{{Text: "alias: " + err.Error(), Color: theme.Fav}}
	}
	return []screens.CmdLine{{Text: fmt.Sprintf("removed preset %q", name), Color: theme.Green}}
}
