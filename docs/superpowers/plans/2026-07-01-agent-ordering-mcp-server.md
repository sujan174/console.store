# `console mcp` Server Implementation Plan (Plan 1 of 2)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `console mcp` subcommand that runs a local stdio MCP server exposing Swiggy ordering tools (discovery, cart, a two-tool order-commit gate, agent-driven sign-in, and a local taste card) to local agents, over the same `broker.Service` the TUI/CLI use.

**Architecture:** A new `internal/mcp` package holds an `mcp.Server` wrapper whose tool handlers call an account-pinned `Backend` (satisfied by the existing `*datasource.BrokerBackend`). A new `internal/localstore/card.go` persists an auto-derived taste card next to `presets.json`. `cmd/store/main.go` gains an `mcp` route through the existing `run()` (so auto-update fires at startup) and an `Authenticator` adapter wrapping the existing OAuth manager + loopback callback. The official Go MCP SDK handles JSON-RPC/stdio/schema.

**Tech Stack:** Go 1.26, `github.com/modelcontextprotocol/go-sdk` (new dep), existing `broker`/`datasource`/`localstore`/`auth` packages, stdlib `crypto/rand`, `crypto/sha256`.

## Global Constraints

- Go 1.26; stdlib-only except the **one** justified new dep `github.com/modelcontextprotocol/go-sdk`. Run `go vet ./...` + `gofmt -w` on every changed file.
- `screens` must not import `tui`; `internal/cli` must not import `tui`; **`internal/mcp` must not import `tui`** (depend on `broker`/`datasource`/`localstore`/`broker/api` only).
- Arming unchanged: orders gated by `swiggy.LiveOrdersEnabled()` (build `liveOrdersDefault` / env `CONSOLE_LIVE_ORDERS=1`). **Never auto-retry `place_food_order`** (a 5xx may mean it placed). **Never place a real order from code or tests** — tests use fakes; arming defaults OFF under `go test`.
- Rate limiting must persist: the MCP reuses the single per-account `swiggy.Client` via the same `broker.Service` (no second limiter).
- Tests that touch persistence set `t.Setenv("XDG_CONFIG_HOME", t.TempDir())` to isolate keyring/config.
- The token is never returned to the agent (PKCE front channel only).
- MCP SDK v1.x API (verbatim): server = `mcp.NewServer(&mcp.Implementation{Name, Version}, nil)`; register = `mcp.AddTool(server, &mcp.Tool{Name, Description}, handler)`; handler = `func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error)`; run = `server.Run(ctx, &mcp.StdioTransport{})`. A returned non-nil `error` becomes a tool error the agent sees; on success return `(nil, out, nil)` and the SDK marshals `out` as structured content.

---

### Task 1: Add the SDK dep and a `console mcp` skeleton

**Files:**
- Modify: `go.mod`, `go.sum` (via `go get`)
- Create: `internal/mcp/server.go`
- Create: `internal/mcp/server_test.go`
- Modify: `cmd/store/main.go` (dispatch `mcp` in `run()`)

**Interfaces:**
- Produces: `mcp.Backend` interface; `mcp.NewServer(be Backend, auth Authenticator) *Server`; `mcp.Serve(ctx context.Context, s *Server) error`; `Server.handleServerInfo` (a no-auth smoke tool).
- Consumes (from main): `bootstrap()` returns `be *datasource.BrokerBackend` (already satisfies `Backend`).

- [ ] **Step 1: Add the dependency**

Run:
```bash
cd /Users/sujan/Developer/console.store
go get github.com/modelcontextprotocol/go-sdk@latest
```
Expected: `go.mod` gains a `github.com/modelcontextprotocol/go-sdk vX.Y.Z` require line (v1.x).

- [ ] **Step 2: Write the failing test** — `internal/mcp/server_test.go`

```go
package mcp

import (
	"context"
	"testing"
)

func TestServerInfoReportsVersion(t *testing.T) {
	s := NewServer(nil, nil)
	res, out, err := s.handleServerInfo(context.Background(), nil, ServerInfoIn{})
	if err != nil {
		t.Fatalf("handleServerInfo: %v", err)
	}
	if res != nil && res.IsError {
		t.Fatalf("unexpected error result")
	}
	if out.Name != "consolestore" {
		t.Fatalf("Name = %q, want consolestore", out.Name)
	}
}
```

- [ ] **Step 3: Run it — must fail to compile**

Run: `go test ./internal/mcp/ -run TestServerInfoReportsVersion`
Expected: FAIL (package/symbols undefined).

- [ ] **Step 4: Write the minimal implementation** — `internal/mcp/server.go`

```go
// Package mcp serves consolestore's Swiggy ordering tools to local agents over
// a stdio MCP server. It is a second front-end over broker.Service (alongside
// the TUI and the headless CLI) and MUST NOT import internal/tui.
package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"consolestore/internal/broker/api"
	"consolestore/internal/version"
)

// Backend is the account-pinned slice of broker capability the tools need.
// *datasource.BrokerBackend satisfies it.
type Backend interface {
	Addresses() ([]api.Address, error)
	SearchOrganic(addressID, query string) ([]api.Restaurant, string, error)
	PlacesQuery(addressID, query string) ([]api.Restaurant, error)
	Usuals(addressID string) ([]api.Restaurant, error)
	Menu(addressID, restaurantID string) (api.Menu, error)
	ItemOptions(addressID, restaurantID, itemName, menuItemID string) ([]api.OptionGroup, error)
	GetCart(addressID, restaurantName string) (api.Cart, error)
	UpdateCart(addressID, restaurantID, restaurantName string, items []api.CartItem) (api.Cart, error)
	ClearCart() error
	PlaceOrder(addressID string) (api.Order, error)
	TrackOrder(orderID string) (api.Tracking, error)
	ActiveOrders(addressID string) ([]api.Order, error)
}

// Authenticator drives first-run sign-in without exposing the token. Implemented
// in package main over the OAuth manager + loopback callback server.
type Authenticator interface {
	TokenPresent(ctx context.Context) bool
	// Start begins (or resumes) the loopback OAuth flow and returns the browser
	// authorize URL plus a flow id to poll. It also ensures the loopback callback
	// server is listening.
	Start(ctx context.Context) (authorizeURL, flowID string, err error)
	Authorized(flowID string) bool
}

type Server struct {
	be      Backend
	auth    Authenticator
	pending *confirmStore
}

func NewServer(be Backend, auth Authenticator) *Server {
	return &Server{be: be, auth: auth, pending: newConfirmStore()}
}

type ServerInfoIn struct{}
type ServerInfoOut struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (s *Server) handleServerInfo(_ context.Context, _ *mcp.CallToolRequest, _ ServerInfoIn) (*mcp.CallToolResult, ServerInfoOut, error) {
	return nil, ServerInfoOut{Name: "consolestore", Version: version.Version}, nil
}

// Serve registers all tools and runs the stdio server until ctx is done.
func Serve(ctx context.Context, s *Server) error {
	srv := mcp.NewServer(&mcp.Implementation{Name: "consolestore", Version: version.Version}, nil)
	s.register(srv)
	return srv.Run(ctx, &mcp.StdioTransport{})
}

// register wires every tool. Later tasks append to it.
func (s *Server) register(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{Name: "server_info", Description: "consolestore server name and version"}, s.handleServerInfo)
}
```

Also create `internal/mcp/confirm.go` minimal stub so it compiles (filled in Task 5):

```go
package mcp

type confirmStore struct{}

func newConfirmStore() *confirmStore { return &confirmStore{} }
```

- [ ] **Step 5: Run the test — must pass**

Run: `go test ./internal/mcp/ -run TestServerInfoReportsVersion`
Expected: PASS.

- [ ] **Step 6: Route `mcp` in `cmd/store/main.go`**

In `run()`, immediately after `be, signedIn, launchTUI, err := bootstrap(ctx)` and its error check (main.go:92-95), and BEFORE the `len(args) == 0` TUI branch, add:

```go
	if len(args) > 0 && args[0] == "mcp" {
		// Agent surface: stdio MCP server over the same broker. Updater already
		// ran above (run() → updater.RunDefault), so this serves the latest build.
		authn := newMCPAuth(ctx, authMgr, ls, redirect)
		if err := consolemcp.Serve(ctx, consolemcp.NewServer(be, authn)); err != nil {
			return fmt.Errorf("mcp server: %w", err)
		}
		return nil
	}
```

Add the import `consolemcp "consolestore/internal/mcp"` to the import block. `authMgr`, `ls`, and `redirect` are currently locals inside `bootstrap()` — Task 6 exposes them; for THIS task, temporarily wire a nil authenticator to prove the route compiles and serves:

```go
	if len(args) > 0 && args[0] == "mcp" {
		if err := consolemcp.Serve(ctx, consolemcp.NewServer(be, nil)); err != nil {
			return fmt.Errorf("mcp server: %w", err)
		}
		return nil
	}
```

(Task 6 replaces `nil` with the real `newMCPAuth(...)`.)

