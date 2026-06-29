# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`console.store` — a **terminal-native** food/snack ordering shop, Tokyo Night themed. Run the `store` binary with no args and you get a `bubbletea` TUI that brokers real orders through **Swiggy's Food MCP API**. Run it with a subcommand (`store status`, `store order <name>`, `store alias …`, `store help`) and it acts as a **headless CLI** — plain text, no TUI. First run does a one-time browser authorize (loopback OAuth); the token lives in the OS keyring. The whole app runs **in-process** — there is no server and no database.

> The "ssh consolestore.in" wordmark on the splash is a deliberate aesthetic (the app reads like a remote shell session). It is NOT a real SSH server — don't add one, a socket broker, or a database. Everything runs in one process.

## Commands

```bash
go run ./cmd/store                # run the TUI (live Swiggy backend). First run authorizes in the browser.
go run ./cmd/store help           # headless: usage. Also: status, order <name>, alias list|rm
go build ./...                    # build
go test ./...                     # all tests
go test ./internal/tui -run TestFlowMenuToCart   # single test
go test ./internal/cli            # the headless CLI package
go vet ./...
gofmt -w <file>                   # format
./scripts/build.sh                # gated LOCAL build of both dev binaries (see below)
```

Go 1.26. No linter config — `go vet` + `gofmt` are the bar. Stdlib only; no new dependencies without reason.

**Two LOCAL dev binaries (`scripts/build.sh`).** It gates on `go vet ./...` + `go test ./...`, then installs into `$BIN` (default `~/.local/bin`):
- **`localstore` = ARMED** — built with `-ldflags "-X console.store/internal/swiggy.liveOrdersDefault=1"`. Places REAL Swiggy orders on checkout/CLI confirm.
- **`localsafestore` = disarmed** — plain build. Browse + cart only; place-order is blocked.
- Both are stamped `Version=dev`, so they NEVER auto-update. The names are deliberately distinct from `store` so a local build never clobbers the installed, auto-updating production binary in `~/.local/bin`.
- Plain `go build` / `go run` stays disarmed. Orders are also gated by env `CONSOLE_LIVE_ORDERS=1`.

**Production binary = `store`** — the armed, auto-updating binary users install via `curl -fsSL consolestore.in/install | sh`. Built/signed/published by the release pipeline, NOT by `build.sh`. To ship a release, see **Releasing & channels** below and [`RELEASING.md`](RELEASING.md).

**NEVER place a real order** from the implementation or tests — the user does that. Tests use mock backends only and arming defaults OFF under `go test`. `place_food_order` is never auto-retried (a 5xx may mean the order placed → duplicate risk).

## Composition (what's real)

`cmd/store/main.go` is a dispatcher: no args → TUI; a subcommand → `internal/cli`. Both share one `bootstrap()` that wires the auth/broker stack in-process:

```
cmd/store/main.go     entrypoint: os.Args dispatch (TUI vs headless), shared bootstrap()
                      (OAuth registration from cached client.json, keyring token store,
                      loopback callback server, broker.Service, datasource backend).
internal/auth/        OAuth 2.1: Dynamic Client Registration + PKCE + loopback authorize
                      (127.0.0.1:8765/cb) + token refresh.
internal/localstore/  OS keyring token store (go-keyring) + cached DCR client.json +
                      active-order.json (last placed order) + presets.json (order aliases).
                      Single machine, single account keyed by LocalAccountID = "local".
internal/swiggy/      Swiggy Food + Instamart MCP client (tool calls, cart, orders, tracking;
                      429/5xx retry with backoff; arming via liveOrdersEnabled).
internal/broker/      broker.Service — composition root tying auth + keyring + swiggy
                      together. Runs in-process (NOT a socket server).
internal/cli/         the headless CLI: command router (Dispatch), `status`, `order <name>`,
                      `alias list|rm`, `help`, plain-text bill rendering. Imports broker/api
                      + localstore only — NEVER internal/tui.
internal/tui/         the whole TUI app (root model + screens + components + render + theme).
internal/tui/datasource/  the seam between the TUI and the broker: InProc adapter +
                      Backend interface + tea.Cmds (LoadAddresses/Places/Menu/Cart…).
internal/catalog/     data-seam types + Repository interface. mem/ = in-memory mock;
                      swiggy/ = live Snapshot + Repository filled from broker results.
internal/config/      optional seed config (instant first paint) + cuisine chips.
cmd/capture/          read-only dev tool: polls the tracking tools for a live order and dumps
                      raw JSON (CONSOLE_DEBUG_SWIGGY). Never places an order.
```

