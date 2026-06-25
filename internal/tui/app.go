package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"console.store/internal/broker/api"
	"console.store/internal/catalog"
	"console.store/internal/catalog/mem"
	swiggysnap "console.store/internal/catalog/swiggy"
	"console.store/internal/tui/components"
	"console.store/internal/tui/datasource"
	"console.store/internal/tui/render"
	"console.store/internal/tui/screens"
)

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

	// conflict modal: shown when adding an item from a restaurant other than
	// the one the cart holds (Swiggy allows one restaurant per cart).
	conflictOpen   bool
	conflict       screens.CartConflict
	conflictSel    int             // focused button: 0 = start new, 1 = keep current
	pendingItem    catalog.Item    // item awaiting the start-new-cart confirmation
	pendingAddOns  []catalog.AddOn // its chosen add-ons
	pendingRest    string          // its restaurant name
	pendingSection catalog.Section

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
	needsAuth    bool // set when a load returns datasource.ErrNeedsAuth
	seeded       bool // true when catalog/swiggy.Snapshot was pre-seeded from config; skips live init loads

	placingOrder bool   // true while PlaceOrderCmd is in-flight; blocks double-fire
	cartSyncErr  string // last cart-sync error; shown in status bar (non-fatal)
	orderErr     string // last order-placement error; shown in status bar
}

