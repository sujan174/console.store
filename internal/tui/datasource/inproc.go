package datasource

import (
	"context"

	"console.store/internal/broker/api"
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
	ItemOptions(ctx context.Context, accountID, addressID, restaurantID, itemName, menuItemID string) ([]api.OptionGroup, error)
	UpdateCart(ctx context.Context, a api.UpdateCartArgs) (api.Cart, error)
	GetCart(ctx context.Context, accountID, addressID, restaurantName string) (api.Cart, error)
	ClearCart(ctx context.Context, accountID string) error
	PlaceOrder(ctx context.Context, accountID, addressID string) (api.Order, error)
	Logout(ctx context.Context, accountID string) error
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
