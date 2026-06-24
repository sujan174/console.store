# `internal/broker` + `cmd/broker` — Composition Root & Internal RPC · Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The broker process: it owns the `store` (Postgres/KMS), the `auth` Manager, and per-account `swiggy.Client`s, and exposes an account-scoped API to the TUI over a **Unix-domain-socket RPC**. The TUI never holds a Swiggy token and never links `swiggy.Client` — it speaks only to the broker via a thin client + plain DTOs.

**Architecture:** A shared `internal/broker/api` package defines gob-encodable DTOs, the RPC arg/reply types, and a typed `Client` (used by the TUI). `internal/broker` holds the `Service` (server core): it resolves an account, gets-or-builds that account's `swiggy.Client` (token pulled from the store via a `TokenSource`), and maps `swiggy` results into `api` DTOs. `cmd/broker` wires real `store`/`auth`/`swiggy` and serves the socket. Store access goes through narrow interfaces so the server core is unit-testable without a database.

**Tech Stack:** Go 1.26 stdlib `net/rpc` (gob) over `net.Listen("unix", …)`; `net/http/httptest` + fakes for tests. Depends on `internal/{store,swiggy,auth}`. No new external deps.

## Global Constraints

- Module `console.store`; Go floor `go 1.26.4`. `gofmt` clean, `go vet ./...` clean, tests pass.
- **No live Swiggy calls in any automated test.** The server core is tested against an httptest fake MCP server (reuse the `swiggy` package's real `Client` pointed at the fake) and fake store interfaces.
- `internal/broker/api` must import only stdlib (it is shared with the TUI; it must NOT pull in `swiggy`, `store`, or `auth`). `internal/broker` (server) may import `store`, `swiggy`, `auth`. `cmd/broker` wires them.
- The TUI (later slice) imports only `internal/broker/api` — never `swiggy`/`store`/`auth`. This keeps tokens + the Swiggy capability out of the SSH-facing binary.
- Account binding is idempotent/self-healing: `FindOrCreateAccount` is stable per phone, `LinkPubkey` is `ON CONFLICT DO NOTHING`, `PutToken` upserts — a partially-failed bind is repaired on the next authorize. (Resolves the auth slice's carry-note pragmatically; no cross-statement transaction required.)
- Order methods inherit `swiggy`'s `CONSOLE_LIVE_ORDERS` gate + verify-before-retry — the broker adds no second gate, just passes through.
- Socket path from env `CONSOLE_BROKER_SOCKET` (default `/tmp/console-broker.sock`); socket file mode `0600` (owner-only).

---

### Task 1: `api` package — DTOs, RPC types, Client skeleton

**Files:**
- Create: `internal/broker/api/dto.go`
- Create: `internal/broker/api/rpc.go`
- Create: `internal/broker/api/client.go`
- Test: `internal/broker/api/client_test.go`

**Interfaces:**
- Produces (stdlib-only DTOs + arg/reply + a Client over `*rpc.Client`):
  ```go
  // package api
  type Address struct { ID, Label, City, Line, Full string; Lat, Lng float64 }
  type Restaurant struct { ID, Name, City, ETA, Description string; Rating float64 }
  type MenuItem struct { ID, Name string; Price int; Veg bool; Description string; Rating float64 }
  type Menu struct { RestaurantID string; Items []MenuItem }
  type CartItem struct { ItemID string; Quantity int }
  type CartLine struct { ItemID, Name string; Quantity, Price int }
  type Cart struct { CartID string; Total int; Lines []CartLine }
  type Order struct { ID, Status, Restaurant string; Total int; PlacedAt string }
  type AuthStart struct { FlowID, AuthorizeURL string }

  // RPC arg/reply types (one pair per method) live in rpc.go.
  type Client struct{ rc *rpc.Client }
  func Dial(socketPath string) (*Client, error)
  func (c *Client) Close() error
  ```

- [ ] **Step 1: Write `dto.go`**

```go
// Package api defines the broker's wire types and a typed RPC client. It is
// shared by the broker (server) and the TUI (client) and imports only stdlib —
// it must never pull in swiggy/store/auth, so the Swiggy capability and tokens
// stay out of the SSH-facing TUI binary.
package api

type Address struct {
	ID    string
	Label string
	City  string
	Line  string
	Full  string
	Lat   float64
	Lng   float64
}

type Restaurant struct {
	ID          string
	Name        string
	City        string
	ETA         string
	Description string
	Rating      float64
}

type MenuItem struct {
	ID          string
	Name        string
	Price       int
	Veg         bool
	Description string
	Rating      float64
}

type Menu struct {
	RestaurantID string
	Items        []MenuItem
}

type CartItem struct {
	ItemID   string
	Quantity int
}

type CartLine struct {
	ItemID   string
	Name     string
	Quantity int
	Price    int
}

type Cart struct {
	CartID string
	Total  int
	Lines  []CartLine
}

type Order struct {
	ID         string
	Status     string
	Restaurant string
	Total      int
	PlacedAt   string
}

type AuthStart struct {
	FlowID       string
	AuthorizeURL string
}
```

- [ ] **Step 2: Write `rpc.go`**

```go
package api

// RPC method names (used by both server registration and the client).
const ServiceName = "Broker"

// Args/Reply pairs. AccountID scopes every data call; the TUI obtains it from
// AccountForPubkey after the SSH handshake.

type StartAuthArgs struct{ Pubkey string }
type StartAuthReply struct{ Start AuthStart }

type AuthStatusArgs struct{ FlowID string }
type AuthStatusReply struct{ Authorized bool }

type AccountForPubkeyArgs struct{ Pubkey string }
type AccountForPubkeyReply struct {
	AccountID string
	OK        bool
}

type AddressesArgs struct{ AccountID string }
type AddressesReply struct{ Addresses []Address }

type RestaurantsArgs struct {
	AccountID string
	AddressID string
	Query     string
}
type RestaurantsReply struct{ Restaurants []Restaurant }

type MenuArgs struct {
	AccountID    string
	AddressID    string
	RestaurantID string
}
type MenuReply struct{ Menu Menu }

type UpdateCartArgs struct {
	AccountID      string
	AddressID      string
	RestaurantID   string
	RestaurantName string
	Items          []CartItem
}
type UpdateCartReply struct{ Cart Cart }

type PlaceOrderArgs struct {
	AccountID string
	AddressID string
}
type PlaceOrderReply struct{ Order Order }

type LogoutArgs struct{ AccountID string }
type LogoutReply struct{}
```

- [ ] **Step 3: Write `client.go`**

```go
package api

import (
	"fmt"
	"net/rpc"
)

// Client is the TUI-side handle to the broker over a Unix socket.
type Client struct{ rc *rpc.Client }

func Dial(socketPath string) (*Client, error) {
	rc, err := rpc.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("broker dial %s: %w", socketPath, err)
	}
	return &Client{rc: rc}, nil
}

func (c *Client) Close() error { return c.rc.Close() }

func (c *Client) StartAuth(pubkey string) (AuthStart, error) {
	var rep StartAuthReply
	err := c.rc.Call(ServiceName+".StartAuth", StartAuthArgs{Pubkey: pubkey}, &rep)
	return rep.Start, err
}

func (c *Client) AuthStatus(flowID string) (bool, error) {
	var rep AuthStatusReply
	err := c.rc.Call(ServiceName+".AuthStatus", AuthStatusArgs{FlowID: flowID}, &rep)
	return rep.Authorized, err
}

func (c *Client) AccountForPubkey(pubkey string) (string, bool, error) {
	var rep AccountForPubkeyReply
	err := c.rc.Call(ServiceName+".AccountForPubkey", AccountForPubkeyArgs{Pubkey: pubkey}, &rep)
	return rep.AccountID, rep.OK, err
}

func (c *Client) Addresses(accountID string) ([]Address, error) {
	var rep AddressesReply
	err := c.rc.Call(ServiceName+".Addresses", AddressesArgs{AccountID: accountID}, &rep)
	return rep.Addresses, err
}

func (c *Client) Restaurants(accountID, addressID, query string) ([]Restaurant, error) {
	var rep RestaurantsReply
	err := c.rc.Call(ServiceName+".Restaurants", RestaurantsArgs{AccountID: accountID, AddressID: addressID, Query: query}, &rep)
	return rep.Restaurants, err
}

func (c *Client) Menu(accountID, addressID, restaurantID string) (Menu, error) {
	var rep MenuReply
	err := c.rc.Call(ServiceName+".Menu", MenuArgs{AccountID: accountID, AddressID: addressID, RestaurantID: restaurantID}, &rep)
	return rep.Menu, err
}

func (c *Client) UpdateCart(a UpdateCartArgs) (Cart, error) {
	var rep UpdateCartReply
	err := c.rc.Call(ServiceName+".UpdateCart", a, &rep)
	return rep.Cart, err
}

func (c *Client) PlaceOrder(accountID, addressID string) (Order, error) {
	var rep PlaceOrderReply
	err := c.rc.Call(ServiceName+".PlaceOrder", PlaceOrderArgs{AccountID: accountID, AddressID: addressID}, &rep)
	return rep.Order, err
}

func (c *Client) Logout(accountID string) error {
	var rep LogoutReply
	return c.rc.Call(ServiceName+".Logout", LogoutArgs{AccountID: accountID}, &rep)
}
```

- [ ] **Step 4: Write a compile/encode test** (`internal/broker/api/client_test.go`)

```go
package api

import (
	"bytes"
	"encoding/gob"
	"testing"
)

// DTOs must round-trip through gob (the RPC codec).
func TestDTOsGobRoundTrip(t *testing.T) {
	in := AddressesReply{Addresses: []Address{{ID: "a1", Label: "home", Lat: 12.9}}}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(in); err != nil {
		t.Fatal(err)
	}
	var out AddressesReply
	if err := gob.NewDecoder(&buf).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Addresses) != 1 || out.Addresses[0].ID != "a1" || out.Addresses[0].Lat != 12.9 {
		t.Fatalf("round-trip = %+v", out)
	}
}
```

- [ ] **Step 5: Run + verify `api` imports only stdlib**

Run:
```bash
go test ./internal/broker/api/ -v
go list -deps ./internal/broker/api/ | grep -E 'console.store/internal/(swiggy|store|auth)' && echo "LEAK: api imports a server pkg" || echo "ok: api is stdlib-only re internal"
```
Expected: test PASS; the grep prints nothing → "ok: api is stdlib-only re internal".

- [ ] **Step 6: Commit**

```bash
git add internal/broker/api/
git commit -m "feat(broker/api): wire DTOs + RPC types + typed Client (stdlib-only)"
```

---

### Task 2: Server core — account-scoped Swiggy access + DTO mapping

**Files:**
- Create: `internal/broker/service.go`
- Create: `internal/broker/tokensource.go`
- Create: `internal/broker/mapping.go`
- Test: `internal/broker/service_test.go`

**Interfaces:**
- Consumes: `internal/broker/api`, `internal/swiggy`, `internal/auth`.
- Produces:
  ```go
  // package broker
  // TokenStore is the narrow store surface the broker needs (real store.Store
  // satisfies it via an adapter in cmd/broker; tests use a fake).
  type TokenStore interface {
      AccountForPubkey(ctx context.Context, pubkey string) (accountID string, ok bool, err error)
      GetToken(ctx context.Context, accountID string) (token string, ok bool, err error)
      PurgeToken(ctx context.Context, accountID string) error
  }
  // Authorizer is the auth surface (auth.Manager satisfies it).
  type Authorizer interface {
      Start(pubkey string) (auth.Pending, error)
      Authorized(flowID string) bool
  }
  type Config struct {
      Store        TokenStore
      Auth         Authorizer
      FoodBaseURL  string
      ImBaseURL    string
      HTTPClient   *http.Client
  }
  type Service struct{ /* cfg + per-account client cache + mu */ }
  func NewService(cfg Config) *Service
  // foodClient returns (building+caching) the account's Food swiggy.Client,
  // whose TokenSource reads that account's token from the store.
  func (s *Service) foodClient(accountID string) *swiggy.Client
  // The RPC methods (Task 3) call exported helpers here:
  func (s *Service) Addresses(ctx context.Context, accountID string) ([]api.Address, error)
  func (s *Service) Restaurants(ctx context.Context, accountID, addressID, query string) ([]api.Restaurant, error)
  func (s *Service) Menu(ctx context.Context, accountID, addressID, restaurantID string) (api.Menu, error)
  func (s *Service) UpdateCart(ctx context.Context, a api.UpdateCartArgs) (api.Cart, error)
  func (s *Service) PlaceOrder(ctx context.Context, accountID, addressID string) (api.Order, error)
  func (s *Service) Logout(ctx context.Context, accountID string) error
  ```

- [ ] **Step 1: Write `tokensource.go`**

```go
package broker

import (
	"context"
	"fmt"

	"console.store/internal/swiggy"
)

// storeTokenSource adapts the broker's TokenStore + an account id into a
// swiggy.TokenSource. It pulls the account's access token at call time; a
// missing token surfaces as an error so callers can drive re-auth.
type storeTokenSource struct {
	store     TokenStore
	accountID string
}

func (s storeTokenSource) Token(ctx context.Context) (string, error) {
	tok, ok, err := s.store.GetToken(ctx, s.accountID)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("%w (account not authorized)", swiggy.ErrTokenExpired)
	}
	return tok, nil
}
```

- [ ] **Step 2: Write `mapping.go`** (swiggy types → api DTOs)

```go
package broker

import (
	"console.store/internal/broker/api"
	"console.store/internal/swiggy"
)

func mapAddresses(in []swiggy.Address) []api.Address {
	out := make([]api.Address, len(in))
	for i, a := range in {
		out[i] = api.Address{ID: a.ID, Label: a.Label, City: a.City, Line: a.Line, Full: a.Full, Lat: a.Lat, Lng: a.Lng}
	}
	return out
}

func mapRestaurants(in []swiggy.Restaurant) []api.Restaurant {
	out := make([]api.Restaurant, len(in))
	for i, r := range in {
		out[i] = api.Restaurant{ID: r.ID, Name: r.Name, City: r.City, ETA: r.ETA, Description: r.Desc, Rating: r.Rating}
	}
	return out
}

func mapMenu(in swiggy.Menu) api.Menu {
	items := make([]api.MenuItem, len(in.Items))
	for i, m := range in.Items {
		items[i] = api.MenuItem{ID: m.ID, Name: m.Name, Price: m.Price, Veg: m.Veg, Description: m.Desc, Rating: m.Rating}
	}
	return api.Menu{RestaurantID: in.RestaurantID, Items: items}
}

func mapCart(in swiggy.Cart) api.Cart {
	lines := make([]api.CartLine, len(in.Items))
	for i, l := range in.Items {
		lines[i] = api.CartLine{ItemID: l.ItemID, Name: l.Name, Quantity: l.Quantity, Price: l.Price}
	}
	return api.Cart{CartID: in.CartID, Total: in.Total, Lines: lines}
}

func mapOrder(in swiggy.Order) api.Order {
	return api.Order{ID: in.ID, Status: in.Status, Restaurant: in.Restaurant, Total: in.Total, PlacedAt: in.PlacedAt}
}

func mapCartItems(in []api.CartItem) []swiggy.CartItem {
	out := make([]swiggy.CartItem, len(in))
	for i, c := range in {
		out[i] = swiggy.CartItem{ItemID: c.ItemID, Quantity: c.Quantity}
	}
	return out
}
```

- [ ] **Step 3: Write `service.go`**

```go
package broker

import (
	"context"
	"net/http"
	"sync"

	"console.store/internal/auth"
	"console.store/internal/broker/api"
	"console.store/internal/swiggy"
)

type TokenStore interface {
	AccountForPubkey(ctx context.Context, pubkey string) (string, bool, error)
	GetToken(ctx context.Context, accountID string) (string, bool, error)
	PurgeToken(ctx context.Context, accountID string) error
}

type Authorizer interface {
	Start(pubkey string) (auth.Pending, error)
	Authorized(flowID string) bool
}

type Config struct {
	Store       TokenStore
	Auth        Authorizer
	FoodBaseURL string
	ImBaseURL   string
	HTTPClient  *http.Client
}

type Service struct {
	cfg Config
	mu  sync.Mutex
	// per-account Food client cache (each carries that account's TokenSource).
	food map[string]*swiggy.Client
}

func NewService(cfg Config) *Service {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	return &Service{cfg: cfg, food: map[string]*swiggy.Client{}}
}

func (s *Service) foodClient(accountID string) *swiggy.Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.food[accountID]; ok {
		return c
	}
	c := swiggy.NewClient(s.cfg.FoodBaseURL,
		storeTokenSource{store: s.cfg.Store, accountID: accountID},
		swiggy.WithHTTPClient(s.cfg.HTTPClient))
	s.food[accountID] = c
	return c
}

func (s *Service) Addresses(ctx context.Context, accountID string) ([]api.Address, error) {
	a, err := s.foodClient(accountID).GetAddresses(ctx)
	if err != nil {
		return nil, err
	}
	return mapAddresses(a), nil
}

func (s *Service) Restaurants(ctx context.Context, accountID, addressID, query string) ([]api.Restaurant, error) {
	r, err := s.foodClient(accountID).SearchRestaurants(ctx, addressID, query, 0)
	if err != nil {
		return nil, err
	}
	return mapRestaurants(r), nil
}

func (s *Service) Menu(ctx context.Context, accountID, addressID, restaurantID string) (api.Menu, error) {
	m, err := s.foodClient(accountID).GetRestaurantMenu(ctx, addressID, restaurantID, 0, 50)
	if err != nil {
		return api.Menu{}, err
	}
	return mapMenu(m), nil
}

func (s *Service) UpdateCart(ctx context.Context, a api.UpdateCartArgs) (api.Cart, error) {
	c, err := s.foodClient(a.AccountID).UpdateFoodCart(ctx, a.AddressID, a.RestaurantID, a.RestaurantName, mapCartItems(a.Items))
	if err != nil {
		return api.Cart{}, err
	}
	return mapCart(c), nil
}

func (s *Service) PlaceOrder(ctx context.Context, accountID, addressID string) (api.Order, error) {
	o, err := s.foodClient(accountID).PlaceFoodOrder(ctx, swiggy.PlaceFoodOrderRequest{AddressID: addressID})
	if err != nil {
		return api.Order{}, err
	}
	return mapOrder(o), nil
}

func (s *Service) Logout(ctx context.Context, accountID string) error {
	// drop the cached client (and its token source) then purge the token.
	s.mu.Lock()
	delete(s.food, accountID)
	s.mu.Unlock()
	return s.cfg.Store.PurgeToken(ctx, accountID)
}
```

- [ ] **Step 4: Write `service_test.go`** (real swiggy.Client → httptest fake MCP; fake TokenStore)

```go
package broker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"console.store/internal/auth"
	"console.store/internal/broker/api"
)

type fakeStore struct {
	tokens map[string]string
	purged []string
}

func (f *fakeStore) AccountForPubkey(_ context.Context, pubkey string) (string, bool, error) {
	return "acct-" + pubkey, true, nil
}
func (f *fakeStore) GetToken(_ context.Context, accountID string) (string, bool, error) {
	tok, ok := f.tokens[accountID]
	return tok, ok, nil
}
func (f *fakeStore) PurgeToken(_ context.Context, accountID string) error {
	f.purged = append(f.purged, accountID)
	delete(f.tokens, accountID)
	return nil
}

type fakeAuthz struct{ started string }

func (f *fakeAuthz) Start(pubkey string) (auth.Pending, error) {
	f.started = pubkey
	return auth.Pending{FlowID: "flow-1", AuthorizeURL: "https://authz/x"}, nil
}
func (f *fakeAuthz) Authorized(string) bool { return true }

// fakeMCP answers tools/call for the named handlers (mirrors swiggy's fake).
func fakeMCP(t *testing.T, handlers map[string]func(map[string]any) any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&msg)
		w.Header().Set("Content-Type", "application/json")
		switch msg.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "s")
			json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": msg.ID, "result": map[string]any{}})
		case "notifications/initialized":
			w.WriteHeader(202)
		case "tools/call":
			h := handlers[msg.Params.Name]
			json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": msg.ID,
				"result": map[string]any{"structuredContent": h(msg.Params.Arguments)}})
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestServiceAddressesMapsDTO(t *testing.T) {
	mcp := fakeMCP(t, map[string]func(map[string]any) any{
		"get_addresses": func(map[string]any) any {
			return []map[string]any{{"id": "a1", "annotation": "home", "lat": 12.9}}
		},
	})
	store := &fakeStore{tokens: map[string]string{"acct-X": "tok"}}
	svc := NewService(Config{Store: store, Auth: &fakeAuthz{}, FoodBaseURL: mcp.URL, HTTPClient: mcp.Client()})
	got, err := svc.Addresses(context.Background(), "acct-X")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "a1" || got[0].Label != "home" {
		t.Fatalf("addresses = %+v", got)
	}
}

func TestServiceLogoutPurgesAndDropsClient(t *testing.T) {
	store := &fakeStore{tokens: map[string]string{"acct-X": "tok"}}
	svc := NewService(Config{Store: store, Auth: &fakeAuthz{}, FoodBaseURL: "http://unused"})
	svc.foodClient("acct-X") // populate cache
	if err := svc.Logout(context.Background(), "acct-X"); err != nil {
		t.Fatal(err)
	}
	if len(store.purged) != 1 || store.purged[0] != "acct-X" {
		t.Fatalf("purged = %+v", store.purged)
	}
	if _, ok := svc.food["acct-X"]; ok {
		t.Fatal("client cache not dropped on logout")
	}
}

func TestServiceMissingTokenSurfacesError(t *testing.T) {
	mcp := fakeMCP(t, map[string]func(map[string]any) any{})
	store := &fakeStore{tokens: map[string]string{}} // no token for acct-X
	svc := NewService(Config{Store: store, Auth: &fakeAuthz{}, FoodBaseURL: mcp.URL, HTTPClient: mcp.Client()})
	if _, err := svc.Addresses(context.Background(), "acct-X"); err == nil {
		t.Fatal("expected error when account has no token")
	}
	_ = api.Address{}
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/broker/ -v 2>&1 | tail -20`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/broker/service.go internal/broker/tokensource.go internal/broker/mapping.go internal/broker/service_test.go
git commit -m "feat(broker): account-scoped Service (per-account client cache + DTO mapping)"
```

---

### Task 3: RPC server (Unix socket) + round-trip integration test

**Files:**
- Create: `internal/broker/rpcserver.go`
- Test: `internal/broker/rpcserver_test.go`

**Interfaces:**
- Consumes: `Service` (Task 2), `api` (Task 1), stdlib `net/rpc`.
- Produces:
  ```go
  // rpcAdapter exposes Service methods in net/rpc shape (Args,*Reply)error.
  type rpcAdapter struct{ svc *Service }
  // Serve registers the adapter and serves the Unix socket until ctx is done.
  func Serve(ctx context.Context, svc *Service, socketPath string) error
  ```

- [ ] **Step 1: Write `rpcserver.go`**

```go
package broker

import (
	"context"
	"net"
	"net/rpc"
	"os"

	"console.store/internal/broker/api"
)

// rpcAdapter wraps Service in the net/rpc method shape. The ServiceName
// registered is api.ServiceName ("Broker").
type rpcAdapter struct{ svc *Service }

func (a *rpcAdapter) StartAuth(args api.StartAuthArgs, rep *api.StartAuthReply) error {
	p, err := a.svc.cfg.Auth.Start(args.Pubkey)
	if err != nil {
		return err
	}
	rep.Start = api.AuthStart{FlowID: p.FlowID, AuthorizeURL: p.AuthorizeURL}
	return nil
}

func (a *rpcAdapter) AuthStatus(args api.AuthStatusArgs, rep *api.AuthStatusReply) error {
	rep.Authorized = a.svc.cfg.Auth.Authorized(args.FlowID)
	return nil
}

func (a *rpcAdapter) AccountForPubkey(args api.AccountForPubkeyArgs, rep *api.AccountForPubkeyReply) error {
	id, ok, err := a.svc.cfg.Store.AccountForPubkey(context.Background(), args.Pubkey)
	rep.AccountID, rep.OK = id, ok
	return err
}

func (a *rpcAdapter) Addresses(args api.AddressesArgs, rep *api.AddressesReply) error {
	out, err := a.svc.Addresses(context.Background(), args.AccountID)
	rep.Addresses = out
	return err
}

func (a *rpcAdapter) Restaurants(args api.RestaurantsArgs, rep *api.RestaurantsReply) error {
	out, err := a.svc.Restaurants(context.Background(), args.AccountID, args.AddressID, args.Query)
	rep.Restaurants = out
	return err
}

func (a *rpcAdapter) Menu(args api.MenuArgs, rep *api.MenuReply) error {
	out, err := a.svc.Menu(context.Background(), args.AccountID, args.AddressID, args.RestaurantID)
	rep.Menu = out
	return err
}

func (a *rpcAdapter) UpdateCart(args api.UpdateCartArgs, rep *api.UpdateCartReply) error {
	out, err := a.svc.UpdateCart(context.Background(), args)
	rep.Cart = out
	return err
}

func (a *rpcAdapter) PlaceOrder(args api.PlaceOrderArgs, rep *api.PlaceOrderReply) error {
	out, err := a.svc.PlaceOrder(context.Background(), args.AccountID, args.AddressID)
	rep.Order = out
	return err
}

func (a *rpcAdapter) Logout(args api.LogoutArgs, rep *api.LogoutReply) error {
	return a.svc.Logout(context.Background(), args.AccountID)
}

// Serve registers the adapter under api.ServiceName and serves the Unix socket
// (mode 0600) until ctx is cancelled.
func Serve(ctx context.Context, svc *Service, socketPath string) error {
	srv := rpc.NewServer()
	if err := srv.RegisterName(api.ServiceName, &rpcAdapter{svc: svc}); err != nil {
		return err
	}
	_ = os.Remove(socketPath) // clear a stale socket
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	defer ln.Close()
	if err := os.Chmod(socketPath, 0o600); err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return err
			}
		}
		go srv.ServeConn(conn)
	}
}
```

- [ ] **Step 2: Write `rpcserver_test.go`** (serve on a temp socket; dial with api.Client)

```go
package broker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"console.store/internal/broker/api"
)

