// Package datasource wires the broker (via internal/broker/api) into the TUI as
// async bubbletea Cmds that fill a catalog/swiggy.Snapshot. The TUI reads the
// Snapshot synchronously through a swiggy.Repository; these Cmds are the only
// thing that performs broker I/O. The TUI never imports swiggy/store/auth.
package datasource

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/broker/api"
	"console.store/internal/catalog"
	swiggysnap "console.store/internal/catalog/swiggy"
)

// ErrNeedsAuth signals the account has no usable token; the Model shows the
// authorize gate. Backends should return it (or wrap it) on a missing-token
// error from the broker.
var ErrNeedsAuth = errors.New("datasource: account not authorized")

// Backend abstracts the data source for async load Cmds. The broker-backed
// BrokerBackend implements it for live use; tests use a fake.
type Backend interface {
	Addresses() ([]api.Address, error)
	Places(addressID string, section catalog.Section) ([]api.Restaurant, error)
	PlacesQuery(addressID, query string) ([]api.Restaurant, error)
	Menu(addressID, restaurantID string) (api.Menu, error)
	ItemOptions(addressID, restaurantID, itemName, menuItemID string) ([]api.OptionGroup, error)
	UpdateCart(addressID, restaurantID, restaurantName string, items []api.CartItem) (api.Cart, error)
	ClearCart() error
	PlaceOrder(addressID string) (api.Order, error)
}

type (
	AddressesLoadedMsg struct{ Err error }
	PlacesLoadedMsg    struct {
		Section catalog.Section
		Query   string
		Err     error
	}
	MenuLoadedMsg struct {
		PlaceID string
		Err     error
	}
	ItemOptionsLoadedMsg struct {
		ItemID string
		Groups []catalog.OptionGroup
		Err    error
	}
	CartSyncedMsg struct {
		Cart api.Cart
		Err  error
	}
	OrderPlacedMsg struct {
		Order api.Order
		Err   error
	}
)

func LoadAddresses(b Backend, snap *swiggysnap.Snapshot) tea.Cmd {
	return func() tea.Msg {
		got, err := b.Addresses()
		if err != nil {
			return AddressesLoadedMsg{Err: err}
		}
		snap.SetAddresses(toAddresses(got))
		return AddressesLoadedMsg{}
	}
}

func LoadPlaces(b Backend, snap *swiggysnap.Snapshot, addressID string, section catalog.Section) tea.Cmd {
	return func() tea.Msg {
		got, err := b.Places(addressID, section)
		if err != nil {
			return PlacesLoadedMsg{Section: section, Err: err}
		}
		snap.SetPlaces(addressID, string(section), toPlaces(got, section))
		return PlacesLoadedMsg{Section: section}
	}
}

// LoadPlacesQuery runs a free/chip restaurant search and caches it under the
// query key.
func LoadPlacesQuery(b Backend, snap *swiggysnap.Snapshot, addressID, query string) tea.Cmd {
	return func() tea.Msg {
		got, err := b.PlacesQuery(addressID, query)
		if err != nil {
			return PlacesLoadedMsg{Query: query, Err: err}
		}
		snap.SetPlaces(addressID, query, toPlaces(got, catalog.SectionCoffee))
		return PlacesLoadedMsg{Query: query}
	}
}

func LoadMenu(b Backend, snap *swiggysnap.Snapshot, addressID, restaurantID string) tea.Cmd {
	return func() tea.Msg {
		got, err := b.Menu(addressID, restaurantID)
		if err != nil {
			return MenuLoadedMsg{PlaceID: restaurantID, Err: err}
		}
		snap.SetMenu(toMenuPlace(got))
		return MenuLoadedMsg{PlaceID: restaurantID}
	}
}

// LoadItemOptions fetches an item's customization groups (variants/addons) so
// the TUI can open the customize sheet before adding it to the cart.
func LoadItemOptions(b Backend, addressID, restaurantID, itemName, menuItemID string) tea.Cmd {
	return func() tea.Msg {
		groups, err := b.ItemOptions(addressID, restaurantID, itemName, menuItemID)
		if err != nil {
			return ItemOptionsLoadedMsg{ItemID: menuItemID, Err: err}
		}
		return ItemOptionsLoadedMsg{ItemID: menuItemID, Groups: toOptionGroups(groups)}
	}
}

// SyncCart calls UpdateCart on the backend with the current cart contents and
// returns the resulting cart (with Swiggy's real bill breakdown) in the Msg so
// the checkout can show an accurate split. Errors are non-fatal: the TUI shows
// them in the status bar and continues.
func SyncCart(b Backend, snap *swiggysnap.Snapshot, addressID, restaurantID, restaurantName string, items []api.CartItem) tea.Cmd {
	return func() tea.Msg {
		cart, err := b.UpdateCart(addressID, restaurantID, restaurantName, items)
		return CartSyncedMsg{Cart: cart, Err: err}
	}
}

// ClearCartCmd empties the Swiggy cart (flush_food_cart). Used when the TUI cart
// goes empty — UpdateCart can't express an empty cart (it needs a restaurant id).
func ClearCartCmd(b Backend) tea.Cmd {
	return func() tea.Msg {
		return CartSyncedMsg{Err: b.ClearCart()}
	}
}

// PlaceOrderCmd submits the order through the broker. The TUI must have already
// synced the cart via SyncCart before calling this. On success the broker returns
// the placed order; on failure the TUI shows the error and stays on scrCheckout.
func PlaceOrderCmd(b Backend, snap *swiggysnap.Snapshot, addressID string) tea.Cmd {
	return func() tea.Msg {
		order, err := b.PlaceOrder(addressID)
		return OrderPlacedMsg{Order: order, Err: err}
	}
}
