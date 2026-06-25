# Config File Seed + Restaurant Navigation Load · Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a JSON config file that pre-populates the TUI with a specific restaurant and curated items (using real Swiggy IDs), so the demo boots straight into a real restaurant without requiring Swiggy search; and fire `LoadMenu` when the user navigates into any restaurant in live mode so real item data always loads.

**Architecture:** A new `internal/config` package parses `console.json` (path via `CONSOLE_CONFIG` env, default `console.json`). `cmd/sshd` reads the config on startup, seeds the `catalog/swiggy.Snapshot` via existing `SetAddresses`/`SetPlaces`/`SetMenu` calls, and passes a `WithSeededSnapshot()` option to `New()`. `New()` now adopts the first address AFTER options apply, so the seeded address is picked up automatically. `liveInitCmds` skips live loads when seeded. `app.go` scrMenu `enter` fires a `LoadMenu` Cmd in live mode to refresh real item data when the user taps a restaurant.

**Tech Stack:** Go 1.26, `encoding/json` (stdlib — no new deps), `catalog/swiggy.Snapshot`, bubbletea `Cmd`.

## Global Constraints

- Module `console.store`; Go floor `go 1.26.4`. `gofmt` clean, `go vet ./...` clean, `go test ./...` green.
- `internal/config` imports only stdlib. No YAML dependency — config files are JSON.
- `internal/catalog/swiggy` must NOT import `internal/config` (avoids cycle). Seeding happens in `cmd/sshd`.
- Mock path unaffected: `New(caps)` with no options must behave exactly as today; all existing tests pass.
- `p.SwiggyID` (not `p.ID`) is used when firing `LoadMenu` for a live restaurant — SwiggyID is the real Swiggy restaurant ID passed to the broker.
- Config file is optional. If absent or `CONSOLE_CONFIG` is unset and `console.json` doesn't exist, live path works exactly as before (loads addresses from Swiggy API).

---

### Task 1: `internal/config` — JSON config package

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `internal/config/testdata/console.json`

**Interfaces:**
- Consumes: nothing (stdlib only)
- Produces:
  ```go
  // package config (import path console.store/internal/config)
  type ConfigItem struct {
      ID      string `json:"id"`       // real Swiggy item ID
      Name    string `json:"name"`
      Price   int    `json:"price"`
      Veg     bool   `json:"veg"`
      Desc    string `json:"desc"`
      Section string `json:"section"`  // "coffee"|"food"|"snacks"
  }
  type Seed struct {
      AddressID      string       `json:"address_id"`
      RestaurantID   string       `json:"restaurant_id"`   // real Swiggy restaurant ID
      RestaurantName string       `json:"restaurant_name"`
      Section        string       `json:"section"`
      Items          []ConfigItem `json:"items"`
  }
  type Config struct {
      Seed Seed `json:"seed"`
  }
  // Load reads the JSON config at path. Returns nil, nil when the file doesn't exist.
  func Load(path string) (*Config, error)
  // DefaultPath returns $CONSOLE_CONFIG or "console.json".
  func DefaultPath() string
  ```

- [ ] **Step 1: Write the failing test**