- [ ] **Step 7: Build and vet**

Run: `go build ./... && go vet ./...`
Expected: builds clean.

- [ ] **Step 8: Commit**

```bash
gofmt -w internal/mcp/server.go internal/mcp/confirm.go cmd/store/main.go
git add internal/mcp cmd/store/main.go go.mod go.sum
git commit -m "feat(mcp): console mcp subcommand + stdio server skeleton"
```

---

### Task 2: Taste card storage (`internal/localstore/card.go`)

**Files:**
- Create: `internal/localstore/card.go`
- Create: `internal/localstore/card_test.go`

**Interfaces:**
- Produces: `localstore.Card`, `localstore.CardFavorite`, `LoadCard() (Card, error)`, `SaveCard(Card) error`, `RecordOrder(addrID, addrLabel, restID, restName string, nowUnix int64) error`, `ReconcileCard(c Card, addrs []api.Address) (Card, []string)`.
- Consumes: `configPath()` (existing, used by `presetsPath`), `consolestore/internal/broker/api`.

- [ ] **Step 1: Write the failing test** — `internal/localstore/card_test.go`

```go
package localstore

import (
	"testing"

	"consolestore/internal/broker/api"
)

func TestRecordOrderBumpsFavoriteAndDefault(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := RecordOrder("addr1", "Home", "r1", "McDonald's", 1000); err != nil {
		t.Fatalf("RecordOrder: %v", err)
	}
	if err := RecordOrder("addr1", "Home", "r1", "McDonald's", 2000); err != nil {
		t.Fatalf("RecordOrder 2: %v", err)
	}
	c, err := LoadCard()
	if err != nil {
		t.Fatalf("LoadCard: %v", err)
	}
	if c.DefaultAddrID != "addr1" || c.AddrLabel != "Home" {
		t.Fatalf("default = %q/%q", c.DefaultAddrID, c.AddrLabel)
	}
	if len(c.Favorites) != 1 || c.Favorites[0].Count != 2 || c.Favorites[0].RestaurantID != "r1" {
		t.Fatalf("favorites = %+v", c.Favorites)
	}
}

func TestReconcileCardWarnsOnMissingAddress(t *testing.T) {
	c := Card{Version: 1, DefaultAddrID: "gone", AddrLabel: "Home"}
	got, warns := ReconcileCard(c, []api.Address{{ID: "other", Label: "Office"}})
	if len(warns) != 1 {
		t.Fatalf("warns = %v", warns)
	}
	if got.DefaultAddrID != "" {
		t.Fatalf("expected default cleared, got %q", got.DefaultAddrID)
	}
}
```

- [ ] **Step 2: Run it — must fail**

Run: `go test ./internal/localstore/ -run 'TestRecordOrder|TestReconcileCard'`
Expected: FAIL (undefined symbols).

- [ ] **Step 3: Write the implementation** — `internal/localstore/card.go`

```go
package localstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"consolestore/internal/broker/api"
)

// CardFavorite is one remembered restaurant, ranked by Count/LastUsedUnix.
type CardFavorite struct {
	RestaurantID   string `json:"restaurantId"`
	RestaurantName string `json:"name"`
	Count          int    `json:"count"`
	LastUsedUnix   int64  `json:"lastUsed"`
}

// Card is the local, auto-derived taste profile. It is never built by a wizard:
// RecordOrder accretes it from real placements (TUI, CLI, or MCP), and
// ReconcileCard heals stale references against live addresses.
type Card struct {
	Version       int            `json:"version"`
	DefaultAddrID string         `json:"defaultAddressId"`
	AddrLabel     string         `json:"addressLabel"`
	Favorites     []CardFavorite `json:"favorites"`
	Prefs         []string       `json:"prefs"`
	UpdatedAtUnix int64          `json:"updatedAt"`
}

func cardPath() (string, error) {
	p, err := configPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "card.json"), nil
}

func LoadCard() (Card, error) {
	p, err := cardPath()
	if err != nil {
		return Card{}, err
	}
	raw, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return Card{Version: 1}, nil
	}
	if err != nil {
		return Card{}, err
	}
	var c Card
	if err := json.Unmarshal(raw, &c); err != nil {
		return Card{}, err
	}
	return c, nil
}

func SaveCard(c Card) error {
	if c.Version == 0 {
		c.Version = 1
	}
	p, err := cardPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, raw, 0o600)
}

// RecordOrder updates the card after a successful placement: bump the
// restaurant favorite and set the most-recent address as the default.
func RecordOrder(addrID, addrLabel, restID, restName string, nowUnix int64) error {
	c, err := LoadCard()
	if err != nil {
		return err
	}
	if addrID != "" {
		c.DefaultAddrID = addrID
		c.AddrLabel = addrLabel
	}
	c.bumpFavorite(restID, restName, nowUnix)
	c.UpdatedAtUnix = nowUnix
	return SaveCard(c)
}

func (c *Card) bumpFavorite(restID, restName string, nowUnix int64) {
	if restID == "" {
		return
	}
	for i := range c.Favorites {
		if c.Favorites[i].RestaurantID == restID {
			c.Favorites[i].Count++
			c.Favorites[i].LastUsedUnix = nowUnix
			if restName != "" {
				c.Favorites[i].RestaurantName = restName
			}
			return
		}
	}
	c.Favorites = append(c.Favorites, CardFavorite{
		RestaurantID: restID, RestaurantName: restName, Count: 1, LastUsedUnix: nowUnix,
	})
}

// ReconcileCard heals the card against live addresses. If the default address no
// longer exists it is cleared and a warning is returned for the agent to surface.
func ReconcileCard(c Card, addrs []api.Address) (Card, []string) {
	var warns []string
	if c.DefaultAddrID != "" {
		found := false
		for _, a := range addrs {
			if a.ID == c.DefaultAddrID {
				found = true
				c.AddrLabel = a.Label
				break
			}
		}
		if !found {
			warns = append(warns, fmt.Sprintf("saved default address %q no longer exists — pick a new one on your next order", c.AddrLabel))
			c.DefaultAddrID = ""
		}
	}
	return c, warns
}
```

- [ ] **Step 4: Run the tests — must pass**

Run: `go test ./internal/localstore/ -run 'TestRecordOrder|TestReconcileCard'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/localstore/card.go internal/localstore/card_test.go
git add internal/localstore/card.go internal/localstore/card_test.go
git commit -m "feat(localstore): auto-derived taste card storage + reconcile"
```

---

### Task 3: Discovery tools (read-only) with the not-signed-in guard

**Files:**
- Create: `internal/mcp/tools_discovery.go`
- Create: `internal/mcp/fake_test.go` (shared fake backend + auth)
- Create: `internal/mcp/tools_discovery_test.go`
- Modify: `internal/mcp/server.go` (`register` adds the discovery tools)

**Interfaces:**
- Produces: handlers `handleListAddresses`, `handleSearchRestaurants`, `handleListUsuals`, `handleGetMenu`, `handleGetItemOptions`, `handleListActiveOrders`, `handleTrackOrder`, `handleListPresets`; helper `requireAuth(ctx) error`; `fakeBackend`, `fakeAuth` test doubles.
- Consumes: `Backend`, `Authenticator` (Task 1), `localstore` presets (Task in this plan reuses `localstore.LoadPresets`).

- [ ] **Step 1: Write the shared test doubles** — `internal/mcp/fake_test.go`

```go
package mcp

import (
	"context"

	"consolestore/internal/broker/api"
)

type fakeBackend struct {
	addrs    []api.Address
	search   []api.Restaurant
	menu     api.Menu
	cart     api.Cart
	order    api.Order
	placeErr error
	placed   int
}

func (f *fakeBackend) Addresses() ([]api.Address, error) { return f.addrs, nil }
func (f *fakeBackend) SearchOrganic(addressID, query string) ([]api.Restaurant, string, error) {
	return f.search, query, nil
}
func (f *fakeBackend) PlacesQuery(addressID, query string) ([]api.Restaurant, error) {
	return f.search, nil
}
func (f *fakeBackend) Usuals(addressID string) ([]api.Restaurant, error) { return f.search, nil }
func (f *fakeBackend) Menu(addressID, restaurantID string) (api.Menu, error) { return f.menu, nil }
func (f *fakeBackend) ItemOptions(addressID, restaurantID, itemName, menuItemID string) ([]api.OptionGroup, error) {
	return nil, nil
}
func (f *fakeBackend) GetCart(addressID, restaurantName string) (api.Cart, error) { return f.cart, nil }
func (f *fakeBackend) UpdateCart(addressID, restaurantID, restaurantName string, items []api.CartItem) (api.Cart, error) {
	return f.cart, nil
}
func (f *fakeBackend) ClearCart() error { return nil }
func (f *fakeBackend) PlaceOrder(addressID string) (api.Order, error) {
	f.placed++
	if f.placeErr != nil {
		return api.Order{}, f.placeErr
	}
	return f.order, nil
}
func (f *fakeBackend) TrackOrder(orderID string) (api.Tracking, error)   { return api.Tracking{}, nil }
func (f *fakeBackend) ActiveOrders(addressID string) ([]api.Order, error) { return nil, nil }

type fakeAuth struct {
	token bool
	url   string
	flow  string
	done  bool
}

func (a *fakeAuth) TokenPresent(ctx context.Context) bool { return a.token }
func (a *fakeAuth) Start(ctx context.Context) (string, string, error) {
	return a.url, a.flow, nil
}
func (a *fakeAuth) Authorized(flowID string) bool { return a.done }
```

