# `catalog/swiggy` + TUI Async Layer + `cmd/sshd` Wiring · Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fill the existing `catalog.Repository` seam with a live, broker-backed implementation so the TUI shows real Swiggy data — without breaking the mock path or any existing test. A per-session **snapshot cache** satisfies the sync `Repository`; async bubbletea `Cmd`s fetch from the broker and fill it. The live path is **gated**: when no live backend is injected, the app behaves exactly as today (mock).

**Architecture:** `internal/catalog/swiggy` owns a thread-safe `Snapshot` (holding `catalog` types) and a `Repository` that reads it synchronously (empty on miss — exactly what screens already treat as "nothing yet"). It imports no TUI code (avoids the `catalog`↔`tui` cycle). `internal/tui/datasource` defines a `Backend` interface, async `Cmd`s that call the broker, map `api` DTOs → `catalog` types, write the `Snapshot`, and return load `Msg`s. `app.go` gains a variadic `New(caps, ...Option)`: `WithLiveBackend(...)` swaps in the snapshot `Repository`, dispatches loads, and shows an authorize gate when the account has no token. `cmd/sshd` selects backend via `CONSOLE_BACKEND`, captures the SSH pubkey, and resolves the account id from it (the account scope is **never** taken from client input).

**Tech Stack:** Go 1.26, bubbletea `Cmd`/`Msg`, `internal/broker/api` (RPC client), `charmbracelet/ssh` pubkey auth, `golang.org/x/crypto/ssh` for `MarshalAuthorizedKey`. No new external deps.

## Global Constraints

- Module `console.store`; Go floor `go 1.26.4`. `gofmt` clean, `go vet ./...` clean, tests pass.
- **`internal/catalog/swiggy` must NOT import `internal/tui` or `internal/tui/...`** (would create a cycle: `tui` imports `catalog`). The `Snapshot` therefore lives in `catalog/swiggy`.
- The mock path is the **default and CI default**: `New(caps)` with no options must behave exactly as today; all existing `internal/tui` tests must still pass unchanged.
- The TUI must import only `internal/broker/api` for broker access — never `swiggy`/`store`/`auth`. (`datasource` imports `api` + `catalog` + `catalog/swiggy`.)
- **Account scope is derived from the SSH session's public key**, via `api.Client.AccountForPubkey`. The account id is NEVER read from user/client input. (Closes the broker slice's AccountID-trust concern — the TUI is the trusted intermediary.)
- No live Swiggy calls in any automated test. Live wiring is exercised with a fake `Backend`/fake broker RPC.
- `CONSOLE_BACKEND=mock|live` (default `mock`); `CONSOLE_BROKER_SOCKET` for the socket path (default `/tmp/console-broker.sock`).

---

### Task 1: `catalog/swiggy` — Snapshot + Repository

**Files:**
- Create: `internal/catalog/swiggy/snapshot.go`
- Create: `internal/catalog/swiggy/repository.go`
- Test: `internal/catalog/swiggy/repository_test.go`

**Interfaces:**
- Consumes: `internal/catalog` types only.
- Produces:
  ```go
  // package swiggy  (import path console.store/internal/catalog/swiggy)
  type Snapshot struct{ /* mu + maps */ }
  func NewSnapshot() *Snapshot
  func (s *Snapshot) SetAddresses(a []catalog.Address)
  func (s *Snapshot) SetPlaces(addrID string, section catalog.Section, places []catalog.Place)
  func (s *Snapshot) SetMenu(p catalog.Place)
  func (s *Snapshot) SetInstamart(addrID string, items []catalog.Item)
  // Repository implements catalog.Repository by reading the Snapshot (sync,
  // empty/false on miss). It triggers no I/O.
  type Repository struct{ snap *Snapshot }
  func NewRepository(snap *Snapshot) *Repository
  ```

- [ ] **Step 1: Write the failing test** (`internal/catalog/swiggy/repository_test.go`)

