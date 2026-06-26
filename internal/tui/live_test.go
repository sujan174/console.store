package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/broker/api"
	"console.store/internal/catalog"
	swiggysnap "console.store/internal/catalog/swiggy"
	"console.store/internal/config"
	"console.store/internal/tui/datasource"
	"console.store/internal/tui/render"
	"console.store/internal/tui/screens"
)

type liveFake struct {
	addrs []api.Address
	err   error
}

func (f *liveFake) Addresses() ([]api.Address, error) { return f.addrs, f.err }
func (f *liveFake) Places(string, catalog.Section) ([]api.Restaurant, error) {
	return nil, f.err
}
func (f *liveFake) PlacesQuery(string, string) ([]api.Restaurant, error) { return nil, f.err }
func (f *liveFake) SearchOrganic(string, string) ([]api.Restaurant, string, error) {
	return nil, "", f.err
}
func (f *liveFake) Usuals(string) ([]api.Restaurant, error) { return nil, f.err }
func (f *liveFake) Menu(string, string) (api.Menu, error)   { return api.Menu{}, f.err }
func (f *liveFake) ItemOptions(string, string, string, string) ([]api.OptionGroup, error) {
	return nil, f.err
}
func (f *liveFake) UpdateCart(string, string, string, []api.CartItem) (api.Cart, error) {
	return api.Cart{}, f.err
}
func (f *liveFake) GetCart(string, string) (api.Cart, error) { return api.Cart{}, f.err }
func (f *liveFake) ClearCart() error                         { return f.err }
func (f *liveFake) PlaceOrder(string) (api.Order, error)     { return api.Order{}, f.err }

func TestMockPathUnaffected(t *testing.T) {
	m := New(render.Caps{})
	if m.live {
		t.Fatal("default New must not be live")
	}
	if len(m.repo.Addresses()) == 0 {
		t.Fatal("mock repo should have seed addresses")
	}
}

func TestLiveAddressesMsgAdoptsAddress(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	be := &liveFake{addrs: []api.Address{{ID: "live-1", Label: "home"}}}
	m := New(render.Caps{}, WithLiveBackend(be, snap, "acct-1", "https://authz/x"))
	if !m.live {
		t.Fatal("expected live model")
	}
	snap.SetAddresses([]catalog.Address{{ID: "live-1", Label: "home"}})
	updated, _ := m.Update(datasource.AddressesLoadedMsg{})
	if updated.(Model).addr.ID != "live-1" {
		t.Fatalf("model did not adopt live address: %+v", updated.(Model).addr)
	}
}

func TestLiveNeedsAuthOnAuthError(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "acct-1", "https://authz/x"))
	updated, _ := m.Update(datasource.AddressesLoadedMsg{Err: datasource.ErrNeedsAuth})
	if !updated.(Model).needsAuth {
		t.Fatal("expected needsAuth after ErrNeedsAuth load")
	}
}

func TestLiveMenuEnterFiresLoadMenu(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	// Home's "popular near you" reads the first category's query ("coffee").
	snap.SetPlaces("a1", "coffee", []catalog.Place{
		{ID: "r1", SwiggyID: "swiggy-r1", Name: "Blue Tokai", Section: catalog.SectionCoffee},
	})
	be := &liveFake{}
	m := New(render.Caps{},
		WithLiveBackend(be, snap, "acct-1", ""),
		WithSeededSnapshot(),
		WithChips([]config.Category{{Label: "Coffee", Query: "coffee"}}),
	)
	// Force window size so menu renders.
	m.w, m.h = 100, 40
	// Navigate to the menu screen (model starts at scrSplash).
	m.screen = scrMenu
	// Focus the main list (the browse now lands on the rail; → moves to the list).
	m.railFocus = false

	// Simulate pressing enter on the menu (restaurant is first in the Home list).
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um := updated.(Model)
	if um.screen != scrRestaurant {
		t.Fatalf("screen = %v after enter; want scrRestaurant", um.screen)
	}
	if cmd == nil {
		t.Fatal("live mode: enter on menu must return a non-nil LoadMenu Cmd")
	}
}

func TestSeededPathSkipsLiveLoads(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	// Pre-seed the snapshot with an address (simulates what sshd does from config).
	snap.SetAddresses([]catalog.Address{{ID: "seed-addr", Label: "home"}})
	be := &liveFake{}
	m := New(render.Caps{},
		WithLiveBackend(be, snap, "acct-1", "https://authz/x"),
		WithSeededSnapshot(),
	)
	if !m.seeded {
		t.Fatal("expected seeded=true")
	}
	// addr should be picked up from the seeded snapshot, not the mock fallback.
	if m.addr.ID != "seed-addr" {
		t.Fatalf("addr.ID = %q; want seed-addr", m.addr.ID)
	}
	// Init() should not return a batch that includes LoadAddresses/LoadPlaces when seeded.
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init must return tick() even when seeded")
	}
	// Seeded skips re-fetching ADDRESSES, but still loads the Home view (usuals +
	// the popular list) for the seeded address — otherwise the browse is empty.
	if c := m.liveInitCmds(); c == nil {
		t.Fatal("seeded liveInitCmds must load Home (usuals + popular) for the seeded address")
	}
}