- [ ] **Step 2: Write the failing tests** — `internal/mcp/tools_discovery_test.go`

```go
package mcp

import (
	"context"
	"testing"

	"consolestore/internal/broker/api"
)

func TestListAddressesRequiresAuth(t *testing.T) {
	s := NewServer(&fakeBackend{}, &fakeAuth{token: false})
	_, _, err := s.handleListAddresses(context.Background(), nil, ListAddressesIn{})
	if err == nil {
		t.Fatalf("expected not-signed-in error")
	}
}

func TestListAddressesReturnsAddresses(t *testing.T) {
	be := &fakeBackend{addrs: []api.Address{{ID: "a1", Label: "Home", Full: "12 Main St"}}}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleListAddresses(context.Background(), nil, ListAddressesIn{})
	if err != nil {
		t.Fatalf("handleListAddresses: %v", err)
	}
	if len(out.Addresses) != 1 || out.Addresses[0].ID != "a1" || out.Addresses[0].Label != "Home" {
		t.Fatalf("addresses = %+v", out.Addresses)
	}
}

func TestSearchRestaurantsReturnsResults(t *testing.T) {
	be := &fakeBackend{search: []api.Restaurant{{ID: "r1", Name: "McDonald's", ETA: "30 mins"}}}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleSearchRestaurants(context.Background(), nil, SearchRestaurantsIn{AddressID: "a1", Query: "mcd"})
	if err != nil {
		t.Fatalf("handleSearchRestaurants: %v", err)
	}
	if len(out.Restaurants) != 1 || out.Restaurants[0].Name != "McDonald's" {
		t.Fatalf("restaurants = %+v", out.Restaurants)
	}
}
```

- [ ] **Step 3: Run them — must fail**

Run: `go test ./internal/mcp/ -run 'TestListAddresses|TestSearchRestaurants'`
Expected: FAIL (undefined).

- [ ] **Step 4: Write the discovery tools** — `internal/mcp/tools_discovery.go`

```go
package mcp

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"consolestore/internal/broker/api"
	"consolestore/internal/localstore"
)

// requireAuth gates every data/order tool. The agent is told to call sign_in.
func (s *Server) requireAuth(ctx context.Context) error {
	if s.auth == nil || !s.auth.TokenPresent(ctx) {
		return errors.New("not signed in — call the sign_in tool to authorize, then retry")
	}
	return nil
}

// --- DTOs (lean projections of api.* so the agent gets stable, documented shapes) ---

type AddressDTO struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Full  string `json:"full"`
}
type RestaurantDTO struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	ETA         string  `json:"eta"`
	Rating      float64 `json:"rating"`
	Offer       string  `json:"offer,omitempty"`
	Unavailable bool    `json:"unavailable"`
}
type MenuItemDTO struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Price        int    `json:"price"`
	Veg          bool   `json:"veg"`
	InStock      bool   `json:"inStock"`
	Customizable bool   `json:"customizable"`
}

func toAddressDTOs(in []api.Address) []AddressDTO {
	out := make([]AddressDTO, 0, len(in))
	for _, a := range in {
		out = append(out, AddressDTO{ID: a.ID, Label: a.Label, Full: a.Full})
	}
	return out
}
func toRestaurantDTOs(in []api.Restaurant) []RestaurantDTO {
	out := make([]RestaurantDTO, 0, len(in))
	for _, r := range in {
		out = append(out, RestaurantDTO{ID: r.ID, Name: r.Name, ETA: r.ETA, Rating: r.Rating, Offer: r.Offer, Unavailable: r.Unavailable})
	}
	return out
}
func toMenuItemDTOs(in []api.MenuItem) []MenuItemDTO {
	out := make([]MenuItemDTO, 0, len(in))
	for _, m := range in {
		out = append(out, MenuItemDTO{ID: m.ID, Name: m.Name, Price: m.Price, Veg: m.Veg, InStock: m.InStock, Customizable: m.Customizable})
	}
	return out
}

// --- list_addresses ---

type ListAddressesIn struct{}
type ListAddressesOut struct {
	Addresses []AddressDTO `json:"addresses"`
}

func (s *Server) handleListAddresses(ctx context.Context, _ *mcp.CallToolRequest, _ ListAddressesIn) (*mcp.CallToolResult, ListAddressesOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, ListAddressesOut{}, err
	}
	addrs, err := s.be.Addresses()
	if err != nil {
		return nil, ListAddressesOut{}, err
	}
	return nil, ListAddressesOut{Addresses: toAddressDTOs(addrs)}, nil
}

// --- search_restaurants ---

type SearchRestaurantsIn struct {
	AddressID string `json:"address_id" jsonschema:"the delivery address id from list_addresses"`
	Query     string `json:"query" jsonschema:"restaurant or dish to search for"`
}
type SearchRestaurantsOut struct {
	Restaurants []RestaurantDTO `json:"restaurants"`
	Corrected   string          `json:"corrected_query,omitempty"`
}

func (s *Server) handleSearchRestaurants(ctx context.Context, _ *mcp.CallToolRequest, in SearchRestaurantsIn) (*mcp.CallToolResult, SearchRestaurantsOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, SearchRestaurantsOut{}, err
	}
	res, effective, err := s.be.SearchOrganic(in.AddressID, in.Query)
	if err != nil {
		return nil, SearchRestaurantsOut{}, err
	}
	out := SearchRestaurantsOut{Restaurants: toRestaurantDTOs(res)}
	if effective != in.Query {
		out.Corrected = effective
	}
	return nil, out, nil
}

// --- list_usuals ---

type ListUsualsIn struct {
	AddressID string `json:"address_id"`
}
type ListUsualsOut struct {
	Restaurants []RestaurantDTO `json:"restaurants"`
}

func (s *Server) handleListUsuals(ctx context.Context, _ *mcp.CallToolRequest, in ListUsualsIn) (*mcp.CallToolResult, ListUsualsOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, ListUsualsOut{}, err
	}
	res, err := s.be.Usuals(in.AddressID)
	if err != nil {
		return nil, ListUsualsOut{}, err
	}
	return nil, ListUsualsOut{Restaurants: toRestaurantDTOs(res)}, nil
}

// --- get_menu ---

type GetMenuIn struct {
	AddressID    string `json:"address_id"`
	RestaurantID string `json:"restaurant_id"`
}
type GetMenuOut struct {
	RestaurantID string        `json:"restaurant_id"`
	Items        []MenuItemDTO `json:"items"`
}

func (s *Server) handleGetMenu(ctx context.Context, _ *mcp.CallToolRequest, in GetMenuIn) (*mcp.CallToolResult, GetMenuOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, GetMenuOut{}, err
	}
	m, err := s.be.Menu(in.AddressID, in.RestaurantID)
	if err != nil {
		return nil, GetMenuOut{}, err
	}
	return nil, GetMenuOut{RestaurantID: m.RestaurantID, Items: toMenuItemDTOs(m.Items)}, nil
}

// --- get_item_options ---

type GetItemOptionsIn struct {
	AddressID    string `json:"address_id"`
	RestaurantID string `json:"restaurant_id"`
	ItemName     string `json:"item_name"`
	MenuItemID   string `json:"menu_item_id"`
}
type OptionChoiceDTO struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Price   int    `json:"price"`
	InStock bool   `json:"inStock"`
}
type OptionGroupDTO struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Min      int               `json:"min"`
	Max      int               `json:"max"`
	Variant  bool              `json:"variant"`
	Absolute bool              `json:"absolute"`
	Choices  []OptionChoiceDTO `json:"choices"`
}
type GetItemOptionsOut struct {
	Groups []OptionGroupDTO `json:"groups"`
}

func (s *Server) handleGetItemOptions(ctx context.Context, _ *mcp.CallToolRequest, in GetItemOptionsIn) (*mcp.CallToolResult, GetItemOptionsOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, GetItemOptionsOut{}, err
	}
	groups, err := s.be.ItemOptions(in.AddressID, in.RestaurantID, in.ItemName, in.MenuItemID)
	if err != nil {
		return nil, GetItemOptionsOut{}, err
	}
	out := GetItemOptionsOut{Groups: make([]OptionGroupDTO, 0, len(groups))}
	for _, g := range groups {
		dg := OptionGroupDTO{ID: g.ID, Name: g.Name, Min: g.Min, Max: g.Max, Variant: g.Variant, Absolute: g.Absolute}
		for _, c := range g.Choices {
			dg.Choices = append(dg.Choices, OptionChoiceDTO{ID: c.ID, Name: c.Name, Price: c.Price, InStock: c.InStock})
		}
		out.Groups = append(out.Groups, dg)
	}
	return nil, out, nil
}

// --- list_active_orders ---

type ListActiveOrdersIn struct {
	AddressID string `json:"address_id"`
}
type OrderDTO struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	Restaurant string `json:"restaurant"`
	Total      int    `json:"total"`
	ETA        string `json:"eta"`
}
type ListActiveOrdersOut struct {
	Orders []OrderDTO `json:"orders"`
}

func toOrderDTO(o api.Order) OrderDTO {
	return OrderDTO{ID: o.ID, Status: o.Status, Restaurant: o.Restaurant, Total: o.Total, ETA: o.ETA}
}

func (s *Server) handleListActiveOrders(ctx context.Context, _ *mcp.CallToolRequest, in ListActiveOrdersIn) (*mcp.CallToolResult, ListActiveOrdersOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, ListActiveOrdersOut{}, err
	}
	orders, err := s.be.ActiveOrders(in.AddressID)
	if err != nil {
		return nil, ListActiveOrdersOut{}, err
	}
	out := ListActiveOrdersOut{Orders: make([]OrderDTO, 0, len(orders))}
	for _, o := range orders {
		out.Orders = append(out.Orders, toOrderDTO(o))
	}
	return nil, out, nil
}

// --- track_order ---

type TrackOrderIn struct {
	OrderID string `json:"order_id"`
}
type TrackOrderOut struct {
	Status string `json:"status"`
	ETA    string `json:"eta"`
}

func (s *Server) handleTrackOrder(ctx context.Context, _ *mcp.CallToolRequest, in TrackOrderIn) (*mcp.CallToolResult, TrackOrderOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, TrackOrderOut{}, err
	}
	tr, err := s.be.TrackOrder(in.OrderID)
	if err != nil {
		return nil, TrackOrderOut{}, err
	}
	return nil, TrackOrderOut{Status: tr.Status, ETA: tr.ETA}, nil
}

// --- list_presets (local, no Swiggy call, still gated for consistency) ---

type ListPresetsIn struct{}
type PresetDTO struct {
	Name           string `json:"name"`
	RestaurantName string `json:"restaurant"`
	AddrLine       string `json:"address"`
	Lines          int    `json:"line_count"`
}
type ListPresetsOut struct {
	Presets []PresetDTO `json:"presets"`
}

func (s *Server) handleListPresets(ctx context.Context, _ *mcp.CallToolRequest, _ ListPresetsIn) (*mcp.CallToolResult, ListPresetsOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, ListPresetsOut{}, err
	}
	ps, err := localstore.LoadPresets()
	if err != nil {
		return nil, ListPresetsOut{}, err
	}
	out := ListPresetsOut{Presets: make([]PresetDTO, 0, len(ps.Items))}
	for _, p := range ps.Items {
		out.Presets = append(out.Presets, PresetDTO{Name: p.Name, RestaurantName: p.RestaurantName, AddrLine: p.AddrLine, Lines: len(p.Lines)})
	}
	return nil, out, nil
}
```