```go
package swiggy

import (
	"testing"

	"console.store/internal/catalog"
)

func TestRepositoryReadsSnapshot(t *testing.T) {
	snap := NewSnapshot()
	repo := NewRepository(snap)

	// Empty snapshot: everything is empty/absent (screens treat this as "loading").
	if got := repo.Addresses(); len(got) != 0 {
		t.Fatalf("addresses before load = %v", got)
	}
	addr := catalog.Address{ID: "a1", Label: "home"}
	if _, ok := repo.Usual(addr); ok {
		t.Fatal("usual should be absent on empty snapshot")
	}

	snap.SetAddresses([]catalog.Address{addr})
	snap.SetPlaces("a1", catalog.SectionCoffee, []catalog.Place{{ID: "p1", Name: "Blue Tokai", Section: catalog.SectionCoffee}})
	snap.SetMenu(catalog.Place{ID: "p1", Name: "Blue Tokai", Items: []catalog.Item{{ID: "i1", Name: "Latte", Price: 250}}})

	if got := repo.Addresses(); len(got) != 1 || got[0].ID != "a1" {
		t.Fatalf("addresses = %v", got)
	}
	if got := repo.Places(addr, catalog.SectionCoffee); len(got) != 1 || got[0].ID != "p1" {
		t.Fatalf("places = %v", got)
	}
	p, ok := repo.Menu("p1")
	if !ok || len(p.Items) != 1 || p.Items[0].Name != "Latte" {
		t.Fatalf("menu = %+v ok=%v", p, ok)
	}
}

func TestRepositorySatisfiesInterface(t *testing.T) {
	var _ catalog.Repository = NewRepository(NewSnapshot())
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/catalog/swiggy/ -v`
Expected: FAIL — undefined.

- [ ] **Step 3: Write `snapshot.go`**

```go
// Package swiggy provides a live, broker-backed catalog.Repository. A per-session
// Snapshot caches catalog data; the TUI's datasource fills it via async Cmds and
// the Repository reads it synchronously. It imports no TUI code (the catalog
// layer must not depend on tui).
package swiggy

import (
	"sync"

	"console.store/internal/catalog"
)

type placeKey struct {
	addrID  string
	section catalog.Section
}

// Snapshot is the per-session cache the live Repository reads. All access is
// mutex-guarded so the async fill Cmds and the synchronous Repository reads
// (which run on the bubbletea update goroutine) never race.
type Snapshot struct {
	mu        sync.RWMutex
	addresses []catalog.Address
	places    map[placeKey][]catalog.Place
	menus     map[string]catalog.Place // by place id
	instamart map[string][]catalog.Item
}

func NewSnapshot() *Snapshot {
	return &Snapshot{
		places:    map[placeKey][]catalog.Place{},
		menus:     map[string]catalog.Place{},
		instamart: map[string][]catalog.Item{},
	}
}

func (s *Snapshot) SetAddresses(a []catalog.Address) {
	s.mu.Lock()
	s.addresses = a
	s.mu.Unlock()
}

func (s *Snapshot) SetPlaces(addrID string, section catalog.Section, places []catalog.Place) {
	s.mu.Lock()
	s.places[placeKey{addrID, section}] = places
	s.mu.Unlock()
}

func (s *Snapshot) SetMenu(p catalog.Place) {
	s.mu.Lock()
	s.menus[p.ID] = p
	s.mu.Unlock()
}

func (s *Snapshot) SetInstamart(addrID string, items []catalog.Item) {
	s.mu.Lock()
	s.instamart[addrID] = items
	s.mu.Unlock()
}

func (s *Snapshot) getAddresses() []catalog.Address {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.addresses
}
func (s *Snapshot) getPlaces(addrID string, section catalog.Section) []catalog.Place {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.places[placeKey{addrID, section}]
}
func (s *Snapshot) getMenu(placeID string) (catalog.Place, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.menus[placeID]
	return p, ok
}
func (s *Snapshot) getInstamart(addrID string) []catalog.Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.instamart[addrID]
}
```

- [ ] **Step 4: Write `repository.go`**

```go
package swiggy

import "console.store/internal/catalog"

// Repository implements catalog.Repository over a Snapshot. Reads are sync and
// never do I/O; a cache miss returns the zero value (empty list / ok=false),
// which the screens already render as an empty/loading state.
type Repository struct{ snap *Snapshot }

func NewRepository(snap *Snapshot) *Repository { return &Repository{snap: snap} }

func (r *Repository) Addresses() []catalog.Address { return r.snap.getAddresses() }

func (r *Repository) Places(addr catalog.Address, section catalog.Section) []catalog.Place {
	return r.snap.getPlaces(addr.ID, section)
}

func (r *Repository) Menu(placeID string) (catalog.Place, bool) {
	return r.snap.getMenu(placeID)
}

// Usual/Trending are not yet sourced live (no broker method wired); they are
// absent on the live backend, which the menu screen renders without a "usual".
func (r *Repository) Usual(addr catalog.Address) (catalog.Usual, bool) {
	return catalog.Usual{}, false
}

func (r *Repository) Trending(addr catalog.Address) (catalog.Trending, bool) {
	return catalog.Trending{}, false
}

func (r *Repository) InstamartItems(addr catalog.Address) []catalog.Item {
	return r.snap.getInstamart(addr.ID)
}
```

