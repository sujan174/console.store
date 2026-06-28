package datasource

import (
	"errors"
	"testing"

	"console.store/internal/broker/api"
	"console.store/internal/catalog"
)

type fakeRPC struct {
	lastAccount string
	lastQuery   string
	err         error // if set, all methods return this error
}

func (f *fakeRPC) Addresses(accountID string) ([]api.Address, error) {
	f.lastAccount = accountID
	if f.err != nil {
		return nil, f.err
	}
	return []api.Address{{ID: "a1"}}, nil
}
func (f *fakeRPC) Restaurants(accountID, addressID, query string) ([]api.Restaurant, error) {
	f.lastAccount, f.lastQuery = accountID, query
	if f.err != nil {
		return nil, f.err
	}
	return []api.Restaurant{{ID: "r1"}}, nil
}
func (f *fakeRPC) SearchOrganic(accountID, addressID, query string) ([]api.Restaurant, string, error) {
	f.lastAccount, f.lastQuery = accountID, query
	if f.err != nil {
		return nil, query, f.err
	}
	return []api.Restaurant{{ID: "r1"}}, query, nil
}
func (f *fakeRPC) Usuals(accountID, addressID string) ([]api.Restaurant, error) {
	f.lastAccount = accountID
	return nil, f.err
}
func (f *fakeRPC) Menu(accountID, addressID, restaurantID string) (api.Menu, error) {
	f.lastAccount = accountID
	if f.err != nil {
		return api.Menu{}, f.err
	}
	return api.Menu{RestaurantID: restaurantID}, nil
}
func (f *fakeRPC) ItemOptions(accountID, addressID, restaurantID, itemName, menuItemID string) ([]api.OptionGroup, error) {
	f.lastAccount = accountID
	return nil, nil
}
func (f *fakeRPC) UpdateCart(a api.UpdateCartArgs) (api.Cart, error) {
	f.lastAccount = a.AccountID
	if f.err != nil {
		return api.Cart{}, f.err
	}
	return api.Cart{}, nil
}
func (f *fakeRPC) GetCart(accountID, addressID, restaurantName string) (api.Cart, error) {
	f.lastAccount = accountID
	if f.err != nil {
		return api.Cart{}, f.err
	}
	return api.Cart{}, nil
}
func (f *fakeRPC) ClearCart(accountID string) error { f.lastAccount = accountID; return nil }
func (f *fakeRPC) PlaceOrder(accountID, addressID string) (api.Order, error) {
	f.lastAccount = accountID
	if f.err != nil {
		return api.Order{}, f.err
	}
	return api.Order{}, nil
}

func TestBrokerBackendPinsAccountAndMapsSection(t *testing.T) {
	rpc := &fakeRPC{}
	be := NewBrokerBackend(rpc, "acct-7")

	if _, err := be.Addresses(); err != nil || rpc.lastAccount != "acct-7" {
		t.Fatalf("addresses: account=%q err=%v", rpc.lastAccount, err)
	}
	if _, err := be.Places("a1", catalog.SectionCoffee); err != nil {
		t.Fatal(err)
	}
	if rpc.lastAccount != "acct-7" || rpc.lastQuery == "" {
		t.Fatalf("places: account=%q query=%q (query should map from section)", rpc.lastAccount, rpc.lastQuery)
	}
	if _, err := be.Menu("a1", "r1"); err != nil || rpc.lastAccount != "acct-7" {
		t.Fatalf("menu: account=%q err=%v", rpc.lastAccount, err)
	}
}

func TestBrokerBackendUpdateCartPinsAccount(t *testing.T) {
	rpc := &fakeRPC{}
	be := NewBrokerBackend(rpc, "acct-7")
	items := []api.CartItem{{ItemID: "item-1", Quantity: 2}}
	if _, err := be.UpdateCart("a1", "r1", "Blue Tokai", items); err != nil {
		t.Fatal(err)
	}
	if rpc.lastAccount != "acct-7" {
		t.Fatalf("UpdateCart account = %q; want acct-7", rpc.lastAccount)
	}
}

func TestBrokerBackendPlaceOrderPinsAccount(t *testing.T) {
	rpc := &fakeRPC{}
	be := NewBrokerBackend(rpc, "acct-7")
	if _, err := be.PlaceOrder("a1"); err != nil {
		t.Fatal(err)
	}
	if rpc.lastAccount != "acct-7" {
		t.Fatalf("PlaceOrder account = %q; want acct-7", rpc.lastAccount)
	}
}

func TestBrokerBackendIsBackend(t *testing.T) {
	var _ Backend = NewBrokerBackend(&fakeRPC{}, "x")
}

func TestBrokerBackendWrapsAuthErr(t *testing.T) {
	cases := []struct {
		name    string
		errText string
	}{
		{"token expired", "swiggy: access token expired (401)"},
		{"account not authorized", "swiggy: access token expired (401) (account not authorized)"},
		{"session revoked", "swiggy: session revoked (419)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rpc := &fakeRPC{err: errors.New(tc.errText)}
			be := NewBrokerBackend(rpc, "x")
			_, err := be.Addresses()
			if !errors.Is(err, ErrNeedsAuth) {
				t.Fatalf("expected ErrNeedsAuth wrapping %q, got %v", tc.errText, err)
			}
		})
	}
}

func TestBrokerBackendNonAuthErrPassedThrough(t *testing.T) {
	rpc := &fakeRPC{err: errors.New("swiggy: http 500: internal server error")}
	be := NewBrokerBackend(rpc, "x")
	_, err := be.Addresses()
	if errors.Is(err, ErrNeedsAuth) {
		t.Fatal("non-auth error should not be wrapped as ErrNeedsAuth")
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func (f *fakeRPC) TrackOrder(accountID, orderID string) (api.Tracking, error) {
	f.lastAccount = accountID
	return api.Tracking{}, f.err
}
func (f *fakeRPC) ActiveFoodOrders(accountID, addressID string) ([]api.Order, error) {
	f.lastAccount = accountID
	return nil, f.err
}
func (f *fakeRPC) FoodOrders(accountID, addressID string, activeOnly bool) ([]api.Order, error) {
	f.lastAccount = accountID
	return nil, f.err
}
func (f *fakeRPC) Logout(accountID string) error { f.lastAccount = accountID; return nil }

func TestBrokerBackendLogoutForwardsAccount(t *testing.T) {
	f := &fakeRPC{}
	if err := NewBrokerBackend(f, "acct-9").Logout(); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if f.lastAccount != "acct-9" {
		t.Fatalf("Logout forwarded account %q, want acct-9", f.lastAccount)
	}
}