func TestRPCRoundTripAddresses(t *testing.T) {
	mcp := fakeMCP(t, map[string]func(map[string]any) any{
		"get_addresses": func(map[string]any) any {
			return []map[string]any{{"id": "a1", "annotation": "home"}}
		},
	})
	store := &fakeStore{tokens: map[string]string{"acct-X": "tok"}}
	svc := NewService(Config{Store: store, Auth: &fakeAuthz{}, FoodBaseURL: mcp.URL, HTTPClient: mcp.Client()})

	sock := filepath.Join(t.TempDir(), "b.sock")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go Serve(ctx, svc, sock)
	waitForSocket(t, sock)

	cli, err := api.Dial(sock)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	got, err := cli.Addresses("acct-X")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "a1" {
		t.Fatalf("round-trip addresses = %+v", got)
	}

	// auth passthrough
	st, err := cli.StartAuth("ssh-key")
	if err != nil || st.FlowID != "flow-1" {
		t.Fatalf("startauth = %+v err=%v", st, err)
	}
}

func waitForSocket(t *testing.T, path string) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if c, err := api.Dial(path); err == nil {
			c.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("broker socket never came up")
}

var _ = json.Marshal // keep encoding/json import if unused after edits
var _ = http.DefaultClient
var _ = httptest.NewServer
```

> Executor note: remove the three `var _ =` keep-alive lines if those imports are actually used; they exist only to avoid an unused-import error if you trim the test. Ensure the final file has no unused imports (`go vet`).

- [ ] **Step 3: Run tests**

Run: `go test ./internal/broker/ -run TestRPC -v`
Expected: PASS.

- [ ] **Step 4: Race + vet + fmt**

Run:
```bash
go test -race ./internal/broker/... 2>&1 | tail -10
go vet ./internal/broker/...
gofmt -l internal/broker
```
Expected: PASS; `gofmt -l` prints nothing.

- [ ] **Step 5: Commit**

```bash
git add internal/broker/rpcserver.go internal/broker/rpcserver_test.go
git commit -m "feat(broker): net/rpc Unix-socket server + round-trip integration test"
```

---

### Task 4: `cmd/broker` — wiring + store adapters

**Files:**
- Create: `cmd/broker/main.go`
- Create: `cmd/broker/adapters.go`
- Test: `cmd/broker/adapters_test.go`

**Interfaces:**
- Consumes: `store`, `auth`, `broker`, `swiggy`.
- Produces: `main()` that reads config from env, migrates, builds the auth Manager + Service, and serves. Two adapters bridging `*store.Store` to the broker/auth interfaces:
  ```go
  // brokerStore adapts *store.Store to broker.TokenStore.
  type brokerStore struct{ s *store.Store }
  func (b brokerStore) AccountForPubkey(ctx, pubkey) (string, bool, error)
  func (b brokerStore) GetToken(ctx, accountID) (string, bool, error)  // unwraps store.Token.AccessToken
  func (b brokerStore) PurgeToken(ctx, accountID) error
  // authStore adapts *store.Store to auth.AccountStore.
  type authStore struct{ s *store.Store }
  func (a authStore) FindOrCreateAccount(ctx, phone) (string, error)
  func (a authStore) LinkPubkey(ctx, accountID, pubkey) error
  func (a authStore) PutToken(ctx, accountID, accessToken string, expiresAt time.Time) error // wraps store.Token
  ```

- [ ] **Step 1: Write `adapters.go`**

```go
package main

