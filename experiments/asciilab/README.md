# asciilab

Sandbox of terminal animation experiments for the consolestore TUI. Own Go
module — root builds, tests, and CI never see it.

```bash
cd experiments/asciilab
go run .                      # interactive: ←/→ or 1-9 to switch, space pause, r reset, q quit
go run . -list                # list demos
go run . -demo fire           # start on a demo
go run . -demo steam -snap 90 -w 80 -h 20   # headless: print one frame (no tty needed)
go run . -tick 33ms           # smoother than the app's 60ms tick
```

## Demos → integration targets

| demo     | technique                                        | where it could land |
|----------|--------------------------------------------------|---------------------|
| spinners | glyph-cycle spinners, eighth-block + gradient + indeterminate bars | loading states, cart "syncing" chip |
| boot     | typewriter + spinner→status script, frame-deterministic | splash boot sequence |
| logo     | gradient sweep, letter bob, glitch flicker       | splash wordmark |
| steam    | particle system with sine drift + age fade       | first-run ramen animation, empty cart |
| rider    | parallax layers, sprite bob, scrolling road      | tracking-page courier |
| confetti | burst particles with gravity + banner            | order-placed moment |
| rain     | per-column drops, head highlight, tail fade      | idle/auth-wait background |
| fire     | doom-fire automaton on half-block framebuffer    | spicy accents, drama |
| plasma   | sine interference over palette gradient          | hero background ambience |
| donut    | donut.c torus, z-buffer, luminance glyph ramp    | fun loading state |

## Building blocks (the reusable part)

- `theme.go` — Tokyo Night palette as raw RGB + `lerp`/`gradient` and truecolor
  ANSI helpers. Mirrors `internal/tui/theme` colors.
- `surface.go` — two render surfaces:
  - `Grid`: rune + fg color per cell, painted on the theme background.
  - `FB`: half-block framebuffer (the [timg](https://github.com/hzeller/timg)
    technique) — each cell is `▀` with fg = top pixel, bg = bottom pixel, so a
    W×H cell area gives W×2H pixels.
- `demo.go` — tiny `Demo` interface (`Init/Step/View`). Frame-driven, no
  internal timers, so everything ports straight onto the app's single 60ms
  `tickMsg` (state advanced from `Model.frame`, exactly the app convention).

## Porting notes

- Demos that are pure functions of the frame number (`boot`, `logo`, `plasma`,
  `donut`, `spinners`) drop into a screen `View` with zero state.
- Particle demos (`steam`, `rain`, `confetti`, `fire`, `rider` stars) carry a
  small state struct — advance it in `onTick`, matching how splash/tracking
  animation already works.
- Half-block demos assume truecolor; gate on `render.Caps.Truecolor` and skip
  (or fall back to `Comment`-toned glyphs) otherwise.
- Everything here avoids emoji/wide runes on purpose — width-1 glyphs only, so
  no runewidth surprises.