## Headless CLI + order presets (`internal/cli`, `internal/localstore/presets.go`)

`store <subcommand>` runs without the TUI:
- `store status` — live order status (active orders + `track_food_order` ETA), or "no live orders".
- `store order <name>` — order a saved **preset**: push it to the cart (overrides any existing cart), pull the real bill, confirm with Enter, then place (armed) / no-op (disarmed `localsafestore`). Aborts if an item is sold out or the restaurant won't serve the address. Multiple presets can share a name (you pick).
- `store alias list | rm <name> [n]` — manage presets from the shell.

A **preset** is a named cart snapshot (`presets.json`): restaurant id + saved address + lines (item SwiggyID, qty, variant/addon selections). Created **inside the TUI** via the `:alias set <name>` palette command, which captures the current cart. Presets exist because Swiggy's order API returns NO line items (`get_food_orders`/`get_food_order_details` are coarse text only) — so "reorder" is sourced from our own snapshots. Bound to one saved address (the terminal can't know GPS; the user manages region-specific presets). `cli.Backend` is structurally satisfied by `datasource.BrokerBackend`, so the CLI reuses the same account-pinned backend.

## Releasing & channels (install + auto-update)

`store` installs via `curl -fsSL consolestore.in/install | sh` and **self-updates on every launch** from its channel. Three channels: **stable** (the bare curl), **beta**, **alpha** (invite-only, per-person codes). Full details + the trust model are in [`RELEASING.md`](RELEASING.md).

