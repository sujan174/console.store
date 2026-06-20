package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/mock"
	"console.store/internal/tui/screens"
)

type screen int

const (
	scrMenu screen = iota
	scrRestaurant
	scrCart
)

type Model struct {
	screen         screen
	menu           screens.Menu
	rest           screens.Restaurant
	cart           screens.Cart
	lines          []screens.CartLine
	cartRestaurant string // name of the restaurant whose items are in the cart
}

func New() Model {
	return Model{
		screen: scrMenu,
		menu:   screens.NewMenu(mock.Restaurants, mock.Addresses[0], 0),
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) cartTotal() int {
	t := 0
	for _, l := range m.lines {
		t += l.Item.Price * l.Qty
	}
	return t
}

// cartHeader returns the restaurant name for the cart header, falling back to
// "your order" if no restaurant name has been recorded yet.
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
				m.rest = screens.NewRestaurant(m.menu.Selected(), m.cartTotal())
				m.screen = scrRestaurant
				return m, nil
			case "c":
				m.cart = screens.NewCart(m.cartHeader(), m.lines)
				m.screen = scrCart
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
					// Record the restaurant name the first time an item is added.
					m.cartRestaurant = m.rest.RestaurantData().Name
				}
				// Update cart totals without resetting the list cursor.
				m.menu = m.menu.WithCartTotal(m.cartTotal())
				m.rest = m.rest.WithCartTotal(m.cartTotal())
				return m, nil
			case "c":
				m.cart = screens.NewCart(m.rest.RestaurantData().Name, m.lines)
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
