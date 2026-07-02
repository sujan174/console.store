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
	s := err.Error()
	if strings.Contains(s, "token expired") ||
		strings.Contains(s, "account not authorized") ||
		strings.Contains(s, "session revoked") {
		return fmt.Errorf("%w: %v", ErrNeedsAuth, err)
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