Confirm the `api.Tracking` field names before relying on them: open `internal/broker/api/tracking.go` and use the actual `Status`/`ETA` field names (adjust `handleTrackOrder` if they differ).

- [ ] **Step 5: Register the discovery tools** — in `internal/mcp/server.go`, extend `register`:

```go
func (s *Server) register(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{Name: "server_info", Description: "consolestore server name and version"}, s.handleServerInfo)
	mcp.AddTool(srv, &mcp.Tool{Name: "list_addresses", Description: "the user's saved Swiggy delivery addresses"}, s.handleListAddresses)
	mcp.AddTool(srv, &mcp.Tool{Name: "search_restaurants", Description: "search restaurants/dishes deliverable to an address"}, s.handleSearchRestaurants)
	mcp.AddTool(srv, &mcp.Tool{Name: "list_usuals", Description: "the user's frequently ordered restaurants for an address"}, s.handleListUsuals)
	mcp.AddTool(srv, &mcp.Tool{Name: "get_menu", Description: "menu items for a restaurant at an address"}, s.handleGetMenu)
	mcp.AddTool(srv, &mcp.Tool{Name: "get_item_options", Description: "variant/add-on groups for a customizable item"}, s.handleGetItemOptions)
	mcp.AddTool(srv, &mcp.Tool{Name: "list_active_orders", Description: "live (in-progress) orders for an address"}, s.handleListActiveOrders)
	mcp.AddTool(srv, &mcp.Tool{Name: "track_order", Description: "live status + ETA for an order id"}, s.handleTrackOrder)
	mcp.AddTool(srv, &mcp.Tool{Name: "list_presets", Description: "saved order presets (named cart snapshots)"}, s.handleListPresets)
}
```

- [ ] **Step 6: Run the tests — must pass**

Run: `go test ./internal/mcp/ -run 'TestListAddresses|TestSearchRestaurants'`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
gofmt -w internal/mcp/
git add internal/mcp/
git commit -m "feat(mcp): discovery tools (addresses, search, menu, options, orders, presets)"
```

---

### Task 4: Cart tools (`get_cart`, `update_cart`, `clear_cart`)

**Files:**
- Create: `internal/mcp/tools_cart.go`
- Create: `internal/mcp/tools_cart_test.go`
- Modify: `internal/mcp/server.go` (`register`)

**Interfaces:**
- Produces: `handleGetCart`, `handleUpdateCart`, `handleClearCart`; `CartDTO`, `cartToDTO(api.Cart) CartDTO` (reused by the order task).
- Consumes: `Backend`, `api.CartItem`, `api.Cart`.

- [ ] **Step 1: Write the failing test** — `internal/mcp/tools_cart_test.go`

```go
package mcp

import (
	"context"
	"testing"

	"consolestore/internal/broker/api"
)

func TestGetCartReturnsBill(t *testing.T) {
	be := &fakeBackend{cart: api.Cart{Total: 250, ItemTotal: 200, Delivery: 30, Taxes: 20,
		Lines: []api.CartLine{{ItemID: "i1", Name: "Burger", Quantity: 1, Price: 200, Available: true}}}}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleGetCart(context.Background(), nil, GetCartIn{AddressID: "a1"})
	if err != nil {
		t.Fatalf("handleGetCart: %v", err)
	}
	if out.Cart.Total != 250 || len(out.Cart.Lines) != 1 || out.Cart.Lines[0].Name != "Burger" {
		t.Fatalf("cart = %+v", out.Cart)
	}
}

