# First-Run Manual + Enhanced Help — Design

**Date:** 2026-06-30
**Status:** approved, ready for implementation

## Overview

New users on a **fresh install** get a short, reading-only **manual** the first time
they launch the TUI. It is not a separate component: we **enhance the existing help
modal** (`internal/tui/screens/help.go`) into a **paginated** reference and
**auto-open it once** on first run. Returning users, and anyone updating an existing
install, never see the auto-open (a marker in the config dir gates it, and the config
dir survives binary self-updates).

An interactive guided tour is **explicitly out of scope** ("proper guide later"). This
design deliberately leaves room for it: the marker + page infrastructure are reusable.

## Goals

- A few curated pages teaching what the app is, the keybinds, and how aliases work.
- Auto-open **only** for genuinely new users on a fresh install — never on a beta/stable
  update, never for existing users.
- Reuse the existing help modal's look (gold rounded card, same chrome as every other
  modal). Same modal reachable any time via `?` / `:help`.
- Zero network, zero auth dependency, zero order risk (reading only, runs pre-auth).

## Non-Goals

- No interactive/guided tour, no mock sandbox driving the real screens, no alias-set gate.
- No new modal style. No changes to other modals.

## Components

### 1. Paginated help modal — `internal/tui/screens/help.go`

Keep the existing card chrome verbatim (rounded gold border, `BrandStyle` title,
`helpCardWidth`, centered overlay). Change the content model from one long scroll to
**discrete pages**.

API:

- `Help` gains `page int` (keep `scroll int` for within-page overflow on short terminals).
- New builder: `func (h Help) WithPage(n int) Help` (stores raw; clamp in `View`).
- `helpContent() []string` → `helpPages() [][]string` (one `[]string` per page).
- `func HelpPageCount() int` — number of pages (so the root can clamp `page`).
- `HelpMaxScroll(viewportH int)` stays, but is computed against the **current page's**
  content length. Signature may become `HelpMaxScroll(page, viewportH int)` — update all
  callers. (If simpler, keep per-page clamping inside `View` and have the root clamp page
  via `HelpPageCount()`; scroll clamping can stay inside `View`.)
- `View()`: render the clamped current page; footer shows a page indicator and controls,
  e.g. `‹ 2/5 ›   ↑↓ scroll · ← → page · esc close`. When a page fits without scroll,
  omit the `↑↓ scroll` hint (mirror the existing footer logic).

Pages (reuse the copy already in `helpContent()`, regrouped, plus a new page 1):

1. **Welcome / contents** — one line on what consolestore is; the safety line
   ("orders are real & can't be cancelled here — call Swiggy"); "→ flip · esc close";
   and a short index listing pages 2–5.
2. **Move & select** — `↑↓ k j`, `← → h l`, `↵`, `esc` / `esc esc` home, `tab`, `ctrl-c`.
3. **Browse & inside a restaurant** — `/` search, `i` info, `a` address, `c` cart;
   add `↵ +` / remove `−`, `← →` category, `v` veg only, dish info.
4. **Cart, checkout & tracking** — `↑↓` pick line, `← → + −` qty, `⌫` remove line,
   `↵` place (cash on delivery, asks first); tracking `d` dismiss / `esc` back.
5. **Aliases & the shell** — `:alias set / list / rm`, `:help`; what an alias is; the
   payoff `console order <name>` and `console status` from the shell.

Number keys `1`–`5` jump directly to a page (nice-to-have; implement if trivial).

### 2. First-run marker + detection — `internal/localstore` (new file `onboarding.go`)

Marker file lives in the config dir alongside `client.json` (reuse the existing dir;
`configPath()` resolves it — honor `XDG_CONFIG_HOME`). Marker filename: `onboarded`.

API:

- `func Onboarded() bool` — true if the marker file exists.
- `func MarkOnboarded() error` — `mkdir -p` the config dir, write the marker (mode `0600`,
  contents can be a timestamp/version string; existence is what matters).
- `func ShouldOnboard() bool` — the single decision, in this order:
  1. `CONSOLE_NO_ONBOARDING=1` → `false`.
  2. `CONSOLE_FORCE_ONBOARDING=1` → `true`.
  3. `Onboarded()` → `false`.
  4. **Grandfather:** if any prior state exists — `client.json` **or** `presets.json`
     **or** `active-order.json` in the config dir — the user has used the app before, so
     call `MarkOnboarded()` silently and return `false`.
  5. otherwise → `true` (genuinely fresh).

Notes:
- The config dir persisting across binary self-updates is what makes updates never
  re-trigger — no version/channel check needed.
