package datasource

import (
	"errors"
	"testing"

	"consolestore/internal/broker/api"
	"consolestore/internal/catalog"
	swiggysnap "consolestore/internal/catalog/swiggy"
)

type fakeBackend struct {
	addrs       []api.Address
	rests       []api.Restaurant
	restaurants []api.Restaurant
	menu        api.Menu
	cart        api.Cart
	order       api.Order
	tracking    api.Tracking
	orders      []api.Order
	err         error
	updateCalls int
	placeCalls  int

	imProducts    []api.IMProduct
	imCart        api.IMCart
	imOrder       api.Order
	imOrders      []api.IMOrder
	imTracking    api.Tracking
	imUpdateCalls int
	imPlaceCalls  int
	imClearCalls  int
	imSearchQuery string
}

func (f *fakeBackend) Addresses() ([]api.Address, error) { return f.addrs, f.err }
func (f *fakeBackend) Places(string, catalog.Section) ([]api.Restaurant, error) {
	return f.rests, f.err
}
func (f *fakeBackend) PlacesQuery(string, string) ([]api.Restaurant, error) {
	return f.rests, f.err
}
func (f *fakeBackend) SearchOrganic(string, string) ([]api.Restaurant, string, error) {
	return f.rests, "", f.err
}
func (f *fakeBackend) Usuals(string) ([]api.Restaurant, error) { return f.restaurants, f.err }
func (f *fakeBackend) Menu(string, string) (api.Menu, error)   { return f.menu, f.err }
func (f *fakeBackend) MenuPage(string, string, int) (api.Menu, bool, error) {
	return f.menu, false, f.err
}
func (f *fakeBackend) PlacesQueryPage(string, string, int) ([]api.Restaurant, int, bool, error) {
	return f.rests, len(f.rests), false, f.err
}
func (f *fakeBackend) ItemOptions(string, string, string, string) ([]api.OptionGroup, error) {
	return nil, f.err
}
func (f *fakeBackend) UpdateCart(string, string, string, []api.CartItem) (api.Cart, error) {
	f.updateCalls++
	return f.cart, f.err
}
func (f *fakeBackend) GetCart(string, string) (api.Cart, error) { return f.cart, f.err }
func (f *fakeBackend) ClearCart() error                         { return f.err }
func (f *fakeBackend) PlaceOrder(string) (api.Order, error) {
	f.placeCalls++
	return f.order, f.err
}

func TestLoadAddressesFillsSnapshot(t *testing.T) {
	b := &fakeBackend{addrs: []api.Address{{ID: "a1", Label: "home"}}}
	snap := swiggysnap.NewSnapshot()
	msg := LoadAddresses(b, snap)()
	if m, ok := msg.(AddressesLoadedMsg); !ok || m.Err != nil {
		t.Fatalf("msg = %#v", msg)
	}
	repo := swiggysnap.NewRepository(snap)
	if got := repo.Addresses(); len(got) != 1 || got[0].ID != "a1" {
		t.Fatalf("snapshot not filled: %v", got)
	}
}

func TestLoadPlacesPropagatesError(t *testing.T) {
	b := &fakeBackend{err: ErrNeedsAuth}
	snap := swiggysnap.NewSnapshot()
	msg := LoadPlaces(b, snap, "a1", catalog.SectionCoffee)()
	m, ok := msg.(PlacesLoadedMsg)
	if !ok || !errors.Is(m.Err, ErrNeedsAuth) || m.Section != catalog.SectionCoffee {
		t.Fatalf("msg = %#v", msg)
	}
}

func TestLoadMenuFillsSnapshot(t *testing.T) {
	b := &fakeBackend{menu: api.Menu{RestaurantID: "p1", Items: []api.MenuItem{{ID: "i1", Name: "Latte", Price: 250}}}}
	snap := swiggysnap.NewSnapshot()
	if msg := LoadMenu(b, snap, "a1", "p1")(); msg.(MenuLoadedMsg).Err != nil {
		t.Fatalf("menu load err: %v", msg)
	}
	if p, ok := swiggysnap.NewRepository(snap).Menu("p1"); !ok || len(p.Items) != 1 {
		t.Fatalf("menu not filled: %+v ok=%v", p, ok)
	}
}

