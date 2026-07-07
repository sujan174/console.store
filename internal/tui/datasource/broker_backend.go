package datasource

import (
	"fmt"
	"strings"

	"consolestore/internal/broker/api"
	"consolestore/internal/catalog"
)

type brokerRPC interface {
	Addresses(accountID string) ([]api.Address, error)
	Restaurants(accountID, addressID, query string) ([]api.Restaurant, error)
	SearchOrganic(accountID, addressID, query string) ([]api.Restaurant, string, error)
	Usuals(accountID, addressID string) ([]api.Restaurant, error)
	Menu(accountID, addressID, restaurantID string) (api.Menu, error)
	MenuPage(accountID, addressID, restaurantID string, page int) (api.Menu, bool, error)
	RestaurantsPage(accountID, addressID, query string, offset int) ([]api.Restaurant, int, bool, error)
	ItemOptions(accountID, addressID, restaurantID, itemName, menuItemID string) ([]api.OptionGroup, error)
	UpdateCart(a api.UpdateCartArgs) (api.Cart, error)
	GetCart(accountID, addressID, restaurantName string) (api.Cart, error)
	ClearCart(accountID string) error
	PlaceOrder(accountID, addressID string) (api.Order, error)
	TrackOrder(accountID, orderID string) (api.Tracking, error)
	ActiveFoodOrders(accountID, addressID string) ([]api.Order, error)
	Logout(accountID string) error

	IMSearch(accountID, addressID, query string) ([]api.IMProduct, error)
	IMGoTo(accountID, addressID string) ([]api.IMProduct, error)
	IMGetCart(accountID string) (api.IMCart, error)
	IMUpdateCart(accountID, addressID string, items []api.IMCartItem) (api.IMCart, error)
	IMClearCart(accountID string) error
	IMPlaceOrder(accountID, addressID string) (api.Order, error)
	IMOrders(accountID string, activeOnly bool) ([]api.IMOrder, error)
	IMTrack(accountID, orderID string, lat, lng float64) (api.Tracking, error)
}

// BrokerBackend adapts the broker RPC client into a datasource.Backend, pinned
// to ONE account id (the fixed local account; see localstore.LocalAccountID).
// The account id is fixed at construction and never read from a call argument,
// so a session can only ever act as its own account.
type BrokerBackend struct {
	rpc       brokerRPC
	accountID string
}

func NewBrokerBackend(rpc brokerRPC, accountID string) *BrokerBackend {
	return &BrokerBackend{rpc: rpc, accountID: accountID}
}

// wrapAuthErr wraps a broker error to ErrNeedsAuth if the error text indicates
// a missing or expired token. net/rpc serialises errors as plain strings, so
// we cannot use errors.Is — string matching is the intended seam here.
func wrapAuthErr(err error) error {
	if err == nil {
		return nil
	}
	s := strings.ToLower(err.Error())
	// Matched case-insensitively and broadened to the phrasings Swiggy actually
	// returns — a 401/403 insufficient_scope from the account-restriction path was
	// previously shown as a dead-end red status line instead of the authorize
	// gate. (Numeric-only codes are avoided to prevent false positives from
	// prices/ids; "http 401/403" is specific enough.)
	for _, needle := range []string{
		"token expired", "account not authorized", "session revoked",
		"insufficient_scope", "unauthenticated", "unauthorized",
		"invalid_token", "invalid token", "http 401", "http 403",
		// A dead refresh token surfaces as the OAuth token endpoint's rejection
		// ("auth: refresh status 400: {"error":"invalid_grant"}"). Without these
		// the runtime path would leave the user on a silently-failing screen
		// instead of the authorize gate.
		"invalid_grant", "refresh status 4",
	} {
		if strings.Contains(s, needle) {
			return fmt.Errorf("%w: %v", ErrNeedsAuth, err)
		}
	}
	return err
}

func (b *BrokerBackend) Addresses() ([]api.Address, error) {
	r, err := b.rpc.Addresses(b.accountID)
	return r, wrapAuthErr(err)
}

func (b *BrokerBackend) Places(addressID string, section catalog.Section) ([]api.Restaurant, error) {
	r, err := b.rpc.Restaurants(b.accountID, addressID, sectionQuery(section))
	return r, wrapAuthErr(err)
}

func (b *BrokerBackend) PlacesQuery(addressID, query string) ([]api.Restaurant, error) {
	r, err := b.rpc.Restaurants(b.accountID, addressID, query)
	return r, wrapAuthErr(err)
}

func (b *BrokerBackend) SearchOrganic(addressID, query string) ([]api.Restaurant, string, error) {
	r, eff, err := b.rpc.SearchOrganic(b.accountID, addressID, query)
	return r, eff, wrapAuthErr(err)
}

