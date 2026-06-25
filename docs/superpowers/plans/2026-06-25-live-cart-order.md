# Live Cart Sync + Order Placement · Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire real Swiggy cart sync (eager — every item add/remove) and live order placement (PlaceOrder RPC through the existing COD confirmation gate) into the TUI, gated behind `m.live`.

**Architecture:** `datasource.Backend` gains `UpdateCart` and `PlaceOrder` methods. `BrokerBackend` implements them with `accountID` pinned. A `liveSyncCart()` helper on `Model` assembles `api.CartItem` slices from `m.lines` using `Item.SwiggyID` and dispatches a `SyncCart` Cmd; it is called after every cart-mutating key in `app.go`. The `scrCheckout` enter key in live mode sets `m.placingOrder = true` and fires a `PlaceOrderCmd` instead of the fake path. `OrderPlacedMsg` transitions to `scrConfirm` with the real order ID. Cart sync errors (non-fatal) show in the status bar; order errors stay on `scrCheckout` for retry. A `WithPlacing(bool)` builder on `Checkout` changes the CTA bar text to "placing order…" while in-flight.

**Tech Stack:** Go 1.26, bubbletea `Cmd`/`Msg`, `internal/broker/api` (`UpdateCartArgs`, `CartItem`, `Order`), existing `datasource.SyncCart`/`PlaceOrderCmd` pattern.

## Global Constraints

- Module `console.store`; Go floor `go 1.26.4`. `gofmt` clean, `go vet ./...` clean.
- **Real orders are real money (COD, non-cancellable).** `PlaceOrderCmd` fires ONLY when `m.live && !m.placingOrder` and the user explicitly presses `enter` on `scrCheckout`. Double-fire is blocked by `m.placingOrder`.
- Mock path (`m.live == false`) keeps the existing fake `m.checkout.Placed(orderID(...), "~40 min")` flow unchanged.
- Cart sync errors are non-fatal: shown in status bar, TUI continues.
- `imLines` (Instamart cart) is NOT synced live in this slice — food cart only.
- `api.CartItem` is `{ItemID string, Quantity int}` (from `internal/broker/api`).
- Items with empty `SwiggyID` are skipped in cart sync (they cannot be referenced by Swiggy).

---

### Task 1: `datasource` — `UpdateCart`/`PlaceOrder` on Backend + new Cmds/Msgs

**Files:**
- Modify: `internal/tui/datasource/datasource.go`
- Modify: `internal/tui/datasource/datasource_test.go`

**Interfaces:**
- Consumes: `internal/broker/api` (`CartItem`, `Cart`, `Order`)
- Produces:
  ```go
  // Extended Backend interface:
  type Backend interface {
      Addresses() ([]api.Address, error)
      Places(addressID string, section catalog.Section) ([]api.Restaurant, error)
      Menu(addressID, restaurantID string) (api.Menu, error)
      UpdateCart(addressID, restaurantID, restaurantName string, items []api.CartItem) (api.Cart, error)
      PlaceOrder(addressID string) (api.Order, error)
  }
  type CartSyncedMsg struct{ Err error }
  type OrderPlacedMsg struct {
      Order api.Order
      Err   error
  }
  func SyncCart(b Backend, snap *swiggysnap.Snapshot, addressID, restaurantID, restaurantName string, items []api.CartItem) tea.Cmd
  func PlaceOrderCmd(b Backend, addressID string) tea.Cmd
  ```

- [ ] **Step 1: Add `UpdateCart` + `PlaceOrder` to `fakeBackend` in the test file first**

In `internal/tui/datasource/datasource_test.go`, add methods to `fakeBackend` and add new test functions. Replace the full test file:

```go
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
func (f *fakeBackend) Menu(string, string) (api.Menu, error) { return f.menu, f.err }
func (f *fakeBackend) UpdateCart(string, string, string, []api.CartItem) (api.Cart, error) {
	f.updateCalls++
	return f.cart, f.err
}
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
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/tui/datasource/ -v`
Expected: FAIL — `SyncCart`, `PlaceOrderCmd`, `CartSyncedMsg`, `OrderPlacedMsg` undefined; `fakeBackend` missing `UpdateCart`/`PlaceOrder`.