func TestSyncCartCallsUpdateCart(t *testing.T) {
	b := &fakeBackend{cart: api.Cart{CartID: "cart-1", Total: 220}}
	snap := swiggysnap.NewSnapshot()
	items := []api.CartItem{{ItemID: "item-1", Quantity: 2}}
	msg := SyncCart(b, snap, "a1", "r1", "Blue Tokai", items)()
	m, ok := msg.(CartSyncedMsg)
	if !ok {
		t.Fatalf("msg type = %T", msg)
	}
	if m.Err != nil {
		t.Fatalf("CartSyncedMsg.Err = %v", m.Err)
	}
	if b.updateCalls != 1 {
		t.Fatalf("UpdateCart called %d times; want 1", b.updateCalls)
	}
}

func TestSyncCartPropagatesError(t *testing.T) {
	b := &fakeBackend{err: errors.New("network error")}
	snap := swiggysnap.NewSnapshot()
	msg := SyncCart(b, snap, "a1", "r1", "Blue Tokai", []api.CartItem{{ItemID: "i1", Quantity: 1}})()
	m, ok := msg.(CartSyncedMsg)
	if !ok || m.Err == nil {
		t.Fatalf("expected CartSyncedMsg with error; got %#v", msg)
	}
}

func TestPlaceOrderCmdReturnsOrder(t *testing.T) {
	b := &fakeBackend{order: api.Order{ID: "order-42", Status: "placed"}}
	snap := swiggysnap.NewSnapshot()
	msg := PlaceOrderCmd(b, snap, "a1")()
	m, ok := msg.(OrderPlacedMsg)
	if !ok {
		t.Fatalf("msg type = %T", msg)
	}
	if m.Err != nil || m.Order.ID != "order-42" {
		t.Fatalf("OrderPlacedMsg = %+v", m)
	}
	if b.placeCalls != 1 {
		t.Fatalf("PlaceOrder called %d times; want 1", b.placeCalls)
	}
}

func TestPlaceOrderCmdPropagatesError(t *testing.T) {
	b := &fakeBackend{err: errors.New("order failed")}
	snap := swiggysnap.NewSnapshot()
	msg := PlaceOrderCmd(b, snap, "a1")()
	m, ok := msg.(OrderPlacedMsg)
	if !ok || m.Err == nil {
		t.Fatalf("expected OrderPlacedMsg with error; got %#v", msg)
	}
}

func TestLoadUsualsCachesUnderUsualsKey(t *testing.T) {
	b := &fakeBackend{restaurants: []api.Restaurant{{ID: "r1", Name: "Blue Tokai"}}}
	snap := swiggysnap.NewSnapshot()
	msg := LoadUsuals(b, snap, "a1")()
	if m, ok := msg.(UsualsLoadedMsg); !ok || m.Err != nil {
		t.Fatalf("expected clean UsualsLoadedMsg, got %#v", msg)
	}
	repo := swiggysnap.NewRepository(snap)
	got := repo.PlacesByQuery(catalog.Address{ID: "a1"}, UsualsKey)
	if len(got) != 1 || got[0].Name != "Blue Tokai" {
		t.Fatalf("usuals not cached under UsualsKey: %+v", got)
	}
}

func (f *fakeBackend) TrackOrder(string) (api.Tracking, error) { return f.tracking, f.err }
func (f *fakeBackend) PlaceUPI(string) (api.PendingPayment, bool, error) {
	return api.PendingPayment{}, false, f.err
}
func (f *fakeBackend) PollPayment(api.PendingPayment) (api.PaymentStatus, error) {
	return api.PayPending, f.err
}
func (f *fakeBackend) ConfirmOrder(api.PendingPayment) (api.Order, error) { return api.Order{}, f.err }
func (f *fakeBackend) ActiveOrders(string) ([]api.Order, error)           { return f.orders, f.err }
func (f *fakeBackend) Logout() error                                      { return f.err }

