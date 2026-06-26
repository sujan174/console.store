# console.store → native CLI (`store`) — Design

**Date:** 2026-06-26
**Status:** Approved for planning
**Worktree:** `.claude/worktrees/swiggy-live` (branch `worktree-swiggy-live`)

## Goal

Replace the SSH-served TUI with a single native binary, `store`. A user runs
`store`; if no Swiggy token is saved on the machine, a one-time browser
authorize completes (loopback, on the same PC) and the token is sealed into the
OS keyring; thereafter `store` opens straight into the app. Fast cold start: a
returning user with a valid token does **zero network on startup** until they
act.

## Scope

**In scope (this spec):**
- New `cmd/store` binary: in-process composition of the existing TUI + broker
  service + Swiggy client + OAuth, run against the real terminal.
- Keyring-backed local token + client-registration store (replaces Postgres+KMS).
- In-process backend adapter so the TUI's datasource talks to `broker.Service`
  directly (no Unix socket, no `net/rpc`).
- First-run auth gate redesigned for native: copyable URL + Enter-to-open the
  default browser; loopback callback auto-completes and the app advances. No
  reconnect step.
- Order arming: release builds default `CONSOLE_LIVE_ORDERS` ON via a
  build-stamped default; dev (`go run`/plain `go build`) stays disarmed.
- Deletion of the SSH server, the broker daemon wrapper, the RPC transport, and
  the Postgres+KMS store.

**Out of scope (explicitly NOT this spec):**
- `consolestore.in` website, the `curl | sh` install script, GoReleaser, binary
  hosting. A later, separate spec — not referenced again here.
- QR codes, phone authorization, device-code / paste-the-code flows. Authorize
  happens in a browser on the same PC, full stop.
- Instamart, multi-account on one machine, Windows-specific polish (the keyring
  lib covers Windows, but macOS + Linux are the validated targets).

## Global Constraints

- Go 1.26. `go vet ./...` clean, `gofmt`. No new linter/Make config.
- `internal/tui/screens` MUST NOT import `internal/tui` (import cycle).
- All catalog/data access stays behind the existing `Backend` / `catalog`
  seams. No hardcoded catalog data in screens.
- The mock path (`internal/catalog/mem`) MUST stay green: `store` with no live
  backend configured still runs the demo data and all existing tests pass.
- Dev builds MUST default to orders **disarmed** so no real order can fire
  during development. `CONSOLE_LIVE_ORDERS=1` remains an explicit override.
- The keyring is the only place the Swiggy token is persisted. The token is
  never written to a file or logged.

## Architecture

One process, one binary. The three former processes collapse:

```
cmd/store/main.go
  ├─ localstore (NEW)        keyring token store + client.json registration cache
  │                          implements auth.AccountStore AND broker.TokenStore,
  │                          fixed accountID = "local"
  ├─ auth.Manager (reused)   DCR + PKCE authorize URL + loopback callback handler
  ├─ broker.Service (reused) the only caller of Swiggy; returns api.* types
  ├─ inproc backend (NEW)    implements datasource brokerRPC by calling Service
  │                          directly (ctx + "local"); wrapped by BrokerBackend
  └─ tui.Model (reused)      run via tea.NewProgram on the real TTY
```

### Why this shape

`broker.Service` already returns `api.*` types and the datasource `Backend`
already consumes those same `api.*` types ([service.go], [broker_backend.go]).
The RPC `Client` is a pure transport pass-through. So an in-process adapter that
implements the existing `brokerRPC` interface by calling `Service` methods needs
**zero remapping**, and `BrokerBackend` is reused verbatim — every screen, the
datasource Cmds, and the snapshot are untouched.

### Local identity ("local" account)

There is no SSH pubkey and no multi-user concept on a machine. A single fixed
`accountID = "local"` is used everywhere `broker.Service` needs an account key
(its per-account Swiggy-client cache, the token-store key). The phone claim that
`HandleCallback` extracts is irrelevant here: the `localstore`
`FindOrCreateAccount` ignores it and always returns `"local"`, `LinkPubkey` is a
no-op, and the token is stored under `"local"`. This keeps `auth.Manager` and
`broker.Service` unchanged while giving them one consistent account key.