func New(caps render.Caps, opts ...Option) Model {
	repo := mem.New()
	section := catalog.SectionCoffee
	m := Model{repo: repo, section: section, screen: scrSplash, caps: caps, lastEscFrame: -escDoubleWindow - 1}
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
	m.splash = screens.NewSplash().WithCaps(caps)
	m.splashPhrase = screens.RandomPhrase("")
	m.menu = m.buildMenu()
	return m
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
	return screens.NewMenu(m.repo.Places(m.addr, m.section), m.addr, m.section, usual, ok, m.cartChip()).
		WithCounts(counts)
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

// appendOrInc adds item to lines, incrementing the qty if it is already present.
func appendOrInc(lines []screens.CartLine, item catalog.Item, addons []catalog.AddOn) []screens.CartLine {
	key := screens.LineKey(item, addons)
	for i := range lines {
		if lines[i].Key() == key {
			lines[i].Qty++
			return lines
		}
	}
	return append(lines, screens.CartLine{Item: item, Qty: 1, AddOns: addons})
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
func (m Model) startNewCart(item catalog.Item, addons []catalog.AddOn, rest string, section catalog.Section) Model {
	m.lines = []screens.CartLine{{Item: item, Qty: 1, AddOns: addons}}
	m.cartRestaurant = rest
	m.cartSection = section
	return m
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
	return m.commitAdd(item, nil, rest, section)
}

// commitAdd adds item (with its chosen add-ons) from rest (section) to the cart,
// raising the cart-conflict modal first when the cart belongs to an incompatible
// owner (different restaurant outside the snacks section).
func (m Model) commitAdd(item catalog.Item, addons []catalog.AddOn, rest string, section catalog.Section) Model {
	if m.conflictsWithCart(rest, section) {
		m.pendingItem = item
		m.pendingAddOns = addons
		m.pendingRest = rest
		m.pendingSection = section
		m.conflict = screens.NewCartConflict(m.cartHeader(), rest, item.Name)
		m.conflictSel = 1
		m.conflictOpen = true
		return m
	}
	wasEmpty := len(m.lines) == 0
	m.lines = appendOrInc(m.lines, item, addons)
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
		m.rest = screens.NewRestaurant(m.rest.PlaceData(), m.qtyMap(), m.cartChip()).
			WithAddr(m.addr).WithCursor(ci).WithInfo(info)
	}
	return m
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
	if c := m.liveInitCmds(); c != nil {
		return tea.Batch(tick(), c)
	}
	return tick()
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

// cartPlaceID finds the id of the cart's restaurant by name across sections.
func (m Model) cartPlaceID() string {
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
		return m, tick()
	}
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.w, m.h = ws.Width, ws.Height
		components.SetFrameWidth(m.w)
		return m, nil
	}
	switch dm := msg.(type) {
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
			return m, datasource.LoadPlaces(m.backend, m.snap, m.addr.ID, m.section)
		}
		return m, nil
	case datasource.PlacesLoadedMsg:
		if errIsNeedsAuth(dm.Err) {
			m.needsAuth = true
			return m, nil
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
				ci := m.rest.CursorIndex()
				info := m.rest.InfoOpen()
				m.rest = screens.NewRestaurant(p, m.qtyMap(), m.cartChip()).
					WithAddr(m.addr).WithCursor(ci).WithInfo(info)
			}
		}
		return m, nil
	case datasource.CartSyncedMsg:
		if dm.Err != nil {
			m.cartSyncErr = "cart sync: " + dm.Err.Error()
		} else {
			m.cartSyncErr = ""
		}
		return m, nil
	case datasource.OrderPlacedMsg:
		m.placingOrder = false
		if dm.Err != nil {
			m.orderErr = "order failed: " + dm.Err.Error()
			return m, nil
		}
		m.orderErr = ""
		m.checkout = m.checkout.Placed(dm.Order.ID, "~40 min")
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
					m = m.startNewCart(m.pendingItem, m.pendingAddOns, m.pendingRest, m.pendingSection)
					m = m.refreshAfterAdd()
					syncCmd = m.liveSyncCart()
				}
				m.conflictOpen = false
				return m, syncCmd
			case "esc":
				m.conflictOpen = false
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
				item := m.customize.Item()
				addons := m.customize.SelectedAddOns()
				m.customizeOpen = false
				m = m.commitAdd(item, addons, m.pendingRest, m.pendingSection)
				if !m.conflictOpen { // committed directly (no restaurant clash)
					m = m.refreshAfterAdd()
					return m, m.liveSyncCart()
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
			if m.screen == scrMenu && !m.menu.Searching() {
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
			if m.menu.Searching() {
				nm, cmd := m.menu.Update(msg)
				m.menu = nm.(screens.Menu)
				return m, cmd
			}
			switch k.String() {
			case "enter":
				if p, ok := m.menu.Selected(); ok {
					m.rest = screens.NewRestaurant(p, m.qtyMap(), m.cartChip()).WithAddr(m.addr)
					m.screen = scrRestaurant
					if m.live && p.SwiggyID != "" {
						return m, datasource.LoadMenu(m.backend, m.snap, m.addr.ID, p.SwiggyID)
					}
				}
				return m, nil
			case "right", "l":
				// non-cyclable: clamp at the last tab (snacks)
				if i := sectionIndex(m.section); i < len(menuTabs)-1 {
					m.section = menuTabs[i+1]
					m.menu = m.buildMenu()
				}
				return m, nil
			case "left", "h":
				// non-cyclable: clamp at the first tab (coffee)
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
				m.cart = screens.NewCart(m.cartHeader(), m.lines).WithEta(m.cartEta())
				m.screen = scrCart
				return m, nil
			case "a":
				m.addrScreen = screens.NewAddress(m.repo.Addresses(), m.addr.ID)
				m.screen = scrAddress
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
			switch k.String() {
			case "esc":
				m.screen = scrMenu
				return m, nil
			case "enter", "right", "l":
				it, ok := m.rest.Selected()
				if !ok {
					return m, nil
				}
				m = m.beginAdd(it, m.rest.PlaceData().Name, m.rest.PlaceData().Section)
				if m.customizeOpen || m.conflictOpen {
					return m, nil // a modal will finish the add
				}
				m = m.refreshAfterAdd()
				return m, m.liveSyncCart()
			case "left", "h":
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
				return m, m.liveSyncCart()
			case "c":
				m.cart = screens.NewCart(m.rest.PlaceData().Name, m.lines).WithEta(m.cartEta())
				m.screen = scrCart
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
				if len(m.lines) > 0 {
					m.checkout = screens.NewCheckout(m.cartHeader(), m.addr, m.lines, m.cartEta())
					m.screen = scrCheckout
					return m, nil
				}
			case "j", "down":
				m.cart = m.cart.Down()
			case "k", "up":
				m.cart = m.cart.Up()
			case "right", "l":
				m.cart = m.cart.Right()
			case "left", "h":
				m.cart = m.cart.Left()
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
			return m, nil
		case scrAddress:
			switch k.String() {
			case "esc":
				m.screen = scrMenu
				return m, nil
			case "enter":
				m.addr = m.addrScreen.Selected()
				if !m.cartRestaurantServes(m.addr) {
					m.lines = nil
					m.cartRestaurant = ""
					m.cartSection = ""
				}
				m.menu = m.buildMenu()
				m.screen = scrMenu
				return m, nil
			default:
				na, cmd := m.addrScreen.Update(msg)
				m.addrScreen = na.(screens.Address)
				return m, cmd
			}
		case scrCheckout:
			switch k.String() {
			case "esc":
				m.screen = scrCart
				return m, nil
			case "enter":
				if m.live && !m.placingOrder {
					m.placingOrder = true
					m.orderErr = ""
					return m, datasource.PlaceOrderCmd(m.backend, m.snap, m.addr.ID)
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
			case "esc":
				m.screen = scrMenu
				return m, nil
			case "enter", "right", "l":
				it, ok := m.inst.Selected()
				if !ok {
					return m, nil
				}
				m.imLines = appendOrInc(m.imLines, it, nil)
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
			"  1. Open this link on your phone and log in to Swiggy:\n\n" +
			"     " + m.authorizeURL + "\n\n" +
			"  2. Approve access, then press  r  to retry.\n"
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
	if m.customizeOpen {
		dialog := m.customize.View()
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

	var body string
	infoPanel := "" // restaurant detail panel ('i'); pinned above the hints below
	switch m.screen {
	case scrRestaurant:
		chrome := 14 + screens.BrandHeaderLines
		if m.rest.InfoOpen() {
			infoPanel = m.rest.InfoView(components.ContentWidth())
			chrome += lipgloss.Height(infoPanel) + 1
		}
		body = m.rest.WithMaxRows(m.listRows(chrome)).View()
	case scrCart:
		body = m.cart.View()
	case scrAddress:
		body = m.addrScreen.View()
	case scrCheckout, scrConfirm:
		body = m.checkout.WithPlacing(m.placingOrder).View(m.frame)
	case scrTracking:
		body = m.track.View(m.trackStep, m.frame, m.spin())
	case scrInstamart:
		body = m.inst.WithMaxRows(m.listRows(11 + screens.BrandHeaderLines)).View()
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
	if infoPanel != "" {
		footer += infoPanel + "\n"
	}
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
func (m Model) liveSyncCart() tea.Cmd {
	if !m.live || len(m.lines) == 0 {
		return nil
	}
	p, ok := m.repo.Menu(m.cartPlaceID())
	if !ok || p.SwiggyID == "" {
		return nil
	}
	items := make([]api.CartItem, 0, len(m.lines))
	for _, l := range m.lines {
		if l.Item.SwiggyID != "" {
			items = append(items, api.CartItem{ItemID: l.Item.SwiggyID, Quantity: l.Qty})
		}
	}
	if len(items) == 0 {
		return nil
	}
	return datasource.SyncCart(m.backend, m.snap, m.addr.ID, p.SwiggyID, m.cartRestaurant, items)
}
