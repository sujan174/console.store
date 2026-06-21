# Loading screen: glitch-decode wordmark

**Date:** 2026-06-22
**Branch:** `feat/loading-decode`
**Status:** approved design

## Problem

The splash boot phase streams fake terminal logs:

```
guest@laptop ~ % ssh console.store
  ⊙ resolving console.store … 12.4ms
  ⊙ tls handshake … ed25519 ✓
  ⊙ auth guest@hsr-layout … ok
  ⊙ 247 devs online · kitchen warm ☕
```

These read as cheap and generic — the wrong first impression for a terminal-native
shop that wants to feel special. Replace them with a stunning, on-brand loading
moment.

## Decision summary

| Question | Answer |
|----------|--------|
| Concept | Kinetic ASCII reveal of the wordmark |
| Motion | Glitch / decode — random glyphs resolve into the real wordmark, column-by-column |
| Pacing | Snappy, ~0.8s, then settle |
| After decode | Auto-settle to the existing home landing (cup + steam + `go to shop`), which holds for a keypress |
| Implementation | Pure frame→string function (deterministic, testable) |

The fake log lines are deleted entirely.

## Architecture

### Component: `internal/tui/render/decode.go`

One pure, stateless function:

```go
// DecodeWordmark renders the block wordmark mid-decode. step is decode progress
// (0..DecodeSteps); frame is the global animation tick (drives glitch shimmer).
// At step==DecodeSteps it returns the settled wordmark (identical to the resolved
// portion of render.Logo for the text paths).
func DecodeWordmark(caps Caps, step, frame int) string
```

```go
// DecodeSteps is the number of ticks the decode runs (~0.8s at the 60ms tick).
const DecodeSteps = 13
```

Rationale for a pure function over a stateful animator: at 0.8s a clean
left→right resolve front reads as intentional, while per-cell chaos tends to look
like noise. Statelessness keeps it on the existing single-tick model and makes it
trivially unit-testable.

### Decode algorithm

Source shape is the existing `asciiLogo` (6 lines of block art, the same shape
every fidelity tier uses). Let `W` be the wordmark display width.

- **Resolve front:** `resolved = step * W / DecodeSteps` columns are locked.
- **Per glyph cell `(x, y)`:**
  - `x < resolved` → the real glyph, in its gradient color (truecolor) or flat
    (non-truecolor).
  - `x == resolved` (the 1-column leading edge) → bright cyan `#7aa2f7` "render
    head" pop.
  - `x > resolved` → if the source cell is non-space, a glitch glyph from the
    charset `01<>/\{}[]#%&$*+=`, chosen as `charset[(x*31 + y*7 + frame) % len]`
    so it shimmers per frame yet stays deterministic for a fixed frame. Source
    spaces stay spaces (the wordmark silhouette is always visible).
- **Lock:** at `step == DecodeSteps`, render the clean gradient wordmark; the app
  holds two frames of a brighter glow pulse, then settles.

Color reuses the existing gradient: each of the 6 lines has one interpolated
blue→purple hex (as in `GradientText`). Resolved glyphs use that line's hex;
glitch glyphs render dim; the leading edge is cyan.

### Degradation

- The decode is a glyph effect, so it works on flat 256/no-color terminals
  (glitch + resolve still read; just no gradient).
- Truecolor adds the blue→purple gradient and the glow pulse.
- The Kitty-graphics PNG path (`caps.KittyGraphics && KittyFlag`) **skips the
  decode** and settles straight to the bloom logo — a rasterized bitmap can't be
  glyph-decoded. `DecodeWordmark` returns `render.Logo(caps, 64)` (the same call
  the settled splash uses) unchanged in that case, so the splash boot branch
  needs no special casing.

## Integration

### `internal/tui/app.go`

- Rename `bootStep` → `decodeStep`.
- `onTick`: while on the splash and `decodeStep < render.DecodeSteps`, increment
  **every** frame (not every 6th). No auto-advance to the menu — the splash
  remains a landing that holds for a keypress (current behavior).
- Splash key handler: a key while `decodeStep < DecodeSteps` **skips** the decode
  (set `decodeStep = DecodeSteps`) rather than jumping to the menu. Once settled,
  the existing up/down nav + activate logic applies.
- `View`: pass `decodeStep` and `frame` through to the splash.

### `internal/tui/screens/splash.go`

- Delete `bootLines`, `BootLineCount`, and `Taglines` (the fake-log machinery).
- Boot branch renders `render.DecodeWordmark(s.caps, s.decodeStep, s.frame)`,
  centered, with the section subtitle below; replaces the streamed lines.
- Settled home (cup + steam + `go to shop` button, holds for keypress) is
  unchanged.
- Add a `decodeStep` field and `WithDecode(step)` builder (mirrors `WithFrame`).

## Testing

- `render/decode_test.go`:
  - `step == 0` → output contains glitch chars, correct line count/width, silhouette spaces preserved.
  - `step == DecodeSteps` → equals the settled wordmark; contains the block glyphs; no glitch chars.
  - mid `step` → left columns are real glyphs, right columns are glitch (assert a known resolved column vs an unresolved one).
  - deterministic: same `(step, frame)` → identical output across calls.
  - Kitty path: returns `render.Logo` unchanged.
- `screens/splash_test.go`: decode phase shows partial wordmark; settled phase still shows `go to shop` and the gold `.store`.
- `internal/tui/app_test.go`: after `DecodeSteps` ticks the splash holds (does not auto-advance); a key during decode skips to the settled home; `enter` on settled home → menu.

## Out of scope

- Sound, real network/load measurement, per-cell organic scatter, configurable
  duration. The glitch charset and `DecodeSteps` are constants, tunable later.