## Components

### 1. `internal/localstore` (NEW package)

A keyring-backed store satisfying **both** interfaces the reused code needs:

- `auth.AccountStore` — `FindOrCreateAccount(_, phone) → ("local", nil)`,
  `LinkPubkey(...) → nil`, `PutToken(ctx, "local", access, refresh, expiresAt)`.
- `broker.TokenStore` — `AccountForPubkey(_, _) → ("local", tokenExists, nil)`,
  `GetTokenFull(ctx, "local")`, `PutToken(...)`, `PurgeToken(...)`.

**Token persistence:** `github.com/zalando/go-keyring`. Service name
`"console.store"`, key `"local"`. Value is the existing JSON token blob
(`{access, refresh, expiresAt}`) — reuse the encode/decode shape from
`internal/store`. Keyring backends: macOS Keychain, Linux Secret Service,
Windows Credential Manager. go-keyring ships a mock backend
(`keyring.MockInit()`) for tests.

**Client-registration cache:** OAuth `client_id` + discovered endpoints
(`authorization_endpoint`, `token_endpoint`) are NOT secret and are cached in a
plain file `~/.config/console-store/client.json` (0600). Written once after the
first Discover+Register; read on every later startup so a returning user makes
no network call to start. Path honors `XDG_CONFIG_HOME`.

### 2. OAuth wiring (reuses `internal/auth`)

`cmd/store` builds the registration:
- If `client.json` exists → load `client_id` + endpoints. **No network.**
- Else → `auth.Discover` + `auth.Register` (DCR, `token_endpoint_auth_method:
  none`, PKCE), then write `client.json`. Network, first run only.

`auth.NewManager` is constructed with the cached `ClientID`, `Metadata`,
`RedirectURI = http://127.0.0.1:8765/cb` (fixed port, overridable via
`CONSOLE_REDIRECT_URI`), `Scope = "mcp:tools"`, `Store = localstore`. The
loopback callback server (`auth.Manager.CallbackHandler` on `127.0.0.1:8765`) is
started exactly as `cmd/broker` does today.

The `Refresher` (`oauthRefresher`, moved from `cmd/broker/adapters.go` into
`cmd/store`) is built from the cached `token_endpoint` + `client_id`, so token
refresh works with no re-discovery.

### 3. `internal/tui/datasource` in-process backend (NEW, small)

A type implementing the existing `brokerRPC` interface ([broker_backend.go:11])
by calling `broker.Service` methods, supplying `context.Background()` and the
fixed `"local"` accountID. Example shape:

```go
type InProc struct{ svc *broker.Service }

// Service.Restaurants(ctx, accountID, addressID, query, organic) ([]api.Restaurant, string, error)
func (p InProc) Restaurants(accountID, addressID, query string) ([]api.Restaurant, error) {
    r, _, err := p.svc.Restaurants(context.Background(), accountID, addressID, query, false)
    return r, err
}
func (p InProc) SearchOrganic(accountID, addressID, query string) ([]api.Restaurant, string, error) {
    return p.svc.Restaurants(context.Background(), accountID, addressID, query, true)
}
// ...one forwarder per brokerRPC method (Addresses, Usuals, Menu, ItemOptions,
// UpdateCart, GetCart, ClearCart, PlaceOrder), each calling the matching
// Service method with context.Background() + accountID.
```