- [ ] **Step 3: Update `internal/tui/datasource/datasource.go`**

Replace the full file:

```go
// Package datasource wires the broker (via internal/broker/api) into the TUI as
// async bubbletea Cmds that fill a catalog/swiggy.Snapshot. The TUI reads the
// Snapshot synchronously through a swiggy.Repository; these Cmds are the only
// thing that performs broker I/O. The TUI never imports swiggy/store/auth.
package datasource

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/broker/api"
	"console.store/internal/catalog"
	swiggysnap "console.store/internal/catalog/swiggy"
)

// ErrNeedsAuth signals the account has no usable token; the Model shows the
// authorize gate. Backends should return it (or wrap it) on a missing-token
// error from the broker.
var ErrNeedsAuth = errors.New("datasource: account not authorized")

// Backend abstracts the data source for async load Cmds. The broker-backed
// BrokerBackend implements it for live use; tests use a fake.
type Backend interface {
	Addresses() ([]api.Address, error)
	Places(addressID string, section catalog.Section) ([]api.Restaurant, error)
	Menu(addressID, restaurantID string) (api.Menu, error)
	UpdateCart(addressID, restaurantID, restaurantName string, items []api.CartItem) (api.Cart, error)
	PlaceOrder(addressID string) (api.Order, error)
}

type (
	AddressesLoadedMsg struct{ Err error }
	PlacesLoadedMsg    struct {
		Section catalog.Section
		Err     error
	}
	MenuLoadedMsg struct {
		PlaceID string
		Err     error
	}
	CartSyncedMsg struct{ Err error }
	OrderPlacedMsg struct {
		Order api.Order
		Err   error
	}
)

func LoadAddresses(b Backend, snap *swiggysnap.Snapshot) tea.Cmd {
	return func() tea.Msg {
		got, err := b.Addresses()
		if err != nil {
			return AddressesLoadedMsg{Err: err}
		}
		snap.SetAddresses(toAddresses(got))
		return AddressesLoadedMsg{}
	}
}

func LoadPlaces(b Backend, snap *swiggysnap.Snapshot, addressID string, section catalog.Section) tea.Cmd {
	return func() tea.Msg {
		got, err := b.Places(addressID, section)
		if err != nil {
			return PlacesLoadedMsg{Section: section, Err: err}
		}
		snap.SetPlaces(addressID, section, toPlaces(got, section))
		return PlacesLoadedMsg{Section: section}
	}
}

func LoadMenu(b Backend, snap *swiggysnap.Snapshot, addressID, restaurantID string) tea.Cmd {
	return func() tea.Msg {
		got, err := b.Menu(addressID, restaurantID)
		if err != nil {
			return MenuLoadedMsg{PlaceID: restaurantID, Err: err}
		}
		snap.SetMenu(toMenuPlace(got))
		return MenuLoadedMsg{PlaceID: restaurantID}
	}
}

// SyncCart calls UpdateCart on the backend with the current cart contents and
// records the returned cart in the snapshot. Errors are non-fatal: the TUI shows
// them in the status bar and continues.
func SyncCart(b Backend, snap *swiggysnap.Snapshot, addressID, restaurantID, restaurantName string, items []api.CartItem) tea.Cmd {
	return func() tea.Msg {
		_, err := b.UpdateCart(addressID, restaurantID, restaurantName, items)
		return CartSyncedMsg{Err: err}
	}
}

// PlaceOrderCmd submits the order through the broker. The TUI must have already
// synced the cart via SyncCart before calling this. On success the broker returns
// the placed order; on failure the TUI shows the error and stays on scrCheckout.
func PlaceOrderCmd(b Backend, snap *swiggysnap.Snapshot, addressID string) tea.Cmd {
	return func() tea.Msg {
		order, err := b.PlaceOrder(addressID)
		return OrderPlacedMsg{Order: order, Err: err}
	}
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/tui/datasource/ -v`
Expected: ALL PASS (8 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/tui/datasource/datasource.go internal/tui/datasource/datasource_test.go
git commit -m "feat(tui/datasource): UpdateCart + PlaceOrder on Backend + SyncCart/PlaceOrderCmd"
```

---

### Task 2: `BrokerBackend` implements `UpdateCart` + `PlaceOrder`

**Files:**
- Modify: `internal/tui/datasource/broker_backend.go`
- Modify: `internal/tui/datasource/broker_backend_test.go`

**Interfaces:**
- Consumes: `api.Client.UpdateCart(UpdateCartArgs)`, `api.Client.PlaceOrder(accountID, addressID)`
- Produces: `BrokerBackend` satisfies the extended `Backend` interface

- [ ] **Step 1: Write the failing test**

Add to `internal/tui/datasource/broker_backend_test.go` (append after existing tests):

```go
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
```

Also add `UpdateCart` and `PlaceOrder` to `fakeRPC` in the same file:

```go
func (f *fakeRPC) UpdateCart(a api.UpdateCartArgs) (api.Cart, error) {
	f.lastAccount = a.AccountID
	return api.Cart{}, nil
}
func (f *fakeRPC) PlaceOrder(accountID, addressID string) (api.Order, error) {
	f.lastAccount = accountID
	return api.Order{ID: "test-order"}, nil
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/tui/datasource/ -run TestBrokerBackend -v`
Expected: FAIL — `fakeRPC` missing `UpdateCart`/`PlaceOrder`; `BrokerBackend` missing methods.

- [ ] **Step 3: Update `brokerRPC` interface and add methods in `broker_backend.go`**

Replace the full file:

```go
package datasource

import (
	"console.store/internal/broker/api"
	"console.store/internal/catalog"
)

type brokerRPC interface {
	Addresses(accountID string) ([]api.Address, error)
	Restaurants(accountID, addressID, query string) ([]api.Restaurant, error)
	Menu(accountID, addressID, restaurantID string) (api.Menu, error)
	UpdateCart(a api.UpdateCartArgs) (api.Cart, error)
	PlaceOrder(accountID, addressID string) (api.Order, error)
}

// BrokerBackend adapts the broker RPC client into a datasource.Backend, pinned
// to ONE account id (resolved from the SSH session's pubkey by cmd/sshd). The
// account id is fixed at construction and never read from a call argument, so a
// session can only ever act as its own account.
type BrokerBackend struct {
	rpc       brokerRPC
	accountID string
}

func NewBrokerBackend(rpc brokerRPC, accountID string) *BrokerBackend {
	return &BrokerBackend{rpc: rpc, accountID: accountID}
}

func (b *BrokerBackend) Addresses() ([]api.Address, error) {
	return b.rpc.Addresses(b.accountID)
}

func (b *BrokerBackend) Places(addressID string, section catalog.Section) ([]api.Restaurant, error) {
	return b.rpc.Restaurants(b.accountID, addressID, sectionQuery(section))
}

func (b *BrokerBackend) Menu(addressID, restaurantID string) (api.Menu, error) {
	return b.rpc.Menu(b.accountID, addressID, restaurantID)
}

func (b *BrokerBackend) UpdateCart(addressID, restaurantID, restaurantName string, items []api.CartItem) (api.Cart, error) {
	return b.rpc.UpdateCart(api.UpdateCartArgs{
		AccountID:      b.accountID,
		AddressID:      addressID,
		RestaurantID:   restaurantID,
		RestaurantName: restaurantName,
		Items:          items,
	})
}

func (b *BrokerBackend) PlaceOrder(addressID string) (api.Order, error) {
	return b.rpc.PlaceOrder(b.accountID, addressID)
}

// sectionQuery maps a catalogue lane to a Swiggy restaurant-search query.
func sectionQuery(s catalog.Section) string {
	switch s {
	case catalog.SectionCoffee:
		return "coffee"
	case catalog.SectionFood:
		return "food"
	case catalog.SectionSnacks:
		return "snacks"
	default:
		return string(s)
	}
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/tui/datasource/ -v`
Expected: ALL PASS (10 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/tui/datasource/broker_backend.go internal/tui/datasource/broker_backend_test.go
git commit -m "feat(tui/datasource): BrokerBackend implements UpdateCart + PlaceOrder"
```

---

### Task 3: `Checkout` screen — `WithPlacing(bool)` builder

**Files:**
- Modify: `internal/tui/screens/checkout.go`
- Modify: `internal/tui/screens/checkout_test.go` (if it exists, else create)

**Interfaces:**
- Consumes: existing `Checkout` struct
- Produces:
  ```go
  func (c Checkout) WithPlacing(placing bool) Checkout
  // summaryView: when c.placing == true, CTA bar shows "placing order…" instead of "> place order"
  ```

- [ ] **Step 1: Write the failing test**

Check if `internal/tui/screens/checkout_test.go` exists:
```bash
ls internal/tui/screens/checkout_test.go 2>/dev/null && echo exists || echo missing
```

If missing, create it. Add this test (append to existing if file exists):

```go
// internal/tui/screens/checkout_test.go
package screens

import (
	"strings"
	"testing"

	"console.store/internal/catalog"
)

func TestCheckoutWithPlacingChangesCTA(t *testing.T) {
	addr := catalog.Address{ID: "a1", Label: "home", Line: "HSR Layout"}
	lines := []CartLine{{Item: catalog.Item{ID: "i1", Name: "Cold Coffee", Price: 220}, Qty: 1}}
	c := NewCheckout("Blue Tokai", addr, lines, "~35 min")

	normal := c.View(0)
	if !strings.Contains(normal, "place order") {
		t.Errorf("normal view should contain 'place order'; got:\n%s", normal)
	}

	placing := c.WithPlacing(true).View(0)
	if !strings.Contains(placing, "placing") {
		t.Errorf("placing view should contain 'placing'; got:\n%s", placing)
	}
	if strings.Contains(placing, "> place order") {
		t.Errorf("placing view should NOT show '> place order' CTA; got:\n%s", placing)
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/tui/screens/ -run TestCheckoutWithPlacing -v`
Expected: FAIL — `WithPlacing` undefined.

- [ ] **Step 3: Add `placing bool` field and `WithPlacing` + update `summaryView` in `checkout.go`**

Add `placing bool` to the `Checkout` struct (after `eta string`):

```go
type Checkout struct {
	restaurant string
	addr       catalog.Address
	lines      []CartLine
	placed     bool
	orderID    string
	eta        string
	placing    bool // true while PlaceOrderCmd is in-flight
}
```

Add `WithPlacing` builder after `Placed`:

```go
// WithPlacing returns a copy in the "placing order" in-flight state (disables the CTA).
func (c Checkout) WithPlacing(placing bool) Checkout {
	c.placing = placing
	return c
}
```

In `summaryView`, replace the CTA bar block:

```go
	// Full-bleed place-order bar: green left bar + selected-row background.
	barLabel := " > place order "
	if c.placing {
		barLabel = " placing order… "
	}
	bar := theme.GreenStyle.Render("▌") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.Bright)).
			Background(lipgloss.Color(theme.SelRowBg)).
			Render(padTo(barLabel, components.FrameWidth()-1))
	b.WriteString(bar + "\n\n")
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/tui/screens/ -v 2>&1 | tail -15`
Expected: ALL PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/screens/checkout.go internal/tui/screens/checkout_test.go
git commit -m "feat(screens/checkout): WithPlacing builder — shows 'placing order…' CTA while in-flight"
```

---

### Task 4: `app.go` — eager cart sync + live order placement + Msg handlers

**Files:**
- Modify: `internal/tui/app.go` (fields; `liveSyncCart` helper; dispatch SyncCart from 4 cart-mutation sites; scrCheckout enter live path; CartSyncedMsg + OrderPlacedMsg handlers; statusBar shows errors; checkout View passes `WithPlacing`)
- Modify: `internal/tui/live_test.go` (add `TestLiveCartSyncFires`, `TestLivePlaceOrderTransitionsToConfirm`, `TestLivePlaceOrderErrShowsError`)

**Interfaces:**
- Consumes: `datasource.SyncCart`, `datasource.PlaceOrderCmd`, `datasource.CartSyncedMsg`, `datasource.OrderPlacedMsg`, `screens.Checkout.WithPlacing(bool)`, `api.CartItem`
- Produces: cart-mutating keys dispatch `SyncCart`; checkout enter dispatches `PlaceOrderCmd`; Msgs drive state transitions

- [ ] **Step 1: Write the failing tests**

Add to `internal/tui/live_test.go`:

```go
func TestLiveCartSyncFires(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	snap.SetPlaces("a1", catalog.SectionCoffee, []catalog.Place{
		{ID: "r1", SwiggyID: "swiggy-r1", Name: "Blue Tokai", Section: catalog.SectionCoffee},
	})
	snap.SetMenu(catalog.Place{
		ID: "r1", SwiggyID: "swiggy-r1", Name: "Blue Tokai",
		Items: []catalog.Item{{ID: "i1", SwiggyID: "swiggy-i1", Name: "Latte", Price: 250, Veg: true}},
	})
	be := &liveFake{}
	m := New(render.Caps{},
		WithLiveBackend(be, snap, "acct-1", ""),
		WithSeededSnapshot(),
	)
	m.w, m.h = 100, 40

	// Navigate to the restaurant and add an item.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // enter restaurant
	// Simulate MenuLoadedMsg arriving.
	m3, _ := m2.(Model).Update(datasource.MenuLoadedMsg{PlaceID: "r1"})
	// Now add item (enter on restaurant screen).
	_, cmd := m3.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("adding item in live mode must return a SyncCart cmd")
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
```

Also add `"errors"` and `screens` import to `live_test.go` if not already present:
```go
import (
    "errors"
    "testing"

    "console.store/internal/broker/api"
    "console.store/internal/catalog"
    swiggysnap "console.store/internal/catalog/swiggy"
    "console.store/internal/tui/datasource"
    "console.store/internal/tui/render"
    "console.store/internal/tui/screens"
)
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/tui/ -run "TestLiveCart|TestLivePlace" -v`
Expected: FAIL — `placingOrder`, `orderErr`, `liveSyncCart` etc. undefined.

- [ ] **Step 3: Add fields to `Model` struct in `internal/tui/app.go`**

In the `type Model struct` block, after `needsAuth bool` and `seeded bool`, add:

```go
	placingOrder bool   // true while PlaceOrderCmd is in-flight; blocks double-fire
	cartSyncErr  string // last cart-sync error; shown in status bar (non-fatal)
	orderErr     string // last order-placement error; shown in status bar
```

Also add `"console.store/internal/broker/api"` to the import block in `app.go`.

- [ ] **Step 4: Add `liveSyncCart` helper to `internal/tui/app.go`**

Add after `errIsNeedsAuth`:

```go
// liveSyncCart assembles the current food cart and dispatches a SyncCart Cmd
// to keep Swiggy's cart in sync. No-op when not live, cart is empty, or the
// restaurant has no SwiggyID (e.g. if menu hasn't loaded yet). Items without
// a SwiggyID are skipped — they can't be referenced by Swiggy.
func (m Model) liveSyncCart() tea.Cmd {
	if !m.live || len(m.lines) == 0 {
		return nil
	}
	p, ok := m.repo.Menu(m.cartPlaceID())
	if !ok || p.SwiggyID == "" {
		return nil
	}
	items := make([]api.CartItem, 0, len(m.lines))
	for _, l := range m.lines {
		if l.Item.SwiggyID != "" {
			items = append(items, api.CartItem{ItemID: l.Item.SwiggyID, Quantity: l.Qty})
		}
	}
	if len(items) == 0 {
		return nil
	}
	return datasource.SyncCart(m.backend, m.snap, m.addr.ID, p.SwiggyID, m.cartRestaurant, items)
}
```

- [ ] **Step 5: Dispatch `liveSyncCart` from cart-mutation sites in `Update`**

Four sites where `m.lines` changes for food cart. Apply `return m, m.liveSyncCart()` at each exit.

**Site 1** — `scrRestaurant` `"enter/right/l"` (item add, no modal):

```go
			case "enter", "right", "l":
				it, ok := m.rest.Selected()
				if !ok {
					return m, nil
				}
				m = m.beginAdd(it, m.rest.PlaceData().Name, m.rest.PlaceData().Section)
				if m.customizeOpen || m.conflictOpen {
					return m, nil // a modal will finish the add
				}
				m = m.refreshAfterAdd()
				return m, m.liveSyncCart()  // ← was: return m, nil
```

**Site 2** — `scrRestaurant` `"left/h"` (item remove):

```go
			case "left", "h":
				it, ok := m.rest.Selected()
				if !ok {
					return m, nil
				}
				m.lines = decLastByItem(m.lines, it.ID)
				if len(m.lines) == 0 {
					m.cartRestaurant = ""
					m.cartSection = ""
				}
				m = m.refreshAfterAdd()
				return m, m.liveSyncCart()  // ← was: return m, nil
```

**Site 3** — conflict modal `"enter"` → `startNewCart`:

Find (around line 557):
```go
			case "enter":
				if m.conflictSel == 0 { // start new
					m = m.startNewCart(m.pendingItem, m.pendingAddOns, m.pendingRest, m.pendingSection)
					m = m.refreshAfterAdd()
				}
				m.conflictOpen = false
			case "esc":
				m.conflictOpen = false
			}
			return m, nil
```

Replace with:
```go
			case "enter":
				var syncCmd tea.Cmd
				if m.conflictSel == 0 { // start new
					m = m.startNewCart(m.pendingItem, m.pendingAddOns, m.pendingRest, m.pendingSection)
					m = m.refreshAfterAdd()
					syncCmd = m.liveSyncCart()
				}
				m.conflictOpen = false
				return m, syncCmd
			case "esc":
				m.conflictOpen = false
			}
			return m, nil
```

**Site 4** — customize modal `"enter"` → `commitAdd`:

Find (around line 584):
```go
				case "enter":
					item := m.customize.Item()
					addons := m.customize.SelectedAddOns()
					m.customizeOpen = false
					m = m.commitAdd(item, addons, m.pendingRest, m.pendingSection)
					if !m.conflictOpen { // committed directly (no restaurant clash)
						m = m.refreshAfterAdd()
					}
				}
				return m, nil
```

Replace with:
```go
				case "enter":
					item := m.customize.Item()
					addons := m.customize.SelectedAddOns()
					m.customizeOpen = false
					m = m.commitAdd(item, addons, m.pendingRest, m.pendingSection)
					if !m.conflictOpen { // committed directly (no restaurant clash)
						m = m.refreshAfterAdd()
						return m, m.liveSyncCart()
					}
				}
				return m, nil