- [ ] **Step 5: Run + verify no tui import**

Run:
```bash
go test ./internal/catalog/swiggy/ -v
go list -deps ./internal/catalog/swiggy/ | grep 'console.store/internal/tui' && echo "CYCLE RISK: imports tui" || echo "ok: no tui import"
```
Expected: tests PASS; grep prints nothing → "ok: no tui import".

- [ ] **Step 6: Commit**

```bash
git add internal/catalog/swiggy/
git commit -m "feat(catalog/swiggy): session Snapshot + sync Repository over it"
```

---

### Task 2: `datasource` — Backend, load Cmds, api→catalog mapping

**Files:**
- Create: `internal/tui/datasource/datasource.go`
- Create: `internal/tui/datasource/mapping.go`
- Test: `internal/tui/datasource/datasource_test.go`

**Interfaces:**
- Consumes: `internal/broker/api`, `internal/catalog`, `internal/catalog/swiggy`.
- Produces:
  ```go
  // package datasource
  type Backend interface {
      Addresses() ([]api.Address, error)
      Places(addressID string, section catalog.Section) ([]api.Restaurant, error)
      Menu(addressID, restaurantID string) (api.Menu, error)
  }
  type AddressesLoadedMsg struct{ Err error }
  type PlacesLoadedMsg struct{ Section catalog.Section; Err error }
  type MenuLoadedMsg struct{ PlaceID string; Err error }
  // ErrNeedsAuth reports the account has no usable token (drive the authorize gate).
  var ErrNeedsAuth = errors.New("datasource: account not authorized")
  func LoadAddresses(b Backend, snap *swiggy.Snapshot) tea.Cmd
  func LoadPlaces(b Backend, snap *swiggy.Snapshot, addressID string, section catalog.Section) tea.Cmd
  func LoadMenu(b Backend, snap *swiggy.Snapshot, addressID, restaurantID string) tea.Cmd
  ```

- [ ] **Step 1: Write `mapping.go`** (api DTO → catalog)

```go
package datasource

import (
	"console.store/internal/broker/api"
	"console.store/internal/catalog"
)

func toAddresses(in []api.Address) []catalog.Address {
	out := make([]catalog.Address, len(in))
	for i, a := range in {
		out[i] = catalog.Address{ID: a.ID, Label: a.Label, City: a.City, Line: a.Line, Full: a.Full, Lat: a.Lat, Lng: a.Lng}
	}
	return out
}

func toPlaces(in []api.Restaurant, section catalog.Section) []catalog.Place {
	out := make([]catalog.Place, len(in))
	for i, r := range in {
		out[i] = catalog.Place{
			ID: r.ID, SwiggyID: r.ID, Name: r.Name, City: r.City,
			Section: section, ETA: r.ETA, Rating: r.Rating, Description: r.Description,
		}
	}
	return out
}

func toMenuPlace(m api.Menu) catalog.Place {
	items := make([]catalog.Item, len(m.Items))
	for i, it := range m.Items {
		items[i] = catalog.Item{
			ID: it.ID, SwiggyID: it.ID, Name: it.Name, Price: it.Price,
			Veg: it.Veg, Desc: it.Description, Rating: it.Rating,
		}
	}
	return catalog.Place{ID: m.RestaurantID, SwiggyID: m.RestaurantID, Items: items}
}
```

- [ ] **Step 2: Write `datasource.go`**