`datasource.NewBrokerBackend(InProc{svc}, "local")` yields the `Backend` the TUI
already uses. (Service's `Restaurants` returns `([]api.Restaurant, string,
error)`; the two `brokerRPC` methods adapt that — `Restaurants` drops the
effective-query string, `SearchOrganic` keeps it. This mirrors what the RPC
`Client` does today in [client.go:45-58].)

### 4. Native auth gate (modifies `internal/tui`)

The current gate ([app.go:2427]) instructs the user to reconnect over SSH. The
native gate:

- On entering the gate, attempt `browser.OpenURL(authorizeURL)`
  (`github.com/pkg/browser`). On failure (headless), the URL remains shown to
  copy.
- Render: the authorize URL (copyable) + a `[ Enter: open in browser ]`
  affordance + a "waiting for authorization…" line.
- On each existing 60ms tick, fire a poll Cmd that calls
  `auth.Manager.Authorized(flowID)` (exposed to the TUI via a small
  `AuthPoller` interface on the backend, or a poll Cmd closure passed in as a
  new `Option`). When it returns true: clear `needsAuth`, run `liveInitCmds()`,
  app advances. No reconnect.
- `Enter` re-opens the browser; `r` retries; `Ctrl+C` quits (existing keys).

The gate needs the `flowID` from `auth.Manager.Start("local")`. To avoid
churning every existing `WithLiveBackend(be, snap, acct, url)` call site (the
live tests), add a **separate** option rather than changing that signature:

```go
// AuthPoller reports whether the loopback callback for flowID has completed.
type AuthPoller interface{ Authorized(flowID string) bool }

func WithAuthFlow(flowID string, p AuthPoller) Option // sets m.authFlowID, m.authPoller
```

`auth.Manager` already satisfies `AuthPoller` (`Authorized(flowID) bool`).
`cmd/store` passes `WithLiveBackend(be, snap, "local", p.AuthorizeURL)` +
`WithAuthFlow(p.FlowID, authMgr)` + `WithPendingAuth()`. The tick handler polls
`m.authPoller.Authorized(m.authFlowID)`.

### 5. `cmd/store/main.go` (NEW)

Startup sequence:
1. Resolve OAuth registration (load `client.json` or Discover+Register+write).
2. Build `localstore`, `auth.Manager`, start loopback callback server.
3. Build `broker.Service` (Store/Auth/Refresher = local wiring,
   `FoodBaseURL = swiggy.FoodBaseURL`).
4. Build `InProc` backend + `datasource.NewBrokerBackend(_, "local")`.
5. Detect caps: `render.DetectCaps(os.Getenv("TERM"), os.Environ(), truecolor)`
   where `truecolor` comes from termenv on the real TTY
   (`termenv.NewOutput(os.Stdout).ColorProfile() == termenv.TrueColor`, or the
   `COLORTERM` env check termenv uses).
6. Token check: `localstore.GetTokenFull(ctx, "local")`.
   - Valid/refreshable → `tui.New(caps, WithLiveBackend(be, snap, "local", ""))`
     + initial loads.
   - Absent → `p := authMgr.Start("local")`; `tui.New(caps,
     WithLiveBackend(be, snap, "local", p.AuthorizeURL, p.FlowID, poller),
     WithPendingAuth())`.
7. `tea.NewProgram(model, tea.WithAltScreen())` on `os.Stdin/os.Stdout`. Emit
   OSC 11 (canvas bg `theme.Bg`) on start and OSC 111 on exit — the native
   equivalent of the SSH `canvasMiddleware`.

Native color: a real TTY lets lipgloss/termenv detect color automatically, so
the SSH-only `lipgloss.SetColorProfile(renderer.ColorProfile())` workaround is
unnecessary.

### 6. Order arming (modifies `internal/swiggy/orders.go`)

`liveOrdersEnabled()` ([orders.go:74]) becomes:

```go
var liveOrdersDefault = "0" // ldflags-stamped to "1" in release builds