func TestUpdateCartReturnsCart(t *testing.T) {
	be := &fakeBackend{cart: api.Cart{Total: 200}}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleUpdateCart(context.Background(), nil, UpdateCartIn{
		AddressID: "a1", RestaurantID: "r1",
		Items: []CartItemIn{{ItemID: "i1", Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("handleUpdateCart: %v", err)
	}
	if out.Cart.Total != 200 {
		t.Fatalf("total = %d", out.Cart.Total)
	}
}
```

- [ ] **Step 2: Run it — must fail**

Run: `go test ./internal/mcp/ -run 'TestGetCart|TestUpdateCart'`
Expected: FAIL.

- [ ] **Step 3: Write the cart tools** — `internal/mcp/tools_cart.go`

```go
package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"consolestore/internal/broker/api"
)

type CartLineDTO struct {
	ItemID    string `json:"item_id"`
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	Price     int    `json:"price"`
	Available bool   `json:"available"`
}
type CartDTO struct {
	Restaurant string        `json:"restaurant"`
	ItemTotal  int           `json:"item_total"`
	Delivery   int           `json:"delivery"`
	Taxes      int           `json:"taxes"`
	Total      int           `json:"total"`
	Lines      []CartLineDTO `json:"lines"`
}

func cartToDTO(c api.Cart) CartDTO {
	d := CartDTO{Restaurant: c.Restaurant, ItemTotal: c.ItemTotal, Delivery: c.Delivery, Taxes: c.Taxes, Total: c.Total}
	for _, l := range c.Lines {
		d.Lines = append(d.Lines, CartLineDTO{ItemID: l.ItemID, Name: l.Name, Quantity: l.Quantity, Price: l.Price, Available: l.Available})
	}
	return d
}

// CartItemIn mirrors api.CartItem with snake_case selection groups.
type CartVariantSelIn struct {
	GroupID     string `json:"group_id"`
	VariationID string `json:"variation_id"`
}
type CartAddonSelIn struct {
	GroupID  string `json:"group_id"`
	ChoiceID string `json:"choice_id"`
}
type CartItemIn struct {
	ItemID         string             `json:"item_id"`
	Quantity       int                `json:"quantity"`
	VariantsV2     []CartVariantSelIn `json:"variants_v2,omitempty"`
	VariantsLegacy []CartVariantSelIn `json:"variants_legacy,omitempty"`
	Addons         []CartAddonSelIn   `json:"addons,omitempty"`
}

func toCartItems(in []CartItemIn) []api.CartItem {
	out := make([]api.CartItem, 0, len(in))
	for _, ci := range in {
		item := api.CartItem{ItemID: ci.ItemID, Quantity: ci.Quantity}
		for _, v := range ci.VariantsV2 {
			item.VariantsV2 = append(item.VariantsV2, api.CartVariantSel{GroupID: v.GroupID, VariationID: v.VariationID})
		}
		for _, v := range ci.VariantsLegacy {
			item.VariantsLegacy = append(item.VariantsLegacy, api.CartVariantSel{GroupID: v.GroupID, VariationID: v.VariationID})
		}
		for _, a := range ci.Addons {
			item.Addons = append(item.Addons, api.CartAddonSel{GroupID: a.GroupID, ChoiceID: a.ChoiceID})
		}
		out = append(out, item)
	}
	return out
}

// --- get_cart ---

type GetCartIn struct {
	AddressID string `json:"address_id"`
}
type GetCartOut struct {
	Cart CartDTO `json:"cart"`
}

func (s *Server) handleGetCart(ctx context.Context, _ *mcp.CallToolRequest, in GetCartIn) (*mcp.CallToolResult, GetCartOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, GetCartOut{}, err
	}
	c, err := s.be.GetCart(in.AddressID, "")
	if err != nil {
		return nil, GetCartOut{}, err
	}
	return nil, GetCartOut{Cart: cartToDTO(c)}, nil
}

// --- update_cart ---

type UpdateCartIn struct {
	AddressID      string       `json:"address_id"`
	RestaurantID   string       `json:"restaurant_id"`
	RestaurantName string       `json:"restaurant_name,omitempty"`
	Items          []CartItemIn `json:"items" jsonschema:"the full desired set of cart lines (this replaces the cart for the restaurant)"`
}
type UpdateCartOut struct {
	Cart CartDTO `json:"cart"`
}

func (s *Server) handleUpdateCart(ctx context.Context, _ *mcp.CallToolRequest, in UpdateCartIn) (*mcp.CallToolResult, UpdateCartOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, UpdateCartOut{}, err
	}
	c, err := s.be.UpdateCart(in.AddressID, in.RestaurantID, in.RestaurantName, toCartItems(in.Items))
	if err != nil {
		return nil, UpdateCartOut{}, err
	}
	return nil, UpdateCartOut{Cart: cartToDTO(c)}, nil
}

// --- clear_cart ---

type ClearCartIn struct{}
type ClearCartOut struct {
	Cleared bool `json:"cleared"`
}

func (s *Server) handleClearCart(ctx context.Context, _ *mcp.CallToolRequest, _ ClearCartIn) (*mcp.CallToolResult, ClearCartOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, ClearCartOut{}, err
	}
	if err := s.be.ClearCart(); err != nil {
		return nil, ClearCartOut{}, err
	}
	return nil, ClearCartOut{Cleared: true}, nil
}
```

- [ ] **Step 4: Register the cart tools** — extend `register` in `server.go`:

```go
	mcp.AddTool(srv, &mcp.Tool{Name: "get_cart", Description: "the current cart with the authoritative Swiggy bill"}, s.handleGetCart)
	mcp.AddTool(srv, &mcp.Tool{Name: "update_cart", Description: "set the cart lines for a restaurant (replaces the cart)"}, s.handleUpdateCart)
	mcp.AddTool(srv, &mcp.Tool{Name: "clear_cart", Description: "empty the cart"}, s.handleClearCart)
```

- [ ] **Step 5: Run the tests — must pass**

Run: `go test ./internal/mcp/ -run 'TestGetCart|TestUpdateCart'`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -w internal/mcp/
git add internal/mcp/
git commit -m "feat(mcp): cart tools (get_cart, update_cart, clear_cart)"
```

---

### Task 5: Two-tool order-commit gate (`prepare_order`, `place_order`, `order_preset`)

**Files:**
- Modify: `internal/mcp/confirm.go` (real confirmation store)
- Create: `internal/mcp/tools_order.go`
- Create: `internal/mcp/tools_order_test.go`
- Modify: `internal/mcp/server.go` (`register`)
- Modify: `internal/cli/order.go` (extract `presetToCartItems` → `localstore.PresetCartItems`)
- Create/Modify: `internal/localstore/presets.go` (add `PresetCartItems`) + a test

**Interfaces:**
- Produces: `confirmStore` with `put(addressID string, c api.Cart) string` and `take(id string, nowUnix int64) (pendingOrder, bool)`; handlers `handlePrepareOrder`, `handlePlaceOrder`, `handleOrderPreset`; `localstore.PresetCartItems(p Preset) []api.CartItem`.
- Consumes: `Backend.GetCart/UpdateCart/PlaceOrder`, `localstore.RecordOrder` (Task 2), `localstore.SaveActiveOrder`, `localstore.ParseETAMinutes`.

- [ ] **Step 1: Extract the preset→cart mapping** — `internal/localstore/presets.go`

Add (this is the verbatim body of `presetToCartItems` from `internal/cli/order.go:149-166`, moved here so both the CLI and the MCP reuse it). Add `"consolestore/internal/broker/api"` to the file's imports:

```go
// PresetCartItems maps a preset's lines to api.CartItem, replaying the exact
// channel routing the TUI uses (variantsV2 / variantsLegacy / addons).
func PresetCartItems(p Preset) []api.CartItem {
	out := make([]api.CartItem, 0, len(p.Lines))
	for _, l := range p.Lines {
		ci := api.CartItem{ItemID: l.ItemID, Quantity: l.Qty}
		for _, s := range l.Sels {
			switch {
			case s.Variant && s.Absolute:
				ci.VariantsV2 = append(ci.VariantsV2, api.CartVariantSel{GroupID: s.GroupID, VariationID: s.ChoiceID})
			case s.Variant:
				ci.VariantsLegacy = append(ci.VariantsLegacy, api.CartVariantSel{GroupID: s.GroupID, VariationID: s.ChoiceID})
			default:
				ci.Addons = append(ci.Addons, api.CartAddonSel{GroupID: s.GroupID, ChoiceID: s.ChoiceID})
			}
		}
		out = append(out, ci)
	}
	return out
}
```

Then in `internal/cli/order.go`, delete the local `presetToCartItems` func (lines 147-166) and replace its one call site `presetToCartItems(p)` with `localstore.PresetCartItems(p)`.

- [ ] **Step 2: Write the failing tests** — `internal/mcp/tools_order_test.go`

```go
package mcp

import (
	"context"
	"errors"
	"testing"

	"consolestore/internal/broker/api"
)

func TestPrepareThenPlaceSucceeds(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{
		cart:  api.Cart{Restaurant: "McDonald's", Total: 250, Lines: []api.CartLine{{ItemID: "i1", Quantity: 1, Price: 250, Available: true}}},
		order: api.Order{ID: "OID1", Status: "placed", Restaurant: "McDonald's", Total: 250, ETA: "30 mins"},
	}
	s := NewServer(be, &fakeAuth{token: true})

	_, prep, err := s.handlePrepareOrder(context.Background(), nil, PrepareOrderIn{AddressID: "a1"})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if prep.ConfirmationID == "" || prep.Bill.Total != 250 {
		t.Fatalf("prep = %+v", prep)
	}
	_, plc, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: prep.ConfirmationID})
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	if plc.Order.ID != "OID1" {
		t.Fatalf("order = %+v", plc.Order)
	}
	if be.placed != 1 {
		t.Fatalf("placed = %d, want 1", be.placed)
	}
}

func TestPlaceRejectsUnknownConfirmation(t *testing.T) {
	be := &fakeBackend{cart: api.Cart{Total: 250}}
	s := NewServer(be, &fakeAuth{token: true})
	_, _, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: "nope"})
	if err == nil {
		t.Fatalf("expected rejection for unknown confirmation id")
	}
	if be.placed != 0 {
		t.Fatalf("placed = %d, want 0", be.placed)
	}
}

func TestPlaceRejectsChangedCart(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{cart: api.Cart{Restaurant: "X", Total: 250, Lines: []api.CartLine{{ItemID: "i1", Quantity: 1, Available: true}}}}
	s := NewServer(be, &fakeAuth{token: true})
	_, prep, _ := s.handlePrepareOrder(context.Background(), nil, PrepareOrderIn{AddressID: "a1"})
	be.cart.Total = 999 // cart drifted after prepare
	_, _, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: prep.ConfirmationID})
	if err == nil {
		t.Fatalf("expected rejection for changed cart")
	}
	if be.placed != 0 {
		t.Fatalf("placed = %d, want 0", be.placed)
	}
}

func TestPlaceOrderNeverRetries(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{
		cart:     api.Cart{Restaurant: "X", Total: 250, Lines: []api.CartLine{{ItemID: "i1", Quantity: 1, Available: true}}},
		placeErr: errors.New("502 bad gateway"),
	}
	s := NewServer(be, &fakeAuth{token: true})
	_, prep, _ := s.handlePrepareOrder(context.Background(), nil, PrepareOrderIn{AddressID: "a1"})
	_, _, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: prep.ConfirmationID})
	if err == nil {
		t.Fatalf("expected place error")
	}
	if be.placed != 1 {
		t.Fatalf("PlaceOrder called %d times, want exactly 1 (no retry)", be.placed)
	}
}
```