func TestLiveCartSyncFires(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	place := catalog.Place{
		ID: "r1", SwiggyID: "swiggy-r1", Name: "Blue Tokai",
		Section: catalog.SectionCoffee,
		Items:   []catalog.Item{{ID: "i1", SwiggyID: "swiggy-i1", Name: "Latte", Price: 250, Veg: true}},
	}
	snap.SetPlaces("a1", string(catalog.SectionCoffee), []catalog.Place{place})
	snap.SetMenu(place)
	be := &liveFake{}
	m := New(render.Caps{},
		WithLiveBackend(be, snap, "acct-1", ""),
		WithSeededSnapshot(),
	)
	m.w, m.h = 100, 40

	// Skip to scrRestaurant with the restaurant loaded.
	m.screen = scrRestaurant
	m.rest = screens.NewRestaurant(place, m.qtyMap(), m.cartChip()).WithAddr(m.addr)

	// Add an item — in live mode with SwiggyID set, must return a SyncCart cmd.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("adding item in live mode must return a SyncCart cmd")
	}
}

// TestCartPlaceIDResolvesNonCoffeeChip is the regression for "Pizza Hut didn't
// sync": cartPlaceID resolved the cart restaurant only across the fixed mock
// sections, so a restaurant opened under a chip whose query != a section name
// (e.g. "pizza") resolved to "" and cart sync silently no-op'd.
func TestCartPlaceIDResolvesNonCoffeeChip(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	// Pizza Hut lives under the "pizza" chip query — NOT a mock section key.
	ph := catalog.Place{ID: "ph1", SwiggyID: "ph1", Name: "Pizza Hut"}
	snap.SetPlaces("a1", "pizza", []catalog.Place{ph})
	snap.SetMenu(catalog.Place{ID: "ph1", SwiggyID: "ph1", Items: []catalog.Item{
		{ID: "p1", SwiggyID: "p1", Name: "Margherita", Price: 300},
	}})
	be := &liveFake{}
	m := New(render.Caps{},
		WithLiveBackend(be, snap, "acct-1", ""),
		WithSeededSnapshot(),
		WithChips([]config.Category{{Label: "Pizza", Query: "pizza"}}),
	)
	m.w, m.h = 100, 40
	m.cartRestaurant = "Pizza Hut"

	if got := m.cartPlaceID(); got != "ph1" {
		t.Fatalf("cartPlaceID for a pizza-chip restaurant = %q, want \"ph1\"", got)
	}
}

func TestLivePlaceOrderTransitionsToConfirm(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	be := &liveFake{}
	m := New(render.Caps{},
		WithLiveBackend(be, snap, "acct-1", ""),
		WithSeededSnapshot(),
	)
	// Put model on scrCheckout with a line in the cart.
	m.screen = scrCheckout
	m.lines = []screens.CartLine{{Item: catalog.Item{ID: "i1", Name: "Latte", Price: 250}, Qty: 1}}
	m.cartRestaurant = "Blue Tokai"
	m.checkout = screens.NewCheckout("Blue Tokai", m.addr, m.lines, "~35 min")

	// Press enter → should set placingOrder=true and return a PlaceOrderCmd.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um := updated.(Model)
	if !um.placingOrder {
		t.Fatal("expected placingOrder=true after checkout enter in live mode")
	}
	if cmd == nil {
		t.Fatal("expected PlaceOrderCmd to be returned")
	}

	// Simulate OrderPlacedMsg success.
	updated2, _ := um.Update(datasource.OrderPlacedMsg{
		Order: api.Order{ID: "order-99", Status: "placed"},
	})
	um2 := updated2.(Model)
	if um2.screen != scrConfirm {
		t.Fatalf("screen = %v after OrderPlacedMsg; want scrConfirm", um2.screen)
	}
	if um2.placingOrder {
		t.Fatal("placingOrder must be cleared after success")
	}
}

func TestRailCategoryFiresQuery(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	be := &liveFake{}
	m := New(render.Caps{},
		WithLiveBackend(be, snap, "acct-1", ""),
		WithSeededSnapshot(),
		WithChips([]config.Category{{Label: "Coffee", Query: "coffee"}, {Label: "Pizza", Query: "pizza"}}),
	)
	m.w, m.h = 100, 40
	m.screen = scrMenu
	// ← focuses the rail
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	um2 := m2.(Model)
	if !um2.railFocus {
		t.Fatal("← must focus the rail")
	}
	// ↓ moves rail to Coffee (index 2, first category)
	m3, _ := um2.Update(tea.KeyMsg{Type: tea.KeyDown})
	um3 := m3.(Model)
	// Enter commits to that category and must fire a LoadPlacesQuery cmd
	m4, cmd := um3.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = m4
	if cmd == nil {
		t.Fatal("selecting a category on the rail must fire a places query")
	}
}

func TestLivePlaceOrderErrShowsError(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	be := &liveFake{}
	m := New(render.Caps{},
		WithLiveBackend(be, snap, "acct-1", ""),
		WithSeededSnapshot(),
	)
	m.screen = scrCheckout
	m.lines = []screens.CartLine{{Item: catalog.Item{ID: "i1", Name: "Latte", Price: 250}, Qty: 1}}
	m.placingOrder = true

	updated, _ := m.Update(datasource.OrderPlacedMsg{
		Err: errors.New("order failed: restaurant closed"),
	})
	um := updated.(Model)
	if um.screen != scrCheckout {
		t.Fatalf("screen = %v after error; want scrCheckout", um.screen)
	}
	if um.placingOrder {
		t.Fatal("placingOrder must be cleared after error")
	}
	if um.orderErr == "" {
		t.Fatal("orderErr must be set after PlaceOrder error")
	}
}

func (f *liveFake) Logout() error { return f.err }