**Agent release commands — when the user says "push to <channel>", cut the matching git tag (CI does the rest):**
- "push to **alpha**" → `git tag vX.Y.Z-alpha.N && git push origin vX.Y.Z-alpha.N`
- "push to **beta**" → re-tag the SAME commit `git tag vX.Y.Z-beta.N && git push origin …` (promote, don't rebuild)
- "push to **main** / **production** / **stable**" → `git tag vX.Y.Z && git push origin vX.Y.Z`

The release workflow (`.github/workflows/release.yml`) gates (vet+test) → GoReleaser cross-compiles the armed `store` → `cmd/signtool` signs the manifest envelope with `CONSOLE_SIGN_KEY` → publishes to the GitHub Release. Promotion = re-tagging the same commit up the chain (alpha→beta→stable); never rebuild to promote. **Read [`RELEASING.md`](RELEASING.md) before tagging** — it covers version-bump rules, prerequisites (signing key, landing deploy), and how to confirm a release went out. The signing private key lives ONLY in the GH secret `CONSOLE_SIGN_KEY`; never commit it. Alpha tester codes live in the Railway env `CONSOLE_ALPHA_CODES`.

Self-update internals: `internal/updater` (launch-time `RunDefault`, ed25519 signed-manifest verify against the embedded pubkey in `pubkey.go`, atomic binary swap + re-exec). The OS keyring token is never touched by the updater, so auth survives every update. Local `localstore`/`localsafestore` builds are `Version=dev` and never update.

## Architecture (the TUI)

**Single root model owns everything.** `internal/tui/app.go` defines `Model`, which holds every screen as a struct field, the active `screen` enum, the cart lines, current address/section, and `render.Caps`. `Update` is one big router: it switches on the active screen and dispatches keys. Screens are **passive value types**; the root drives them.

**Screens** (`internal/tui/screens/`) are value types with `New*(...)` constructors, chained `With*(...)` builders (return a copy), and a `.View() string`. Some expose an `Update` for sub-states like search/list cursor. They render to strings; the root composes body + command palette + status bar and pads to terminal height. The cart and checkout are **one merged page** (`screens/checkout.go`, screen `scrCheckout`); `scrCart` is a leftover enum no longer navigated to. (The standalone orders-history page was removed — Swiggy's order API has no items to show.)

**Import direction matters:** `screens` must NOT import `tui` (would cycle), and `internal/cli` must NOT import `tui`. Consequence: shared constants like the bill math (`DeliveryFee`, `CouponAmount`) are **deliberately duplicated** in `app.go` and `screens/cart.go`. Keep them in sync; don't "fix" by importing across the boundary.

**`internal/catalog` is the data seam.** Screens depend only on `catalog` types (`Address`, `Place`, `Item`, `Section`, `Usual`, option/variant types) and the `catalog.Repository` interface. `catalog/mem` is the hardcoded mock; `catalog/swiggy` is a live RW-mutex `Snapshot` + `Repository` the `datasource` Cmds fill from broker results. **All data access goes through `Repository`** — never hardcode catalog data in screens.

**`internal/tui/datasource`** is the live data layer: a `Backend` interface, an `InProc` adapter over `broker.Service`, and `tea.Cmd`s (`LoadAddresses`, `LoadPlaces*`, `LoadMenu`, `LoadCart`, `SyncCart`, `PlaceOrderCmd`, `LoadItemOptions`, `LoadActiveOrdersCmd`, `PollTrackingCmd`…) that write the `Snapshot` and return `*LoadedMsg`s the root handles. Stale async responses are guarded by identity (e.g. `MenuLoadedMsg.PlaceID` vs the open restaurant).

**Cart invariants.** The local cart never holds two restaurants. `cartForeign` marks a cart seeded at launch from a pre-existing Swiggy cart we can't attribute (the launch `get_food_cart` returns no restaurant); a cross-restaurant add raises the keep/start-fresh **conflict modal** (`screens.CartConflict`), resolved before any variant picker. The first in-app add to an empty cart clears `cartForeign`. Cart edits are optimistic and **roll back** on a failed sync. Per-item availability comes from the cart response (`update_food_cart`/`get_food_cart`), flags sold-out lines, and blocks the order.

**Order tracking.** After a real placement (or discovered on Start Screen entry), `track_food_order` is polled (~30s) for the live status + ETA — authoritative, the same data the CLI prints. The tracking page prefers the live ETA over the local time estimate; the courier sprite's road position is proportional to progress (elapsed vs live ETA remaining), not the discrete stage. Raw Swiggy statuses map to friendly phrases (`screens.StatusDisplay`/`ShortStatus`): "Arrived at location" (ETA "N/A") → "rider's outside". Swiggy exposes no rider name/phone. The splash "track order" button shows the live ETA, fetched when the Start Screen's active-order check finds an order.

**`internal/tui/render`** owns terminal capability detection and hero art. `DetectCaps(term, env, truecolor)` runs once in `main.go` and returns `Caps{Truecolor, KittyGraphics}`. Hero art renders via the Kitty graphics protocol when available, else a portable half-block/truecolor fallback. Screens take `Caps`, never inspect TERM themselves.

**`internal/tui/theme`** is the Tokyo Night palette + lipgloss styles. `theme.Bg` is the canvas color `main.go` pushes to the terminal via OSC 11 (and resets via OSC 111 on exit) so the whole viewport sits on the design background.

**Truecolor detection:** `cmd/store/main.go` `truecolor()` treats `COLORTERM=truecolor/24bit`, Windows Terminal (`WT_SESSION`), and VS Code (`TERM_PROGRAM=vscode`) as truecolor — without this, lipgloss strips the palette on Windows.

**Animation:** one `tickMsg` every 60ms (`tickInterval`) increments `Model.frame`; `onTick` advances time-based screen state (splash boot sequence, tracking polls). Frame-derived cadences (spinner, blink, rotating hints) are computed from `frame` modulo. Keep new animation on this single tick — don't add competing timers.

## Testing

- `internal/tui/flow_test.go` uses `charmbracelet/x/exp/teatest` for full key-sequence flows (send keys, assert on rendered output via `teatest.WaitFor` + `bytes.Contains`).
- Live wiring is tested against fake backends (`liveFake`/`railFake` in `internal/tui`, `fakeBackend` in `internal/cli`) — no network, no real orders.
- Screen/component/cli packages have unit tests asserting on rendered substrings. No golden files / no `-update` flag. When changing rendered copy, update the matching test strings.
- Tests that touch persistence set `t.Setenv("XDG_CONFIG_HOME", t.TempDir())` to isolate from real keyring/config state.

## Conventions

- Some comments cite a design script (`design line NNN`); that spec is gone. Treat the existing rendered copy/spacing in the code and its tests as the source of truth.
- Keep packages single-responsibility; one screen per file.
- Live-data findings worth remembering (Swiggy response shapes, arming, tracking strings) are recorded in the memory dir, not re-derived.