```go
// internal/config/config_test.go
package config

import (
	"testing"
)

func TestLoadParsesFile(t *testing.T) {
	cfg, err := Load("testdata/console.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Seed.AddressID != "addr-001" {
		t.Errorf("address_id = %q", cfg.Seed.AddressID)
	}
	if cfg.Seed.RestaurantID != "rest-001" {
		t.Errorf("restaurant_id = %q", cfg.Seed.RestaurantID)
	}
	if cfg.Seed.RestaurantName != "Blue Tokai Coffee" {
		t.Errorf("restaurant_name = %q", cfg.Seed.RestaurantName)
	}
	if cfg.Seed.Section != "coffee" {
		t.Errorf("section = %q", cfg.Seed.Section)
	}
	if len(cfg.Seed.Items) != 2 {
		t.Fatalf("items len = %d", len(cfg.Seed.Items))
	}
	it := cfg.Seed.Items[0]
	if it.ID != "item-001" || it.Name != "Cold Coffee" || it.Price != 220 || !it.Veg {
		t.Errorf("item[0] = %+v", it)
	}
}

func TestLoadMissingFileReturnsNil(t *testing.T) {
	cfg, err := Load("testdata/does-not-exist.json")
	if err != nil {
		t.Fatalf("unexpected error for missing file: %v", err)
	}
	if cfg != nil {
		t.Fatal("expected nil config for missing file")
	}
}

func TestDefaultPath(t *testing.T) {
	t.Setenv("CONSOLE_CONFIG", "")
	if got := DefaultPath(); got != "console.json" {
		t.Errorf("DefaultPath() = %q", got)
	}
	t.Setenv("CONSOLE_CONFIG", "/etc/console.json")
	if got := DefaultPath(); got != "/etc/console.json" {
		t.Errorf("DefaultPath() = %q", got)
	}
}
```

- [ ] **Step 2: Create the test fixture**

```json
// internal/config/testdata/console.json
{
  "seed": {
    "address_id": "addr-001",
    "restaurant_id": "rest-001",
    "restaurant_name": "Blue Tokai Coffee",
    "section": "coffee",
    "items": [
      {
        "id": "item-001",
        "name": "Cold Coffee",
        "price": 220,
        "veg": true,
        "desc": "Single origin cold brew",
        "section": "coffee"
      },
      {
        "id": "item-002",
        "name": "Espresso",
        "price": 180,
        "veg": true,
        "desc": "Double shot",
        "section": "coffee"
      }
    ]
  }
}
```

- [ ] **Step 3: Run to verify fail**

Run: `go test ./internal/config/ -v`
Expected: FAIL — undefined.

- [ ] **Step 4: Implement `internal/config/config.go`**

```go
// Package config loads the optional console.json seed file that pre-populates
// the TUI with a specific restaurant and curated items for live demo use.
package config

import (
	"encoding/json"
	"errors"
	"os"
)

// ConfigItem is one menu item in the seed config, with its real Swiggy item ID.
type ConfigItem struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Price   int    `json:"price"`
	Veg     bool   `json:"veg"`
	Desc    string `json:"desc"`
	Section string `json:"section"`
}

// Seed is the pre-populated restaurant configuration.
type Seed struct {
	AddressID      string       `json:"address_id"`
	RestaurantID   string       `json:"restaurant_id"`
	RestaurantName string       `json:"restaurant_name"`
	Section        string       `json:"section"`
	Items          []ConfigItem `json:"items"`
}

// Config is the top-level console.json structure.
type Config struct {
	Seed Seed `json:"seed"`
}

// DefaultPath returns the config file path: $CONSOLE_CONFIG or "console.json".
func DefaultPath() string {
	if p := os.Getenv("CONSOLE_CONFIG"); p != "" {
		return p
	}
	return "console.json"
}

// Load reads and parses the JSON config at path.
// Returns nil, nil when the file does not exist (missing config is not an error).
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
```

- [ ] **Step 5: Run to verify pass**

