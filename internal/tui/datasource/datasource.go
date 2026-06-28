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
	SearchOrganic(addressID, query string) ([]api.Restaurant, string, error)
	Usuals(addressID string) ([]api.Restaurant, error)
	Menu(addressID, restaurantID string) (api.Menu, error)
	ItemOptions(addressID, restaurantID, itemName, menuItemID string) ([]api.OptionGroup, error)
	UpdateCart(addressID, restaurantID, restaurantName string, items []api.CartItem) (api.Cart, error)
	GetCart(addressID, restaurantName string) (api.Cart, error)
	ClearCart() error
	PlaceOrder(addressID string) (api.Order, error)
	TrackOrder(orderID string) (api.Tracking, error)
	ActiveOrders(addressID string) ([]api.Order, error)
	Logout() error
}

type (
	AddressesLoadedMsg struct{ Err error }
	PlacesLoadedMsg    struct {
		Section   catalog.Section
		Query     string
		Corrected string // non-empty when search spell-corrected to a different query
		Err       error
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
	// CartLoadedMsg carries the live Swiggy cart fetched on cart-screen entry —
	// the source of truth for the displayed lines + bill.
	CartLoadedMsg struct {
		Cart api.Cart
		Err  error
	}
	// CartPulledMsg carries the account cart fetched once at launch (to detect a
	// pre-existing foreign cart). Distinct from CartLoadedMsg so launch-time
	// errors stay silent and only this path seeds the local cart.
	CartPulledMsg struct {
		Cart api.Cart
		Err  error
	}
	OrderPlacedMsg struct {
		Order api.Order
		Err   error
	}
	// UsualsLoadedMsg signals the account's most-ordered restaurants were
	// fetched into the snapshot (under UsualsKey). Err non-nil on failure;
	// empty history is NOT an error (the section just renders nothing).
	UsualsLoadedMsg struct{ Err error }
	// LoggedOutMsg reports the result of disconnecting the Swiggy account
	// (purging the stored token). Err is non-nil if the purge failed.
	LoggedOutMsg struct{ Err error }
	// TrackingPolledMsg carries the live status + ETA for an order, or an error
	// if the poll failed.
	TrackingPolledMsg struct {
		Tracking api.Tracking
		Err      error
	}
	// ActiveOrdersLoadedMsg carries the account's currently-active orders, or an
	// error if the fetch failed.
	ActiveOrdersLoadedMsg struct {
		Orders []api.Order
		Err    error
	}
)

// Logout purges the stored Swiggy token (disconnects the account).
func Logout(b Backend) tea.Cmd {
	return func() tea.Msg { return LoggedOutMsg{Err: b.Logout()} }
}

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

// SearchKey is the snapshot key prefix for global-search results. Search is
// cached separately from the cuisine categories so an ad-free search for
// "pizza" never overwrites (or is overwritten by) the ads-allowed "pizza"
// category, even though both share the raw query string.
func SearchKey(query string) string { return "__search__:" + query }

// LoadSearch runs an ad-free global restaurant search (SearchOrganic) and caches
// it under SearchKey(query). The returned msg carries the RAW query so the app's
// searchPending gate matches what the user submitted.
func LoadSearch(b Backend, snap *swiggysnap.Snapshot, addressID, query string) tea.Cmd {
	return func() tea.Msg {
		got, effective, err := b.SearchOrganic(addressID, query)
		if err != nil {
			return PlacesLoadedMsg{Query: query, Err: err}
		}
		snap.SetPlaces(addressID, SearchKey(query), toPlaces(got, catalog.SectionCoffee))
		corrected := ""
		if effective != "" && effective != query {
			corrected = effective // a spelling correction matched
		}
		return PlacesLoadedMsg{Query: query, Corrected: corrected}
	}
}

// UsualsKey is the reserved snapshot query key under which the account's
// most-ordered restaurants are cached (so the Repository reads them via
// PlacesByQuery without a new snapshot field).
const UsualsKey = "__usuals__"

// LoadUsuals fetches the account's most-ordered restaurants and caches them
// under UsualsKey. Errors are non-fatal (the Home view drops the section).
func LoadUsuals(b Backend, snap *swiggysnap.Snapshot, addressID string) tea.Cmd {
	return func() tea.Msg {
		got, err := b.Usuals(addressID)
		if err == nil {
			snap.SetPlaces(addressID, UsualsKey, toPlaces(got, catalog.SectionFood))
		}
		return UsualsLoadedMsg{Err: err}
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

// LoadCart fetches the live Swiggy cart (get_food_cart) so the cart/checkout
// screens render Swiggy's real items + pricing rather than the local in-memory
// approximation. restaurantName scopes the fetch to the current restaurant.
func LoadCart(b Backend, addressID, restaurantName string) tea.Cmd {
	return func() tea.Msg {
		cart, err := b.GetCart(addressID, restaurantName)
		return CartLoadedMsg{Cart: cart, Err: err}
	}
}

// PullCart fetches the account cart once at launch (no restaurant scope) so the
// TUI can detect a cart already built on the Swiggy app/website and seed it
// locally — making a later cross-restaurant add raise the keep/override modal.
func PullCart(b Backend, addressID string) tea.Cmd {
	return func() tea.Msg {
		cart, err := b.GetCart(addressID, "")
		return CartPulledMsg{Cart: cart, Err: err}
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

// PollTrackingCmd fetches the live status + ETA for an order.
func PollTrackingCmd(b Backend, orderID string) tea.Cmd {
	return func() tea.Msg {
		t, err := b.TrackOrder(orderID)
		return TrackingPolledMsg{Tracking: t, Err: err}
	}
}

// LoadActiveOrdersCmd fetches the account's currently-active orders (the real
// "is an order live?" signal for the splash).
func LoadActiveOrdersCmd(b Backend, addressID string) tea.Cmd {
	return func() tea.Msg {
		os, err := b.ActiveOrders(addressID)
		return ActiveOrdersLoadedMsg{Orders: os, Err: err}
	}
}
