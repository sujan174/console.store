# Splash Redesign — Coffee Brewing Animation

**Date:** 2026-06-21
**Goal:** Replace the current splash/startup (streaming SSH boot-log → big block-art wordmark) with one clean screen: a brewing-coffee ASCII animation and a smaller `console.store` wordmark.

## Context

Today `internal/tui/screens/splash.go` streams a 5-line boot log, then renders the 6-row block-art `CONSOLE` wordmark (via `render.Logo`) + tagline + "press any key". Boot timing lives in `app.go onTick` (`bootStep`/`bootHold`/`frame`); the splash is centered by the root `View` via `lipgloss.Place`. `render.Logo` and the whole hero-art subsystem (`hero.go`, `glow.go`, `kitty.go`, `halfblock.go`, `bitmap.go`) are used **only** by the splash.

## Decisions (locked in brainstorming)

1. **Animation concept:** cup fills with coffee, then steam rises (Concept A).
2. **One screen:** no boot log. Coffee mug + a smaller `console.store` wordmark.
3. **Wordmark:** clean styled single-line text `console.store` (lowercase, gold `.store` accent, blue→purple tint on truecolor). **Not** ASCII block art.
4. **Hero-art code:** delete the orphaned subsystem (`hero.go`, `glow.go`, `kitty.go`, `halfblock.go`, `bitmap.go` + tests). Simplify `render.Caps` to `Truecolor` only.

## The screen

Centered block (root `View` already centers via `lipgloss.Place`):

```
       ) ( )           ← steam (animated, brewed phase only)
       ( ) (
      .------.
      |▓▓▓▓▓▓|)        ← mug; ▓ coffee fills bottom-up during brew
      |▓▓▓▓▓▓|
      '------'

       console.store   ← styled text wordmark, gold .store
     coffee · food · snacks

      press any key to connect ▋   ← appears once brewed; blinks
```

### Animation phases (driven by the existing 60ms tick)

**Fill** — mug interior is 3 rows. Coffee fills bottom-up one row at a time, ~8 frames per row (~1.6s total). 4 visual states (empty → 3 rows filled):

```
 .------.     .------.     .------.     .------.
 |      |)    |      |)    |      |)    |▓▓▓▓▓▓|)
 |      |     |      |     |▓▓▓▓▓▓|     |▓▓▓▓▓▓|
 |      |     |▓▓▓▓▓▓|     |▓▓▓▓▓▓|     |▓▓▓▓▓▓|
 '------'     '------'     '------'     '------'
```

**Brewed** — once full: steam wisps cycle (3 frames), the rotating brew caption settles, and the "press any key to connect ▋" prompt appears (cursor blinks on the existing blink cadence).

```
   ) (          ( )         ) ( )
  ( )          ) (          ( ) (
```

### Timing & transition

- Fill runs to completion (~1.6s), then a short hold (~1s) on the brewed frame, then auto-advance to `scrMenu` — mirrors today's auto-advance behavior.
- **Any key advances to the menu at any time** (preserves `flow_test.go`, which sends `x` to leave the splash).

### Color

| Element | Truecolor | Fallback |
|---|---|---|
| Coffee `▓` | coffee brown (new `theme` constant) | flat |
| Mug outline | `theme.Dim`/`theme.Text` | same |
| Steam | `theme.Faint` | same |
| `console` | blue→purple tint | flat `theme.Text` |
| `.store` | `theme.Gold` | `theme.Gold` |

## Code surface

- **`screens/splash.go`** — rewrite. Remove `bootLines`, `BootLineCount`, boot streaming, `render.Logo` call, `logoCache`. Add small frame builders (mug at a fill level, steam at a frame). Replace `WithBoot(step, spin, tagline)` with `WithBrew(level int, steamFrame int, showPrompt, blink bool)` (+ keep a rotating caption). Keep `caps render.Caps` only for the truecolor wordmark tint. If art helpers grow, split into `splash_art.go`.
- **`app.go`** — replace boot logic in `onTick` (`bootStep`/`bootHold`) with brew logic: a brew tick that advances fill, then holds, then sets `scrMenu`. Update the `View` splash branch to call `WithBrew`. Drop now-unused fields.
- **`theme`** — add a coffee-brown constant; everything else reused.
- **Delete:** `render/hero.go`, `render/glow.go`, `render/kitty.go`, `render/halfblock.go`, `render/bitmap.go` and their `_test.go` files. Simplify `render/caps.go`: `Caps{Truecolor bool}` (drop `KittyGraphics`); remove `KittyFlag`. `DetectCaps` keeps its signature but only sets `Truecolor`; `main.go` call site unchanged.
- **Tests:** rewrite `splash_test.go` substring assertions for the new content (mug glyphs, `console.store`, prompt, brewed state). `flow_test.go` unchanged.
- **Docs:** update `CLAUDE.md` (the `render` paragraph claims Kitty-graphics/hero-art) to reflect the simplified `render` package after deletion.

## Out of scope

- No change to any other screen, the command palette, or status bar.
- No change to the tick interval or the global animation model (one `tickMsg`/60ms).
- Confirm-order "hero art" (a planned future use of the deleted subsystem) is **not** built here; if wanted later it's a fresh, smaller implementation.

## Success criteria

- Splash shows the mug filling, then steam + `console.store` + prompt, on one screen, no boot log.
- Any key and the auto-advance both reach the menu.
- `go test ./...`, `go vet ./...` green; no references to the deleted render files remain.
