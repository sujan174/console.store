package mcp

import (
	"context"

	"consolestore/internal/broker/api"
)

type fakeBackend struct {
	addrs           []api.Address
	search          []api.Restaurant
	searchNext      int
	searchMore      bool
	searchPageErr   error
	menu            api.Menu
	menuCalls       int
	searchPageCalls int
	itemOpts        []api.OptionGroup
	cart            api.Cart
	order           api.Order
	placeErr        error
	placed          int

	// UPI payment fakes. The default is the legacy COD fallback (PlaceUPI
	// ok=false → PlaceCOD), so existing food place_order tests stay immediate-
	// placement regression coverage. Set upi=true to exercise the online path:
	// PlaceUPI returns `pending`, then PollPayment/ConfirmOrder drive check/confirm.
	upi          bool
	pending      api.PendingPayment
	placeUPIErr  error
	placedUPI    int
	polls        int
	payStatus    api.PaymentStatus
	payErr       error
	confirmOrder api.Order
	confirmErr   error
	confirmed    int

	// optional scripted behavior for cart flows; nil falls back to `cart`.
	getFn    func(addressID string) (api.Cart, error)
	updateFn func(addressID, restaurantID, restaurantName string, items []api.CartItem) (api.Cart, error)
	updates  int
	cleared  int

	// Instamart fakes.
	imProducts   []api.IMProduct
	imCart       api.IMCart
	imOrder      api.Order
	imPlaceErr   error
	imPlaced     int
	imOrders     []api.IMOrder
	imOrdersErr  error
	imTracking   api.Tracking
	imTrackErr   error
	imGetFn      func() (api.IMCart, error)
	imUpdateFn   func(addressID string, items []api.IMCartItem) (api.IMCart, error)
	imUpdates    int
	imUpdateArgs []api.IMCartItem
	imCleared    int

	// Instamart UPI payment fakes. Mirror the food upi/pending/pollStatus set:
	// default imUpi=false → IMPlaceOrderUPI reports "no scan-to-pay method" so
	// callers fall back to the COD IMPlaceOrder (keeps existing IM place tests
	// immediate-placement). Set imUpi=true to exercise the online path.
	imUpi          bool
	imPending      api.PendingPayment
	imPlaceUPIErr  error
	imPlacedUPI    int
	imPolls        int
	imPayStatus    api.PaymentStatus
	imPayErr       error
	imConfirmOrder api.Order
	imConfirmErr   error
	imConfirmed    int
}

func (f *fakeBackend) Addresses() ([]api.Address, error) { return f.addrs, nil }
func (f *fakeBackend) SearchOrganic(addressID, query string) ([]api.Restaurant, string, error) {
	return f.search, query, nil
}
func (f *fakeBackend) SearchOrganicPage(addressID, query string, offset int) ([]api.Restaurant, string, int, bool, error) {
	f.searchPageCalls++
	if f.searchPageErr != nil {
		return nil, query, offset, false, f.searchPageErr
	}
	return f.search, query, f.searchNext, f.searchMore, nil
}
func (f *fakeBackend) PlacesQuery(addressID, query string) ([]api.Restaurant, error) {
	return f.search, nil
}
func (f *fakeBackend) Usuals(addressID string) ([]api.Restaurant, error) { return f.search, nil }
func (f *fakeBackend) Menu(addressID, restaurantID string) (api.Menu, error) {
	f.menuCalls++
	return f.menu, nil
}
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
func (f *fakeBackend) PlaceUPI(addressID string) (api.PendingPayment, bool, error) {
	if !f.upi {
		return api.PendingPayment{}, false, nil // legacy no-UPI user → COD fallback
	}
	if f.placeUPIErr != nil {
		return api.PendingPayment{}, true, f.placeUPIErr
	}
	f.placedUPI++
	return f.pending, true, nil
}
func (f *fakeBackend) PlaceCOD(addressID string) (api.Order, error) {
	f.placed++ // same "an order was placed" counter the COD/legacy tests assert on
	if f.placeErr != nil {
		return api.Order{}, f.placeErr
	}
	return f.order, nil
}
func (f *fakeBackend) PollPayment(p api.PendingPayment) (api.PaymentStatus, error) {
	f.polls++
	return f.payStatus, f.payErr
}
func (f *fakeBackend) ConfirmOrder(p api.PendingPayment) (api.Order, error) {
	f.confirmed++
	if f.confirmErr != nil {
		return api.Order{}, f.confirmErr
	}
	return f.confirmOrder, nil
}
func (f *fakeBackend) TrackOrder(orderID string) (api.Tracking, error)    { return api.Tracking{}, nil }
func (f *fakeBackend) ActiveOrders(addressID string) ([]api.Order, error) { return nil, nil }

func (f *fakeBackend) IMSearch(addressID, query string) ([]api.IMProduct, error) {
	return f.imProducts, nil
}
func (f *fakeBackend) IMGetCart() (api.IMCart, error) {
	if f.imGetFn != nil {
		return f.imGetFn()
	}
	return f.imCart, nil
}
func (f *fakeBackend) IMUpdateCart(addressID string, items []api.IMCartItem) (api.IMCart, error) {
	f.imUpdates++
	f.imUpdateArgs = items
	if f.imUpdateFn != nil {
		return f.imUpdateFn(addressID, items)
	}
	return f.imCart, nil
}
func (f *fakeBackend) IMClearCart() error { f.imCleared++; return nil }
func (f *fakeBackend) IMPlaceOrder(addressID string) (api.Order, error) {
	f.imPlaced++
	if f.imPlaceErr != nil {
		return api.Order{}, f.imPlaceErr
	}
	return f.imOrder, nil
}
func (f *fakeBackend) IMPlaceOrderUPI(addressID string) (api.PendingPayment, bool, error) {
	if !f.imUpi {
		return api.PendingPayment{}, false, nil // no scan-to-pay method → COD fallback
	}
	if f.imPlaceUPIErr != nil {
		return api.PendingPayment{}, true, f.imPlaceUPIErr
	}
	f.imPlacedUPI++
	return f.imPending, true, nil
}
func (f *fakeBackend) IMPollPayment(p api.PendingPayment) (api.PaymentStatus, error) {
	f.imPolls++
	return f.imPayStatus, f.imPayErr
}
func (f *fakeBackend) IMConfirmOrder(p api.PendingPayment) (api.Order, error) {
	f.imConfirmed++
	if f.imConfirmErr != nil {
		return api.Order{}, f.imConfirmErr
	}
	return f.imConfirmOrder, nil
}
func (f *fakeBackend) IMOrders(activeOnly bool) ([]api.IMOrder, error) {
	return f.imOrders, f.imOrdersErr
}
func (f *fakeBackend) IMTrack(orderID string, lat, lng float64) (api.Tracking, error) {
	return f.imTracking, f.imTrackErr
}

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
