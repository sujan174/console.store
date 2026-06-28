package cli

import (
	"bytes"
	"strings"
	"testing"

	"console.store/internal/broker/api"
)

// fakeBackend implements cli.Backend for tests. No network, no real orders.
type fakeBackend struct {
	addrs    []api.Address
	cart     api.Cart
	placed   api.Order
	active   []api.Order
	tracking api.Tracking
	placeErr error
	updErr   error
	getErr   error
	placeN   int
}

func (f *fakeBackend) Addresses() ([]api.Address, error) { return f.addrs, nil }
func (f *fakeBackend) UpdateCart(_, _, _ string, _ []api.CartItem) (api.Cart, error) {
	return f.cart, f.updErr
}
func (f *fakeBackend) GetCart(_, _ string) (api.Cart, error)    { return f.cart, f.getErr }
func (f *fakeBackend) PlaceOrder(string) (api.Order, error)     { f.placeN++; return f.placed, f.placeErr }
func (f *fakeBackend) ActiveOrders(string) ([]api.Order, error) { return f.active, nil }
func (f *fakeBackend) TrackOrder(string) (api.Tracking, error)  { return f.tracking, nil }

func TestDispatchHelp(t *testing.T) {
	var out bytes.Buffer
	code := Dispatch([]string{"help"}, Deps{Out: &out})
	if code != 0 {
		t.Fatalf("help exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "store order") || !strings.Contains(out.String(), "store status") {
		t.Fatalf("help should list commands:\n%s", out.String())
	}
}

func TestDispatchUnknown(t *testing.T) {
	var out bytes.Buffer
	code := Dispatch([]string{"frobnicate"}, Deps{Out: &out})
	if code == 0 {
		t.Fatal("unknown command must return non-zero")
	}
}

func TestDispatchNeedsAuth(t *testing.T) {
	var out bytes.Buffer
	code := Dispatch([]string{"status"}, Deps{Out: &out, SignedIn: false, Backend: &fakeBackend{}})
	if code == 0 {
		t.Fatal("a backend command while signed-out must return non-zero")
	}
	if !strings.Contains(strings.ToLower(out.String()), "run `store`") && !strings.Contains(out.String(), "sign in") {
		t.Fatalf("should tell the user to sign in:\n%s", out.String())
	}
}