- [ ] **Step 3: Run them — must fail**

Run: `go test ./internal/mcp/ -run 'TestPrepare|TestPlace'`
Expected: FAIL.

- [ ] **Step 4: Implement the confirmation store** — replace `internal/mcp/confirm.go`

```go
package mcp

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"

	"consolestore/internal/broker/api"
)

const confirmTTLSeconds = 600 // 10 minutes

type pendingOrder struct {
	addressID  string
	restaurant string
	total      int
	hash       string
	createdAt  int64
}

type confirmStore struct {
	mu sync.Mutex
	m  map[string]pendingOrder
}

func newConfirmStore() *confirmStore { return &confirmStore{m: map[string]pendingOrder{}} }

// cartHash binds a confirmation to the exact lines + address + total the user saw.
func cartHash(addressID string, c api.Cart) string {
	type kv struct {
		id  string
		qty int
	}
	lines := make([]kv, 0, len(c.Lines))
	for _, l := range c.Lines {
		lines = append(lines, kv{l.ItemID, l.Quantity})
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].id < lines[j].id })
	h := sha256.New()
	fmt.Fprintf(h, "addr=%s;total=%d;", addressID, c.Total)
	for _, l := range lines {
		fmt.Fprintf(h, "%s:%d;", l.id, l.qty)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (s *confirmStore) put(addressID string, c api.Cart, nowUnix int64) string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	id := hex.EncodeToString(b[:])
	s.mu.Lock()
	s.m[id] = pendingOrder{
		addressID: addressID, restaurant: c.Restaurant, total: c.Total,
		hash: cartHash(addressID, c), createdAt: nowUnix,
	}
	s.mu.Unlock()
	return id
}

// take removes and returns the pending order if present and not expired.
func (s *confirmStore) take(id string, nowUnix int64) (pendingOrder, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.m[id]
	if !ok {
		return pendingOrder{}, false
	}
	delete(s.m, id)
	if nowUnix-p.createdAt > confirmTTLSeconds {
		return pendingOrder{}, false
	}
	return p, true
}
```

- [ ] **Step 5: Implement the order tools** — `internal/mcp/tools_order.go`

```go
package mcp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"consolestore/internal/broker/api"
	"consolestore/internal/localstore"
)

func nowUnix() int64 { return time.Now().Unix() }

// prepare syncs the cart, validates availability, stores a confirmation bound to
// the bill, and returns both. Shared by prepare_order and order_preset.
func (s *Server) prepare(addressID string, c api.Cart) (string, CartDTO, error) {
	if len(c.Lines) == 0 {
		return "", CartDTO{}, errors.New("cart is empty — add items before preparing an order")
	}
	for _, l := range c.Lines {
		if !l.Available {
			return "", CartDTO{}, fmt.Errorf("%q is sold out — remove it before ordering", l.Name)
		}
	}
	id := s.pending.put(addressID, c, nowUnix())
	return id, cartToDTO(c), nil
}

type PrepareOrderIn struct {
	AddressID string `json:"address_id"`
}
type PrepareOrderOut struct {
	ConfirmationID string  `json:"confirmation_id"`
	Bill           CartDTO `json:"bill"`
	Note           string  `json:"note"`
}

func (s *Server) handlePrepareOrder(ctx context.Context, _ *mcp.CallToolRequest, in PrepareOrderIn) (*mcp.CallToolResult, PrepareOrderOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, PrepareOrderOut{}, err
	}
	c, err := s.be.GetCart(in.AddressID, "")
	if err != nil {
		return nil, PrepareOrderOut{}, err
	}
	id, bill, err := s.prepare(in.AddressID, c)
	if err != nil {
		return nil, PrepareOrderOut{}, err
	}
	return nil, PrepareOrderOut{
		ConfirmationID: id, Bill: bill,
		Note: "show this bill to the user; call place_order with this confirmation_id ONLY after they confirm.",
	}, nil
}

type PlaceOrderIn struct {
	ConfirmationID string `json:"confirmation_id"`
}
type PlaceOrderOut struct {
	Order OrderDTO `json:"order"`
}

func (s *Server) handlePlaceOrder(ctx context.Context, _ *mcp.CallToolRequest, in PlaceOrderIn) (*mcp.CallToolResult, PlaceOrderOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, PlaceOrderOut{}, err
	}
	p, ok := s.pending.take(in.ConfirmationID, nowUnix())
	if !ok {
		return nil, PlaceOrderOut{}, errors.New("unknown or expired confirmation_id — call prepare_order again")
	}
	// Re-fetch and verify the cart still matches what the user confirmed.
	c, err := s.be.GetCart(p.addressID, "")
	if err != nil {
		return nil, PlaceOrderOut{}, err
	}
	if cartHash(p.addressID, c) != p.hash || c.Total != p.total {
		return nil, PlaceOrderOut{}, errors.New("cart changed since prepare_order — call prepare_order again to re-confirm")
	}
	order, err := s.be.PlaceOrder(p.addressID) // never retried
	if err != nil {
		return nil, PlaceOrderOut{}, fmt.Errorf("order failed: %w — run list_active_orders before retrying in case it was placed", err)
	}
	// Persist for `console status`/tracking and accrete the taste card.
	etaLo, etaHi := localstore.ParseETAMinutes(order.ETA)
	_ = localstore.SaveActiveOrder(localstore.ActiveOrder{
		OrderID: order.ID, Restaurant: order.Restaurant, ETALoMin: etaLo, ETAHiMin: etaHi,
		Total: order.Total, PlacedAt: nowUnix(),
	})
	_ = localstore.RecordOrder(p.addressID, "", order.Restaurant, p.restaurant, nowUnix())
	return nil, PlaceOrderOut{Order: toOrderDTO(order)}, nil
}

type OrderPresetIn struct {
	Name  string `json:"name"`
	Index int    `json:"index,omitempty" jsonschema:"0-based pick among presets sharing a name; default 0"`
}
type OrderPresetOut struct {
	ConfirmationID string  `json:"confirmation_id"`
	Bill           CartDTO `json:"bill"`
	Note           string  `json:"note"`
}

func (s *Server) handleOrderPreset(ctx context.Context, _ *mcp.CallToolRequest, in OrderPresetIn) (*mcp.CallToolResult, OrderPresetOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, OrderPresetOut{}, err
	}
	ps, err := localstore.LoadPresets()
	if err != nil {
		return nil, OrderPresetOut{}, err
	}
	matches := ps.ByName(in.Name)
	if len(matches) == 0 {
		return nil, OrderPresetOut{}, fmt.Errorf("no preset named %q", in.Name)
	}
	if in.Index < 0 || in.Index >= len(matches) {
		return nil, OrderPresetOut{}, fmt.Errorf("preset %q has %d entries; index %d out of range", in.Name, len(matches), in.Index)
	}
	p := matches[in.Index]
	c, err := s.be.UpdateCart(p.AddrID, p.RestaurantID, p.RestaurantName, localstore.PresetCartItems(p))
	if err != nil {
		return nil, OrderPresetOut{}, err
	}
	id, bill, err := s.prepare(p.AddrID, c)
	if err != nil {
		return nil, OrderPresetOut{}, err
	}
	return nil, OrderPresetOut{ConfirmationID: id, Bill: bill,
		Note: "show this bill; call place_order with this confirmation_id ONLY after the user confirms."}, nil
}
```

Confirm `localstore.ActiveOrder` field names and `localstore.ParseETAMinutes`/`SaveActiveOrder` signatures by reading `internal/localstore` (they are used identically in `internal/cli/order.go:134-138`); match them exactly.

- [ ] **Step 6: Register the order tools** — extend `register` in `server.go`:

```go
	mcp.AddTool(srv, &mcp.Tool{Name: "prepare_order", Description: "sync the cart and return the real bill + a confirmation_id (does NOT place)"}, s.handlePrepareOrder)
	mcp.AddTool(srv, &mcp.Tool{Name: "place_order", Description: "place the order for a confirmation_id from prepare_order (real, charges COD; never call without user confirmation)"}, s.handlePlaceOrder)
	mcp.AddTool(srv, &mcp.Tool{Name: "order_preset", Description: "load a saved preset into the cart and return a bill + confirmation_id (does NOT place)"}, s.handleOrderPreset)
```

- [ ] **Step 7: Run all mcp + cli + localstore tests — must pass**

Run: `go test ./internal/mcp/ ./internal/cli/ ./internal/localstore/`
Expected: PASS (the CLI still passes after the `PresetCartItems` extraction).

