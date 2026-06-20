package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/catalog"
	"console.store/internal/catalog/mem"
	"console.store/internal/tui/screens"
)

type screen int

const (
	scrMenu screen = iota
	scrRestaurant
	scrCart
	scrAddress
	scrCheckout
	scrConfirm
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
}

func New() Model {
	repo := mem.New()
	addr := repo.Addresses()[0]
	section := catalog.SectionCoffee
	m := Model{repo: repo, addr: addr, section: section, screen: scrMenu}
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

func (m Model) Init() tea.Cmd { return nil }

func (m Model) cartTotal() int {
	t := 0
	for _, l := range m.lines {
		t += l.Item.Price * l.Qty
	}
	return t
}

func (m Model) cartHeader() string {
	if m.cartRestaurant != "" {
		return m.cartRestaurant
	}
	return "your order"
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
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
					m.rest = screens.NewRestaurant(p, m.cartTotal())
					m.screen = scrRestaurant
				}
				return m, nil
			case "c":
				m.cart = screens.NewCart(m.cartHeader(), m.lines)
				m.screen = scrCart
				return m, nil
			case "1", "2", "3":
				idx := map[string]int{"1": 0, "2": 1, "3": 2}[k.String()]
				m.section = catalog.MenuSections[idx]
				m.menu = m.buildMenu()
				return m, nil
			case "u":
				if usual, ok := m.repo.Usual(m.addr); ok {
					if p, ok := m.repo.Menu(usual.PlaceID); ok {
						m.lines = []screens.CartLine{{Item: usual.Item, Qty: 1}}
						m.cartRestaurant = p.Name
						m.cart = screens.NewCart(p.Name, m.lines)
						m.screen = scrCart
					}
				}
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
			case "enter":
				wasEmpty := len(m.lines) == 0
				m.lines = append(m.lines, screens.CartLine{Item: m.rest.Selected(), Qty: 1})
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
			case "+", "=":
				m.cart = m.cart.Inc()
			case "-":
				m.cart = m.cart.Dec()
			case "x":
				m.cart = m.cart.Remove()
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
				m.checkout = m.checkout.Placed(orderID(m.lines))
				m.screen = scrConfirm
				return m, nil
			}
		case scrConfirm:
			if k.String() == "esc" || k.String() == "enter" {
				m.lines = nil
				m.cartRestaurant = ""
				m.menu = m.buildMenu()
				m.screen = scrMenu
				return m, nil
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	switch m.screen {
	case scrRestaurant:
		return m.rest.View()
	case scrCart:
		return m.cart.View()
	case scrAddress:
		return m.addrScreen.View()
	case scrCheckout, scrConfirm:
		return m.checkout.View()
	default:
		return m.menu.View()
	}
}
