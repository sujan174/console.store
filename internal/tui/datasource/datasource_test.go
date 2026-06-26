package datasource

import (
	"errors"
	"testing"

	"console.store/internal/broker/api"
	"console.store/internal/catalog"
	swiggysnap "console.store/internal/catalog/swiggy"
)

type fakeBackend struct {
	addrs       []api.Address
	rests       []api.Restaurant
	restaurants []api.Restaurant
	menu        api.Menu
	cart        api.Cart
	order       api.Order
	err         error
	updateCalls int
	placeCalls  int
}

func (f *fakeBackend) Addresses() ([]api.Address, error) { return f.addrs, f.err }
func (f *fakeBackend) Places(string, catalog.Section) ([]api.Restaurant, error) {
	return f.rests, f.err
}
func (f *fakeBackend) PlacesQuery(string, string) ([]api.Restaurant, error) {
	return f.rests, f.err
}
func (f *fakeBackend) Usuals(string) ([]api.Restaurant, error) { return f.restaurants, f.err }
func (f *fakeBackend) Menu(string, string) (api.Menu, error)   { return f.menu, f.err }
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