```

- [ ] **Step 6: Wire `scrCheckout` enter for live order placement**

Find the existing `scrCheckout` enter handler (around line 802):

```go
		case scrCheckout:
			switch k.String() {
			case "esc":
				m.screen = scrCart
				return m, nil
			case "enter":
				m.checkout = m.checkout.Placed(orderID(m.checkout.Lines()), "~40 min")
				m.screen = scrConfirm
				return m, nil
			}
```

Replace with:

```go
		case scrCheckout:
			switch k.String() {
			case "esc":
				m.screen = scrCart
				return m, nil
			case "enter":
				if m.live && !m.placingOrder {
					m.placingOrder = true
					m.orderErr = ""
					return m, datasource.PlaceOrderCmd(m.backend, m.snap, m.addr.ID)
				}
				if !m.live {
					m.checkout = m.checkout.Placed(orderID(m.checkout.Lines()), "~40 min")
					m.screen = scrConfirm
				}
				return m, nil
			}
```

- [ ] **Step 7: Add `CartSyncedMsg` and `OrderPlacedMsg` handlers in `Update`**

In the `switch dm := msg.(type)` block (the datasource msg switch inserted in Slice 5), add two new cases after `MenuLoadedMsg`:

```go
	case datasource.CartSyncedMsg:
		if dm.Err != nil {
			m.cartSyncErr = "cart sync: " + dm.Err.Error()
		} else {
			m.cartSyncErr = ""
		}
		return m, nil
	case datasource.OrderPlacedMsg:
		m.placingOrder = false
		if dm.Err != nil {
			m.orderErr = "order failed: " + dm.Err.Error()
			return m, nil
		}
		m.orderErr = ""
		m.checkout = m.checkout.Placed(dm.Order.ID, "~40 min")
		m.screen = scrConfirm
		m.lines = nil
		m.cartRestaurant = ""
		m.cartSection = ""
		return m, nil