Run: `go test ./internal/config/ -v`
Expected: PASS (3 tests).

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go internal/config/testdata/console.json
git commit -m "feat(config): JSON seed config loader (restaurant + curated items)"
```

---

### Task 2: `New()` addr-after-opts + `WithSeededSnapshot` + `liveInitCmds` skip

**Files:**
- Modify: `internal/tui/app.go` (Model struct: add `seeded bool`; `New()`: adopt addr after opts)
- Modify: `internal/tui/live.go` (add `WithSeededSnapshot()` Option; update `liveInitCmds`)
- Modify: `internal/tui/live_test.go` (add `TestSeededPathSkipsLiveLoads`)

**Interfaces:**
- Consumes: existing `WithLiveBackend`, `swiggy.Snapshot`, `datasource.LoadAddresses/LoadPlaces`
- Produces:
  ```go
  // package tui
  func WithSeededSnapshot() Option  // sets m.seeded = true
  // New(caps, opts...): addr adopted AFTER opts so seeded snapshot addr is picked up
  ```

- [ ] **Step 1: Write the failing test**

Add to `internal/tui/live_test.go`:

```go
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
	// We can't inspect the batch contents directly; instead verify liveInitCmds returns nil.
	if c := m.liveInitCmds(); c != nil {
		t.Fatal("liveInitCmds must return nil when seeded (no live loads on boot)")
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/tui/ -run TestSeededPath -v`
Expected: FAIL — `WithSeededSnapshot` undefined; `m.seeded` undefined.

- [ ] **Step 3: Add `seeded bool` to Model and update `New()` in `internal/tui/app.go`**

In the `type Model struct { ... }` block, add after `needsAuth bool`:
```go
	seeded bool // true when catalog/swiggy.Snapshot was pre-seeded from config; skips live init loads
```

Replace `New()`:
```go
func New(caps render.Caps, opts ...Option) Model {
	repo := mem.New()
	section := catalog.SectionCoffee
	m := Model{repo: repo, section: section, screen: scrSplash, caps: caps, lastEscFrame: -escDoubleWindow - 1}
	for _, o := range opts {
		o(&m)
	}
	// Adopt first address after opts: live path may have seeded the snapshot already
	// (WithLiveBackend swaps m.repo; if the snapshot has addresses, use them).
	if m.addr.ID == "" {
		if addrs := m.repo.Addresses(); len(addrs) > 0 {
			m.addr = addrs[0]
		}
	}
	m.splash = screens.NewSplash().WithCaps(caps)
	m.splashPhrase = screens.RandomPhrase("")
	m.menu = m.buildMenu()
	return m
}
```

- [ ] **Step 4: Add `WithSeededSnapshot()` and update `liveInitCmds` in `internal/tui/live.go`**

Replace the file content:

```go
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	swiggysnap "console.store/internal/catalog/swiggy"
	"console.store/internal/tui/datasource"
)

// Option configures a Model at construction (functional-options so the existing
// New(caps) call site is unchanged).
type Option func(*Model)

// WithLiveBackend arms the live (broker-backed) data path: it replaces the mock
// Repository with a Snapshot-backed one and stores the async backend. When set,
// Init dispatches the initial loads and a missing-token error flips the Model to
// the authorize gate (showing authorizeURL).
func WithLiveBackend(b datasource.Backend, snap *swiggysnap.Snapshot, accountID, authorizeURL string) Option {
	return func(m *Model) {
		m.live = true
		m.backend = b
		m.snap = snap
		m.accountID = accountID
		m.authorizeURL = authorizeURL
		m.repo = swiggysnap.NewRepository(snap)
	}
}

// WithSeededSnapshot marks that the Snapshot was pre-populated from a config
// file. liveInitCmds skips live API loads — the seed data drives the first view.
// LoadMenu still fires when the user navigates into a restaurant.
func WithSeededSnapshot() Option {
	return func(m *Model) { m.seeded = true }
}