```go
// Package datasource wires the broker (via internal/broker/api) into the TUI as
// async bubbletea Cmds that fill a catalog/swiggy.Snapshot. The TUI reads the
// Snapshot synchronously through a swiggy.Repository; these Cmds are the only
// thing that performs broker I/O. The TUI never imports swiggy/store/auth.
package datasource

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/catalog"
	"console.store/internal/catalog/swiggy"
)

// ErrNeedsAuth signals the account has no usable token; the Model shows the
// authorize gate. Backends should return it (or wrap it) on a missing-token
// error from the broker.
var ErrNeedsAuth = errors.New("datasource: account not authorized")

type Backend interface {
	Addresses() ([]api_Address, error)
	Places(addressID string, section catalog.Section) ([]api_Restaurant, error)
	Menu(addressID, restaurantID string) (api_Menu, error)
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
)

func LoadAddresses(b Backend, snap *swiggy.Snapshot) tea.Cmd {
	return func() tea.Msg {
		got, err := b.Addresses()
		if err != nil {
			return AddressesLoadedMsg{Err: err}
		}
		snap.SetAddresses(toAddresses(got))
		return AddressesLoadedMsg{}
	}
}

func LoadPlaces(b Backend, snap *swiggy.Snapshot, addressID string, section catalog.Section) tea.Cmd {
	return func() tea.Msg {
		got, err := b.Places(addressID, section)
		if err != nil {
			return PlacesLoadedMsg{Section: section, Err: err}
		}
		snap.SetPlaces(addressID, section, toPlaces(got, section))
		return PlacesLoadedMsg{Section: section}
	}
}

func LoadMenu(b Backend, snap *swiggy.Snapshot, addressID, restaurantID string) tea.Cmd {
	return func() tea.Msg {
		got, err := b.Menu(addressID, restaurantID)
		if err != nil {
			return MenuLoadedMsg{PlaceID: restaurantID, Err: err}
		}
		snap.SetMenu(toMenuPlace(got))
		return MenuLoadedMsg{PlaceID: restaurantID}
	}
}
```

> Executor note: the `api_Address`/`api_Restaurant`/`api_Menu` names above are placeholders to avoid a forward-reference in this snippet. Replace them with the real `api.Address`/`api.Restaurant`/`api.Menu` and add `"console.store/internal/broker/api"` to the import block. (The `Backend` interface methods return `[]api.Address`, `[]api.Restaurant`, `api.Menu`.)

- [ ] **Step 3: Write `datasource_test.go`** (fake Backend; invoke Cmd directly)

```go
package datasource

import (
	"errors"
	"testing"

	"console.store/internal/broker/api"
	"console.store/internal/catalog"
	"console.store/internal/catalog/swiggy"
)

type fakeBackend struct {
	addrs []api.Address
	rests []api.Restaurant
	menu  api.Menu
	err   error
}

func (f *fakeBackend) Addresses() ([]api.Address, error) { return f.addrs, f.err }
func (f *fakeBackend) Places(string, catalog.Section) ([]api.Restaurant, error) {
	return f.rests, f.err
}
func (f *fakeBackend) Menu(string, string) (api.Menu, error) { return f.menu, f.err }

func TestLoadAddressesFillsSnapshot(t *testing.T) {
	b := &fakeBackend{addrs: []api.Address{{ID: "a1", Label: "home"}}}
	snap := swiggy.NewSnapshot()
	msg := LoadAddresses(b, snap)()
	if m, ok := msg.(AddressesLoadedMsg); !ok || m.Err != nil {
		t.Fatalf("msg = %#v", msg)
	}
	repo := swiggy.NewRepository(snap)
	if got := repo.Addresses(); len(got) != 1 || got[0].ID != "a1" {
		t.Fatalf("snapshot not filled: %v", got)
	}
}

func TestLoadPlacesPropagatesError(t *testing.T) {
	b := &fakeBackend{err: ErrNeedsAuth}
	snap := swiggy.NewSnapshot()
	msg := LoadPlaces(b, snap, "a1", catalog.SectionCoffee)()
	m, ok := msg.(PlacesLoadedMsg)
	if !ok || !errors.Is(m.Err, ErrNeedsAuth) || m.Section != catalog.SectionCoffee {
		t.Fatalf("msg = %#v", msg)
	}
}

func TestLoadMenuFillsSnapshot(t *testing.T) {
	b := &fakeBackend{menu: api.Menu{RestaurantID: "p1", Items: []api.MenuItem{{ID: "i1", Name: "Latte", Price: 250}}}}
	snap := swiggy.NewSnapshot()
	if msg := LoadMenu(b, snap, "a1", "p1")(); msg.(MenuLoadedMsg).Err != nil {
		t.Fatalf("menu load err: %v", msg)
	}
	if p, ok := swiggy.NewRepository(snap).Menu("p1"); !ok || len(p.Items) != 1 {
		t.Fatalf("menu not filled: %+v ok=%v", p, ok)
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/tui/datasource/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/datasource/datasource.go internal/tui/datasource/mapping.go internal/tui/datasource/datasource_test.go
git commit -m "feat(tui/datasource): Backend + async load Cmds filling the Snapshot"
```

