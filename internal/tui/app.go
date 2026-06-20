package tui

import (
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
)

type Model struct {
	repo    catalog.Repository
	addr    catalog.Address
	section catalog.Section

	screen         screen
	menu           screens.Menu
	rest           screens.Restaurant
	cart           screens.Cart
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
			default:
				nm, cmd := m.menu.Update(msg)
				m.menu = nm.(screens.Menu)
				return m, cmd
			}
		case scrRestaurant:
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
			if k.String() == "esc" {
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
	default:
		return m.menu.View()
	}
}
