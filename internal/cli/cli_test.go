package cli

import (
	"bytes"
	"strings"
	"testing"

	"consolestore/internal/broker/api"
)

// fakeBackend implements cli.Backend for tests. No network, no real orders.
type fakeBackend struct {
	addrs     []api.Address
	cart      api.Cart
	placed    api.Order
	active    []api.Order
	tracking  api.Tracking
	placeErr  error
	updErr    error
	getErr    error
	logoutErr error
	placeN    int
	logoutN   int
}

func (f *fakeBackend) Addresses() ([]api.Address, error) { return f.addrs, nil }
func (f *fakeBackend) UpdateCart(_, _, _ string, _ []api.CartItem) (api.Cart, error) {
	return f.cart, f.updErr
}
func (f *fakeBackend) GetCart(_, _ string) (api.Cart, error)    { return f.cart, f.getErr }
func (f *fakeBackend) PlaceOrder(string) (api.Order, error)     { f.placeN++; return f.placed, f.placeErr }
func (f *fakeBackend) ActiveOrders(string) ([]api.Order, error) { return f.active, nil }
func (f *fakeBackend) TrackOrder(string) (api.Tracking, error)  { return f.tracking, nil }
func (f *fakeBackend) Logout() error                            { f.logoutN++; return f.logoutErr }

func TestLogoutAndWhoami(t *testing.T) {
	var out bytes.Buffer
	be := &fakeBackend{addrs: []api.Address{{Label: "Home", Line: "FD 46 Enclave, Bengaluru"}}}

	// logout while signed in disconnects.
	if code := Dispatch([]string{"logout"}, Deps{SignedIn: true, Out: &out, Backend: be}); code != 0 {
		t.Fatalf("logout exit = %d", code)
	}
	if be.logoutN != 1 {
		t.Fatalf("logout should call Logout once, got %d", be.logoutN)
	}

	// logout while signed out is a no-op (does not call Logout).
	be.logoutN = 0
	out.Reset()
	Dispatch([]string{"logout"}, Deps{SignedIn: false, Out: &out, Backend: be})
	if be.logoutN != 0 {
		t.Fatal("logout while signed out must not call Logout")
	}

	// whoami signed in shows the address.
	out.Reset()
	Dispatch([]string{"whoami"}, Deps{SignedIn: true, Out: &out, Backend: be})
	if !strings.Contains(out.String(), "Home") {
		t.Fatalf("whoami should list saved addresses:\n%s", out.String())
	}

	// whoami signed out says so.
	out.Reset()
	Dispatch([]string{"whoami"}, Deps{SignedIn: false, Out: &out, Backend: be})
	if !strings.Contains(strings.ToLower(out.String()), "not signed in") {
		t.Fatalf("whoami signed out should say so:\n%s", out.String())
	}
}

func TestDispatchHelp(t *testing.T) {
	var out bytes.Buffer
	code := Dispatch([]string{"help"}, Deps{Out: &out})
	if code != 0 {
		t.Fatalf("help exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "console order") || !strings.Contains(out.String(), "console status") {
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
	if !strings.Contains(strings.ToLower(out.String()), "run `console`") && !strings.Contains(out.String(), "sign in") {
		t.Fatalf("should tell the user to sign in:\n%s", out.String())
	}
}