---

### Task 3: Broker-backed Backend adapter (+ section→query)

**Files:**
- Create: `internal/tui/datasource/broker_backend.go`
- Test: `internal/tui/datasource/broker_backend_test.go`

**Interfaces:**
- Consumes: `internal/broker/api`, `internal/catalog`.
- Produces:
  ```go
  // brokerRPC is the subset of *api.Client the backend uses (lets tests fake it).
  type brokerRPC interface {
      Addresses(accountID string) ([]api.Address, error)
      Restaurants(accountID, addressID, query string) ([]api.Restaurant, error)
      Menu(accountID, addressID, restaurantID string) (api.Menu, error)
  }
  type BrokerBackend struct{ rpc brokerRPC; accountID string }
  func NewBrokerBackend(rpc brokerRPC, accountID string) *BrokerBackend
  // (implements Backend; maps section → search query and the account id is fixed
  //  to this session — never taken from a call argument.)
  ```

- [ ] **Step 1: Write the failing test** (`internal/tui/datasource/broker_backend_test.go`)

```go
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

// Backend interface satisfaction.
func TestBrokerBackendIsBackend(t *testing.T) {
	var _ Backend = NewBrokerBackend(&fakeRPC{}, "x")
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/tui/datasource/ -run TestBrokerBackend -v`
Expected: FAIL — undefined.

- [ ] **Step 3: Write `broker_backend.go`**

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

- [ ] **Step 4: Run tests**

Run: `go test ./internal/tui/datasource/ -v`
Expected: PASS (all datasource tests).

- [ ] **Step 5: Commit**

```bash
git add internal/tui/datasource/broker_backend.go internal/tui/datasource/broker_backend_test.go
git commit -m "feat(tui/datasource): broker-backed Backend pinned to the session account"
```

---

### Task 4: app.go live wiring (gated) + cmd/sshd backend selection

**Files:**
- Modify: `internal/tui/app.go` (New signature → variadic options; add live fields; Init; Update msg cases; View authorize gate)
- Create: `internal/tui/live.go` (live options + helpers, keeps app.go churn contained)
- Test: `internal/tui/live_test.go`
- Modify: `cmd/sshd/main.go` (backend selection + pubkey→account resolution)

**Interfaces:**
- Consumes: `internal/tui/datasource`, `internal/catalog/swiggy`, `internal/broker/api`.
- Produces:
  ```go
  // package tui
  type Option func(*Model)
  func New(caps render.Caps, opts ...Option) Model   // existing New(caps) calls keep working
  // WithLiveBackend swaps the mock Repository for a snapshot-backed live one and
  // arms async loading + the authorize gate. accountID is the session's account.
  func WithLiveBackend(b datasource.Backend, snap *swiggy.Snapshot, accountID, authorizeURL string) Option
  ```

- [ ] **Step 1: Write `internal/tui/live.go`**

```go
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/catalog/swiggy"
	"console.store/internal/tui/datasource"
)

// Option configures a Model at construction (functional-options so the existing
// New(caps) call site is unchanged).
type Option func(*Model)

// WithLiveBackend arms the live (broker-backed) data path: it replaces the mock
// Repository with a Snapshot-backed one and stores the async backend. When set,
// Init dispatches the initial loads and a missing-token error flips the Model to
// the authorize gate (showing authorizeURL).
func WithLiveBackend(b datasource.Backend, snap *swiggy.Snapshot, accountID, authorizeURL string) Option {
	return func(m *Model) {
		m.live = true
		m.backend = b
		m.snap = snap
		m.accountID = accountID
		m.authorizeURL = authorizeURL
		m.repo = swiggy.NewRepository(snap)
	}
}

// liveInitCmds returns the initial fetches for a live session: addresses + the
// current section's places. No-op (nil) for the mock path.
func (m Model) liveInitCmds() tea.Cmd {
	if !m.live {
		return nil
	}
	return tea.Batch(
		datasource.LoadAddresses(m.backend, m.snap),
		datasource.LoadPlaces(m.backend, m.snap, m.addr.ID, m.section),
	)
}
```

- [ ] **Step 2: Add live fields to the `Model` struct (`internal/tui/app.go`)**

