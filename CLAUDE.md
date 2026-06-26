# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`console.store` — a **terminal-native CLI** food/snack ordering shop, Tokyo Night themed. You run the `store` binary, get a `bubbletea` TUI that brokers real orders through **Swiggy's Food MCP API**. First run does a one-time browser authorize (loopback OAuth); the token lives in the OS keyring. There is no SSH server and no database — the app runs in-process.

> History: this was once an SSH-served TUI with a separate privileged broker + Postgres token store. That era is gone (`cmd/sshd`, `cmd/broker`, `internal/store`, the Makefile/docker-compose were removed). Don't reintroduce them.

## Commands

```bash
go run ./cmd/store                # run the TUI (live Swiggy backend). First run authorizes in the browser.
go build ./...                    # build
go test ./...                     # all tests
go test ./internal/tui -run TestFlowMenuToCart   # single test
go test ./internal/tui/screens -run TestCart     # single package/test
go vet ./...
gofmt -w <file>                   # format
./scripts/build.sh                # gated build of both binaries (see below)
```

Go 1.26. No linter config — `go vet` + `gofmt` are the bar.

**Two binaries (`scripts/build.sh`).** It gates on `go vet ./...` + `go test ./...`, then installs into `$BIN` (default `~/.local/bin`):
- **`store` = ARMED** — built with `-ldflags "-X console.store/internal/swiggy.liveOrdersDefault=1"`. Places REAL Swiggy orders on checkout confirm.
- **`safestore` = disarmed** — plain build. Browse + cart only; place-order is blocked.
- Plain `go build` / `go run` stays disarmed. Orders are also gated by env `CONSOLE_LIVE_ORDERS=1`.

**NEVER place a real order** from the implementation or tests — the user does that. Tests use mock backends only.

## Composition (what's real)

`cmd/store/main.go` wires the whole app in-process:

```
cmd/store/main.go     entrypoint: resolve OAuth registration (cached client.json),
                      keyring token store, loopback callback server, broker.Service,
                      datasource.InProc adapter, then runs the bubbletea program.
internal/auth/        OAuth 2.1: Dynamic Client Registration + PKCE + loopback authorize
                      (127.0.0.1:8765/cb) + token refresh.
internal/localstore/  OS keyring token store (go-keyring) + cached DCR client.json.
                      Single machine, single account keyed by LocalAccountID = "local".
internal/swiggy/      Swiggy Food + Instamart MCP client (tool calls, cart, orders).
internal/broker/      broker.Service — composition root tying auth + keyring + swiggy
                      together. Runs in-process (NOT a socket server anymore).
internal/tui/         the whole app (root model + screens + components + render + theme).
internal/tui/datasource/  the seam between the TUI and the broker: InProc adapter +
                      Backend interface + tea.Cmds (LoadAddresses/Places/Menu/Cart…).
internal/catalog/     data-seam types + Repository interface. mem/ = in-memory mock;
                      swiggy/ = live Snapshot + Repository filled from broker results.
internal/config/      optional seed config (instant first paint) + cuisine chips.
```

## Architecture (the TUI)

**Single root model owns everything.** `internal/tui/app.go` defines `Model`, which holds every screen as a struct field, the active `screen` enum, the cart lines, current address/section, and `render.Caps`. `Update` is one big router: it switches on the active screen and dispatches keys. Screens are **passive value types**; the root drives them.

**Screens** (`internal/tui/screens/`) are value types with `New*(...)` constructors, chained `With*(...)` builders (return a copy), and a `.View() string`. Some expose an `Update` for sub-states like search/list cursor. They render to strings; the root composes body + command palette + status bar and pads to terminal height. The cart and checkout are **one merged page** (`screens/checkout.go`, screen `scrCheckout`); `scrCart` is a leftover enum no longer navigated to.

**Import direction matters:** `screens` must NOT import `tui` (would cycle). Consequence: shared constants like the bill math (`DeliveryFee`, `CouponAmount`) are **deliberately duplicated** in `app.go` and `screens/cart.go`. Keep them in sync; don't "fix" by importing across the boundary.

**`internal/catalog` is the data seam.** Screens depend only on `catalog` types (`Address`, `Place`, `Item`, `Section`, `Usual`, option/variant types) and the `catalog.Repository` interface. `catalog/mem` is the hardcoded mock; `catalog/swiggy` is a live RW-mutex `Snapshot` + `Repository` the `datasource` Cmds fill from broker results. **All data access goes through `Repository`** — never hardcode catalog data in screens.

**`internal/tui/datasource`** is the live data layer: a `Backend` interface, an `InProc` adapter over `broker.Service`, and `tea.Cmd`s (`LoadAddresses`, `LoadPlaces*`, `LoadMenu`, `LoadCart`, `SyncCart`, `PlaceOrderCmd`, `LoadItemOptions`…) that write the `Snapshot` and return `*LoadedMsg`s the root handles. Stale async responses are guarded by identity (e.g. `MenuLoadedMsg.PlaceID` vs the open restaurant).

**`internal/tui/render`** owns terminal capability detection and hero art. `DetectCaps(term, env, truecolor)` runs once in `main.go` and returns `Caps{Truecolor, KittyGraphics}`. Hero art renders via the Kitty graphics protocol when available, else a portable half-block/truecolor fallback. Screens take `Caps`, never inspect TERM themselves.

**`internal/tui/theme`** is the Tokyo Night palette + lipgloss styles. `theme.Bg` is the canvas color `main.go` pushes to the terminal via OSC 11 (and resets via OSC 111 on exit) so the whole viewport sits on the design background.

**Truecolor detection:** `cmd/store/main.go` `truecolor()` treats `COLORTERM=truecolor/24bit`, Windows Terminal (`WT_SESSION`), and VS Code (`TERM_PROGRAM=vscode`) as truecolor — without this, lipgloss strips the palette on Windows.

**Animation:** one `tickMsg` every 60ms (`tickInterval`) increments `Model.frame`; `onTick` advances time-based screen state (splash boot sequence, tracking steps). Frame-derived cadences (spinner, blink, rotating hints) are computed from `frame` modulo. Keep new animation on this single tick — don't add competing timers.

## Testing

- `internal/tui/flow_test.go` uses `charmbracelet/x/exp/teatest` for full key-sequence flows (send keys, assert on rendered output via `teatest.WaitFor` + `bytes.Contains`).
- Live wiring is tested against fake backends (`liveFake`/`railFake` in `internal/tui`) — no network, no real orders.
- Screen/component packages have unit tests asserting on `.View()` substrings. No golden files / no `-update` flag. When changing rendered copy, update the matching test strings.

## Conventions

- Some comments still cite a design script (`design line NNN`); that spec is gone. Treat the existing rendered copy/spacing in the code and its tests as the source of truth.
- Keep packages single-responsibility; one screen per file.