- [ ] **Step 8: Commit**

```bash
gofmt -w internal/mcp/ internal/cli/order.go internal/localstore/presets.go
git add internal/mcp internal/cli/order.go internal/localstore/presets.go
git commit -m "feat(mcp): two-tool order gate (prepare_order/place_order/order_preset)"
```

---

### Task 6: Agent-driven sign-in (`sign_in`, `auth_status`) + real `Authenticator`

**Files:**
- Create: `internal/mcp/tools_auth.go`
- Create: `internal/mcp/tools_auth_test.go`
- Modify: `internal/mcp/server.go` (`register`)
- Modify: `cmd/store/main.go` (`bootstrap` returns auth handles; `newMCPAuth` adapter; `openBrowser`)

**Interfaces:**
- Produces: `handleSignIn`, `handleAuthStatus`; `mcpAuth` (in package main) implementing `Authenticator`.
- Consumes: `Authenticator` (Task 1); from main: `authMgr.Start`, `authMgr.Authorized`, `ls.GetTokenFull`, `serveCallback`, `callbackAddr`.

- [ ] **Step 1: Write the failing tests** — `internal/mcp/tools_auth_test.go`

```go
package mcp

import (
	"context"
	"testing"
)

func TestAuthStatusReportsSignedIn(t *testing.T) {
	s := NewServer(&fakeBackend{}, &fakeAuth{token: true})
	_, out, err := s.handleAuthStatus(context.Background(), nil, AuthStatusIn{})
	if err != nil {
		t.Fatalf("auth_status: %v", err)
	}
	if !out.SignedIn {
		t.Fatalf("expected signed in")
	}
}

func TestSignInReturnsAuthorizeURL(t *testing.T) {
	s := NewServer(&fakeBackend{}, &fakeAuth{token: false, url: "https://auth.example/x", flow: "F1"})
	_, out, err := s.handleSignIn(context.Background(), nil, SignInIn{})
	if err != nil {
		t.Fatalf("sign_in: %v", err)
	}
	if out.AuthorizeURL != "https://auth.example/x" {
		t.Fatalf("url = %q", out.AuthorizeURL)
	}
}

func TestSignInWhenAlreadySignedIn(t *testing.T) {
	s := NewServer(&fakeBackend{}, &fakeAuth{token: true})
	_, out, err := s.handleSignIn(context.Background(), nil, SignInIn{})
	if err != nil {
		t.Fatalf("sign_in: %v", err)
	}
	if !out.AlreadySignedIn {
		t.Fatalf("expected AlreadySignedIn")
	}
}
```

- [ ] **Step 2: Run them — must fail**

Run: `go test ./internal/mcp/ -run 'TestAuthStatus|TestSignIn'`
Expected: FAIL.

- [ ] **Step 3: Implement the auth tools** — `internal/mcp/tools_auth.go`

```go
package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type AuthStatusIn struct{}
type AuthStatusOut struct {
	SignedIn bool `json:"signed_in"`
}

func (s *Server) handleAuthStatus(ctx context.Context, _ *mcp.CallToolRequest, _ AuthStatusIn) (*mcp.CallToolResult, AuthStatusOut, error) {
	return nil, AuthStatusOut{SignedIn: s.auth != nil && s.auth.TokenPresent(ctx)}, nil
}

type SignInIn struct{}
type SignInOut struct {
	AlreadySignedIn bool   `json:"already_signed_in"`
	AuthorizeURL    string `json:"authorize_url,omitempty"`
	FlowID          string `json:"flow_id,omitempty"`
	Note            string `json:"note,omitempty"`
}

func (s *Server) handleSignIn(ctx context.Context, _ *mcp.CallToolRequest, _ SignInIn) (*mcp.CallToolResult, SignInOut, error) {
	if s.auth == nil {
		return nil, SignInOut{}, errAuthUnavailable
	}
	if s.auth.TokenPresent(ctx) {
		return nil, SignInOut{AlreadySignedIn: true, Note: "already signed in"}, nil
	}
	url, flow, err := s.auth.Start(ctx)
	if err != nil {
		return nil, SignInOut{}, err
	}
	return nil, SignInOut{
		AuthorizeURL: url, FlowID: flow,
		Note: "open authorize_url in a browser to sign in (it may have opened automatically); then poll auth_status until signed_in is true.",
	}, nil
}
```

Add to `internal/mcp/server.go` (package-level): `var errAuthUnavailable = errors.New("sign-in is unavailable in this build")` and the `"errors"` import if not already present.

- [ ] **Step 4: Register the auth tools** — extend `register` in `server.go`:

```go
	mcp.AddTool(srv, &mcp.Tool{Name: "sign_in", Description: "start Swiggy sign-in; returns a browser URL (opened automatically when possible)"}, s.handleSignIn)
	mcp.AddTool(srv, &mcp.Tool{Name: "auth_status", Description: "whether the user is signed in"}, s.handleAuthStatus)
```

- [ ] **Step 5: Run the mcp tests — must pass**

Run: `go test ./internal/mcp/ -run 'TestAuthStatus|TestSignIn'`
Expected: PASS.

- [ ] **Step 6: Wire the real `Authenticator` in `cmd/store/main.go`**

First, make `bootstrap` return the auth handles it currently keeps local. Change its signature and the final return:

```go
func bootstrap(ctx context.Context) (be *datasource.BrokerBackend, signedIn bool, launchTUI func() error, authMgr *auth.Manager, ls *localstore.Store, redirect string, err error) {
```

(Adjust the existing early `return nil, false, nil, fmt.Errorf(...)` statements to the new arity: `return nil, false, nil, nil, nil, "", fmt.Errorf(...)`. The final success line becomes `return be, signedIn, launchTUI, authMgr, ls, redirect, nil`. Update the `run()` call site: `be, signedIn, launchTUI, authMgr, ls, redirect, err := bootstrap(ctx)`. Confirm the concrete types of `authMgr` (`*auth.Manager` from `auth.NewManager`) and `ls` (`localstore.New()` return type) and use those exact types.)

Then add the adapter + browser opener (new file `cmd/store/mcpauth.go`):

```go
package main

import (
	"context"
	"os/exec"
	"runtime"

	"consolestore/internal/auth"
	"consolestore/internal/localstore"
)

// mcpAuth adapts the OAuth manager + loopback callback to internal/mcp.Authenticator.
type mcpAuth struct {
	ctx      context.Context
	mgr      *auth.Manager
	ls       *localstore.Store
	redirect string
	started  bool
}

func newMCPAuth(ctx context.Context, mgr *auth.Manager, ls *localstore.Store, redirect string) *mcpAuth {
	return &mcpAuth{ctx: ctx, mgr: mgr, ls: ls, redirect: redirect}
}

func (a *mcpAuth) TokenPresent(ctx context.Context) bool {
	_, _, _, ok, err := a.ls.GetTokenFull(ctx, localstore.LocalAccountID)
	return err == nil && ok
}

func (a *mcpAuth) Start(ctx context.Context) (string, string, error) {
	if !a.started {
		if ln, lerr := netListenCallback(a.redirect); lerr == nil {
			go serveCallback(a.ctx, a.mgr, ln)
			a.started = true
		}
		// If the port is busy, another consolestore holds it; the user can still
		// authorize via that instance, or close it and retry.
	}
	start, err := a.mgr.Start(localstore.LocalAccountID)
	if err != nil {
		return "", "", err
	}
	openBrowser(start.AuthorizeURL) // best-effort; ignored on headless
	return start.AuthorizeURL, start.FlowID, nil
}

func (a *mcpAuth) Authorized(flowID string) bool { return a.mgr.Authorized(flowID) }

// openBrowser best-effort launches the OS browser. Failures are ignored — the
// agent always also returns the URL for the user to open manually.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
```

Add a small helper next to the existing `callbackAddr`/`serveCallback` in main.go to bind the listener (factor the `net.Listen` line out of `launchTUI` so both paths share it):

```go
func netListenCallback(redirect string) (net.Listener, error) {
	return net.Listen("tcp", callbackAddr(redirect))
}
```

Finally, replace the Task 1 placeholder in `run()`'s `mcp` branch:

```go
	if len(args) > 0 && args[0] == "mcp" {
		authn := newMCPAuth(ctx, authMgr, ls, redirect)
		if err := consolemcp.Serve(ctx, consolemcp.NewServer(be, authn)); err != nil {
			return fmt.Errorf("mcp server: %w", err)
		}
		return nil
	}
```

- [ ] **Step 7: Build, vet, full test**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all PASS.

- [ ] **Step 8: Manual smoke (disarmed) — optional but recommended**

Run: `go run ./cmd/store mcp` then, from another shell, drive it with any MCP client (or send a `tools/list` JSON-RPC line). Expected: server starts, lists tools, `auth_status` returns `signed_in:false` on a fresh machine. Do NOT place a real order.

- [ ] **Step 9: Commit**

