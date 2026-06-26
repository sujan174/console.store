# Restaurant Discovery Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the live Restaurants browse (cramped cuisine-chip row + flat list + `/` local-filter) with a left **rail** (🔍 Search · Home · categories), a **Home** view that leads with your most-ordered restaurants ("your usuals", derived from order history) above nearby restaurants, and a real Swiggy-wide search.

**Architecture:** A new `Rail` screen-component renders the left column; the existing `Menu` screen gains a two-pane render path (rail + sectioned main: usuals / nearby / results) used only in live mode. A new data path derives "usuals" by aggregating `get_food_orders` history (defensive: tolerates the empty `{}` the test account returns, resolves missing ids by name). Categories and search reuse the existing `PlacesQuery` seam. The root (`app.go`) owns rail focus/selection and fires the matching load Cmd, replacing the chip-row nav it supersedes.

**Tech Stack:** Go 1.26, bubbletea/lipgloss, Tokyo Night theme; existing `swiggy` → `broker`/`api` → `tui/datasource` → `catalog`/`screens` seams.

## Global Constraints

- Module `console.store`; Go 1.26; no new external dependencies.
- `go test ./...` green after each task; `gofmt -l` empty on touched files; `go vet ./...` clean.
- `internal/tui/screens` must NOT import `internal/tui` (import cycle).
- **Live-only:** rail + usuals + global search render only in live mode (gated exactly like today's chip row: present only when chips/rail data are set). The **mock path renders byte-for-byte as today** (section tabs, single-pane list, `/` filter). When changing rendered copy, update matching test strings.
- Instamart vertical stays a placeholder; the cart/order path is untouched; `CONSOLE_LIVE_ORDERS` untouched.
- All data flows through the existing seam; never hardcode catalog data in screens.
- No golden files — inline substring assertions.

## Phase 0 (already done)

`get_food_orders(addressId, activeOnly)` returns **`{}`** for the test account (no retrievable history). So: the usuals builder must return an **empty slice (not an error)** on `{}`, the populated path is built **defensively** (use payload `restaurantId`/rating/ETA if present, else resolve by name via `search_restaurants`), and live-verifying the *populated* usuals path is **deferred**. The empty path and everything else are verifiable now.

---

## File Structure

- `internal/swiggy/orders.go` — **Create**. `ordersEnvelope` (defensive parse of `{orders:[...]}` / `{}`), `UsualRestaurants(ctx, addressID)`.
- `internal/swiggy/food.go` — **Modify**. Make `GetFoodOrders` use the defensive envelope (currently assumes a bare `[]Order`, which errors on `{}`).
- `internal/broker/api/rpc.go` — **Modify**. `UsualsArgs`/`UsualsReply`.
- `internal/broker/api/client.go` — **Modify**. `Client.Usuals`.
- `internal/broker/service.go` — **Modify**. `Service.Usuals`.
- `internal/broker/rpcserver.go` — **Modify**. `rpcAdapter.Usuals`.
- `internal/tui/datasource/datasource.go` — **Modify**. `Backend.Usuals`, `LoadUsuals` Cmd, `UsualsLoadedMsg`.
- `internal/tui/datasource/broker_backend.go` — **Modify**. `BrokerBackend.Usuals`.
- `internal/catalog/swiggy/snapshot.go` + `repository.go` — usuals reuse the existing `SetPlaces(addr, usualsKey, …)` / `PlacesByQuery(addr, usualsKey)` (no new snapshot field; `usualsKey` is a reserved query string).
- `internal/tui/screens/rail.go` — **Create**. The `Rail` component (entries, active, focus, width budget, render).
- `internal/tui/screens/menu.go` — **Modify**. Two-pane render path + section headers + search-input mode (live); mock path unchanged.
- `internal/tui/app.go` — **Modify**. Rail state + nav, `LoadUsuals` on browse open, `UsualsLoadedMsg`, search-submit, route rail selection → load Cmd.
- Test files alongside each.

---

## Task 1: swiggy — `UsualRestaurants` (defensive order-history aggregation)

**Files:**
- Create: `internal/swiggy/orders.go`
- Modify: `internal/swiggy/food.go` (GetFoodOrders)
- Test: `internal/swiggy/orders_test.go`

**Interfaces:**
- Consumes: `Client.CallTool`, `Client.SearchRestaurants(ctx, addressID, query, offset) ([]Restaurant, error)` (exists), `Restaurant`/`Order` types (exist).
- Produces: `func (c *Client) UsualRestaurants(ctx context.Context, addressID string) ([]Restaurant, error)` — restaurants ranked by order frequency (desc), capped at 5, empty (no error) when history is `{}`. `ordersEnvelope` with `func (e ordersEnvelope) orders() []Order`.

- [ ] **Step 1: Write the failing test**

Create `internal/swiggy/orders_test.go`:

```go
package swiggy

import (
	"encoding/json"
	"testing"
)

func TestOrdersEnvelopeEmptyObject(t *testing.T) {
	// The live test account returns {} — must parse to zero orders, not error.
	var e ordersEnvelope
	if err := json.Unmarshal([]byte(`{}`), &e); err != nil {
		t.Fatalf("{} must unmarshal cleanly: %v", err)
	}
	if got := e.orders(); len(got) != 0 {
		t.Fatalf("empty history must yield no orders, got %d", len(got))
	}
}

func TestOrdersEnvelopeWrapped(t *testing.T) {
	raw := `{"orders":[
		{"orderId":1,"restaurantName":"Blue Tokai","status":"DELIVERED"},
		{"orderId":2,"restaurantName":"Onesta","status":"DELIVERED"},
		{"orderId":3,"restaurantName":"Blue Tokai","status":"DELIVERED"}
	]}`
	var e ordersEnvelope
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := e.orders(); len(got) != 3 {
		t.Fatalf("expected 3 orders, got %d", len(got))
	}
}

func TestRankUsualsByFrequency(t *testing.T) {
	orders := []Order{
		{Restaurant: "Blue Tokai"}, {Restaurant: "Onesta"},
		{Restaurant: "Blue Tokai"}, {Restaurant: "Pizza Hut"},
		{Restaurant: "Blue Tokai"}, {Restaurant: "Onesta"},
	}
	ranked := rankUsuals(orders, 5)
	if len(ranked) != 3 {
		t.Fatalf("3 distinct restaurants, got %d", len(ranked))
	}
	if ranked[0].name != "Blue Tokai" || ranked[0].count != 3 {
		t.Fatalf("most-ordered first: got %+v", ranked[0])
	}
	if ranked[1].name != "Onesta" || ranked[1].count != 2 {
		t.Fatalf("second: got %+v", ranked[1])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/swiggy/ -run "TestOrders|TestRankUsuals" -v`
Expected: FAIL — `ordersEnvelope` / `rankUsuals` undefined.

- [ ] **Step 3: Write `orders.go`**

Create `internal/swiggy/orders.go`:

```go
package swiggy

import (
	"context"
	"sort"
)

// ordersEnvelope decodes get_food_orders. The live response wraps orders in an
// object ({"orders":[...]}) and returns a bare {} when there is no retrievable
// history — both must parse cleanly to a (possibly empty) slice.
type ordersEnvelope struct {
	Orders []Order `json:"orders"`
}

func (e ordersEnvelope) orders() []Order { return e.Orders }

// usualRank is one restaurant's order-frequency tally.
type usualRank struct {
	name  string
	count int
}

// rankUsuals counts orders per restaurant name and returns the most-ordered
// first, capped at limit. Stable for equal counts (first-seen order).
func rankUsuals(orders []Order, limit int) []usualRank {
	idx := map[string]int{}
	var ranks []usualRank
	for _, o := range orders {
		if o.Restaurant == "" {
			continue
		}
		if i, ok := idx[o.Restaurant]; ok {
			ranks[i].count++
			continue
		}
		idx[o.Restaurant] = len(ranks)
		ranks = append(ranks, usualRank{name: o.Restaurant, count: 1})
	}
	sort.SliceStable(ranks, func(i, j int) bool { return ranks[i].count > ranks[j].count })
	if limit > 0 && len(ranks) > limit {
		ranks = ranks[:limit]
	}
	return ranks
}

// UsualRestaurants derives the account's most-ordered restaurants from order
// history. Empty (NOT an error) when history is unavailable. Because the order
// payload may carry only the restaurant NAME, each usual is resolved to a full
// Restaurant via search_restaurants(name) (first match); usuals that don't
// resolve are dropped (never a dead row).
func (c *Client) UsualRestaurants(ctx context.Context, addressID string) ([]Restaurant, error) {
	env, err := decodeResult[ordersEnvelope](c.CallTool(ctx, "get_food_orders", map[string]any{
		"addressId": addressID, "activeOnly": false,
	}))
	if err != nil {
		return nil, err
	}
	ranks := rankUsuals(env.orders(), 5)
	var out []Restaurant
	for _, r := range ranks {
		matches, err := c.SearchRestaurants(ctx, addressID, r.name, 0)
		if err != nil || len(matches) == 0 {
			continue // unresolvable → drop
		}
		out = append(out, matches[0])
	}
	return out, nil
}
```

- [ ] **Step 4: Make `GetFoodOrders` defensive**

In `internal/swiggy/food.go`, change `GetFoodOrders` to parse the envelope (it currently does `decodeResult[[]Order]`, which errors on `{}`):

```go
func (c *Client) GetFoodOrders(ctx context.Context, addressID string, activeOnly bool) ([]Order, error) {
	env, err := decodeResult[ordersEnvelope](c.CallTool(ctx, "get_food_orders", map[string]any{
		"addressId": addressID, "activeOnly": activeOnly,
	}))
	return env.orders(), err
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/swiggy/ -v`
Expected: PASS (all swiggy tests, including the new three).

- [ ] **Step 6: Commit**

```bash
gofmt -w internal/swiggy/orders.go internal/swiggy/orders_test.go internal/swiggy/food.go
git add internal/swiggy/orders.go internal/swiggy/orders_test.go internal/swiggy/food.go
git commit -m "feat(swiggy): UsualRestaurants from order history (defensive, empty-safe)"
```

---

## Task 2: broker — `Usuals` RPC + api

**Files:**
- Modify: `internal/broker/api/rpc.go`, `internal/broker/api/client.go`, `internal/broker/service.go`, `internal/broker/rpcserver.go`
- Test: `internal/broker/service_test.go` (append)

**Interfaces:**
- Consumes: `swiggy.Client.UsualRestaurants` (Task 1); existing `mapRestaurants(in []swiggy.Restaurant) []api.Restaurant`; existing `Service.foodClient(accountID)`.
- Produces: `Client.Usuals(accountID, addressID) ([]api.Restaurant, error)`; `Service.Usuals(ctx, accountID, addressID) ([]api.Restaurant, error)`; `api.UsualsArgs{AccountID, AddressID}` / `api.UsualsReply{Restaurants []Restaurant}`.

- [ ] **Step 1: Write the failing test**

Append to `internal/broker/service_test.go` (matching its existing fake-client style; if the file mocks the swiggy client differently, mirror that). If `service_test.go` has no swiggy-client fake, instead add the test to `internal/broker/rpcserver_test.go` asserting the adapter wires through. Minimal adapter test:

```go
func TestUsualsAdapterWiring(t *testing.T) {
	// The rpc adapter must forward Usuals to the service and return its result.
	// Uses the same fake Service seam the other rpcserver tests use.
	// (If rpcserver_test.go already constructs an rpcAdapter around a fake/real
	// Service, follow that exact construction here.)
}
```

> **Implementer note:** open `internal/broker/service_test.go` and `internal/broker/rpcserver_test.go` first and follow whichever already-present fake/seam those tests use for `GetCart`/`UpdateCart` — replicate that pattern for `Usuals` rather than inventing a new mock. The assertion: `Usuals(accountID, addressID)` returns the mapped restaurants the underlying client produced.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/broker/ -run TestUsuals -v`
Expected: FAIL — `Usuals` undefined.

- [ ] **Step 3: Add the types + methods**

In `internal/broker/api/rpc.go` (next to `GetCartArgs`):

```go
type UsualsArgs struct {
	AccountID string
	AddressID string
}
type UsualsReply struct{ Restaurants []Restaurant }
```

In `internal/broker/api/client.go` (next to `GetCart`):

```go
func (c *Client) Usuals(accountID, addressID string) ([]Restaurant, error) {
	var rep UsualsReply
	err := c.rc.Call(ServiceName+".Usuals", UsualsArgs{AccountID: accountID, AddressID: addressID}, &rep)
	return rep.Restaurants, err
}
```

In `internal/broker/service.go` (next to `GetCart`):

```go
// Usuals returns the account's most-ordered restaurants (from order history),
// empty when there is no retrievable history.
func (s *Service) Usuals(ctx context.Context, accountID, addressID string) ([]api.Restaurant, error) {
	rs, err := s.foodClient(accountID).UsualRestaurants(ctx, addressID)
	if err != nil {
		return nil, err
	}
	return mapRestaurants(rs), nil
}
```

In `internal/broker/rpcserver.go` (next to `GetCart`):

```go
func (a *rpcAdapter) Usuals(args api.UsualsArgs, rep *api.UsualsReply) error {
	out, err := a.svc.Usuals(context.Background(), args.AccountID, args.AddressID)
	rep.Restaurants = out
	return err
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/broker/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/broker/api/rpc.go internal/broker/api/client.go internal/broker/service.go internal/broker/rpcserver.go internal/broker/service_test.go internal/broker/rpcserver_test.go
git add internal/broker/
git commit -m "feat(broker): Usuals RPC (most-ordered restaurants from history)"
```

---

## Task 3: datasource — `LoadUsuals` Cmd + snapshot cache

**Files:**
- Modify: `internal/tui/datasource/datasource.go`, `internal/tui/datasource/broker_backend.go`
- Test: `internal/tui/datasource/datasource_test.go` (append)

**Interfaces:**
- Consumes: `api.Client.Usuals` (Task 2); existing `toPlaces(in []api.Restaurant, section catalog.Section) []catalog.Place`; existing `Snapshot.SetPlaces(addrID, key string, places []catalog.Place)`; the fakes in `datasource_test.go`.
- Produces: `Backend.Usuals(addressID string) ([]api.Restaurant, error)`; `const UsualsKey = "__usuals__"`; `LoadUsuals(b Backend, snap *swiggysnap.Snapshot, addressID string) tea.Cmd` → `UsualsLoadedMsg{Err error}`; the places cached under `SetPlaces(addressID, UsualsKey, …)` for the Repository to read via `PlacesByQuery(addr, UsualsKey)`.

- [ ] **Step 1: Write the failing test**

Append to `internal/tui/datasource/datasource_test.go`:

```go
func TestLoadUsualsCachesUnderUsualsKey(t *testing.T) {
	b := &fakeBackend{restaurants: []api.Restaurant{{ID: "r1", Name: "Blue Tokai"}}}
	snap := swiggysnap.NewSnapshot()
	msg := LoadUsuals(b, snap, "a1")()
	if m, ok := msg.(UsualsLoadedMsg); !ok || m.Err != nil {
		t.Fatalf("expected clean UsualsLoadedMsg, got %#v", msg)
	}
	got := snap.PlacesByQueryForTest("a1", UsualsKey) // see note
	if len(got) != 1 || got[0].Name != "Blue Tokai" {
		t.Fatalf("usuals not cached under UsualsKey: %+v", got)
	}
}
```

> **Implementer note:** `datasource_test.go` already verifies cached places for other Load Cmds — reuse that exact read path (it may read via a `Repository` or a snapshot getter, not a `…ForTest` helper). Replace `PlacesByQueryForTest` with whatever that file already uses to read `SetPlaces` results; do NOT add a new snapshot method if one already serves this. Also add a `restaurants []api.Restaurant` field + `Usuals` method to the existing `fakeBackend` (Step 3).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/datasource/ -run TestLoadUsuals -v`
Expected: FAIL — `LoadUsuals` / `UsualsLoadedMsg` / `UsualsKey` undefined.

- [ ] **Step 3: Implement**

In `internal/tui/datasource/datasource.go`:

Add to the `Backend` interface (next to `PlacesQuery`):

```go
	Usuals(addressID string) ([]api.Restaurant, error)
```

Add the msg (next to `PlacesLoadedMsg`):

```go
	// UsualsLoadedMsg signals the account's most-ordered restaurants were
	// fetched into the snapshot (under UsualsKey). Err non-nil on failure;
	// empty history is NOT an error (the section just renders nothing).
	UsualsLoadedMsg struct{ Err error }
```

Add the key + Cmd (next to `LoadPlacesQuery`):

```go
// UsualsKey is the reserved snapshot query key under which the account's
// most-ordered restaurants are cached (so the Repository reads them via
// PlacesByQuery without a new snapshot field).
const UsualsKey = "__usuals__"

// LoadUsuals fetches the account's most-ordered restaurants and caches them
// under UsualsKey. Errors are non-fatal (the Home view drops the section).
func LoadUsuals(b Backend, snap *swiggysnap.Snapshot, addressID string) tea.Cmd {
	return func() tea.Msg {
		got, err := b.Usuals(addressID)
		if err == nil {
			snap.SetPlaces(addressID, UsualsKey, toPlaces(got, catalog.SectionFood))
		}
		return UsualsLoadedMsg{Err: err}
	}
}
```

In `internal/tui/datasource/broker_backend.go` (next to `PlacesQuery`):

```go
func (b *BrokerBackend) Usuals(addressID string) ([]api.Restaurant, error) {
	r, err := b.rpc.Usuals(b.accountID, addressID)
	return r, wrapAuthErr(err)
}
```

Also add `Usuals` to the `brokerRPC` interface in `broker_backend.go` (next to `PlacesQuery`):

```go
	Usuals(accountID, addressID string) ([]api.Restaurant, error)
```

In `datasource_test.go`, extend the existing `fakeBackend` with a `restaurants` field and:

```go
func (f *fakeBackend) Usuals(string) ([]api.Restaurant, error) { return f.restaurants, f.err }
```

(And add the same `Usuals` to any other `Backend`/`brokerRPC` fakes the package compiles — `live_test.go`'s `liveFake` and `broker_backend_test.go`'s `fakeRPC` — to keep the build green: `func (f *liveFake) Usuals(string) ([]api.Restaurant, error) { return nil, f.err }` and `func (f *fakeRPC) Usuals(accountID, addressID string) ([]api.Restaurant, error) { f.lastAccount = accountID; return nil, f.err }`.)

- [ ] **Step 4: Run tests**

Run: `go test ./internal/tui/datasource/ ./internal/tui/ -v 2>&1 | tail -20`
Expected: PASS (datasource + tui packages compile with the new fakes).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/tui/datasource/ internal/tui/live_test.go
git add internal/tui/datasource/ internal/tui/live_test.go
git commit -m "feat(datasource): LoadUsuals Cmd + snapshot cache under UsualsKey"
```

---

## Task 4: `Rail` component

**Files:**
- Create: `internal/tui/screens/rail.go`
- Test: `internal/tui/screens/rail_test.go`

**Interfaces:**
- Consumes: `theme` styles (`DimStyle`, `CatOffStyle`, `CursorStyle`, `Fg`, `Gold`, `Div2`), `lipgloss` width helpers.
- Produces:
  - `type Rail struct{ … }`
  - `func NewRail(categories []string) Rail` — builds entries `["🔍 Search", "Home", <categories…>]`.
  - `func (r Rail) WithActive(i int) Rail`, `func (r Rail) WithFocus(f bool) Rail`, `func (r Rail) WithHeight(h int) Rail`.
  - `func (r Rail) Active() int`, `func (r Rail) Len() int`, `func (r Rail) EntryLabel(i int) string`.
  - Constants `RailSearch = 0`, `RailHome = 1` (category entries start at index 2).
  - `func (r Rail) Width() int` (fixed, e.g. 14), `func (r Rail) View() string` — a fixed-width left column; active entry gold-underlined; cursor bright when focused, dim when not.

- [ ] **Step 1: Write the failing test**

Create `internal/tui/screens/rail_test.go`:

```go
package screens

import (
	"strings"
	"testing"
)

func TestRailEntriesOrder(t *testing.T) {
	r := NewRail([]string{"Coffee", "Pizza"})
	// 🔍 Search, Home, Coffee, Pizza
	if r.Len() != 4 {
		t.Fatalf("expected 4 entries, got %d", r.Len())
	}
	if !strings.Contains(r.EntryLabel(RailSearch), "Search") {
		t.Errorf("index 0 must be Search: %q", r.EntryLabel(RailSearch))
	}
	if r.EntryLabel(RailHome) != "Home" {
		t.Errorf("index 1 must be Home: %q", r.EntryLabel(RailHome))
	}
	if r.EntryLabel(2) != "Coffee" {
		t.Errorf("categories start at index 2: %q", r.EntryLabel(2))
	}
}

func TestRailViewShowsEntriesAndSearchIcon(t *testing.T) {
	v := NewRail([]string{"Coffee"}).WithActive(RailHome).WithHeight(10).View()
	for _, want := range []string{"🔍", "Search", "Home", "Coffee"} {
		if !strings.Contains(v, want) {
			t.Errorf("rail view missing %q:\n%s", want, v)
		}
	}
}

func TestRailFixedWidth(t *testing.T) {
	r := NewRail([]string{"Coffee"})
	if r.Width() < 10 {
		t.Fatalf("rail width should be a sensible fixed column, got %d", r.Width())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/screens/ -run TestRail -v`
Expected: FAIL — `NewRail` undefined.

- [ ] **Step 3: Implement `rail.go`**

Create `internal/tui/screens/rail.go`:

```go
package screens

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"console.store/internal/tui/theme"
)

// Rail is the left navigation column on the live Restaurants browse: a 🔍 Search
// entry, Home, then the cuisine categories. The root maps the active entry to a
// load command. It is a passive value type (With* return copies).
type Rail struct {
	entries []string
	active  int
	focus   bool
	height  int
}

// Fixed rail entry indices. Category entries begin at railCatBase.
const (
	RailSearch  = 0
	RailHome    = 1
	railCatBase = 2
	railWidth   = 14 // column width incl. the divider gutter
)

// NewRail builds the rail entries: Search, Home, then the categories.
func NewRail(categories []string) Rail {
	entries := make([]string, 0, len(categories)+2)
	entries = append(entries, "🔍 Search", "Home")
	entries = append(entries, categories...)
	return Rail{entries: entries, active: RailHome}
}

func (r Rail) WithActive(i int) Rail { r.active = i; return r.clamp() }
func (r Rail) WithFocus(f bool) Rail { r.focus = f; return r }
func (r Rail) WithHeight(h int) Rail { r.height = h; return r }

func (r Rail) clamp() Rail {
	if r.active < 0 {
		r.active = 0
	}
	if r.active >= len(r.entries) {
		r.active = len(r.entries) - 1
	}
	return r
}

func (r Rail) Active() int              { return r.active }
func (r Rail) Len() int                 { return len(r.entries) }
func (r Rail) Width() int               { return railWidth }
func (r Rail) EntryLabel(i int) string {
	if i < 0 || i >= len(r.entries) {
		return ""
	}
	// strip the icon prefix for the Search entry's logical label
	return strings.TrimPrefix(r.entries[i], "🔍 ")
}

// IsCategory reports whether entry i is a cuisine category (vs Search/Home), and
// returns its 0-based category index.
func (r Rail) IsCategory(i int) (int, bool) {
	if i >= railCatBase && i < len(r.entries) {
		return i - railCatBase, true
	}
	return 0, false
}

func (r Rail) View() string {
	var b strings.Builder
	for i, e := range r.entries {
		cursor := "  "
		label := theme.CatOffStyle.Render(e)
		if i == r.active {
			if r.focus {
				cursor = theme.CursorStyle.Render("▸ ")
			} else {
				cursor = theme.DimStyle.Render("▸ ")
			}
			label = theme.Fg(theme.Gold).Underline(true).Render(e)
		}
		row := cursor + label
		// pad/truncate to the content width (rail minus the divider gutter)
		row = lipgloss.NewStyle().Width(railWidth - 2).Render(row)
		b.WriteString(row + theme.Fg(theme.Div2).Render(" │") + "\n")
	}
	// pad to height so the divider runs the full pane
	for n := len(r.entries); n < r.height; n++ {
		b.WriteString(lipgloss.NewStyle().Width(railWidth-2).Render("") + theme.Fg(theme.Div2).Render(" │") + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
```

> **Width/emoji note:** the 🔍 glyph is double-width in many terminals. `lipgloss.Width`/`.Width(n)` account for that; keep the entries fitting `railWidth-2`. If a category label is longer, lipgloss truncates — acceptable (categories are short).

- [ ] **Step 4: Run tests**

Run: `go test ./internal/tui/screens/ -run TestRail -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/tui/screens/rail.go internal/tui/screens/rail_test.go
git add internal/tui/screens/rail.go internal/tui/screens/rail_test.go
git commit -m "feat(screens): Rail component (Search/Home/categories, focus-aware)"
```

---

## Task 5: `Menu` two-pane render (rail + sectioned main + search input)

**Files:**
- Modify: `internal/tui/screens/menu.go`
- Test: `internal/tui/screens/menu_test.go` (create or append)

**Interfaces:**
- Consumes: `Rail` (Task 4); existing `components.List`, `components.ContentWidth`, `justify`, `theme`.
- Produces (new `Menu` builders + state, all live-gated; mock path unchanged when `rail` is unset):
  - `func (m Menu) WithRail(r Rail) Menu`, `func (m Menu) WithRailFocus(f bool) Menu`.
  - `func (m Menu) WithSections(usuals, nearby []catalog.Place) Menu` — when set (Home view), the main pane shows a `─ your usuals ─` section (omitted if `usuals` empty) above a `─ nearby ─` section; the cursor/Selected() span both.
  - `func (m Menu) WithSearchMode(active bool, query string, results []catalog.Place, resultCount int) Menu` — main pane shows a 🔍 input + results + `N results`.
  - `View()` renders two-pane (`lipgloss.JoinHorizontal(Top, rail.View(), mainPane)`) when a rail is set; otherwise the existing single-pane View (mock).

- [ ] **Step 1: Write the failing test**

Create/append `internal/tui/screens/menu_test.go`:

```go
package screens

import (
	"strings"
	"testing"

	"console.store/internal/catalog"
)

func liveMenu() Menu {
	return NewMenu(nil, catalog.Address{Line: "Home"}, catalog.SectionFood, catalog.Usual{}, false, "🛒 empty").
		WithRail(NewRail([]string{"Coffee", "Pizza"})).WithRailFocus(false).WithMaxRows(20)
}

func TestMenuTwoPaneHomeSections(t *testing.T) {
	m := liveMenu().WithSections(
		[]catalog.Place{{Name: "Blue Tokai", ETA: "25 min"}},
		[]catalog.Place{{Name: "Pizza Hut", ETA: "20 min"}},
	)
	v := m.View()
	for _, want := range []string{"🔍", "Home", "your usuals", "Blue Tokai", "nearby", "Pizza Hut"} {
		if !strings.Contains(v, want) {
			t.Errorf("home view missing %q:\n%s", want, v)
		}
	}
}

func TestMenuUsualsOmittedWhenEmpty(t *testing.T) {
	m := liveMenu().WithSections(nil, []catalog.Place{{Name: "Pizza Hut"}})
	if strings.Contains(m.View(), "your usuals") {
		t.Errorf("empty usuals must omit the section:\n%s", m.View())
	}
}

func TestMenuSearchModeResults(t *testing.T) {
	m := liveMenu().WithSearchMode(true, "pizza", []catalog.Place{{Name: "Pizza Hut"}}, 1)
	v := m.View()
	if !strings.Contains(v, "pizza") || !strings.Contains(v, "Pizza Hut") || !strings.Contains(v, "1 result") {
		t.Errorf("search view missing query/results/count:\n%s", v)
	}
}

func TestMenuSearchNoResults(t *testing.T) {
	m := liveMenu().WithSearchMode(true, "xyz", nil, 0)
	if !strings.Contains(m.View(), `no restaurants for "xyz"`) {
		t.Errorf("empty-results copy missing:\n%s", m.View())
	}
}

func TestMenuMockPaneUnchanged(t *testing.T) {
	// No rail set → the existing single-pane mock render (section tabs present).
	m := NewMenu([]catalog.Place{{Name: "Blue Tokai"}}, catalog.Address{Line: "Home"},
		catalog.SectionCoffee, catalog.Usual{}, false, "🛒 empty")
	v := m.View()
	if !strings.Contains(v, "coffee") || strings.Contains(v, "your usuals") {
		t.Errorf("mock pane must be unchanged (tabs, no rail sections):\n%s", v)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/screens/ -run TestMenu -v`
Expected: FAIL — `WithRail`/`WithSections`/`WithSearchMode` undefined.

- [ ] **Step 3: Implement the two-pane path**

In `internal/tui/screens/menu.go`:

Add fields to `Menu`:

```go
	rail        Rail
	hasRail     bool
	railFocus   bool
	usuals      []catalog.Place
	nearby      []catalog.Place
	hasSections bool
	searchMode  bool
	searchQuery string
	results     []catalog.Place
	resultCount int
}
```

Add builders:

```go
func (m Menu) WithRail(r Rail) Menu      { m.rail = r; m.hasRail = true; return m }
func (m Menu) WithRailFocus(f bool) Menu { m.railFocus = f; return m }

func (m Menu) WithSections(usuals, nearby []catalog.Place) Menu {
	m.usuals, m.nearby, m.hasSections = usuals, nearby, true
	m.searchMode = false
	return m
}

func (m Menu) WithSearchMode(active bool, query string, results []catalog.Place, count int) Menu {
	m.searchMode, m.searchQuery, m.results, m.resultCount = active, query, results, count
	if active {
		m.hasSections = false
	}
	return m
}

// mainPlaces is the flat, cursor-addressable list backing the main pane (usuals
// then nearby on Home, or results in search mode). Selected() reads from it.
func (m Menu) mainPlaces() []catalog.Place {
	switch {
	case m.searchMode:
		return m.results
	case m.hasSections:
		return append(append([]catalog.Place{}, m.usuals...), m.nearby...)
	default:
		return m.places
	}
}
```

Change `Selected()` to read `mainPlaces()` when a rail is set:

```go
func (m Menu) Selected() (catalog.Place, bool) {
	src := m.places
	if m.hasRail {
		src = m.mainPlaces()
	}
	i := m.list.SelectedIndex()
	if i < 0 || i >= len(src) {
		return catalog.Place{}, false
	}
	return src[i], true
}
```

> **Cursor coherence:** when a rail is set, build `m.list.Rows` from `mainPlaces()` (with section-header rows skipped from selection). To keep this task bounded and the cursor 1:1 with `mainPlaces()`, render section HEADERS as plain text lines printed BETWEEN the list rows in `View()` rather than as `components.List` rows — i.e. the `components.List` holds only selectable restaurant rows, and `View()` interleaves the `─ your usuals ─` / `─ nearby ─` labels at the right offsets. If that interleave is too fiddly for one task, an acceptable simpler render: print the usuals block, then the nearby block, each as its own non-cursor text, and keep ONE `components.List` over `mainPlaces()` shown beneath a single combined header — but the test only requires the substrings, so the interleaved-label approach is preferred. Pick the interleaved approach; keep the `components.List` rows aligned to `mainPlaces()` order.

Add the two-pane `View()` branch at the TOP of `View()`:

```go
func (m Menu) View() string {
	if m.hasRail {
		return m.twoPaneView()
	}
	// … existing single-pane body unchanged …
}
```

Add `twoPaneView`:

```go
func (m Menu) twoPaneView() string {
	railH := m.list.MaxRows + 6 // entries + headers breathing room
	if railH < m.rail.Len()+1 {
		railH = m.rail.Len() + 1
	}
	left := m.rail.WithFocus(m.railFocus).WithActive(m.rail.Active()).WithHeight(railH).View()

	var main strings.Builder
	header := theme.DimStyle.Render("deliver to ") + theme.BrightStyle.Render(m.address.Line) +
		theme.DimStyle.Render(" · "+m.address.Label)
	main.WriteString(header + "\n\n")

	switch {
	case m.searchMode:
		main.WriteString("  " + theme.CursorStyle.Render("🔍 "+m.searchQuery+"▏") + "\n")
		if m.searchQuery != "" {
			main.WriteString("  " + theme.DimStyle.Render(plural(m.resultCount, "result", "results")) + "\n\n")
		}
		if m.searchQuery != "" && len(m.results) == 0 {
			main.WriteString("  " + theme.DimStyle.Render(fmt.Sprintf(`no restaurants for "%s"`, m.searchQuery)) + "\n")
		} else {
			main.WriteString(m.list.View())
		}
	case m.hasSections:
		// interleave section labels with the single cursor list (see note)
		main.WriteString(m.sectionedListView())
	default:
		main.WriteString(m.list.View())
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, left, "  "+strings.ReplaceAll(main.String(), "\n", "\n  "))
}

// plural renders "1 result" / "N results".
func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}
```

`sectionedListView` prints `─ your usuals ─` (omitted when `m.usuals` empty), the usuals rows, `─ nearby ─`, the nearby rows — drawn from `m.list` (whose rows are `mainPlaces()` in order) with the cursor marker on the globally-selected index. Implement it to render each place row via the same styling `NewMenu` uses (`theme.ItemStyle` name + `theme.EtaStyle` eta + `★rating`/offer per §5), inserting the dim hairline labels at the usuals/nearby boundary. Keep substrings `your usuals`, `nearby` exact (lowercase).

Ensure `NewMenu` (when a rail will be attached) seeds `m.list.Rows` from `mainPlaces()` — do this in the root when constructing the live Menu (Task 6), not here.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/tui/screens/ -run TestMenu -v`
Expected: PASS (two-pane tests + the mock-unchanged test).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/tui/screens/menu.go internal/tui/screens/menu_test.go
git add internal/tui/screens/menu.go internal/tui/screens/menu_test.go
git commit -m "feat(screens): Menu two-pane (rail + usuals/nearby/results + search)"
```

---

## Task 6: app.go wiring (rail nav, loads, search, replace chip row)

**Files:**
- Modify: `internal/tui/app.go`
- Test: `internal/tui/menu_flow_test.go` (create) + existing `internal/tui/app_test.go`/`flow_test.go` updates if copy changed.

**Interfaces:**
- Consumes: `screens.NewRail`, `Menu.WithRail/WithRailFocus/WithSections/WithSearchMode` (Tasks 4–5); `datasource.LoadUsuals`, `datasource.UsualsLoadedMsg`, `datasource.UsualsKey`, `datasource.LoadPlacesQuery` (Task 3 + existing); `repo.PlacesByQuery` (existing).
- Produces: Model fields `railFocus bool`, `railActive int`, `searchMode bool`, `searchQuery string`; the live browse builds the two-pane Menu; rail nav routes selection → load Cmd; search submit → `LoadPlacesQuery`; `LoadUsuals` fires on entering the live browse.

- [ ] **Step 1: Write the failing test**

Create `internal/tui/menu_flow_test.go` using the existing teatest harness (mirror `flow_test.go`'s setup with a live fake backend that returns canned restaurants for `Usuals`/`PlacesQuery`):

```go
package tui

// TestLiveBrowseRailFocusAndSearch drives: land on Home (rail shows Search/Home/
// categories), press ← to focus rail, ↓ to a category, Enter loads it; ← to
// Search, type, Enter shows results. Assert via rendered substrings.
//
// Follow flow_test.go's teatest pattern: build a live Model (WithLiveBackend +
// seeded snapshot or a fake that answers Usuals/PlacesQuery), send keys, and
// teatest.WaitFor on bytes.Contains for "🔍", "Home", a category name, and a
// searched restaurant name.
```

> **Implementer note:** open `internal/tui/flow_test.go` and `internal/tui/live_test.go` and copy their exact harness (how they construct a live `Model`, the fake backend, and `teatest`). Write the concrete test against that harness. The behavior to assert: (a) on the live browse the rail renders (`🔍`, `Home`, a category); (b) `←` then `↓`+`Enter` on a category swaps the main list to that category's restaurants; (c) `←` to Search + typed query + `Enter` shows results. Keep it to substrings.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/ -run TestLiveBrowseRail -v`
Expected: FAIL — rail not wired / undefined fields.

- [ ] **Step 3: Implement the wiring**

In `internal/tui/app.go`:

3a. Add Model fields (near `chips`): `railFocus bool`, `railActive int`, `searchMode bool`, `searchQuery string`.

3b. In the live-browse Menu construction (where `WithChips` is attached today, ~line 249), REPLACE the chip attachment with the rail + sections:

```go
if m.live && len(m.chips) > 0 {
	cats := make([]string, len(m.chips))
	for i, c := range m.chips {
		cats[i] = c.Label
	}
	rail := screens.NewRail(cats).WithActive(m.railActive).WithFocus(m.railFocus)
	menu = menu.WithRail(rail).WithRailFocus(m.railFocus)
	if m.searchMode {
		results := m.repo.PlacesByQuery(m.addr, m.searchQuery)
		menu = menu.WithSearchMode(true, m.searchQuery, results, len(results))
	} else if catIdx, isCat := rail.IsCategory(m.railActive); isCat {
		menu = menu.WithSections(nil, m.repo.PlacesByQuery(m.addr, m.chips[catIdx].Query)).
			WithSearchMode(false, "", nil, 0)
	} else { // Home
		usuals := m.repo.PlacesByQuery(m.addr, datasource.UsualsKey)
		nearby := m.repo.PlacesByQuery(m.addr, "") // general nearby (query "")
		menu = menu.WithSections(usuals, nearby)
	}
	// IMPORTANT: seed the list rows from the main places (usuals+nearby / results
	// / category) so the cursor maps 1:1 — add a Menu.WithMainList() helper in
	// Task 5 if NewMenu's rows don't already reflect mainPlaces(); otherwise pass
	// the combined places into NewMenu's `places` arg above.
}
```

> Build the live `Menu`'s `places`/list rows from the SAME slice `mainPlaces()` returns for the active view, so `Selected()` and the cursor agree. Simplest: construct `NewMenu(mainPlaces, …)` with the active view's places, then attach the rail/sections for rendering.

3c. Browse-open: when entering the live browse (the same place `LoadPlacesQuery`/menu loads fire today), also fire `datasource.LoadUsuals(m.backend, m.snap, m.addr.ID)` once (guard with the existing ensureLoaded/dedup pattern so it doesn't refire every render). Add a `UsualsLoadedMsg` handler that just rebuilds the menu (no error surfaced; empty is fine):

```go
case datasource.UsualsLoadedMsg:
	if dm.Err != nil {
		dbgTUI("usuals: %v", dm.Err)
	}
	m.menu = m.buildMenu()
	return m, nil
```

3d. Replace the scrMenu key handling (chip `←/→` nav + `/` search) with rail nav:

- `←`: `m.railFocus = true` (focus rail).
- `→` / `esc` (when rail focused): `m.railFocus = false`.
- When `railFocus`: `↑/↓` move `m.railActive` (clamp 0..rail.Len()-1); `enter` (or `→`) commits the entry:
  - `RailSearch` → `m.searchMode = true; m.railFocus = false; m.searchQuery = ""`.
  - `RailHome` → `m.searchMode = false`; fire `LoadUsuals` + nearby (`LoadPlaces`/`LoadPlacesQuery(addr,"")`) if not cached.
  - category → `m.searchMode = false`; fire `LoadPlacesQuery(addr, m.chips[catIdx].Query)` if not cached; set view to that category.
- When NOT `railFocus` and NOT `searchMode`: `↑/↓` move the main list (existing), `enter` opens the restaurant (existing).
- Search mode: typed runes append to `m.searchQuery`; `backspace` trims; `enter` fires `datasource.LoadPlacesQuery(m.backend, m.snap, m.addr.ID, m.searchQuery)` (submit-only — NOT per keystroke); `esc` exits search (`m.searchMode=false`).

Remove the old chip `NextCategory/PrevCategory`/`/`-filter live path (keep the mock `/` filter in `screens.Menu.Update`, which only runs for the mock single-pane).

> **Implementer note:** read the current scrMenu handler block fully before editing; preserve the `a` (address), `c` (cart), `tab` (vertical), `:` (cmd) bindings and the double-Esc home gesture. Only the category-nav + search bindings change.

- [ ] **Step 4: Run tests + build**

Run: `go build ./... && go test ./... 2>&1 | tail -25`
Expected: build clean; all packages PASS. Update any `app_test.go`/`flow_test.go` substring assertions whose rendered copy changed (the live browse now shows the rail; mock assertions must still pass unchanged).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/tui/app.go internal/tui/menu_flow_test.go
git add internal/tui/app.go internal/tui/menu_flow_test.go
git commit -m "feat(tui): rail-driven live browse — usuals Home, categories, global search"
```

---

## Task 7: Live verification (no order placed)

**Files:** none (verification only).

- [ ] **Step 1: Rebuild + restart**

```bash
go build ./...
# Restart broker + sshd as the project does (CONSOLE_DEBUG_TUI=1, CONSOLE_DEBUG_SWIGGY=1).
```

- [ ] **Step 2: Verify the live browse**

ssh in → Restaurants. Confirm:
- The **left rail** renders: `🔍 Search`, `Home`, then the cuisine categories; the active entry is gold-underlined; focus cursor is obvious.
- **Home** shows a `─ nearby ─` section (and `─ your usuals ─` ONLY if this account has history — expected empty on the test account, so likely just nearby). No error, no empty-crash.
- `←` focuses the rail; `↓` to a category + `Enter` swaps the main list to that cuisine's restaurants; `→`/`Esc` returns to the list.
- `←` to `🔍 Search` + `Enter` → type a query + `Enter` → real Swiggy-wide results + `N results`; a nonsense query → `no restaurants for "<q>"`.
- `Enter` on a restaurant → its menu (downstream unchanged).

- [ ] **Step 3: Confirm mock path untouched**

`CONSOLE_BACKEND` unset (mock) → the browse is the OLD single-pane section-tab list with the `/` filter; no rail. (Or rely on the green `TestMenuMockPaneUnchanged` + mock flow tests.)

- [ ] **Step 4: Record outcome**

Note: rail/categories/search verified live; usuals path is empty on this account (no history) and verified not to crash; mock unaffected; no order placed.

---

## Notes for the executor

- **Usuals has no live data on the test account** (`get_food_orders` → `{}`). Build + verify the EMPTY path; the populated path is covered by unit tests (Task 1) and lights up when an account has history.
- **Cursor↔mainPlaces coherence** (Task 5/6) is the one subtle correctness point: the `components.List` rows and `Selected()` must both read the active view's `mainPlaces()` slice in the same order, with section header labels drawn as non-selectable text. Verify by selecting across the usuals→nearby boundary.
- **Mock path is sacred:** the rail/sections/search are gated on the live rail being set; every mock test (`app_test.go`, `flow_test.go`, `TestMenuMockPaneUnchanged`) must stay green untouched.
- Reuse existing seams (`PlacesByQuery`, `LoadPlacesQuery`, `toPlaces`, `mapRestaurants`, the `ensureLoaded` dedup) — do not add parallel data paths.