import (
	"context"
	"time"

	"console.store/internal/store"
)

type brokerStore struct{ s *store.Store }

func (b brokerStore) AccountForPubkey(ctx context.Context, pubkey string) (string, bool, error) {
	return b.s.AccountForPubkey(ctx, pubkey)
}
func (b brokerStore) GetToken(ctx context.Context, accountID string) (string, bool, error) {
	tok, ok, err := b.s.GetToken(ctx, accountID)
	if err != nil || !ok {
		return "", ok, err
	}
	return tok.AccessToken, true, nil
}
func (b brokerStore) PurgeToken(ctx context.Context, accountID string) error {
	return b.s.PurgeToken(ctx, accountID)
}

type authStore struct{ s *store.Store }

func (a authStore) FindOrCreateAccount(ctx context.Context, phone string) (string, error) {
	return a.s.FindOrCreateAccount(ctx, phone)
}
func (a authStore) LinkPubkey(ctx context.Context, accountID, pubkey string) error {
	return a.s.LinkPubkey(ctx, accountID, pubkey)
}
func (a authStore) PutToken(ctx context.Context, accountID, accessToken string, expiresAt time.Time) error {
	return a.s.PutToken(ctx, accountID, store.Token{AccessToken: accessToken, ExpiresAt: expiresAt})
}
```

- [ ] **Step 2: Write `adapters_test.go`** (compile-time interface satisfaction)

```go
package main