```bash
gofmt -w internal/mcp/ cmd/store/
git add internal/mcp cmd/store
git commit -m "feat(mcp): agent-driven sign_in + auth_status, loopback browser auth"
```

---

### Task 7: Card tools (`get_card`, `update_card`)

**Files:**
- Create: `internal/mcp/tools_card.go`
- Create: `internal/mcp/tools_card_test.go`
- Modify: `internal/mcp/server.go` (`register`)

**Interfaces:**
- Produces: `handleGetCard`, `handleUpdateCard`.
- Consumes: `localstore.LoadCard/SaveCard/ReconcileCard` (Task 2), `Backend.Addresses`.

- [ ] **Step 1: Write the failing tests** — `internal/mcp/tools_card_test.go`

```go
package mcp

import (
	"context"
	"testing"

	"consolestore/internal/broker/api"
	"consolestore/internal/localstore"
)

func TestGetCardReconcilesWarnings(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_ = localstore.SaveCard(localstore.Card{Version: 1, DefaultAddrID: "gone", AddrLabel: "Home"})
	be := &fakeBackend{addrs: []api.Address{{ID: "other", Label: "Office"}}}
	s := NewServer(be, &fakeAuth{token: true})

	_, out, err := s.handleGetCard(context.Background(), nil, GetCardIn{})
	if err != nil {
		t.Fatalf("get_card: %v", err)
	}
	if len(out.Warnings) != 1 || out.Card.DefaultAddressID != "" {
		t.Fatalf("out = %+v", out)
	}
}

func TestUpdateCardSetsPrefs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s := NewServer(&fakeBackend{}, &fakeAuth{token: true})
	_, _, err := s.handleUpdateCard(context.Background(), nil, UpdateCardIn{Prefs: []string{"vegetarian"}, DefaultAddressID: "a9"})
	if err != nil {
		t.Fatalf("update_card: %v", err)
	}
	c, _ := localstore.LoadCard()
	if len(c.Prefs) != 1 || c.Prefs[0] != "vegetarian" || c.DefaultAddrID != "a9" {
		t.Fatalf("card = %+v", c)
	}
}
```

- [ ] **Step 2: Run them — must fail**

Run: `go test ./internal/mcp/ -run 'TestGetCard|TestUpdateCard'`
Expected: FAIL (note: `TestGetCart`/`TestUpdateCart` from Task 4 also match `-run TestUpdateCart`; use the exact names `TestGetCardReconcilesWarnings|TestUpdateCardSetsPrefs`).

- [ ] **Step 3: Implement the card tools** — `internal/mcp/tools_card.go`

```go
package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"consolestore/internal/localstore"
)

type CardFavoriteDTO struct {
	RestaurantID   string `json:"restaurant_id"`
	RestaurantName string `json:"name"`
	Count          int    `json:"count"`
}
type CardDTO struct {
	DefaultAddressID string            `json:"default_address_id"`
	AddressLabel     string            `json:"address_label"`
	Favorites        []CardFavoriteDTO `json:"favorites"`
	Prefs            []string          `json:"prefs"`
}

func cardToDTO(c localstore.Card) CardDTO {
	d := CardDTO{DefaultAddressID: c.DefaultAddrID, AddressLabel: c.AddrLabel, Prefs: c.Prefs}
	for _, f := range c.Favorites {
		d.Favorites = append(d.Favorites, CardFavoriteDTO{RestaurantID: f.RestaurantID, RestaurantName: f.RestaurantName, Count: f.Count})
	}
	return d
}

type GetCardIn struct{}
type GetCardOut struct {
	Card     CardDTO  `json:"card"`
	Warnings []string `json:"warnings,omitempty"`
}

func (s *Server) handleGetCard(ctx context.Context, _ *mcp.CallToolRequest, _ GetCardIn) (*mcp.CallToolResult, GetCardOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, GetCardOut{}, err
	}
	c, err := localstore.LoadCard()
	if err != nil {
		return nil, GetCardOut{}, err
	}
	// Reconcile against live addresses; persist any healing so it sticks.
	if addrs, aerr := s.be.Addresses(); aerr == nil {
		healed, warns := localstore.ReconcileCard(c, addrs)
		if healed.DefaultAddrID != c.DefaultAddrID || healed.AddrLabel != c.AddrLabel {
			_ = localstore.SaveCard(healed)
		}
		return nil, GetCardOut{Card: cardToDTO(healed), Warnings: warns}, nil
	}
	return nil, GetCardOut{Card: cardToDTO(c)}, nil
}

type UpdateCardIn struct {
	DefaultAddressID string   `json:"default_address_id,omitempty"`
	Prefs            []string `json:"prefs,omitempty" jsonschema:"replaces the saved prefs list when provided"`
}
type UpdateCardOut struct {
	Card CardDTO `json:"card"`
}

func (s *Server) handleUpdateCard(ctx context.Context, _ *mcp.CallToolRequest, in UpdateCardIn) (*mcp.CallToolResult, UpdateCardOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, UpdateCardOut{}, err
	}
	c, err := localstore.LoadCard()
	if err != nil {
		return nil, UpdateCardOut{}, err
	}
	if in.DefaultAddressID != "" {
		c.DefaultAddrID = in.DefaultAddressID
	}
	if in.Prefs != nil {
		c.Prefs = in.Prefs
	}
	if err := localstore.SaveCard(c); err != nil {
		return nil, UpdateCardOut{}, err
	}
	return nil, UpdateCardOut{Card: cardToDTO(c)}, nil
}
```

- [ ] **Step 4: Register the card tools** — extend `register` in `server.go`:

```go
	mcp.AddTool(srv, &mcp.Tool{Name: "get_card", Description: "the user's local taste card (default address, favorites, prefs) + staleness warnings"}, s.handleGetCard)
	mcp.AddTool(srv, &mcp.Tool{Name: "update_card", Description: "record explicit prefs or a default address on the taste card"}, s.handleUpdateCard)
```

- [ ] **Step 5: Run the card tests + full suite — must pass**

Run: `go test ./internal/mcp/ -run 'TestGetCardReconcilesWarnings|TestUpdateCardSetsPrefs' && go test ./...`
Expected: PASS.

- [ ] **Step 6: Vet + commit**

```bash
gofmt -w internal/mcp/
go vet ./...
git add internal/mcp/
git commit -m "feat(mcp): taste card tools (get_card with reconcile, update_card)"
```

---

## Plan 1 self-review

**Spec coverage:**
- MCP agent surface over broker — Tasks 1,3,4,5,6,7. ✓
- Two-tool order gate + id binding + stale refusal + no-retry — Task 5 (4 tests). ✓
- Taste card storage + auto-derive on order + reconcile/warnings — Tasks 2,5 (RecordOrder on place),7. ✓
- Agent-driven `sign_in`/`auth_status`, token never exposed — Task 6. ✓
- Rate limiting persists (same `broker.Service`/`BrokerBackend`) — `console mcp` uses the `be` from `bootstrap()`; no new limiter. ✓
- Auto-update on the mcp path — Task 1 routes through `run()` (after `updater.RunDefault`). ✓
- Arming + never-place-in-tests — fakes only; `place_order` calls `PlaceOrder` once (Task 5 `TestPlaceOrderNeverRetries`). ✓
- `console agents`/provisioning + SKILL.md bundles — **Plan 2** (out of scope here, by design). ✓

**Placeholder scan:** No TBD/TODO; every code step has full code. Two explicit "confirm the exact field names" notes (api.Tracking fields; localstore.ActiveOrder/ParseETAMinutes/SaveActiveOrder) point at the verbatim existing call site `internal/cli/order.go:134-138` and `tracking.go` — these are verification steps, not placeholders.

**Type consistency:** `Backend` matches `*datasource.BrokerBackend` method set (verified). `Authenticator` is consumed by Task 6's `mcpAuth`. `cartToDTO`/`CartDTO` defined in Task 4, reused in Task 5. `toOrderDTO`/`OrderDTO` defined in Task 3, reused in Task 5. `confirmStore.put` takes `(addressID, cart, nowUnix)` consistently between confirm.go and tools_order.go. Note: `CardDTO` (Task 7, card) and `CartDTO` (Task 4, cart) are distinct names — no collision.

**Known follow-ups for the implementer (not blockers):**
- Verify the official SDK's exact `mcp.Implementation`/`mcp.Tool`/`mcp.StdioTransport`/`mcp.CallToolRequest`/`mcp.CallToolResult` identifiers against the resolved version after `go get`; the quickstart in Global Constraints is from v1.x. If `AddTool`'s handler arity differs in the pinned version, adapt all handlers uniformly.
- `internal/version` must expose `Version` (used already by the CLI `runVersion`); confirm the symbol path.

## Next

Plan 2 (provisioning + skills) is a separate document: `internal/agents` config writers (Claude Desktop/Code/Cursor JSON, Codex TOML), the `console agents install|list|remove` subcommand, the installer hook, and the two embedded `SKILL.md` bundles (`console-order`, `console-card`).
