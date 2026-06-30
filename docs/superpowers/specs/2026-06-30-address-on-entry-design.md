# Address Picker on Entering Shop — Design

**Date:** 2026-06-30
**Status:** approved, ready for implementation

## Overview

Every time the user enters the shop (splash → "Enter Shop"), force them to pick a
delivery address before the shop loads, instead of silently auto-selecting the first
address. The choice persists for the whole session (in memory) and is asked again on
the next launch (no disk persistence). Reuses the existing address switcher modal.

## Decisions (approved)

- **Forced gate, reusing the existing switcher.** Non-dismissible: only Enter picks;
  esc / `a` do nothing. Home loads only after a pick.
- **Ordering:** a reading overlay (onboarding manual / "what's new") shows FIRST; the
  address gate opens when that overlay closes. Returning users (no overlay) hit the
  gate directly.
- **Single address → auto-use** it and skip the picker. Prompt only when 2+ exist.
- **Session-scoped, ask every launch:** `m.addr` is in memory only; no persistence —
  so a process restart naturally re-asks.

## Current behavior (to change)

- `internal/tui/app.go` `AddressesLoadedMsg` handler auto-picks `addrs[0]` and
  immediately loads Home (`ensureHomeLoaded` + `PullCart` + active-order check).
- Address switcher modal already exists: `addrOpen` + `m.addrScreen` (`screens.Address`,
  cursor + `Selected()`), opened with `a`, dismissible with esc/`a`, Enter picks +
  reloads Home if the address changed.
- Onboarding/"what's new" overlays auto-open at the splash→`scrMenu` transition.

## Components

### State (app.go Model)
- `addrGatePending bool` — set true at launch (in `New`); cleared once the gate is
  satisfied. Drives the forced pick.
- `addressesLoaded bool` — set true in `AddressesLoadedMsg`.
- `addrForced bool` — true while the address modal is the forced entry gate (vs the
  user-invoked `a` switcher), making it non-dismissible + routing Enter to the
  load-Home path.

### `maybeOpenAddrGate()` helper (app.go)
Called whenever the relevant state may have changed: at the splash→`scrMenu`
transition, in `AddressesLoadedMsg`, and after a reading overlay closes. Returns a
`tea.Cmd` (may be nil). Logic — only proceed when **all** hold:
`addrGatePending && m.screen == scrMenu && addressesLoaded && !anyEntryOverlayOpen()`
where `anyEntryOverlayOpen() = helpOpen || whatsnewOpen`.

Then by address count (`m.repo.Addresses()`):
- **0** → clear `addrGatePending` (nothing to pick); leave existing empty-state behavior.
- **1** → `m.addr = addrs[0]`; clear `addrGatePending`; return `loadHomeForCurrentAddr()`.
- **2+** → build `m.addrScreen` (cursor 0), set `m.addrOpen = true`, `m.addrForced = true`;
  return nil (wait for the pick — do NOT load Home yet).

Extract the existing Home-load trio into a helper
`loadHomeForCurrentAddr() tea.Cmd` = `tea.Batch(ensureHomeLoaded, PullCart(addr.ID),
activeOrderCheckCmd)` plus the `usualsLoaded=false` reset, reused by the
single-address path and the forced-pick Enter path.

### `AddressesLoadedMsg` change
- Set `addressesLoaded = true`.
- If `addrGatePending`: do NOT auto-pick `addrs[0]` and do NOT auto-load Home. Instead
  `return m, m.maybeOpenAddrGate()` (opens the gate now if conditions are met, else
  waits for an overlay to close). The seeded first-paint address is ignored for the
  gate — the forced pick still happens on live load when 2+ addresses exist.
- If not pending (defensive): keep the old behavior.

### Forced gate key handling (app.go, the `addrOpen` block)
When `m.addrForced`:
- `ctrl+c` → quit.
- `esc` / `a` → ignored (stay open — it's a hard gate).
- `enter` → `m.addr = m.addrScreen.Selected()`; `m.addrOpen = false`;
  `m.addrForced = false`; `m.addrGatePending = false`;
  return `loadHomeForCurrentAddr()` (always load on the first gate pick — no
  "addr changed" check). Also set `m.screen = scrMenu`, `railActive = RailHome`,
  `railFocus = true`, `searchMode = false` (mirror the existing switcher's Enter).
- default → forward to `m.addrScreen.Update` (cursor move).
When NOT forced, the existing dismissible switcher behavior is unchanged.

### Overlay-close → open the gate
At the help-close path (esc/?/q/enter/space) and the whatsnew-close path, after
closing, if `addrGatePending`, fold `maybeOpenAddrGate()` into the returned cmd
(`tea.Batch` with the existing marker/version cmd). So: overlay closes → gate opens.

### Splash→`scrMenu` transition
After setting `m.screen = scrMenu` and handling the onboarding/notes auto-open: if
NO entry overlay was opened, `return m, m.maybeOpenAddrGate()` (opens the gate if
addresses are already loaded, else it opens later via `AddressesLoadedMsg`).
`addrGatePending` is already true (set in `New`).

### Modal polish (screens/address.go)
When rendered as the forced gate, the footer should not advertise "esc close"
(esc is a no-op). Add a builder like `WithForced(bool)` (or pass a flag) so the
footer shows only "↵ choose your address". Keep the normal switcher footer
unchanged. (Small; optional if it complicates — but preferred so the UI isn't
misleading.)

## Edge cases

- **needs-auth:** addresses load post-auth; the gate simply waits — `addressesLoaded`
  stays false until `AddressesLoadedMsg`, which then calls `maybeOpenAddrGate()`.
- **Seeded snapshot:** instant-paint address is ignored for the gate; forced pick
  happens on live address load (2+).
- **Headless CLI (`console order`/`status`):** unaffected — no TUI, uses preset addresses.

## Testing (internal/tui, no network)

- 2+ addresses, no overlay: after the splash→menu transition + `AddressesLoadedMsg`,
  `addrOpen && addrForced` are true and Home is NOT loaded; esc and `a` keep it open;
  Enter sets `m.addr` to the selected one, clears `addrForced`/`addrGatePending`, and
  returns a non-nil Home-load cmd.
- 1 address: auto-used (`m.addr` set), no `addrOpen`, Home-load cmd returned.
- Overlay-first: with onboarding armed (`WithOnboarding(true)`), entering the shop
  opens help (not the addr gate); closing help opens the forced addr gate.
- Returning user (no overlay): entering shop + addresses loaded → forced gate opens.
- Session: after a pick, `addrGatePending` is false; opening/closing help or the `a`
  switcher again does NOT re-open the forced gate.
- Whole repo green: `go test ./...`, `go vet ./...`, `gofmt -l` clean.

## File Change List

- `internal/tui/app.go` — gate state, `maybeOpenAddrGate`, `loadHomeForCurrentAddr`,
  `AddressesLoadedMsg` change, forced-gate keys, overlay-close hook, splash→menu hook (modify).
- `internal/tui/screens/address.go` — forced-mode footer (modify, small).
- `internal/tui/*_test.go` — gate flow test (new/modify).

## Out of Scope

- Persisting the chosen address across launches (the requirement is to ask every
  launch). Remembering a per-session default beyond `m.addr`.
