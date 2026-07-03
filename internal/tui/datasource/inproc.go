package datasource

import (
	"context"

	"consolestore/internal/broker/api"
)

// inprocService is the subset of *broker.Service that the in-process backend
// calls. Declaring it here (rather than importing the concrete Service) keeps
// the adapter unit-testable with a fake and documents exactly what it depends
// on. *broker.Service satisfies this interface structurally.
type inprocService interface {
	Addresses(ctx context.Context, accountID string) ([]api.Address, error)
	Restaurants(ctx context.Context, accountID, addressID, query string, organic bool) ([]api.Restaurant, string, error)
	Usuals(ctx context.Context, accountID, addressID string) ([]api.Restaurant, error)
	Menu(ctx context.Context, accountID, addressID, restaurantID string) (api.Menu, error)
	MenuPage(ctx context.Context, accountID, addressID, restaurantID string, page int) (api.Menu, bool, error)
	RestaurantsPage(ctx context.Context, accountID, addressID, query string, offset int) ([]api.Restaurant, int, bool, error)
	ItemOptions(ctx context.Context, accountID, addressID, restaurantID, itemName, menuItemID string) ([]api.OptionGroup, error)
	UpdateCart(ctx context.Context, a api.UpdateCartArgs) (api.Cart, error)
	GetCart(ctx context.Context, accountID, addressID, restaurantName string) (api.Cart, error)
	ClearCart(ctx context.Context, accountID string) error
	PlaceOrder(ctx context.Context, accountID, addressID string) (api.Order, error)
	TrackOrder(ctx context.Context, accountID, orderID string) (api.Tracking, error)
	ActiveFoodOrders(ctx context.Context, accountID, addressID string) ([]api.Order, error)
	Logout(ctx context.Context, accountID string) error

	IMSearch(ctx context.Context, accountID, addressID, query string) ([]api.IMProduct, error)
	IMGoTo(ctx context.Context, accountID, addressID string) ([]api.IMProduct, error)
	IMGetCart(ctx context.Context, accountID string) (api.IMCart, error)
	IMUpdateCart(ctx context.Context, accountID, addressID string, items []api.IMCartItem) (api.IMCart, error)
	IMClearCart(ctx context.Context, accountID string) error
	IMPlaceOrder(ctx context.Context, accountID, addressID string) (api.Order, error)
	IMOrders(ctx context.Context, accountID string, activeOnly bool) ([]api.IMOrder, error)
	IMTrack(ctx context.Context, accountID, orderID string, lat, lng float64) (api.Tracking, error)
}

// InProc adapts a broker.Service into the brokerRPC interface that
// BrokerBackend expects, calling the service directly in-process (no socket,
// no net/rpc). Each method supplies context.Background() and forwards the
// account id BrokerBackend pins.
type InProc struct{ svc inprocService }

func NewInProc(svc inprocService) InProc { return InProc{svc: svc} }

func (p InProc) Addresses(accountID string) ([]api.Address, error) {
	return p.svc.Addresses(context.Background(), accountID)
}

func (p InProc) Logout(accountID string) error {
	return p.svc.Logout(context.Background(), accountID)
}

func (p InProc) Restaurants(accountID, addressID, query string) ([]api.Restaurant, error) {
	r, _, err := p.svc.Restaurants(context.Background(), accountID, addressID, query, false)
	return r, err
}

func (p InProc) SearchOrganic(accountID, addressID, query string) ([]api.Restaurant, string, error) {
	return p.svc.Restaurants(context.Background(), accountID, addressID, query, true)
}

func (p InProc) Usuals(accountID, addressID string) ([]api.Restaurant, error) {
	return p.svc.Usuals(context.Background(), accountID, addressID)
}

func (p InProc) Menu(accountID, addressID, restaurantID string) (api.Menu, error) {
	return p.svc.Menu(context.Background(), accountID, addressID, restaurantID)
}

func (p InProc) MenuPage(accountID, addressID, restaurantID string, page int) (api.Menu, bool, error) {
	return p.svc.MenuPage(context.Background(), accountID, addressID, restaurantID, page)
}

func (p InProc) RestaurantsPage(accountID, addressID, query string, offset int) ([]api.Restaurant, int, bool, error) {
	return p.svc.RestaurantsPage(context.Background(), accountID, addressID, query, offset)
}

func (p InProc) ItemOptions(accountID, addressID, restaurantID, itemName, menuItemID string) ([]api.OptionGroup, error) {
	return p.svc.ItemOptions(context.Background(), accountID, addressID, restaurantID, itemName, menuItemID)
}

func (p InProc) UpdateCart(a api.UpdateCartArgs) (api.Cart, error) {
	return p.svc.UpdateCart(context.Background(), a)
}

func (p InProc) GetCart(accountID, addressID, restaurantName string) (api.Cart, error) {
	return p.svc.GetCart(context.Background(), accountID, addressID, restaurantName)
}

func (p InProc) ClearCart(accountID string) error {
	return p.svc.ClearCart(context.Background(), accountID)
}

func (p InProc) PlaceOrder(accountID, addressID string) (api.Order, error) {
	return p.svc.PlaceOrder(context.Background(), accountID, addressID)
}

func (p InProc) TrackOrder(accountID, orderID string) (api.Tracking, error) {
	return p.svc.TrackOrder(context.Background(), accountID, orderID)
}

func (p InProc) ActiveFoodOrders(accountID, addressID string) ([]api.Order, error) {
	return p.svc.ActiveFoodOrders(context.Background(), accountID, addressID)
}

func (p InProc) IMSearch(accountID, addressID, query string) ([]api.IMProduct, error) {
	return p.svc.IMSearch(context.Background(), accountID, addressID, query)
}

func (p InProc) IMGoTo(accountID, addressID string) ([]api.IMProduct, error) {
	return p.svc.IMGoTo(context.Background(), accountID, addressID)
}

func (p InProc) IMGetCart(accountID string) (api.IMCart, error) {
	return p.svc.IMGetCart(context.Background(), accountID)
}

func (p InProc) IMUpdateCart(accountID, addressID string, items []api.IMCartItem) (api.IMCart, error) {
	return p.svc.IMUpdateCart(context.Background(), accountID, addressID, items)
}

func (p InProc) IMClearCart(accountID string) error {
	return p.svc.IMClearCart(context.Background(), accountID)
}

func (p InProc) IMPlaceOrder(accountID, addressID string) (api.Order, error) {
	return p.svc.IMPlaceOrder(context.Background(), accountID, addressID)
}

func (p InProc) IMOrders(accountID string, activeOnly bool) ([]api.IMOrder, error) {
	return p.svc.IMOrders(context.Background(), accountID, activeOnly)
}

func (p InProc) IMTrack(accountID, orderID string, lat, lng float64) (api.Tracking, error) {
	return p.svc.IMTrack(context.Background(), accountID, orderID, lat, lng)
}