// liveInitCmds returns the initial fetches for a live session. When seeded,
// the snapshot already has data; skip live loads so the TUI is instantly usable.
func (m Model) liveInitCmds() tea.Cmd {
	if !m.live {
		return nil
	}
	if m.seeded {
		return nil
	}
	return tea.Batch(
		datasource.LoadAddresses(m.backend, m.snap),
		datasource.LoadPlaces(m.backend, m.snap, m.addr.ID, m.section),
	)
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/tui/... -v 2>&1 | tail -20`
Expected: ALL PASS including `TestSeededPathSkipsLiveLoads`. Existing tests unaffected.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/app.go internal/tui/live.go internal/tui/live_test.go
git commit -m "feat(tui): WithSeededSnapshot option; New() adopts addr after opts"
```

---

### Task 3: `cmd/sshd` reads config and seeds Snapshot

**Files:**
- Modify: `cmd/sshd/main.go` (import `internal/config`; seed snapshot in `liveModel`; apply `WithSeededSnapshot()`)

**Interfaces:**
- Consumes: `config.Load`, `config.DefaultPath`, `swiggy.Snapshot.Set*` methods, `consoletui.WithSeededSnapshot()`
- Produces: `liveModel` seeds snapshot from config when file exists

- [ ] **Step 1: Write the failing test**

There's no unit test for `cmd/sshd` (it's a main package). Verify by building:

```bash
go build ./cmd/sshd/ 2>&1
```
Expected: build succeeds (no test to write — verify via build).

- [ ] **Step 2: Modify `cmd/sshd/main.go`**

Add import: `"console.store/internal/config"` and `"console.store/internal/catalog"`.

Replace `liveModel` function:

```go
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
	authURL := ""
	if start, err := cli.StartAuth(pubkey); err == nil {
		authURL = start.AuthorizeURL
	}

	snap := swiggysnap.NewSnapshot()
	be := datasource.NewBrokerBackend(cli, accountID)
	opts := []consoletui.Option{consoletui.WithLiveBackend(be, snap, accountID, authURL)}

	// Load optional seed config to pre-populate the snapshot.
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		log.Printf("live: config load error: %v (continuing without seed)", err)
	}
	if cfg != nil && cfg.Seed.RestaurantID != "" {
		seedSnapshot(snap, cfg)
		opts = append(opts, consoletui.WithSeededSnapshot())
		log.Printf("live: seeded snapshot from config: restaurant=%s items=%d",
			cfg.Seed.RestaurantName, len(cfg.Seed.Items))
	}

	return consoletui.New(caps, opts...), true
}

// seedSnapshot pre-populates snap with the config's restaurant and curated items.
// This lets the TUI boot instantly into the configured restaurant without waiting
// for live Swiggy API calls. LoadMenu still fires when the user navigates in.
func seedSnapshot(snap *swiggysnap.Snapshot, cfg *config.Config) {
	s := cfg.Seed
	section := catalog.Section(s.Section)
	if section == "" {
		section = catalog.SectionCoffee
	}

	// Seed address.
	addr := catalog.Address{ID: s.AddressID, Label: "home"}
	snap.SetAddresses([]catalog.Address{addr})

	// Seed restaurant in the place list.
	place := catalog.Place{
		ID:       s.RestaurantID,
		SwiggyID: s.RestaurantID,
		Name:     s.RestaurantName,
		Section:  section,
	}
	snap.SetPlaces(s.AddressID, section, []catalog.Place{place})

	// Seed menu items.
	items := make([]catalog.Item, len(s.Items))
	for i, it := range s.Items {
		items[i] = catalog.Item{
			ID:       it.ID,
			SwiggyID: it.ID,
			Name:     it.Name,
			Price:    it.Price,
			Veg:      it.Veg,
			Desc:     it.Desc,
			Section:  catalog.Section(it.Section),
		}
	}
	place.Items = items
	snap.SetMenu(place)
}
```

- [ ] **Step 3: Build + full test suite**

Run:
```bash
go build ./... 2>&1
go test ./... 2>&1 | tail -20
go vet ./...
gofmt -l internal/config internal/tui cmd/sshd
```
Expected: builds; all tests pass; `gofmt -l` empty for changed files.

- [ ] **Step 4: Commit**

```bash
git add cmd/sshd/main.go
git commit -m "feat(sshd): load console.json seed config and pre-populate snapshot"
```

---

### Task 4: `scrMenu` enter fires `LoadMenu` in live mode

