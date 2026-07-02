package mcp

import (
	"context"

	"consolestore/internal/broker/api"
)

type fakeBackend struct {
	addrs    []api.Address
	search   []api.Restaurant
	menu     api.Menu
	itemOpts []api.OptionGroup
	cart     api.Cart
	order    api.Order
	placeErr error
	placed   int

	// optional scripted behavior for cart flows; nil falls back to `cart`.
	getFn    func(addressID string) (api.Cart, error)
	updateFn func(addressID, restaurantID, restaurantName string, items []api.CartItem) (api.Cart, error)
	updates  int
	cleared  int
}

func (f *fakeBackend) Addresses() ([]api.Address, error) { return f.addrs, nil }
func (f *fakeBackend) SearchOrganic(addressID, query string) ([]api.Restaurant, string, error) {
	return f.search, query, nil
}
func (f *fakeBackend) PlacesQuery(addressID, query string) ([]api.Restaurant, error) {
	return f.search, nil
}
func (f *fakeBackend) Usuals(addressID string) ([]api.Restaurant, error)     { return f.search, nil }
func (f *fakeBackend) Menu(addressID, restaurantID string) (api.Menu, error) { return f.menu, nil }
func (f *fakeBackend) ItemOptions(addressID, restaurantID, itemName, menuItemID string) ([]api.OptionGroup, error) {
	return f.itemOpts, nil
}
func (f *fakeBackend) GetCart(addressID, restaurantName string) (api.Cart, error) {
	if f.getFn != nil {
		return f.getFn(addressID)
	}
	return f.cart, nil
}
func (f *fakeBackend) UpdateCart(addressID, restaurantID, restaurantName string, items []api.CartItem) (api.Cart, error) {
	f.updates++
	if f.updateFn != nil {
		return f.updateFn(addressID, restaurantID, restaurantName, items)
	}
	return f.cart, nil
}
func (f *fakeBackend) ClearCart() error { f.cleared++; return nil }
func (f *fakeBackend) PlaceOrder(addressID string) (api.Order, error) {
	f.placed++
	if f.placeErr != nil {
		return api.Order{}, f.placeErr
	}
	return f.order, nil
}
func (f *fakeBackend) TrackOrder(orderID string) (api.Tracking, error)    { return api.Tracking{}, nil }
func (f *fakeBackend) ActiveOrders(addressID string) ([]api.Order, error) { return nil, nil }

type fakeAuth struct {
	token bool
	url   string
	flow  string
	done  bool
}

func (a *fakeAuth) TokenPresent(ctx context.Context) bool { return a.token }
func (a *fakeAuth) Start(ctx context.Context) (string, string, error) {
	return a.url, a.flow, nil
}
func (a *fakeAuth) Authorized(flowID string) bool { return a.done }
