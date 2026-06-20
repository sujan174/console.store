package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/catalog"
	"console.store/internal/catalog/mem"
	"console.store/internal/tui/screens"
)

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(110*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
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

	frame int
}

func New() Model {
	repo := mem.New()
	addr := repo.Addresses()[0]
	section := catalog.SectionCoffee
	m := Model{repo: repo, addr: addr, section: section, screen: scrSplash}
	m.splash = screens.NewSplash()
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
	return screens.NewMenu(m.repo.Places(m.addr, m.section), m.addr, m.section, usual, ok, m.cartTotal())
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

func orderID(lines []screens.CartLine) string {
	sum := 0
	for _, l := range lines {
		for _, r := range l.Item.ID + l.Item.Name {
			sum = (sum*31 + int(r)) & 0xffff
		}
		sum = (sum + l.Qty) & 0xffff
	}
	return fmt.Sprintf("CS-%04X", sum)
}

func (m Model) Init() tea.Cmd { return tick() }

// onTick advances time-based screen state; extended by later tasks.
func (m Model) onTick() Model {
	if m.screen == scrSplash {
		if m.bootStep < screens.BootLineCount {
			if m.frame%3 == 0 {
				m.bootStep++
			}
		} else {
			m.bootHold++
			if m.bootHold > 20 { // ~2.2s hold on the logo, then connect
				m.screen = scrMenu
			}
		}
	}
	return m
}

func (m Model) spin() string { return spinFrames[m.frame%len(spinFrames)] }

// blinkOn reports the on-phase of a ~1s cursor blink.
func (m Model) blinkOn() bool { return (m.frame/5)%2 == 0 }

func (m Model) cartTotal() int {
	t := 0
	for _, l := range m.lines {
		t += l.Item.Price * l.Qty
	}
	return t
}

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

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tickMsg); ok {
		m.frame++
		m = m.onTick()
		return m, tick()
	}
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
		if m.screen == scrSplash {
			m.screen = scrMenu
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
				if m.menu.SelectedUsual() {
					if usual, ok := m.repo.Usual(m.addr); ok {
						if p, ok := m.repo.Menu(usual.PlaceID); ok {
							m.lines = []screens.CartLine{{Item: usual.Item, Qty: 1}}
							m.cartRestaurant = p.Name
							m.cart = screens.NewCart(p.Name, m.lines)
							m.screen = scrCart
						}
					}
					return m, nil
				}
				if p, ok := m.menu.Selected(); ok {
					m.rest = screens.NewRestaurant(p, m.cartTotal())
					m.screen = scrRestaurant
				}
				return m, nil
			case "right", "l":
				i := sectionIndex(m.section)
				if i < len(menuTabs)-1 {
					m.section = menuTabs[i+1]
					m.menu = m.buildMenu()
				} else {
					m.inst = screens.NewInstamart(m.repo.InstamartItems(m.addr), m.imCartTotal())
					m.screen = scrInstamart
				}
				return m, nil
			case "left", "h":
				i := sectionIndex(m.section)
				if i > 0 {
					m.section = menuTabs[i-1]
					m.menu = m.buildMenu()
				}
				return m, nil
			case "c":
				m.cart = screens.NewCart(m.cartHeader(), m.lines)
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
			case "esc", "left", "h":
				m.screen = scrMenu
				return m, nil
			case "enter":
				it, ok := m.rest.Selected()
				if !ok {
					return m, nil
				}
				wasEmpty := len(m.lines) == 0
				m.lines = append(m.lines, screens.CartLine{Item: it, Qty: 1})
				if wasEmpty {
					m.cartRestaurant = m.rest.PlaceData().Name
				}
				m.menu = m.menu.WithCartTotal(m.cartTotal())
				m.rest = m.rest.WithCartTotal(m.cartTotal())
				return m, nil
			case "c":
				m.cart = screens.NewCart(m.rest.PlaceData().Name, m.lines)
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
					m.checkout = screens.NewCheckout(m.cartHeader(), m.addr, m.lines)
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
			m.menu = m.menu.WithCartTotal(m.cartTotal())
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
				m.checkout = m.checkout.Placed(orderID(m.checkout.Lines()))
				m.screen = scrConfirm
				return m, nil
			}
		case scrConfirm:
			if k.String() == "esc" || k.String() == "enter" {
				m.lines = nil
				m.imLines = nil
				m.cartRestaurant = ""
				m.menu = m.buildMenu()
				m.screen = scrMenu
				return m, nil
			}
		case scrInstamart:
			switch k.String() {
			case "esc":
				m.screen = scrMenu
				return m, nil
			case "left", "h":
				m.section = catalog.SectionSnacks
				m.menu = m.buildMenu()
				m.screen = scrMenu
				return m, nil
			case "enter":
				it, ok := m.inst.Selected()
				if !ok {
					return m, nil
				}
				m.imLines = append(m.imLines, screens.CartLine{Item: it, Qty: 1})
				m.inst = m.inst.WithCartTotal(m.imCartTotal())
				return m, nil
			case "c":
				m.imCart = screens.NewCart("Instamart", m.imLines)
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
					m.checkout = screens.NewCheckout("Instamart", m.addr, m.imLines)
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

func (m Model) View() string {
	switch m.screen {
	case scrSplash:
		return m.splash.WithBoot(m.bootStep, m.spin(), screens.Taglines[(m.frame/15)%len(screens.Taglines)]).View()
	case scrRestaurant:
		return m.rest.View()
	case scrCart:
		return m.cart.View()
	case scrAddress:
		return m.addrScreen.View()
	case scrCheckout, scrConfirm:
		return m.checkout.View()
	case scrInstamart:
		return m.inst.View()
	case scrImCart:
		return m.imCart.View()
	default:
		return m.menu.View()
	}
}