func liveOrdersEnabled() bool {
    return os.Getenv("CONSOLE_LIVE_ORDERS") == "1" || liveOrdersDefault == "1"
}
```

Release build stamps `-ldflags "-X console.store/internal/swiggy.liveOrdersDefault=1"`.
Dev `go run`/`go build` leave it `"0"` → disarmed unless the env override is
set. Existing `orders_test.go` `t.Setenv` cases are unaffected (env override
still wins; default stays `"0"` under test).

## Deletions

- `cmd/sshd/` — entire SSH server.
- `cmd/broker/main.go`, `cmd/broker/rpcserver`-side wiring, the Unix socket
  serve loop. `cmd/broker/adapters.go`'s `oauthRefresher` moves to `cmd/store`;
  `brokerStore`/`authStore` (Postgres adapters) are deleted.
- `internal/broker/api/client.go`, `internal/broker/rpcserver.go`, the socket
  transport. The `api` **DTO types** (`Address`, `Restaurant`, `Menu`, `Cart`,
  `Order`, `OptionGroup`, `UpdateCartArgs`, …) STAY — they are the shared
  vocabulary between `Service` and the datasource. Only the `net/rpc` plumbing
  goes.
- `internal/store/` (Postgres) and `internal/store/kms/` — replaced by
  `internal/localstore`.
- go.mod: drop `charmbracelet/wish`, `charmbracelet/ssh`, `jackc/pgx/v5`,
  `aws-sdk-go-v2/*`. Add `zalando/go-keyring`, `pkg/browser`.

## Data flow (returning user, valid token)

```
store → load client.json (no net) → keyring GetTokenFull("local") ok
      → tui.New(WithLiveBackend) → tea.NewProgram
      → first key/action → datasource Cmd → BrokerBackend → InProc
      → broker.Service → swiggy.Client (token from keyring, refresh if near expiry)
      → Swiggy MCP → api.* → snapshot → screen
```

## Data flow (first run, no token)

```
store → no client.json → Discover+Register → write client.json
      → keyring GetTokenFull("local") !ok
      → authMgr.Start("local") → {flowID, authURL}
      → tui auth gate (open browser, show URL, poll)
      → user authorizes in browser → Swiggy → 127.0.0.1:8765/cb
      → HandleCallback → Exchange → localstore.PutToken(keyring,"local")
      → gate poll sees Authorized(flowID) → liveInitCmds → app
```

## Error handling

- **Keyring unavailable** (locked/headless Linux without Secret Service):
  `localstore` surfaces a clear error at startup ("could not open system
  keyring: …") and exits non-zero rather than silently falling back to plaintext.
- **Loopback port 8765 busy:** callback server fails fast with a message naming
  the port and the `CONSOLE_REDIRECT_URI` override.
- **Token expired, refresh fails:** `broker.Service` already maps this to an
  auth error; the datasource wraps it to `ErrNeedsAuth`; the TUI shows the auth
  gate. (Re-auth replaces the keyring token.)
- **`browser.OpenURL` fails:** non-fatal; the URL stays on screen to copy.
- **Discover/Register fails (first run, offline):** fatal with a network-error
  message; nothing is written to `client.json`.

## Testing

- `internal/localstore`: round-trip put/get/purge against `keyring.MockInit()`;
  `FindOrCreateAccount` always returns `"local"`; `client.json` read/write +
  XDG path resolution.
- `internal/tui/datasource` InProc: a fake `broker.Service`-shaped dependency
  proves each `brokerRPC` method forwards with `"local"` + a context, and that
  `Restaurants` vs `SearchOrganic` map the effective-query return correctly.
- Auth gate: `.View()` substring tests (URL shown, "open in browser", "waiting
  for authorization"); a fake poller flips `Authorized` → assert the model
  clears `needsAuth` and issues init Cmds.
- Order arming: `liveOrdersEnabled()` table test — default `"0"` disarmed; env
  `"1"` armed; stamped default `"1"` armed. Existing `orders_test.go` stays
  green.
- Mock path: existing `internal/tui` flow tests and screen tests pass unchanged.
- Build sanity: `go build ./...`, `go vet ./...` after the SSH/Postgres
  deletions (no dangling imports).

## Open items / risks

- **DCR per machine at scale:** each install self-registers an OAuth client.
  Swiggy may rate-limit or require approval for production-scale registration.
  This is a Swiggy-policy question, not a code blocker; `client.json` caching
  means it happens once per machine. Flag for the Builders Club review.
- **Loopback redirect port matching:** Swiggy's registered `redirect_uri` must
  match `127.0.0.1:8765/cb`. RFC 8252 allows loopback port flexibility, but if
  Swiggy pins the exact value, 8765 must stay fixed (it is, with an env
  override).
- **Keyring on headless Linux:** Secret Service may be absent over bare SSH.
  Acceptable for the PC-browser target; documented as a hard error, not a
  silent downgrade.
```