**Files:**
- Modify: `internal/tui/app.go` (scrMenu `enter` handler: fire `LoadMenu` Cmd in live mode)
- Modify: `internal/tui/live_test.go` (add `TestLiveMenuEnterFiresLoadMenu`)

**Interfaces:**
- Consumes: `datasource.LoadMenu`, `m.live`, `p.SwiggyID`
- Produces: navigating into a restaurant in live mode returns a non-nil `LoadMenu` Cmd alongside navigation

- [ ] **Step 1: Write the failing test**

Add to `internal/tui/live_test.go`:

```go
func TestLiveMenuEnterFiresLoadMenu(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	snap.SetPlaces("a1", catalog.SectionCoffee, []catalog.Place{
		{ID: "r1", SwiggyID: "swiggy-r1", Name: "Blue Tokai", Section: catalog.SectionCoffee},
	})
	be := &liveFake{}
	m := New(render.Caps{},
		WithLiveBackend(be, snap, "acct-1", ""),
		WithSeededSnapshot(),
	)
	// Force window size so menu renders.
	m.w, m.h = 100, 40

	// Simulate pressing enter on the menu (restaurant is first in list).
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um := updated.(Model)
	if um.screen != scrRestaurant {
		t.Fatalf("screen = %v after enter; want scrRestaurant", um.screen)
	}
	if cmd == nil {
		t.Fatal("live mode: enter on menu must return a non-nil LoadMenu Cmd")
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/tui/ -run TestLiveMenuEnterFiresLoadMenu -v`
Expected: FAIL — `cmd` is nil (enter navigates but doesn't dispatch LoadMenu yet).

- [ ] **Step 3: Modify `scrMenu` enter handler in `internal/tui/app.go`**

Find the existing `scrMenu` `enter` block (around line 649):

```go
			case "enter":
				if p, ok := m.menu.Selected(); ok {
					m.rest = screens.NewRestaurant(p, m.qtyMap(), m.cartChip()).WithAddr(m.addr)
					m.screen = scrRestaurant
				}
				return m, nil
```

Replace with:

```go
			case "enter":
				if p, ok := m.menu.Selected(); ok {
					m.rest = screens.NewRestaurant(p, m.qtyMap(), m.cartChip()).WithAddr(m.addr)
					m.screen = scrRestaurant
					if m.live && p.SwiggyID != "" {
						return m, datasource.LoadMenu(m.backend, m.snap, m.addr.ID, p.SwiggyID)
					}
				}
				return m, nil
```

- [ ] **Step 4: Run tests**

Run:
```bash
go test ./internal/tui/... 2>&1 | tail -15
go vet ./internal/tui/...
gofmt -l internal/tui/app.go
```
Expected: ALL PASS; `gofmt -l` empty.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/app.go internal/tui/live_test.go
git commit -m "feat(tui): fire LoadMenu on restaurant navigate in live mode"
```

---

## Self-Review

**Spec coverage:**
- ✓ `internal/config` package with `Load` + `DefaultPath` (Task 1)
- ✓ `console.json` format (Task 1 testdata)
- ✓ `New()` adopts addr after opts (Task 2)
- ✓ `WithSeededSnapshot()` option (Task 2)
- ✓ `liveInitCmds` skips loads when seeded (Task 2)
- ✓ sshd loads config and seeds snapshot (Task 3)
- ✓ `seedSnapshot` populates address + places + menu from config (Task 3)
- ✓ `scrMenu` enter fires `LoadMenu` in live mode (Task 4)
- ✓ Mock path unaffected (existing tests unchanged)

**Placeholder scan:** None found. All code is complete.

**Type consistency:** `config.Config`, `config.Seed`, `config.ConfigItem` defined in Task 1 and consumed in Task 3. `WithSeededSnapshot()` defined in Task 2 live.go and consumed in Task 3 sshd. `m.seeded` field added in Task 2 app.go and referenced in Task 2 live.go + test. `datasource.LoadMenu` signature matches existing Task 5 (Slice 5) definition.