In the `type Model struct { ... }` block, add (next to `repo`):

```go
	// live data path (nil/false on the mock default). When live, repo is a
	// catalog/swiggy.Repository backed by snap, filled by datasource Cmds.
	live         bool
	backend      datasource.Backend
	snap         *swiggy.Snapshot
	accountID    string
	authorizeURL string
	needsAuth    bool // set when a load returns datasource.ErrNeedsAuth
```

Add the imports to `app.go`:
```go
	"console.store/internal/catalog/swiggy"
	"console.store/internal/tui/datasource"
```

- [ ] **Step 3: Make `New` variadic + apply options (`internal/tui/app.go`)**

Replace the `New` function with:

```go
func New(caps render.Caps, opts ...Option) Model {
	repo := mem.New()
	addr := repo.Addresses()[0]
	section := catalog.SectionCoffee
	m := Model{repo: repo, addr: addr, section: section, screen: scrSplash, caps: caps, lastEscFrame: -escDoubleWindow - 1}
	for _, o := range opts {
		o(&m)
	}
	// A live backend starts with an empty snapshot; the mock addr seed above is
	// only a placeholder until LoadAddresses fills the snapshot and Update swaps
	// m.addr to the first live address.
	m.splash = screens.NewSplash().WithCaps(caps)
	m.splashPhrase = screens.RandomPhrase("")
	m.menu = m.buildMenu()
	return m
}
```

- [ ] **Step 4: Dispatch initial live loads from `Init` (`internal/tui/app.go`)**

Replace `Init` with:

```go
func (m Model) Init() tea.Cmd {
	if c := m.liveInitCmds(); c != nil {
		return tea.Batch(tick(), c)
	}
	return tick()
}
```

- [ ] **Step 5: Handle datasource msgs in `Update` (`internal/tui/app.go`)**

In `Update`, immediately AFTER the `tea.WindowSizeMsg` handler block and BEFORE the `tea.KeyMsg` block, insert:

```go
	switch dm := msg.(type) {
	case datasource.AddressesLoadedMsg:
		if errorsIsNeedsAuth(dm.Err) {
			m.needsAuth = true
			return m, nil
		}
		// Adopt the first live address and refresh places for the section.
		if addrs := m.repo.Addresses(); len(addrs) > 0 {
			m.addr = addrs[0]
		}
		m.menu = m.buildMenu()
		if m.live {
			return m, datasource.LoadPlaces(m.backend, m.snap, m.addr.ID, m.section)
		}
		return m, nil
	case datasource.PlacesLoadedMsg:
		if errorsIsNeedsAuth(dm.Err) {
			m.needsAuth = true
			return m, nil
		}
		m.menu = m.buildMenu()
		return m, nil
	case datasource.MenuLoadedMsg:
		if errorsIsNeedsAuth(dm.Err) {
			m.needsAuth = true
			return m, nil
		}
		if m.screen == scrRestaurant {
			if p, ok := m.repo.Menu(dm.PlaceID); ok {
				m.rest = screens.NewRestaurant(p, m.addr, m.cartChip())
			}
		}
		return m, nil
	}
```

Add a small helper at the bottom of `app.go` (and ensure `"errors"` and `datasource` are imported):

```go
func errorsIsNeedsAuth(err error) bool {
	return err != nil && errors.Is(err, datasource.ErrNeedsAuth)
}
```

> Executor note: `screens.NewRestaurant(...)` must match the existing constructor used elsewhere in app.go for entering a restaurant — copy that exact call (same arguments) rather than the illustrative one above if it differs. Find the existing `screens.NewRestaurant(` call and mirror it.

- [ ] **Step 6: Authorize gate in `View` (`internal/tui/app.go`)**

At the very TOP of `View()` (before the splash branch), insert:

```go
	if m.needsAuth {
		gate := "  console.store needs to connect to your Swiggy account.\n\n" +
			"  1. Open this link on your phone and log in to Swiggy:\n\n" +
			"     " + m.authorizeURL + "\n\n" +
			"  2. Approve access, then press  r  to retry.\n"
		if m.w == 0 || m.h == 0 {
			return gate
		}
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, gate)
	}
```

And handle the `r` retry: in the `tea.KeyMsg` handling, add an early check (right after the `m.cmdOpen` block closes, before normal key routing):