- Reuse the existing path helpers (`configPath`, `presetsPath`, `orderPath`) to locate the
  files; add small `fileExists` helper if needed.

### 3. Root wiring — `internal/tui/app.go` (+ option in `live.go`), `cmd/store/main.go`

- New `Model` field: `onboardingPending bool` (true once auto-opened until the first close
  writes the marker). Optionally `wantOnboarding bool` to defer the open until the start
  screen.
- New option: `func WithOnboarding(show bool) Option` (place with the other options, e.g.
  `live.go`). When `show` is true, arrange for the help modal to auto-open **once**, over
  the **start screen after the splash settles** (not over the animating splash). Use the
  existing splash→start transition point; if the exact seam is unclear, opening it as the
  start screen is first shown is acceptable. On auto-open: `helpOpen = true`,
  help `page = 0`, `helpScroll = 0`, `onboardingPending = true`.
- Help key handling (extend the block at the current `helpOpen` handler):
  - `← →` (and `h` / `l`) change page (clamp via `HelpPageCount()`), reset `helpScroll` to 0.
  - `1`–`5` jump to that page (if implemented), reset scroll.
  - `↑ ↓` scroll within the page (existing behavior, now per-page clamp).
  - `esc` / `?` close. **If `onboardingPending`:** fire a `MarkOnboarded` `tea.Cmd`
    (fire-and-forget; log/ignore error) and clear `onboardingPending`.
- `cmd/store/main.go`: in the TUI option list, append
  `consoletui.WithOnboarding(localstore.ShouldOnboard())`. TUI path only (not headless).
- Render path is unchanged — the `helpOpen` overlay already exists.

## Data Flow

```
main.go (TUI path)
  └─ localstore.ShouldOnboard()  ──true──▶ consoletui.WithOnboarding(true)
                                             └─ Model: wantOnboarding=true
                                                  └─ splash settles → start screen
                                                       └─ helpOpen=true, page=0, onboardingPending=true
                                                            └─ user reads, ←/→ pages, esc closes
                                                                 └─ onboardingPending → MarkOnboarded() cmd
```

Subsequent launches: `ShouldOnboard()` → false (marker present) → no auto-open. `?`/`:help`
open the same paginated modal normally (no `onboardingPending`, no marker write).

## Testing

- `screens/help_test.go`:
  - `HelpPageCount()` ≥ 5; `WithPage` clamps out-of-range to a valid page.
  - `View()` on each page shows that page's expected substrings and a `n/N` page indicator.
  - Footer omits the scroll hint when a page fits.
- `localstore/onboarding_test.go` (use `t.Setenv("XDG_CONFIG_HOME", t.TempDir())`):
  - fresh dir → `ShouldOnboard()` true.
  - after `MarkOnboarded()` → `Onboarded()` true, `ShouldOnboard()` false.
  - grandfather: create a `presets.json` (or `client.json` / `active-order.json`) in the
    dir → `ShouldOnboard()` false **and** marker now written.
  - `CONSOLE_NO_ONBOARDING=1` → false even when fresh; `CONSOLE_FORCE_ONBOARDING=1` → true
    even when marked.
- `tui` flow (mirror existing `*_test.go` style, no network):
  - `WithOnboarding(true)`: after the splash→start transition the model has `helpOpen` true
    and `onboardingPending` true; closing it clears `onboardingPending` (and would write the
    marker — assert the pending flag clears / the close path is taken).
  - `WithOnboarding(false)`: help does not auto-open.
- Whole repo green: `go test ./...`, `go vet ./...`, `gofmt -l` empty on touched files.
  Keep duplicated bill/string constants in sync per CLAUDE.md; update any test strings that
  assert on the old single-scroll help layout.

## File Change List

- `internal/tui/screens/help.go` — paginate (modify).
- `internal/tui/screens/help_test.go` — page tests (modify/add).
- `internal/localstore/onboarding.go` — marker + detection (new).
- `internal/localstore/onboarding_test.go` — detection tests (new).
- `internal/tui/app.go` — `onboardingPending`, help key paging, auto-open wiring (modify).
- `internal/tui/live.go` (or wherever `Option` lives) — `WithOnboarding` (modify).
- `cmd/store/main.go` — pass `WithOnboarding(localstore.ShouldOnboard())` (modify).
- `internal/tui/*_test.go` — onboarding flow test + fix any help-layout assertions (modify).

## Out of Scope (future)

- Interactive guided tour on mock data with an alias-set completion gate. Replay via
  `:tutorial`. The marker (`ShouldOnboard`/`MarkOnboarded`) and the page infrastructure
  built here are intended to be reused by it.