func (f *fakeBackend) IMSearch(_, query string) ([]api.IMProduct, error) {
	f.imSearchQuery = query
	return f.imProducts, f.err
}
func (f *fakeBackend) IMGoTo(string) ([]api.IMProduct, error) { return f.imProducts, f.err }
func (f *fakeBackend) IMGetCart() (api.IMCart, error)         { return f.imCart, f.err }
func (f *fakeBackend) IMUpdateCart(string, []api.IMCartItem) (api.IMCart, error) {
	f.imUpdateCalls++
	return f.imCart, f.err
}
func (f *fakeBackend) IMClearCart() error {
	f.imClearCalls++
	return f.err
}
func (f *fakeBackend) IMPlaceOrder(string) (api.Order, error) {
	f.imPlaceCalls++
	return f.imOrder, f.err
}
func (f *fakeBackend) IMOrders(bool) ([]api.IMOrder, error) { return f.imOrders, f.err }
func (f *fakeBackend) IMTrack(string, float64, float64) (api.Tracking, error) {
	return f.imTracking, f.err
}

func TestPollTrackingCmd(t *testing.T) {
	be := &fakeBackend{tracking: api.Tracking{OrderID: "X1", Status: "Out for delivery", ETA: "11 mins", Active: true}}
	msg := PollTrackingCmd(be, "X1")()
	tp, ok := msg.(TrackingPolledMsg)
	if !ok || tp.Tracking.Status != "Out for delivery" || tp.Tracking.ETA != "11 mins" {
		t.Fatalf("got %#v", msg)
	}
}

func TestLoadActiveOrdersCmd(t *testing.T) {
	be := &fakeBackend{orders: []api.Order{{ID: "O1", Status: "active"}}}
	msg := LoadActiveOrdersCmd(be, "addr1")()
	m, ok := msg.(ActiveOrdersLoadedMsg)
	if !ok || m.Err != nil || len(m.Orders) != 1 || m.Orders[0].ID != "O1" {
		t.Fatalf("got %#v", msg)
	}
}

// LoadIMProducts with an empty query fetches the go-to list (IMGoTo, not
// IMSearch) and writes the mapped items into the snapshot for the address.
func TestLoadIMProductsGoToFillsSnapshot(t *testing.T) {
	b := &fakeBackend{imProducts: []api.IMProduct{
		{ID: "p1", Name: "Milk", InStock: true, Variants: []api.IMVariantSel{{SpinID: "s1", Label: "500ml", Price: 40, InStock: true}}},
	}}
	snap := swiggysnap.NewSnapshot()
	msg := LoadIMProducts(b, snap, "a1", "")()
	m, ok := msg.(IMProductsLoadedMsg)
	if !ok || m.Err != nil || m.Query != "" {
		t.Fatalf("msg = %#v", msg)
	}
	if b.imSearchQuery != "" {
		t.Fatalf("empty query should call IMGoTo, not IMSearch (got query=%q)", b.imSearchQuery)
	}
	got := swiggysnap.NewRepository(snap).InstamartItems(catalog.Address{ID: "a1"})
	if len(got) != 1 || got[0].Name != "Milk" || got[0].SwiggyID != "s1" {
		t.Fatalf("snapshot not filled: %+v", got)
	}
}

// LoadIMProducts with a non-empty query calls IMSearch and returns the query
// on the msg so the root can guard a stale response.
func TestLoadIMProductsSearchUsesQuery(t *testing.T) {
	b := &fakeBackend{imProducts: []api.IMProduct{{ID: "p1", Name: "Bread", InStock: true}}}
	snap := swiggysnap.NewSnapshot()
	msg := LoadIMProducts(b, snap, "a1", "bread")()
	m, ok := msg.(IMProductsLoadedMsg)
	if !ok || m.Err != nil || m.Query != "bread" {
		t.Fatalf("msg = %#v", msg)
	}
	if b.imSearchQuery != "bread" {
		t.Fatalf("IMSearch called with query %q; want \"bread\"", b.imSearchQuery)
	}
}