```go
		if m.needsAuth {
			if k.String() == "r" {
				m.needsAuth = false
				return m, m.liveInitCmds()
			}
			if k.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}
```

- [ ] **Step 7: Write `internal/tui/live_test.go`** (gated behavior via a fake Backend)

```go
package tui

import (
	"testing"

	"console.store/internal/broker/api"
	"console.store/internal/catalog"
	"console.store/internal/catalog/swiggy"
	"console.store/internal/tui/datasource"
	"console.store/internal/tui/render"
)

type liveFake struct {
	addrs []api.Address
	err   error
}

func (f *liveFake) Addresses() ([]api.Address, error) { return f.addrs, f.err }
func (f *liveFake) Places(string, catalog.Section) ([]api.Restaurant, error) {
	return nil, f.err
}
func (f *liveFake) Menu(string, string) (api.Menu, error) { return api.Menu{}, f.err }

func TestMockPathUnaffected(t *testing.T) {
	m := New(render.Caps{}) // no options => mock
	if m.live {
		t.Fatal("default New must not be live")
	}
	if len(m.repo.Addresses()) == 0 {
		t.Fatal("mock repo should have seed addresses")
	}
}

func TestLiveAddressesMsgAdoptsAddress(t *testing.T) {
	snap := swiggy.NewSnapshot()
	be := &liveFake{addrs: []api.Address{{ID: "live-1", Label: "home"}}}
	m := New(render.Caps{}, WithLiveBackend(be, snap, "acct-1", "https://authz/x"))
	if !m.live {
		t.Fatal("expected live model")
	}
	// Simulate the load completing: fill snapshot + deliver the msg.
	snap.SetAddresses([]catalog.Address{{ID: "live-1", Label: "home"}})
	updated, _ := m.Update(datasource.AddressesLoadedMsg{})
	if updated.(Model).addr.ID != "live-1" {
		t.Fatalf("model did not adopt live address: %+v", updated.(Model).addr)
	}
}

func TestLiveNeedsAuthOnAuthError(t *testing.T) {
	snap := swiggy.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "acct-1", "https://authz/x"))
	updated, _ := m.Update(datasource.AddressesLoadedMsg{Err: datasource.ErrNeedsAuth})
	if !updated.(Model).needsAuth {
		t.Fatal("expected needsAuth after ErrNeedsAuth load")
	}
}
```

- [ ] **Step 8: Run TUI tests (existing + new)**

Run:
```bash
go test ./internal/tui/... 2>&1 | tail -20
go vet ./internal/tui/...
gofmt -l internal/tui
```
Expected: ALL pass (existing flow/screen tests unchanged + 3 new live tests); `gofmt -l` empty.

- [ ] **Step 9: Commit**

```bash
git add internal/tui/live.go internal/tui/live_test.go internal/tui/app.go
git commit -m "feat(tui): gated live backend (snapshot repo + async loads + authorize gate)"
```

- [ ] **Step 10: Wire `cmd/sshd/main.go` — backend selection + pubkey→account**

Make these changes to `cmd/sshd/main.go`:

(a) Add a package-level live config resolved in `main()` from env, and a public-key auth handler that records the session's authorized-key string. Add imports: `"console.store/internal/broker/api"`, `"console.store/internal/catalog/swiggy"`, `"console.store/internal/tui/datasource"`, `gossh "golang.org/x/crypto/ssh"`.

(b) Replace `teaHandler` so that, when `CONSOLE_BACKEND=live`, it dials the broker, resolves the account id from the session pubkey, and constructs the live Model:

