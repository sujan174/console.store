package datasource

import (
	"context"
	"testing"

	"console.store/internal/broker/api"
)

// fakeService records the accountID it was called with and returns canned data.
type fakeService struct {
	gotAccount string
	gotOrganic bool
}

func (f *fakeService) Addresses(_ context.Context, a string) ([]api.Address, error) {
	f.gotAccount = a
	return []api.Address{{ID: "addr1"}}, nil
}
func (f *fakeService) Restaurants(_ context.Context, a, _, _ string, organic bool) ([]api.Restaurant, string, error) {
	f.gotAccount, f.gotOrganic = a, organic
	return []api.Restaurant{{ID: "r1"}}, "corrected", nil
}
func (f *fakeService) Usuals(_ context.Context, a, _ string) ([]api.Restaurant, error) {
	f.gotAccount = a
	return nil, nil
}
func (f *fakeService) Menu(_ context.Context, a, _, _ string) (api.Menu, error) {
	f.gotAccount = a
	return api.Menu{}, nil
}
func (f *fakeService) ItemOptions(_ context.Context, a, _, _, _, _ string) ([]api.OptionGroup, error) {
	f.gotAccount = a
	return nil, nil
}
func (f *fakeService) UpdateCart(_ context.Context, args api.UpdateCartArgs) (api.Cart, error) {
	f.gotAccount = args.AccountID
	return api.Cart{}, nil
}
func (f *fakeService) GetCart(_ context.Context, a, _, _ string) (api.Cart, error) {
	f.gotAccount = a
	return api.Cart{}, nil
}
func (f *fakeService) ClearCart(_ context.Context, a string) error {
	f.gotAccount = a
	return nil
}
func (f *fakeService) PlaceOrder(_ context.Context, a, _ string) (api.Order, error) {
	f.gotAccount = a
	return api.Order{}, nil
}

func TestInProcSatisfiesBrokerRPCAndForwardsAccount(t *testing.T) {
	f := &fakeService{}
	var _ brokerRPC = NewInProc(f) // compile-time: InProc satisfies brokerRPC

	be := NewBrokerBackend(NewInProc(f), "local")
	if _, err := be.Addresses(); err != nil {
		t.Fatalf("Addresses: %v", err)
	}
	if f.gotAccount != "local" {
		t.Fatalf("forwarded account = %q; want \"local\"", f.gotAccount)
	}
}

func TestInProcRestaurantsVsSearchOrganic(t *testing.T) {
	f := &fakeService{}
	p := NewInProc(f)

	// Restaurants drops the effective-query string and uses organic=false.
	if _, err := p.Restaurants("local", "addr1", "pizza"); err != nil {
		t.Fatalf("Restaurants: %v", err)
	}
	if f.gotOrganic {
		t.Fatal("Restaurants should call the service with organic=false")
	}

	// SearchOrganic keeps the effective query and uses organic=true.
	r, eff, err := p.SearchOrganic("local", "addr1", "piza")
	if err != nil || len(r) != 1 || eff != "corrected" {
		t.Fatalf("SearchOrganic = %v,%q,%v; want 1 result, \"corrected\", nil", r, eff, err)
	}
	if !f.gotOrganic {
		t.Fatal("SearchOrganic should call the service with organic=true")
	}
}