func (b *BrokerBackend) Usuals(addressID string) ([]api.Restaurant, error) {
	r, err := b.rpc.Usuals(b.accountID, addressID)
	return r, wrapAuthErr(err)
}

func (b *BrokerBackend) Menu(addressID, restaurantID string) (api.Menu, error) {
	r, err := b.rpc.Menu(b.accountID, addressID, restaurantID)
	return r, wrapAuthErr(err)
}

func (b *BrokerBackend) MenuPage(addressID, restaurantID string, page int) (api.Menu, bool, error) {
	r, more, err := b.rpc.MenuPage(b.accountID, addressID, restaurantID, page)
	return r, more, wrapAuthErr(err)
}

func (b *BrokerBackend) PlacesQueryPage(addressID, query string, offset int) ([]api.Restaurant, int, bool, error) {
	r, next, more, err := b.rpc.RestaurantsPage(b.accountID, addressID, query, offset)
	return r, next, more, wrapAuthErr(err)
}

func (b *BrokerBackend) ItemOptions(addressID, restaurantID, itemName, menuItemID string) ([]api.OptionGroup, error) {
	r, err := b.rpc.ItemOptions(b.accountID, addressID, restaurantID, itemName, menuItemID)
	return r, wrapAuthErr(err)
}

func (b *BrokerBackend) UpdateCart(addressID, restaurantID, restaurantName string, items []api.CartItem) (api.Cart, error) {
	r, err := b.rpc.UpdateCart(api.UpdateCartArgs{
		AccountID:      b.accountID,
		AddressID:      addressID,
		RestaurantID:   restaurantID,
		RestaurantName: restaurantName,
		Items:          items,
	})
	return r, wrapAuthErr(err)
}

func (b *BrokerBackend) GetCart(addressID, restaurantName string) (api.Cart, error) {
	r, err := b.rpc.GetCart(b.accountID, addressID, restaurantName)
	return r, wrapAuthErr(err)
}

func (b *BrokerBackend) ClearCart() error {
	return wrapAuthErr(b.rpc.ClearCart(b.accountID))
}

func (b *BrokerBackend) PlaceOrder(addressID string) (api.Order, error) {
	r, err := b.rpc.PlaceOrder(b.accountID, addressID)
	return r, wrapAuthErr(err)
}

func (b *BrokerBackend) TrackOrder(orderID string) (api.Tracking, error) {
	t, err := b.rpc.TrackOrder(b.accountID, orderID)
	return t, wrapAuthErr(err)
}

func (b *BrokerBackend) ActiveOrders(addressID string) ([]api.Order, error) {
	o, err := b.rpc.ActiveFoodOrders(b.accountID, addressID)
	return o, wrapAuthErr(err)
}

func (b *BrokerBackend) Logout() error { return b.rpc.Logout(b.accountID) }

func (b *BrokerBackend) IMSearch(addressID, query string) ([]api.IMProduct, error) {
	r, err := b.rpc.IMSearch(b.accountID, addressID, query)
	return r, wrapAuthErr(err)
}

func (b *BrokerBackend) IMGoTo(addressID string) ([]api.IMProduct, error) {
	r, err := b.rpc.IMGoTo(b.accountID, addressID)
	return r, wrapAuthErr(err)
}

func (b *BrokerBackend) IMGetCart() (api.IMCart, error) {
	r, err := b.rpc.IMGetCart(b.accountID)
	return r, wrapAuthErr(err)
}

func (b *BrokerBackend) IMUpdateCart(addressID string, items []api.IMCartItem) (api.IMCart, error) {
	r, err := b.rpc.IMUpdateCart(b.accountID, addressID, items)
	return r, wrapAuthErr(err)
}

func (b *BrokerBackend) IMClearCart() error {
	return wrapAuthErr(b.rpc.IMClearCart(b.accountID))
}

func (b *BrokerBackend) IMPlaceOrder(addressID string) (api.Order, error) {
	r, err := b.rpc.IMPlaceOrder(b.accountID, addressID)
	return r, wrapAuthErr(err)
}

func (b *BrokerBackend) IMOrders(activeOnly bool) ([]api.IMOrder, error) {
	r, err := b.rpc.IMOrders(b.accountID, activeOnly)
	return r, wrapAuthErr(err)
}

func (b *BrokerBackend) IMTrack(orderID string, lat, lng float64) (api.Tracking, error) {
	r, err := b.rpc.IMTrack(b.accountID, orderID, lat, lng)
	return r, wrapAuthErr(err)
}

// sectionQuery maps a catalogue lane to a Swiggy restaurant-search query.
func sectionQuery(s catalog.Section) string {
	switch s {
	case catalog.SectionCoffee:
		return "coffee"
	case catalog.SectionFood:
		return "food"
	case catalog.SectionSnacks:
		return "snacks"
	default:
		return string(s)
	}
}
