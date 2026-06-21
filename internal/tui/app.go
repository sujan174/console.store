package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"console.store/internal/catalog"
	"console.store/internal/catalog/mem"
	"console.store/internal/tui/components"
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

	inst    screens.Instamart
	imLines []screens.CartLine
	imCart  screens.Cart

	splash   screens.Splash
	bootStep int
	bootHold int

	track     screens.Tracking
	trackStep int
	trackTick int

	cmdOpen bool
	cmd     screens.CmdBar

	frame     int
	w, h      int // terminal size from WindowSizeMsg
	caps      render.Caps
	statsFunc func() (online, orders int)
}

func New(caps render.Caps, statsFunc func() (online, orders int)) Model {
	repo := mem.New()
	addr := repo.Addresses()[0]
	section := catalog.SectionCoffee
	m := Model{repo: repo, addr: addr, section: section, screen: scrSplash, caps: caps, statsFunc: statsFunc}
	m.splash = screens.NewSplash().WithCaps(caps)
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
		WithStats(m.statsFunc).
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
func appendOrInc(lines []screens.CartLine, item catalog.Item) []screens.CartLine {
	for i := range lines {
		if lines[i].Item.ID == item.ID {
			lines[i].Qty++
			return lines
		}
	}
	return append(lines, screens.CartLine{Item: item, Qty: 1})
}

// decItem decrements the qty of item id in lines, removing the line at qty 0.
func decItem(lines []screens.CartLine, id string) []screens.CartLine {
	for i := range lines {
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

func (m Model) Init() tea.Cmd { return tick() }

// onTick advances time-based screen state; extended by later tasks.
func (m Model) onTick() Model {
	if m.screen == scrSplash {
		if m.bootStep < screens.BootLineCount {
			if m.frame%6 == 0 {
				m.bootStep++
			}
		} else {
			m.bootHold++
			if m.bootHold > 37 { // ~2.2s hold on the logo, then connect
				m.screen = scrMenu
			}
		}
	}
	if m.screen == scrTracking {
		m.trackTick++
		if m.trackTick%70 == 0 && m.trackStep < 3 {
			m.trackStep++
		}
	}
	return m
}

func (m Model) spin() string { return spinFrames[m.frame%len(spinFrames)] }

// blinkOn reports the on-phase of a ~1s cursor blink.
func (m Model) blinkOn() bool { return (m.frame/9)%2 == 0 }

func (m Model) cartTotal() int {
	t := 0
	for _, l := range m.lines {
		t += l.Item.Price * l.Qty
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

// cartRestaurantServes reports whether the cart's restaurant is serviceable at addr.
func (m Model) cartRestaurantServes(addr catalog.Address) bool {
	if m.cartRestaurant == "" {
		return true
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
		t += l.Item.Price * l.Qty
	}
	return t
}

// InstamartMin is the Instamart minimum order value (₹).
const InstamartMin = 99

func (m Model) cartHeader() string {
	if m.cartRestaurant != "" {
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
				case "instamart":
					m.cmdOpen = false
					m.inst = screens.NewInstamart(m.repo.InstamartItems(m.addr), m.imQtyMap(), m.imCartChip())
					m.screen = scrInstamart
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

		switch k.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
		if m.screen == scrSplash {
			m.screen = scrMenu
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
					wasEmpty := len(m.lines) == 0
					m.lines = appendOrInc(m.lines, usual.Item)
					if wasEmpty {
						if p, ok := m.repo.Menu(usual.PlaceID); ok {
							m.cartRestaurant = p.Name
						}
					}
					m.menu = m.menu.WithCartChip(m.cartChip())
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
				wasEmpty := len(m.lines) == 0
				m.lines = appendOrInc(m.lines, it)
				if wasEmpty {
					m.cartRestaurant = m.rest.PlaceData().Name
				}
				m.menu = m.menu.WithCartChip(m.cartChip())
				ci := m.rest.CursorIndex()
				m.rest = screens.NewRestaurant(m.rest.PlaceData(), m.qtyMap(), m.cartChip()).WithAddr(m.addr).WithCursor(ci)
				return m, nil
			case "left", "h":
				it, ok := m.rest.Selected()
				if !ok {
					return m, nil
				}
				m.lines = decItem(m.lines, it.ID)
				if len(m.lines) == 0 {
					m.cartRestaurant = ""
				}
				m.menu = m.menu.WithCartChip(m.cartChip())
				ci := m.rest.CursorIndex()
				m.rest = screens.NewRestaurant(m.rest.PlaceData(), m.qtyMap(), m.cartChip()).WithAddr(m.addr).WithCursor(ci)
				return m, nil
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
				m.checkout = m.checkout.Placed(orderID(m.checkout.Lines()), "~40 min")
				m.screen = scrConfirm
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
				m.menu = m.buildMenu()
				m.screen = scrMenu
				return m, nil
			}
		case scrTracking:
			if k.String() == "esc" {
				m.lines = nil
				m.imLines = nil
				m.cartRestaurant = ""
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
				m.imLines = appendOrInc(m.imLines, it)
				ci := m.inst.CursorIndex()
				m.inst = screens.NewInstamart(m.repo.InstamartItems(m.addr), m.imQtyMap(), m.imCartChip()).WithCursor(ci)
				return m, nil
			case "left", "h":
				it, ok := m.inst.Selected()
				if !ok {
					return m, nil
				}
				m.imLines = decItem(m.imLines, it.ID)
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
var statusHints = []string{"type : for commands", "247 devs online", "DEVFRIDAY −₹50", "ssh console.store"}

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
	// Splash is centered in the viewport (design lines 196-228). We render on
	// the terminal's own background — wrapping the frame in a lipgloss
	// Background tears on inner colour resets (banding), and a dark terminal
	// already provides the #15161f-ish canvas.
	if m.screen == scrSplash {
		sp := m.splash.WithBoot(m.bootStep, m.spin(), screens.Taglines[(m.frame/27)%len(screens.Taglines)]).View()
		if m.w == 0 || m.h == 0 {
			return sp
		}
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, sp)
	}

	var body string
	switch m.screen {
	case scrRestaurant:
		body = m.rest.WithMaxRows(m.listRows(14)).View()
	case scrCart:
		body = m.cart.View()
	case scrAddress:
		body = m.addrScreen.View()
	case scrCheckout, scrConfirm:
		body = m.checkout.View()
	case scrTracking:
		body = m.track.View(m.trackStep, m.spin())
	case scrInstamart:
		body = m.inst.WithMaxRows(m.listRows(11)).View()
	case scrImCart:
		body = m.imCart.View()
	default: // scrMenu
		body = m.menu.WithMaxRows(m.listRows(13)).View()
	}

	// The footer — the screen's hint line + optional command palette + status
	// bar — is pinned to the bottom. The hint is the screen's last rendered
	// line; lift it out so it sits WITH the status bar instead of floating
	// after the content with a large void between them.
	content, hint := splitHint(body)

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
