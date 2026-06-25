package datasource

import (
	"testing"

	"console.store/internal/broker/api"
	"console.store/internal/catalog"
)

type fakeRPC struct {
	lastAccount string
	lastQuery   string
}

func (f *fakeRPC) Addresses(accountID string) ([]api.Address, error) {
	f.lastAccount = accountID
	return []api.Address{{ID: "a1"}}, nil
}
func (f *fakeRPC) Restaurants(accountID, addressID, query string) ([]api.Restaurant, error) {
	f.lastAccount, f.lastQuery = accountID, query
	return []api.Restaurant{{ID: "r1"}}, nil
}
func (f *fakeRPC) Menu(accountID, addressID, restaurantID string) (api.Menu, error) {
	f.lastAccount = accountID
	return api.Menu{RestaurantID: restaurantID}, nil
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

func TestBrokerBackendIsBackend(t *testing.T) {
	var _ Backend = NewBrokerBackend(&fakeRPC{}, "x")
}