func TestLoadIMProductsPropagatesError(t *testing.T) {
	b := &fakeBackend{err: errors.New("network error")}
	snap := swiggysnap.NewSnapshot()
	msg := LoadIMProducts(b, snap, "a1", "milk")()
	m, ok := msg.(IMProductsLoadedMsg)
	if !ok || m.Err == nil || m.Query != "milk" {
		t.Fatalf("msg = %#v", msg)
	}
}

func TestSyncIMCartReturnsCart(t *testing.T) {
	b := &fakeBackend{imCart: api.IMCart{Total: 199}}
	items := []api.IMCartItem{{SpinID: "s1", Quantity: 2}}
	msg := SyncIMCart(b, "a1", items)()
	m, ok := msg.(IMCartSyncedMsg)
	if !ok || m.Err != nil || m.Cart.Total != 199 {
		t.Fatalf("msg = %#v", msg)
	}
	if b.imUpdateCalls != 1 {
		t.Fatalf("IMUpdateCart called %d times; want 1", b.imUpdateCalls)
	}
}

func TestSyncIMCartPropagatesError(t *testing.T) {
	b := &fakeBackend{err: errors.New("store closed")}
	msg := SyncIMCart(b, "a1", nil)()
	m, ok := msg.(IMCartSyncedMsg)
	if !ok || m.Err == nil {
		t.Fatalf("expected IMCartSyncedMsg with error; got %#v", msg)
	}
}

func TestPlaceIMOrderCmdReturnsOrder(t *testing.T) {
	b := &fakeBackend{imOrder: api.Order{ID: "im-order-1", Status: "placed"}}
	msg := PlaceIMOrderCmd(b, "a1")()
	m, ok := msg.(IMOrderPlacedMsg)
	if !ok || m.Err != nil || m.Order.ID != "im-order-1" {
		t.Fatalf("msg = %#v", msg)
	}
	if b.imPlaceCalls != 1 {
		t.Fatalf("IMPlaceOrder called %d times; want 1", b.imPlaceCalls)
	}
}

func TestClearIMCartCmd(t *testing.T) {
	b := &fakeBackend{}
	msg := ClearIMCartCmd(b)()
	m, ok := msg.(IMCartSyncedMsg)
	if !ok || m.Err != nil {
		t.Fatalf("msg = %#v", msg)
	}
	if b.imClearCalls != 1 {
		t.Fatalf("IMClearCart called %d times; want 1", b.imClearCalls)
	}
}

func TestLoadIMActiveOrdersCmd(t *testing.T) {
	b := &fakeBackend{imOrders: []api.IMOrder{{ID: "IM1", Status: "active"}}}
	msg := LoadIMActiveOrdersCmd(b)()
	m, ok := msg.(IMActiveOrdersLoadedMsg)
	if !ok || m.Err != nil || len(m.Orders) != 1 || m.Orders[0].ID != "IM1" {
		t.Fatalf("got %#v", msg)
	}
}

func TestPollIMTrackingCmd(t *testing.T) {
	b := &fakeBackend{imTracking: api.Tracking{OrderID: "IM1", Status: "Out for delivery", ETA: "12 mins"}}
	msg := PollIMTrackingCmd(b, "IM1", 12.9, 77.6)()
	m, ok := msg.(IMTrackingPolledMsg)
	if !ok || m.Tracking.Status != "Out for delivery" {
		t.Fatalf("got %#v", msg)
	}
}

func TestPullAndLoadIMCart(t *testing.T) {
	b := &fakeBackend{imCart: api.IMCart{Total: 300}}
	if m, ok := PullIMCart(b)().(IMCartPulledMsg); !ok || m.Cart.Total != 300 {
		t.Fatalf("PullIMCart msg = %#v", m)
	}
	if m, ok := LoadIMCart(b)().(IMCartLoadedMsg); !ok || m.Cart.Total != 300 {
		t.Fatalf("LoadIMCart msg = %#v", m)
	}
}
