// Package datasource wires the broker (via internal/broker/api) into the TUI as
// async bubbletea Cmds that fill a catalog/swiggy.Snapshot. The TUI reads the
// Snapshot synchronously through a swiggy.Repository; these Cmds are the only
// thing that performs broker I/O. The TUI never imports swiggy/store/auth.
package datasource

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"

	"consolestore/internal/broker/api"
	"consolestore/internal/catalog"
	swiggysnap "consolestore/internal/catalog/swiggy"
	"consolestore/internal/localstore"
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
	// MenuPage fetches one category page of a menu (1-indexed); more reports
	// whether another page may exist. The streaming counterpart of Menu.
	MenuPage(addressID, restaurantID string, page int) (api.Menu, bool, error)
	// PlacesQueryPage fetches one page of a category/home restaurant search;
	// nextOffset feeds the following call, more is false when results ran out.
	PlacesQueryPage(addressID, query string, offset int) ([]api.Restaurant, int, bool, error)
	ItemOptions(addressID, restaurantID, itemName, menuItemID string) ([]api.OptionGroup, error)
	UpdateCart(addressID, restaurantID, restaurantName string, items []api.CartItem) (api.Cart, error)
	GetCart(addressID, restaurantName string) (api.Cart, error)
	ClearCart() error
	PlaceOrder(addressID string) (api.Order, error)
	TrackOrder(orderID string) (api.Tracking, error)
	ActiveOrders(addressID string) ([]api.Order, error)
	Logout() error

	// Instamart (grocery) vertical — a separate address-bound cart, keyed by
	// SKU-level spinIds instead of menu_item_ids. See catalog.SectionInstamart.
	IMSearch(addressID, query string) ([]api.IMProduct, error)
	IMGoTo(addressID string) ([]api.IMProduct, error)
	IMGetCart() (api.IMCart, error)
	IMUpdateCart(addressID string, items []api.IMCartItem) (api.IMCart, error)
	IMClearCart() error
	IMPlaceOrder(addressID string) (api.Order, error)
	IMOrders(activeOnly bool) ([]api.IMOrder, error)
	IMTrack(orderID string, lat, lng float64) (api.Tracking, error)
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
	// MenuPageLoadedMsg reports one streamed menu page merged into the
	// snapshot. Gen is the load generation stamped by the root when the
	// stream started — a mismatch means the user re-opened/changed
	// restaurants and this page belongs to a dead stream. Done means no
	// more pages; the root fires the next page's Cmd otherwise (root-driven
	// chain: pages stay serial and cancellation is just "don't continue").
	MenuPageLoadedMsg struct {
		PlaceID string
		Page    int
		Gen     int
		Done    bool
		Err     error
	}
	// PlacesPageLoadedMsg reports one streamed restaurant-search page merged
	// into the snapshot under Query. Same gen/chain contract as
	// MenuPageLoadedMsg.
	PlacesPageLoadedMsg struct {
		Query      string
		Page       int
		Gen        int
		NextOffset int
		Done       bool
		Err        error
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
	// if the poll failed. OrderID echoes the polled order so the root can drop a
	// late response for an order it has since navigated away from / swapped out.
	TrackingPolledMsg struct {
		OrderID  string
		Tracking api.Tracking
		Err      error
	}
	// ActiveOrdersLoadedMsg carries the account's currently-active orders, or an
	// error if the fetch failed.
	ActiveOrdersLoadedMsg struct {
		Orders []api.Order
		Err    error
	}
	// IMProductsLoadedMsg reports an Instamart browse/search load merged into the
	// snapshot (under the address). Query is empty for the go-to ("your usuals")
	// list, non-empty for a search — the root matches it against the live query
	// to guard against a stale response.
	IMProductsLoadedMsg struct {
		Query string
		Err   error
	}
	// IMCartSyncedMsg carries the result of an Instamart cart write (update or
	// clear) — Swiggy's real bill breakdown so checkout shows accurate numbers.
	IMCartSyncedMsg struct {
		Cart api.IMCart
		Err  error
	}
	// IMCartPulledMsg carries the account's Instamart cart fetched once at
	// launch, to seed the local cart from anything already built on the Swiggy
	// app/website. Distinct from IMCartLoadedMsg so launch-time errors stay
	// silent and only this path seeds.
	IMCartPulledMsg struct {
		Cart api.IMCart
		Err  error
	}
	// IMCartLoadedMsg carries the live Instamart cart fetched on cart/checkout
	// screen entry — the source of truth for the displayed lines + bill.
	IMCartLoadedMsg struct {
		Cart api.IMCart
		Err  error
	}
	// IMOrderPlacedMsg carries the result of placing an Instamart order.
	IMOrderPlacedMsg struct {
		Order api.Order
		Err   error
	}
	// IMActiveOrdersLoadedMsg carries the account's currently-active Instamart
	// orders, or an error if the fetch failed.
	IMActiveOrdersLoadedMsg struct {
		Orders []api.IMOrder
		Err    error
	}
	// IMTrackingPolledMsg carries the live status + ETA for an Instamart order.
	// OrderID echoes the polled order so a late response for a swapped-out order
	// can be dropped.
	IMTrackingPolledMsg struct {
		OrderID  string
		Tracking api.Tracking
		Err      error
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

// LoadMenuPage fetches ONE menu page and merges it into the snapshot — page 1
// replaces (dropping any stale copy), later pages append. staged streams the
// pages into the snapshot's staging area instead, for loads that seeded the
// visible menu from the disk cache (the root promotes the staged copy when
// the stream completes, so the seeded menu never shrinks mid-refresh). The
// root renders on every page msg and fires the next page itself, so a big
// menu paints in ~one call's latency instead of the whole loop's. Pages
// remain serial (one in flight), preserving the client's rate-limit posture.
func LoadMenuPage(b Backend, snap *swiggysnap.Snapshot, addressID, restaurantID string, page, gen int, staged bool) tea.Cmd {
	return func() tea.Msg {
		got, more, err := b.MenuPage(addressID, restaurantID, page)
		if err != nil {
			return MenuPageLoadedMsg{PlaceID: restaurantID, Page: page, Gen: gen, Err: err}
		}
		snap.MergeMenuPage(restaurantID, toMenuPlace(got).Items, page == 1, staged)
		return MenuPageLoadedMsg{PlaceID: restaurantID, Page: page, Gen: gen, Done: !more}
	}
}

// LoadPlacesPage fetches ONE page of a category/home restaurant search and
// merges it under the query key — page 1 replaces, later pages append. Same
// root-driven chain contract as LoadMenuPage.
func LoadPlacesPage(b Backend, snap *swiggysnap.Snapshot, addressID, query string, offset, page, gen int) tea.Cmd {
	return func() tea.Msg {
		got, next, more, err := b.PlacesQueryPage(addressID, query, offset)
		if err != nil {
			return PlacesPageLoadedMsg{Query: query, Page: page, Gen: gen, Err: err}
		}
		snap.MergePlacesPage(addressID, query, toPlaces(got, catalog.SectionCoffee), page == 1)
		return PlacesPageLoadedMsg{Query: query, Page: page, Gen: gen, NextOffset: next, Done: !more}
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
		return TrackingPolledMsg{OrderID: orderID, Tracking: t, Err: err}
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

// LoadIMProducts fetches an Instamart browse/search page and writes it into the
// snapshot under {addressID, query}. An empty query fetches the go-to ("your
// usuals") list (IMGoTo); a non-empty query runs a search (IMSearch). The Msg
// carries the query back so the root can guard a stale response against the
// live one — and the query-scoped snapshot key means even a raced write can't
// surface under a different query's view.
func LoadIMProducts(b Backend, snap *swiggysnap.Snapshot, addressID, query string) tea.Cmd {
	return func() tea.Msg {
		var (
			got []api.IMProduct
			err error
		)
		if query == "" {
			got, err = b.IMGoTo(addressID)
		} else {
			got, err = b.IMSearch(addressID, query)
		}
		if err != nil {
			return IMProductsLoadedMsg{Query: query, Err: err}
		}
		snap.SetInstamart(addressID, query, toIMItems(got))
		// Persist for instant first paint on a later relaunch (best-effort;
		// stale-while-revalidate, mirrors the food places/menu cache). Skip empty
		// results: a transient no-results reply must not overwrite a good cache
		// and paint an empty list as "last known" next launch.
		if len(got) > 0 {
			localstore.SaveCachedInstamart(addressID, query, toCachedIM(got))
		}
		return IMProductsLoadedMsg{Query: query}
	}
}

// SyncIMCart calls IMUpdateCart on the backend with the current Instamart cart
// contents (update_cart REPLACES the whole cart) and returns Swiggy's real bill
// breakdown so checkout can show accurate numbers. Errors are non-fatal: the
// TUI shows them and continues.
func SyncIMCart(b Backend, addressID string, items []api.IMCartItem) tea.Cmd {
	return func() tea.Msg {
		cart, err := b.IMUpdateCart(addressID, items)
		return IMCartSyncedMsg{Cart: cart, Err: err}
	}
}

// PullIMCart fetches the account's Instamart cart once at launch so the TUI can
// seed the local cart from anything already built on the Swiggy app/website.
func PullIMCart(b Backend) tea.Cmd {
	return func() tea.Msg {
		cart, err := b.IMGetCart()
		return IMCartPulledMsg{Cart: cart, Err: err}
	}
}

// LoadIMCart fetches the live Instamart cart (get_cart) so the cart/checkout
// screens render Swiggy's real items + pricing rather than the local
// in-memory approximation.
func LoadIMCart(b Backend) tea.Cmd {
	return func() tea.Msg {
		cart, err := b.IMGetCart()
		return IMCartLoadedMsg{Cart: cart, Err: err}
	}
}

// ClearIMCartCmd empties the Instamart cart. Used when the TUI cart goes empty
// — IMUpdateCart can't express an empty cart cleanly (it always replaces with
// the given items).
func ClearIMCartCmd(b Backend) tea.Cmd {
	return func() tea.Msg {
		return IMCartSyncedMsg{Err: b.IMClearCart()}
	}
}

// PlaceIMOrderCmd submits the Instamart order through the broker (COD
// checkout). The TUI must have already synced the cart via SyncIMCart before
// calling this. On success the broker returns the placed order; on failure the
// TUI shows the error and stays on scrCheckout.
func PlaceIMOrderCmd(b Backend, addressID string) tea.Cmd {
	return func() tea.Msg {
		order, err := b.IMPlaceOrder(addressID)
		return IMOrderPlacedMsg{Order: order, Err: err}
	}
}

// LoadIMActiveOrdersCmd fetches the account's currently-active Instamart
// orders (the real "is an order live?" signal for the splash).
func LoadIMActiveOrdersCmd(b Backend) tea.Cmd {
	return func() tea.Msg {
		os, err := b.IMOrders(true)
		return IMActiveOrdersLoadedMsg{Orders: os, Err: err}
	}
}

// PollIMTrackingCmd fetches the live status + ETA for an Instamart order.
// lat/lng come from IMOrders (get_orders) — track_order requires coordinates.
func PollIMTrackingCmd(b Backend, orderID string, lat, lng float64) tea.Cmd {
	return func() tea.Msg {
		t, err := b.IMTrack(orderID, lat, lng)
		return IMTrackingPolledMsg{OrderID: orderID, Tracking: t, Err: err}
	}
}