```

- [ ] **Step 8: Show errors in status bar and pass `WithPlacing` to checkout View**

Update `statusBar()` to surface errors:

```go
func (m Model) statusBar() string {
	hint := statusHints[(m.frame/27)%len(statusHints)]
	if m.orderErr != "" {
		hint = m.orderErr
	} else if m.cartSyncErr != "" {
		hint = m.cartSyncErr
	}
	return components.StatusBar(m.addr.Line, m.screenLabel(), hint, "12.4", m.blinkOn())
}
```

In `View()` find where `m.checkout.View(m.frame)` is called (in the `scrCheckout/scrConfirm` body branch):

```go
	case scrCheckout, scrConfirm:
		body = m.checkout.View(m.frame)
```

Replace with:

```go
	case scrCheckout, scrConfirm:
		body = m.checkout.WithPlacing(m.placingOrder).View(m.frame)
```

- [ ] **Step 9: Run tests**

Run:
```bash
go test ./internal/tui/... 2>&1 | tail -20
go build ./... 2>&1
go vet ./...
gofmt -l internal/tui/app.go internal/tui/live_test.go
```
Expected: ALL PASS; builds clean; `gofmt -l` empty.

- [ ] **Step 10: Commit**

```bash
git add internal/tui/app.go internal/tui/live_test.go
git commit -m "feat(tui): eager cart sync + live PlaceOrder through COD gate"
```

---

## Self-Review

**Spec coverage:**
- ✓ `Backend` gains `UpdateCart` + `PlaceOrder` (Task 1)
- ✓ `CartSyncedMsg` + `OrderPlacedMsg` + `SyncCart` + `PlaceOrderCmd` Cmds (Task 1)
- ✓ `BrokerBackend` implements both with `accountID` pinned (Task 2)
- ✓ `Checkout.WithPlacing(bool)` changes CTA bar text (Task 3)
- ✓ `liveSyncCart` helper fires from all 4 food-cart mutation sites (Task 4)
- ✓ `scrCheckout` enter in live mode fires `PlaceOrderCmd`; blocked by `m.placingOrder` (Task 4)
- ✓ `CartSyncedMsg` handler: non-fatal, shows in status bar (Task 4)
- ✓ `OrderPlacedMsg` handler: success → scrConfirm with real order ID; error → stays on scrCheckout (Task 4)
- ✓ Mock path unchanged: `m.live == false` keeps fake `Placed()` flow (Task 4)
- ✓ `imLines` (Instamart) not synced — food cart only (per spec)
- ✓ Items without `SwiggyID` skipped in `liveSyncCart` (per constraint)
- ✓ Double-fire blocked by `m.placingOrder` gate (per constraint)

**Placeholder scan:** None found. All code complete.

**Type consistency:** `datasource.PlaceOrderCmd(b Backend, snap *swiggysnap.Snapshot, addressID string)` — note: `snap` parameter is included for future cart-state recording even though this slice doesn't use it; `SyncCart` signature matches between datasource.go and all call sites. `api.CartItem{ItemID, Quantity}` used in `liveSyncCart` and `fakeRPC.UpdateCart`. `screens.CartLine.Item.SwiggyID` accessed in `liveSyncCart` — `catalog.Item.SwiggyID` is populated from config seed and from `datasource.toMenuPlace`. `m.checkout.WithPlacing(m.placingOrder)` is a pure value copy — safe to call each `View()` frame.