import (
	"console.store/internal/auth"
	"console.store/internal/broker"
)

// Compile-time assertions that the adapters satisfy the broker/auth seams.
var _ broker.TokenStore = brokerStore{}
var _ auth.AccountStore = authStore{}
```

- [ ] **Step 3: Write `main.go`**

```go
// Command broker is console.store's privileged backend: it holds Swiggy tokens
// (Postgres + KMS), runs the OAuth flow, and serves an account-scoped RPC to the
// SSH-facing TUI over a Unix socket. It is the only component that calls Swiggy.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"console.store/internal/auth"
	"console.store/internal/broker"
	"console.store/internal/store"
	"console.store/internal/store/kms"
	"console.store/internal/swiggy"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("broker: %v", err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dsn := envOr("CONSOLE_DB_DSN", "postgres://console_broker:console_broker_dev@localhost:5432/console")
	sock := envOr("CONSOLE_BROKER_SOCKET", "/tmp/console-broker.sock")
	metaURL := envOr("CONSOLE_SWIGGY_METADATA", "https://mcp.swiggy.com/.well-known/oauth-authorization-server")
	redirect := envOr("CONSOLE_REDIRECT_URI", "http://localhost:8765/cb")

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	k, err := kms.FromEnv(ctx)
	if err != nil {
		return err
	}
	st := store.New(pool, k)

	httpc := &http.Client{Timeout: 30 * time.Second}
	meta, err := auth.Discover(ctx, httpc, metaURL)
	if err != nil {
		return err
	}
	clientID, err := auth.Register(ctx, httpc, meta.RegistrationEndpoint, redirect, "mcp:tools")
	if err != nil {
		return err
	}
	authMgr := auth.NewManager(auth.Config{
		HTTPClient: httpc, Metadata: meta, ClientID: clientID,
		RedirectURI: redirect, Scope: "mcp:tools", Store: authStore{s: st},
	})

	// Local OAuth callback listener (completes cross-device authorize).
	go serveCallback(ctx, authMgr, redirect)

	svc := broker.NewService(broker.Config{
		Store:       brokerStore{s: st},
		Auth:        authMgr,
		FoodBaseURL: swiggy.FoodBaseURL,
		ImBaseURL:   swiggy.InstamartBaseURL,
		HTTPClient:  httpc,
	})

	log.Printf("broker serving on %s", sock)
	return broker.Serve(ctx, svc, sock)
}