```go
func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	renderer := bubbletea.MakeRenderer(s)
	lipgloss.SetColorProfile(renderer.ColorProfile())

	pty, _, _ := s.Pty()
	truecolor := renderer.ColorProfile() == termenv.TrueColor
	caps := render.DetectCaps(pty.Term, s.Environ(), truecolor)

	if os.Getenv("CONSOLE_BACKEND") == "live" {
		if m, ok := liveModel(s, caps); ok {
			return m, []tea.ProgramOption{tea.WithAltScreen()}
		}
		// fall through to mock on any wiring failure (logged in liveModel)
	}
	return consoletui.New(caps), []tea.ProgramOption{tea.WithAltScreen()}
}

// liveModel builds a broker-backed TUI Model for this SSH session. The account
// id comes from the session's public key (never from client input). Returns
// ok=false if the broker is unreachable or no pubkey was presented.
func liveModel(s ssh.Session, caps render.Caps) (tea.Model, bool) {
	pk := s.PublicKey()
	if pk == nil {
		log.Printf("live: session presented no public key; using mock")
		return nil, false
	}
	pubkey := string(gossh.MarshalAuthorizedKey(pk))

	sock := os.Getenv("CONSOLE_BROKER_SOCKET")
	if sock == "" {
		sock = "/tmp/console-broker.sock"
	}
	cli, err := api.Dial(sock)
	if err != nil {
		log.Printf("live: broker dial failed: %v; using mock", err)
		return nil, false
	}
	accountID, _, err := cli.AccountForPubkey(pubkey)
	if err != nil {
		log.Printf("live: AccountForPubkey failed: %v; using mock", err)
		return nil, false
	}
	// Begin a cross-device authorize so the gate has a URL to show if needed.
	authURL := ""
	if start, err := cli.StartAuth(pubkey); err == nil {
		authURL = start.AuthorizeURL
	}

	snap := swiggy.NewSnapshot()
	be := datasource.NewBrokerBackend(cli, accountID)
	return consoletui.New(caps, consoletui.WithLiveBackend(be, snap, accountID, authURL)), true
}
```

(c) In `main()`, enable public-key auth so `s.PublicKey()` is populated (accept any key — identity is the key itself):

```go
	srv, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort(host, port)),
		wish.WithHostKeyPath(".ssh/console_host_key"),
		wish.WithIdleTimeout(5*time.Minute),
		wish.WithPublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool { return true }),
		wish.WithMiddleware(
			bubbletea.Middleware(teaHandler),
			logging.Middleware(),
			canvasMiddleware,
		),
	)
```

> Executor note: confirm `*api.Client` satisfies `datasource.NewBrokerBackend`'s `brokerRPC` interface (it has `Addresses(accountID)`, `Restaurants(accountID,addressID,query)`, `Menu(accountID,addressID,restaurantID)` — it does, from the broker slice). If `s.PublicKey()` is not a method on this `ssh.Session` version, use `gossh.MarshalAuthorizedKey(s.PublicKey())` via the context: `ctx.Value(...)`. Verify by building.

- [ ] **Step 11: Build + full repo test + vet/fmt**

Run:
```bash
go build ./...
go test ./... 2>&1 | tail -20
go vet ./...
gofmt -l internal cmd
```
Expected: builds; whole repo green (store DB tests SKIP without DSN); `gofmt -l` empty.

- [ ] **Step 12: Commit**

```bash
git add cmd/sshd/main.go
git commit -m "feat(sshd): CONSOLE_BACKEND=live wiring with pubkey-derived account scope"
```

---

## Self-Review

**Spec coverage (spec §3.5, §3.6):** live `catalog.Repository` over a session snapshot ✓ (Task 1); `datasource` Cmds + Msgs filling the snapshot, handled in `app.go` Update with loading/auth states ✓ (Tasks 2,4); `mem` stays the default/CI Repository, selected by config ✓ (Task 4 `New` default + `CONSOLE_BACKEND`); broker-backed backend, account from SSH pubkey not client input ✓ (Tasks 3,4 — closes the slice-4 authz concern); TUI imports only `internal/broker/api` (+ catalog/datasource), never swiggy/store/auth ✓.

**Placeholder scan:** The `api_Address` names in Task 2 Step 2 are explicitly flagged with an executor note to replace with real `api.*` types + import — not a silent placeholder. The `screens.NewRestaurant` and `s.PublicKey()` notes direct the executor to mirror the real existing call/verify by building. No TBD/TODO. ✓

**Type consistency:** `datasource.Backend` (Addresses/Places/Menu) matches `BrokerBackend` and the fakes; `swiggy.Snapshot`/`NewRepository` match across tasks; `WithLiveBackend(b, snap, accountID, authorizeURL)` matches the `cmd/sshd` call; `api.Client` methods (`AccountForPubkey`, `StartAuth`, `Addresses`, `Restaurants`, `Menu`) match the broker slice. ✓

**Known-scope limits (documented, not gaps):** Usual/Trending/Instamart/coupons/tracking/order-placement UI are not yet live-wired (Repository returns absent for Usual/Trending; menu/address/places are the live vertical delivered here). The mock remains fully featured. Continuous polling of authorize status is deferred in favour of a manual `r`-retry gate (simpler, less app.go churn). These are intentional v1 scope, called out for the final review.
