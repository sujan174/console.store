# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`console.store` — a terminal-native (SSH) food/snack ordering shop, Tokyo Night themed. Users `ssh console.store`, get a `bubbletea` TUI served over SSH by `charmbracelet/wish`. Long-term plan: broker orders through Swiggy's MCP APIs. **Today only the TUI + a mock data layer exist** — there is no live Swiggy/auth/DB backend yet.

## Commands

```bash
go run ./cmd/sshd                 # start SSH server on 127.0.0.1:2222
ssh localhost -p 2222             # connect (first run generates .ssh/console_host_key)
go build ./...                    # build
go test ./...                     # all tests
go test ./internal/tui -run TestFlowMenuToCart   # single test
go test ./internal/tui/screens -run TestCart     # single package/test
go vet ./...
gofmt -w <file>                   # format
```

Go 1.26. No Makefile, no linter config — `go vet` + `gofmt` are the bar.

## Docs — what's there now

`docs/` holds **only the Swiggy Builders Club application package** — the program console.store will broker orders through. It is design intent for the *integration/approval*, not a map of the code:

- `docs/README.md` — concept/use-case + index
- `docs/security.md` — the detailed security plan (delegated auth, token storage, order integrity); the core artifact Swiggy reviews
- `docs/builders-application.md` — submission details (servers, scope, redirect URI, demo run-sheet, open items)

**The planned backend** (OAuth broker, account/session services, Swiggy MCP client, encrypted token store; packages `internal/{auth,swiggy,account,session,store}` plus `cmd/broker`) **does not exist** — `docs/security.md` frames it as commitments/design, built later at the staging gate. Today only the TUI + in-memory mock exist:

```
cmd/sshd/main.go        wish SSH server; binds lipgloss color profile + OSC 11 bg to each session
internal/tui/           the whole app (root model + screens + components + render + theme)
internal/catalog/       the data seam — interface + in-memory mock (mem/) with curated seed data
```

When the docs and the code disagree, the code wins. Don't scaffold the planned backend unless asked.

## Architecture (what's real)

**Single root model owns everything.** `internal/tui/app.go` defines `Model`, which holds every screen as a struct field, the active `screen` enum, the cart lines, current address/section, and `render.Caps`. `Update` is one big router: it switches on the active screen and dispatches keys. This is **not** the "each screen is its own `tea.Model` returning `tea.Cmd`s" pattern the docs describe — screens are passive value types, the root drives them.

**Screens** (`internal/tui/screens/`) are value types with `New*(...)` constructors, chained `With*(...)` builders (return a copy), and a `.View() string`. Some expose an `Update` for sub-states like search/list cursor. They render to strings; the root composes body + command palette + status bar and pads to terminal height.

**Import direction matters:** `screens` must NOT import `tui` (would cycle). Consequence: shared constants like the bill math (`DeliveryFee`, `CouponAmount`) are **deliberately duplicated** in `app.go` and `screens/cart.go`. Keep them in sync; don't try to "fix" by importing across the boundary.

**`internal/catalog` is the data seam.** Screens depend only on `catalog` types (`Address`, `Place`, `Item`, `Section`, `Usual`) and the `catalog.Repository` interface. `catalog/mem` implements it with hardcoded curated data (per-address serviceability via `ServesAddressIDs`). The schema is already DB/Swiggy-shaped (`SwiggyID`, `Lat/Lng` fields) so a Postgres+Swiggy impl can later fill the same interface with zero screen changes. **All data access goes through `Repository`** — never hardcode catalog data in screens.

**`internal/tui/render`** owns terminal capability detection and hero art. `DetectCaps(term, env, truecolor)` runs once per session in `main.go` and returns `Caps{Truecolor, KittyGraphics}`. Hero art (splash wordmark, confirm art) renders via the Kitty graphics protocol when available, else a portable half-block/truecolor fallback. Screens take `Caps`, never inspect TERM themselves.

**`internal/tui/theme`** is the Tokyo Night palette + lipgloss styles. `theme.Bg` is the canvas color `main.go` pushes to the client via OSC 11 (and resets via OSC 111 on disconnect) so the whole viewport sits on the design background without per-line banding.

**Animation:** one `tickMsg` every 60ms (`tickInterval`) increments `Model.frame`; `onTick` advances time-based screen state (splash boot sequence, tracking steps). Frame-derived cadences (spinner, blink, rotating status hints) are computed from `frame` modulo. Keep new animation on this single tick — don't add competing timers.

**Color over SSH gotcha:** the server has no TTY, so lipgloss defaults to no-color and strips the palette. `main.go` fixes this by `lipgloss.SetColorProfile(renderer.ColorProfile())` from the wish session renderer. Preserve this when touching the SSH wiring.

## Testing

- `internal/tui/flow_test.go` uses `charmbracelet/x/exp/teatest` for full key-sequence flows (send keys, assert on rendered output via `teatest.WaitFor` + `bytes.Contains`).
- Screen/component packages have unit tests asserting on `.View()` substrings.
- No golden files / no `-update` flag — assertions are inline substring checks. When changing rendered copy, update the matching test strings.

## Conventions

- Comments frequently cite the design script (`design line NNN`). That design spec has been removed from the repo; treat the existing rendered copy/spacing in the code and its tests as the source of truth — don't chase the deleted line numbers.
- Keep packages single-responsibility; split a file when it does too much (existing pattern: one screen per file).
- `docs/` now covers only the Builders Club application package (see *Docs* above); there is no in-repo TUI design spec anymore.