func serveCallback(ctx context.Context, m *auth.Manager, redirectURI string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cb", m.CallbackHandler())
	srv := &http.Server{Addr: "127.0.0.1:8765", Handler: mux}
	go func() { <-ctx.Done(); srv.Close() }()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("callback listener: %v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
```

- [ ] **Step 4: Build + adapters test + vet/fmt**

Run:
```bash
go build ./cmd/broker/
go test ./cmd/broker/ -v
go vet ./cmd/broker/...
gofmt -l cmd/broker
go build ./...
```
Expected: builds; adapters test PASS (compile-time assertions); `gofmt -l` empty.

- [ ] **Step 5: Commit**

```bash
git add cmd/broker/
git commit -m "feat(cmd/broker): wire store+kms+auth+swiggy and serve the RPC socket"
```

---

## Self-Review

**Spec coverage (spec §2, §3.4):** separate `cmd/broker` process, sole token holder ✓ (Task 4); Unix-socket internal RPC ✓ (Task 3); account-scoped reads + writes + auth + logout ✓ (Tasks 2,3); `swiggy.Client` token boundary kept out of `api` (verified by `go list -deps` grep, Task 1) ✓; per-account token pulled from store at call time ✓ (`storeTokenSource`); idempotent binding via store's stable/upsert semantics (carry-note resolved) ✓; order path inherits the live-orders gate (no second gate) ✓.

**Placeholder scan:** No TBD/TODO. The `var _ =` keep-alive lines in `rpcserver_test.go` are flagged with an explicit executor note to remove if imports are used — not a silent placeholder. ✓

**Type consistency:** `api` arg/reply names match the `Client` calls and the `rpcAdapter` method signatures (`StartAuthArgs/Reply`, `AddressesArgs/Reply`, …). `swiggy.PlaceFoodOrderRequest`, `swiggy.CartItem`, `swiggy.NewClient/WithHTTPClient`, `auth.Pending{FlowID,AuthorizeURL}`, `auth.Config`, `store.Token{AccessToken,ExpiresAt}`, `store.New`, `kms.FromEnv` all match the prior slices' real signatures. ✓

**Note for executor:** the broker unit/integration tests use fakes + an httptest fake MCP server — they need **no** database and make **no** live calls. `cmd/broker` is build-verified only (running it needs Postgres + a real authz server). The remaining swiggy tools (Instamart reads, coupons, tracking, order history) are intentionally NOT yet exposed over RPC — they are added in the TUI-wiring slice as each screen needs them (YAGNI). Do not add them here.
