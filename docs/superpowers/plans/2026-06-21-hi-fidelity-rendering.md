# Hi-Fidelity Rendering Layer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Push the TUI's visual fidelity toward the web design (glow, crisp letterforms, smooth motion) using **progressive enhancement** — a portable truecolor half-block path that works on any modern terminal, with a Kitty-graphics path layered on top for terminals that support it.

**Architecture:** A new `internal/tui/render` package owns capability detection and three rendering backends for "hero art" (the splash wordmark, the order-confirmed art). At render time the splash asks the package for the best available representation: Kitty image (real gaussian-blur glow) if the terminal supports it AND the feature flag is on, else a truecolor half-block render (2× vertical resolution, per-pixel gradient, coarse glow halo), else the existing box-drawing ASCII. The screens depend only on `render` — never on terminal specifics. Motion is tightened separately (faster tick, eased tracking).

**Tech Stack:** Go 1.26, bubbletea/lipgloss, `golang.org/x/image/font/basicfont` (deterministic bitmap font for crisp letterforms), stdlib `image`/`image/png`/`encoding/base64` (Kitty PNG payloads). No other new deps; the glow blur is a hand-rolled separable box blur.

**Why this order:** Phase 1–2 deliver the wide-range win (half-block crisp logo + glow halo) that works everywhere and is fully testable headless. Phase 3 (Kitty) is isolated and **flag-default-off** because Kitty APC sequences can be mangled by bubbletea's diff renderer — it must be verified on a real Kitty/Ghostty client before enabling, and the half-block path is the guaranteed fallback if it can't be. Phase 4 (motion) and Phase 5 (confirmed art) are independent polish.

---

## File Structure

| File | Responsibility |
|------|----------------|
| `internal/tui/render/caps.go` | `Caps` struct + `DetectCaps(term string, env []string, truecolor bool)` — classify terminal into truecolor / kitty-graphics |
| `internal/tui/render/bitmap.go` | `Bitmap` 1-bit grid + `Wordmark(text string) Bitmap` (rasterize via basicfont) |
| `internal/tui/render/halfblock.go` | `HalfBlock(bm Bitmap, opt HalfBlockOpts) string` — render bitmap to `▀` cells, truecolor fg/bg, vertical gradient, coarse glow halo |
| `internal/tui/render/glow.go` | `GlowImage(bm Bitmap, tint color.RGBA, scale int) image.Image` — scaled raster + box-blur halo + sharp overlay (Kitty payload source) |
| `internal/tui/render/kitty.go` | `KittyImage(img image.Image, cols, rows int) string` — PNG → base64 → chunked APC; `KittyFlag` default-off gate |
| `internal/tui/render/hero.go` | `Logo(caps Caps, w int) string` — the single entry point screens call; picks backend |
| `cmd/sshd/main.go` | Build `render.Caps` from the ssh session, pass into `consoletui.New(caps)` |
| `internal/tui/app.go` | `New(caps render.Caps)`; store `caps`; thread into splash; Phase 4 tick interval |
| `internal/tui/screens/splash.go` | `Splash` carries `caps`; renders the logo via `render.Logo` |
| `internal/tui/screens/checkout.go` | Phase 5: confirmed coffee-cup art via the render package |

Tests sit beside each file (`*_test.go`). All tests are headless and deterministic — basicfont raster output and the backends are pure functions of their inputs.

---

## Task 1: Capability detection

**Files:**
- Create: `internal/tui/render/caps.go`
- Test: `internal/tui/render/caps_test.go`

- [ ] **Step 1: Write the failing test**

```go
package render

import "testing"

func TestDetectCaps(t *testing.T) {
	cases := []struct {
		name      string
		term      string
		env       []string
		truecolor bool
		wantTC    bool
		wantKitty bool
	}{
		{"ghostty", "xterm-ghostty", nil, true, true, true},
		{"kitty", "xterm-kitty", nil, true, true, true},
		{"wezterm via env", "xterm-256color", []string{"TERM_PROGRAM=WezTerm"}, true, true, true},
		{"iterm truecolor no kitty", "xterm-256color", []string{"TERM_PROGRAM=iTerm.app"}, true, true, false},
		{"plain 256 color", "xterm-256color", nil, false, false, false},
		{"dumb", "dumb", nil, false, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DetectCaps(c.term, c.env, c.truecolor)
			if got.Truecolor != c.wantTC {
				t.Errorf("Truecolor = %v, want %v", got.Truecolor, c.wantTC)
			}
			if got.KittyGraphics != c.wantKitty {
				t.Errorf("KittyGraphics = %v, want %v", got.KittyGraphics, c.wantKitty)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/render/ -run TestDetectCaps`
Expected: FAIL — `undefined: DetectCaps`.

- [ ] **Step 3: Write minimal implementation**

```go
// Package render owns terminal capability detection and the rendering
// backends for hero art (the splash wordmark, confirmed-order art). Screens
// depend only on this package, never on terminal specifics.
package render

import "strings"

// Caps describes what the connected terminal can do. Detected once per
// session from the SSH PTY's TERM and environment.
type Caps struct {
	Truecolor     bool // 24-bit colour available (half-block gradients/glow)
	KittyGraphics bool // terminal advertises the Kitty graphics protocol
}

// kittyTerms are TERM values that natively speak the Kitty graphics protocol.
var kittyTerms = []string{"xterm-ghostty", "ghostty", "xterm-kitty", "kitty", "wezterm"}

// kittyTermPrograms are TERM_PROGRAM values (set by some emulators that keep a
// generic TERM) that speak the Kitty graphics protocol.
var kittyTermPrograms = []string{"WezTerm", "ghostty", "kitty"}

// DetectCaps classifies a terminal from its TERM, environment slice
// ("KEY=VALUE" entries, as from ssh.Session.Environ), and whether the colour
// profile negotiated truecolor. Kitty graphics is heuristic (no round-trip
// query): enabled only for known-good terminals, so unknown terminals safely
// fall back to the portable half-block path.
func DetectCaps(term string, env []string, truecolor bool) Caps {
	c := Caps{Truecolor: truecolor}
	t := strings.ToLower(term)
	for _, k := range kittyTerms {
		if strings.Contains(t, k) {
			c.KittyGraphics = true
		}
	}
	prog := envValue(env, "TERM_PROGRAM")
	for _, k := range kittyTermPrograms {
		if strings.EqualFold(prog, k) {
			c.KittyGraphics = true
		}
	}
	// Kitty graphics is meaningless without colour; never claim it on a
	// non-truecolor session.
	if !c.Truecolor {
		c.KittyGraphics = false
	}
	return c
}

// envValue returns the value of KEY from a slice of "KEY=VALUE" entries.
func envValue(env []string, key string) string {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return e[len(prefix):]
		}
	}
	return ""
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/render/ -run TestDetectCaps`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/render/caps.go internal/tui/render/caps_test.go
git commit -m "feat(render): terminal capability detection (truecolor / kitty graphics)"
```

---

## Task 2: Wordmark bitmap (font rasterization)

**Files:**
- Create: `internal/tui/render/bitmap.go`
- Test: `internal/tui/render/bitmap_test.go`

This adds `golang.org/x/image`. Run `go get golang.org/x/image/font/basicfont@latest` as Step 0.

- [ ] **Step 0: Add the dependency**

Run: `go get golang.org/x/image/font/basicfont && go get golang.org/x/image/math/fixed`
Expected: `go.mod` gains `golang.org/x/image` as a direct require.

- [ ] **Step 1: Write the failing test**

```go
package render

import "testing"

func TestWordmark(t *testing.T) {
	bm := Wordmark("hi")
	// basicfont.Face7x13: advance 7px/glyph, 13px tall.
	if bm.H != 13 {
		t.Fatalf("height = %d, want 13", bm.H)
	}
	if bm.W != 14 { // 2 glyphs * 7px
		t.Fatalf("width = %d, want 14", bm.W)
	}
	// A blank wordmark of one space is all-off.
	blank := Wordmark(" ")
	on := 0
	for y := 0; y < blank.H; y++ {
		for x := 0; x < blank.W; x++ {
			if blank.At(x, y) {
				on++
			}
		}
	}
	if on != 0 {
		t.Errorf("space wordmark has %d lit pixels, want 0", on)
	}
	// "hi" must light at least some pixels.
	lit := 0
	for y := 0; y < bm.H; y++ {
		for x := 0; x < bm.W; x++ {
			if bm.At(x, y) {
				lit++
			}
		}
	}
	if lit == 0 {
		t.Error("\"hi\" wordmark is entirely blank")
	}
}

func TestBitmapAtBounds(t *testing.T) {
	bm := Bitmap{W: 2, H: 2, on: []bool{true, false, false, true}}
	if !bm.At(0, 0) || bm.At(1, 0) || bm.At(0, 1) || !bm.At(1, 1) {
		t.Error("At indexing wrong")
	}
	if bm.At(-1, 0) || bm.At(0, 5) {
		t.Error("out-of-bounds At should be false, not panic")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/render/ -run 'TestWordmark|TestBitmapAtBounds'`
Expected: FAIL — `undefined: Wordmark`, `undefined: Bitmap`.

- [ ] **Step 3: Write minimal implementation**

```go
package render

import (
	"image"
	"image/draw"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// Bitmap is a 1-bit raster (row-major, on[y*W+x]). It is the common source for
// both the half-block and Kitty backends so the two stay pixel-identical.
type Bitmap struct {
	W, H int
	on   []bool
}

// At reports whether pixel (x,y) is lit; out-of-bounds is false (never panics).
func (b Bitmap) At(x, y int) bool {
	if x < 0 || y < 0 || x >= b.W || y >= b.H {
		return false
	}
	return b.on[y*b.W+x]
}

// Wordmark rasterizes text with the deterministic 7x13 basic font into a
// Bitmap. Deterministic output makes it unit-testable and identical across
// runs and platforms.
func Wordmark(text string) Bitmap {
	face := basicfont.Face7x13
	w := 7 * len([]rune(text))
	h := 13
	if w <= 0 {
		return Bitmap{W: 0, H: h, on: nil}
	}
	dst := image.NewGray(image.Rect(0, 0, w, h))
	d := font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(image.White),
		Face: face,
		// basicfont ascent is 11; baseline at y=11 keeps the 13px glyph box.
		Dot: fixed.P(0, 11),
	}
	d.DrawString(text)
	on := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if dst.GrayAt(x, y).Y > 128 {
				on[y*w+x] = true
			}
		}
	}
	return Bitmap{W: w, H: h, on: on}
}

// raster draws the lit pixels into an *image.Alpha (used by the glow backend).
func (b Bitmap) raster() *image.Alpha {
	img := image.NewAlpha(image.Rect(0, 0, b.W, b.H))
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			if b.At(x, y) {
				img.SetAlpha(x, y, image.Opaque.C)
			}
		}
	}
	return img
}

var _ = draw.Draw // retained: image/draw used by glow.go in Task 4
```

Note: remove the `var _ = draw.Draw` line and the `image/draw` import if `go vet` flags them as unused at this task; Task 4 (`glow.go`) is a separate file and will import `image/draw` itself. Keeping `bitmap.go` import-clean is preferred — if unused, delete the import and the `_ =` line.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/render/ -run 'TestWordmark|TestBitmapAtBounds'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/tui/render/bitmap.go internal/tui/render/bitmap_test.go
git commit -m "feat(render): wordmark bitmap via basicfont rasterization"
```

---

## Task 3: Half-block renderer (the wide-range core)

**Files:**
- Create: `internal/tui/render/halfblock.go`
- Test: `internal/tui/render/halfblock_test.go`

- [ ] **Step 1: Write the failing test**

```go
package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestHalfBlockDimsAndGlyph(t *testing.T) {
	// 2-row, 3-col all-on bitmap -> exactly 1 text row of three ▀ cells.
	bm := Bitmap{W: 3, H: 2, on: []bool{true, true, true, true, true, true}}
	out := HalfBlock(bm, HalfBlockOpts{Top: "#7aa2f7", Bottom: "#bb9af7"})
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d rows, want 1 (ceil(2/2))", len(lines))
	}
	if w := lipgloss.Width(lines[0]); w != 3 {
		t.Errorf("row width = %d cells, want 3", w)
	}
	if !strings.Contains(lines[0], "▀") {
		t.Errorf("expected upper-half-block ▀ glyph in output")
	}
}

func TestHalfBlockOddHeight(t *testing.T) {
	// 3 rows -> ceil(3/2) = 2 text rows; the bottom row's lower pixel is empty.
	bm := Bitmap{W: 1, H: 3, on: []bool{true, true, true}}
	out := HalfBlock(bm, HalfBlockOpts{Top: "#ffffff", Bottom: "#ffffff"})
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d rows, want 2", len(lines))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/render/ -run TestHalfBlock`
Expected: FAIL — `undefined: HalfBlock`, `undefined: HalfBlockOpts`.

- [ ] **Step 3: Write minimal implementation**

```go
package render

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// HalfBlockOpts controls colour. Top/Bottom are hex endpoints of a vertical
// gradient interpolated across the bitmap height. Glow, when non-empty, paints
// a dim one-pixel halo around lit pixels in that hex (coarse bloom).
type HalfBlockOpts struct {
	Top    string // hex, top of gradient
	Bottom string // hex, bottom of gradient
	Glow   string // hex, optional halo colour ("" = no halo)
}

// HalfBlock renders a 1-bit bitmap at 2× vertical resolution: each text cell
// is one ▀ glyph whose foreground is the upper pixel and background is the
// lower pixel. Lit pixels take the vertical gradient colour; with Glow set,
// unlit pixels adjacent to a lit one take the dim halo colour. Unlit, un-haloed
// pixels render as the terminal's own background (a space-equivalent: a ▀ with
// fg==bg would still paint; instead we emit a literal blank cell there).
func HalfBlock(bm Bitmap, opt HalfBlockOpts) string {
	top, _ := colorful.Hex(orDefault(opt.Top, "#c0caf5"))
	bot, _ := colorful.Hex(orDefault(opt.Bottom, "#c0caf5"))
	var b strings.Builder
	for y := 0; y < bm.H; y += 2 {
		for x := 0; x < bm.W; x++ {
			upLit := bm.At(x, y)
			loLit := bm.At(x, y+1)
			upHalo := opt.Glow != "" && !upLit && neighbourLit(bm, x, y)
			loHalo := opt.Glow != "" && !loLit && neighbourLit(bm, x, y+1)

			if !upLit && !loLit && !upHalo && !loHalo {
				b.WriteString(" ") // empty cell -> terminal canvas shows through
				continue
			}
			fg := pixelColor(upLit, upHalo, gradientAt(top, bot, y, bm.H), opt.Glow)
			bg := pixelColor(loLit, loHalo, gradientAt(top, bot, y+1, bm.H), opt.Glow)
			cell := lipgloss.NewStyle().
				Foreground(lipgloss.Color(fg)).
				Background(lipgloss.Color(bg)).
				Render("▀")
			b.WriteString(cell)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// pixelColor resolves a single half-cell pixel: lit -> gradient hex, halo ->
// glow hex, else a sentinel that never renders (caller guarantees at least one
// of the four sub-pixels is non-empty before styling).
func pixelColor(lit, halo bool, grad, glow string) string {
	switch {
	case lit:
		return grad
	case halo:
		return glow
	default:
		// Neither lit nor halo, but the sibling pixel forced a styled cell.
		// Use the glow colour if present (keeps the cell cohesive), else a
		// near-black that blends with the dark canvas.
		if glow != "" {
			return glow
		}
		return "#15161f"
	}
}

// gradientAt returns the hex colour at row y of an h-tall vertical gradient.
func gradientAt(top, bot colorful.Color, y, h int) string {
	if h <= 1 {
		return top.Hex()
	}
	t := float64(y) / float64(h-1)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return top.BlendLab(bot, t).Clamped().Hex()
}

// neighbourLit reports whether any 8-neighbour of (x,y) is a lit pixel.
func neighbourLit(bm Bitmap, x, y int) bool {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			if bm.At(x+dx, y+dy) {
				return true
			}
		}
	}
	return false
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/render/ -run TestHalfBlock`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/render/halfblock.go internal/tui/render/halfblock_test.go
git commit -m "feat(render): truecolor half-block bitmap renderer with gradient + glow halo"
```

---

## Task 4: Hero entry point + splash integration (wide-range win lands here)

**Files:**
- Create: `internal/tui/render/hero.go`
- Test: `internal/tui/render/hero_test.go`
- Modify: `internal/tui/screens/splash.go`
- Modify: `internal/tui/app.go` (constructor signature — see Task 6 for the full wiring; here add the minimum to pass caps to splash)

This task makes the half-block logo actually appear. Kitty stays out until Task 5, so `render.Logo` here has only two arms: half-block (truecolor) and ASCII (fallback).

- [ ] **Step 1: Write the failing test**

```go
package render

import (
	"strings"
	"testing"
)

func TestLogoTruecolorUsesHalfBlock(t *testing.T) {
	out := Logo(Caps{Truecolor: true}, 64)
	if !strings.Contains(out, "▀") {
		t.Error("truecolor Logo should render via half-blocks (▀)")
	}
}

func TestLogoFallbackIsAscii(t *testing.T) {
	out := Logo(Caps{Truecolor: false}, 64)
	if strings.Contains(out, "▀") {
		t.Error("non-truecolor Logo must not use half-blocks")
	}
	if !strings.Contains(out, "█") {
		t.Error("fallback Logo should be the box-drawing ASCII wordmark")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/render/ -run TestLogo`
Expected: FAIL — `undefined: Logo`.

- [ ] **Step 3: Write minimal implementation**

```go
package render

import "strings"

// asciiLogo is the box-drawing wordmark — the floor for terminals without
// truecolor. Kept identical to the previous splash logo so nothing regresses.
var asciiLogo = []string{
	`██████╗ ██████╗ ███╗  ██╗███████╗ ██████╗ ██╗     ███████╗`,
	`██╔════╝██╔═══██╗████╗ ██║██╔════╝██╔═══██╗██║     ██╔════╝`,
	`██║     ██║   ██║██╔██╗██║███████╗██║   ██║██║     █████╗  `,
	`██║     ██║   ██║██║╚████║╚════██║██║   ██║██║     ██╔══╝  `,
	`╚██████╗╚██████╔╝██║ ╚███║███████║╚██████╔╝███████╗███████╗`,
	` ╚═════╝ ╚═════╝ ╚═╝  ╚══╝╚══════╝ ╚═════╝ ╚══════╝╚══════╝`,
}

// Logo returns the best wordmark for the terminal's capabilities. w is the
// available frame width (reserved for future centring; unused today). The
// caller is responsible for indentation/placement.
//
// Kitty graphics is intentionally NOT consulted here yet (Task 5 adds it
// behind a default-off flag). Order: truecolor half-block, else ASCII.
func Logo(caps Caps, w int) string {
	if caps.Truecolor {
		bm := Wordmark("console")
		return HalfBlock(bm, HalfBlockOpts{
			Top:    "#7aa2f7", // tokyo-night cursor blue (matches design glow hue)
			Bottom: "#bb9af7", // purple — subtle vertical sheen
			Glow:   "#3b4261", // dim blue-grey halo = coarse bloom
		})
	}
	return strings.Join(asciiLogo, "\n") + "\n"
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/render/ -run TestLogo`
Expected: PASS.

- [ ] **Step 5: Wire caps through the splash**

Modify `internal/tui/screens/splash.go`:
- Add import `"console.store/internal/tui/render"`.
- Add a field and setter:

```go
type Splash struct {
	bootStep int
	spin     string
	tagline  string
	caps     render.Caps
}

// WithCaps sets the terminal capabilities used to pick the logo backend.
func (s Splash) WithCaps(c render.Caps) Splash { s.caps = c; return s }
```

- Replace the logo emission loop in `View()`:

```go
	// was: for _, l := range logo { b.WriteString("  " + theme.CursorStyle.Render(l) + "\n") }
	for _, l := range strings.Split(strings.TrimRight(render.Logo(s.caps, 64), "\n"), "\n") {
		b.WriteString("  " + l + "\n")
	}
```

- Delete the now-unused package-level `var logo = []string{...}` (the ASCII fallback lives in `render.asciiLogo`). Leave `bootLines`, `Taglines` untouched.

- [ ] **Step 6: Thread caps into New() (minimal)**

Modify `internal/tui/app.go`:
- Add import `"console.store/internal/tui/render"`.
- Add `caps render.Caps` to the `Model` struct (next to `w, h int`).
- Change `func New() Model` to `func New(caps render.Caps) Model`; store it and pass to the splash:

```go
func New(caps render.Caps) Model {
	repo := mem.New()
	addr := repo.Addresses()[0]
	section := catalog.SectionCoffee
	m := Model{repo: repo, addr: addr, section: section, screen: scrSplash, caps: caps}
	m.splash = screens.NewSplash().WithCaps(caps)
	m.menu = m.buildMenu()
	return m
}
```

- [ ] **Step 7: Update the caller**

Modify `cmd/sshd/main.go` `teaHandler` — build caps from the session and pass them:

```go
func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	renderer := bubbletea.MakeRenderer(s)
	lipgloss.SetColorProfile(renderer.ColorProfile())

	pty, _, _ := s.Pty()
	truecolor := renderer.ColorProfile() == termenv.TrueColor
	caps := render.DetectCaps(pty.Term, s.Environ(), truecolor)

	return consoletui.New(caps), []tea.ProgramOption{tea.WithAltScreen()}
}
```

Add imports: `"console.store/internal/tui/render"` and `"github.com/muesli/termenv"` (already an indirect dep; the `go build` promotes it to direct or it resolves via the module graph — if the build complains, run `go get github.com/muesli/termenv`).

- [ ] **Step 8: Build, test, and verify live**

Run: `go build ./... && go test ./...`
Expected: all packages PASS (the existing splash test, if any asserts the old `█` logo on a truecolor model, must be updated — search: `grep -rn "██████" internal/tui/*_test.go internal/tui/screens/*_test.go` and adjust to assert via `render.Logo`).

Then drive it (the `run` skill — actually launch and look):

```bash
pkill -f 'go run ./cmd/sshd'; sleep 1
nohup go run ./cmd/sshd/ > /tmp/csd.log 2>&1 &
sleep 4
tmux kill-session -t fid 2>/dev/null
tmux new-session -d -s fid -x 100 -y 30
tmux send-keys -t fid 'ssh -o StrictHostKeyChecking=no localhost -p 2222' Enter
sleep 3
tmux capture-pane -t fid -p | head -20
```

Expected: the wordmark renders in `▀` half-blocks with a blue→purple gradient and a faint halo — visibly crisper letterforms than the old box art. Confirm no width overflow (each logo line ≤ frame).

- [ ] **Step 9: Commit**

```bash
git add internal/tui/render/hero.go internal/tui/render/hero_test.go \
        internal/tui/screens/splash.go internal/tui/app.go cmd/sshd/main.go go.mod go.sum
git commit -m "feat(tui): crisp half-block splash wordmark (gradient + glow halo), caps-gated"
```

---

## Task 5: Kitty graphics backend (progressive enhancement, flag-default-off)

**Files:**
- Create: `internal/tui/render/glow.go`
- Create: `internal/tui/render/kitty.go`
- Test: `internal/tui/render/glow_test.go`
- Test: `internal/tui/render/kitty_test.go`
- Modify: `internal/tui/render/hero.go` (add the Kitty arm)

**Risk note for the implementer:** Kitty APC image sequences (`\x1b_G...\x1b\\`) are emitted inside bubbletea's `View()` string, which the standard renderer line-diffs. The renderer may miscount the APC payload's width or split it across a diff and corrupt the image. This task is therefore gated behind `KittyFlag` (a package var, **default false**). Land the code and tests, keep it OFF, and only flip it after a human verifies on a real Kitty/Ghostty client. The half-block path (Task 4) remains the shipping default.

- [ ] **Step 1: Write the failing test (glow)**

```go
package render

import (
	"image/color"
	"testing"
)

func TestGlowImageBounds(t *testing.T) {
	bm := Bitmap{W: 4, H: 2, on: []bool{false, true, true, false, false, true, true, false}}
	img := GlowImage(bm, color.RGBA{122, 162, 247, 255}, 6)
	b := img.Bounds()
	// scaled by 6, plus a blur margin of 8px each side baked in.
	if b.Dx() != 4*6+16 || b.Dy() != 2*6+16 {
		t.Fatalf("bounds = %dx%d, want %dx%d", b.Dx(), b.Dy(), 4*6+16, 2*6+16)
	}
	// A pixel at the centre of a lit glyph block should be near-tint (sharp
	// overlay), i.e. high blue, opaque.
	cx, cy := 8+6, 8+6 // into the first lit pixel's scaled block
	r, g, bl, a := img.At(cx, cy).RGBA()
	if a == 0 {
		t.Errorf("centre of lit glyph is transparent")
	}
	_ = r
	_ = g
	_ = bl
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/render/ -run TestGlowImage`
Expected: FAIL — `undefined: GlowImage`.

- [ ] **Step 3: Implement glow**

```go
package render

import (
	"image"
	"image/color"
	"image/draw"
)

// glowMargin is the transparent border (px) added around the scaled wordmark
// so the blurred halo has room to bleed.
const glowMargin = 8

// GlowImage rasterizes a bitmap scaled by `scale`, then composites: a blurred
// tinted halo underneath and the sharp tinted wordmark on top. The result is
// an RGBA image suitable for a Kitty payload (real sub-pixel bloom — the effect
// the terminal text grid cannot do).
func GlowImage(bm Bitmap, tint color.RGBA, scale int) image.Image {
	w := bm.W*scale + 2*glowMargin
	h := bm.H*scale + 2*glowMargin
	sharp := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < bm.H; y++ {
		for x := 0; x < bm.W; x++ {
			if !bm.At(x, y) {
				continue
			}
			rect := image.Rect(
				glowMargin+x*scale, glowMargin+y*scale,
				glowMargin+(x+1)*scale, glowMargin+(y+1)*scale,
			)
			draw.Draw(sharp, rect, image.NewUniform(tint), image.Point{}, draw.Src)
		}
	}
	// Halo: blur a copy, then draw the sharp wordmark over it.
	halo := boxBlur(sharp, 3, 3) // 3 passes of radius-3 ≈ gaussian bloom
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(out, out.Bounds(), halo, image.Point{}, draw.Src)
	draw.Draw(out, out.Bounds(), sharp, image.Point{}, draw.Over)
	return out
}

// boxBlur applies `passes` iterations of a separable box blur of the given
// radius to an RGBA image (premultiplied-alpha averaging). Hand-rolled to avoid
// an image-processing dependency.
func boxBlur(src *image.RGBA, radius, passes int) *image.RGBA {
	cur := src
	for p := 0; p < passes; p++ {
		cur = boxBlurOnce(cur, radius)
	}
	return cur
}

func boxBlurOnce(src *image.RGBA, radius int) *image.RGBA {
	b := src.Bounds()
	tmp := image.NewRGBA(b)
	// horizontal
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			var r, g, bl, a, n int
			for dx := -radius; dx <= radius; dx++ {
				xx := x + dx
				if xx < b.Min.X || xx >= b.Max.X {
					continue
				}
				c := src.RGBAAt(xx, y)
				r += int(c.R)
				g += int(c.G)
				bl += int(c.B)
				a += int(c.A)
				n++
			}
			tmp.SetRGBA(x, y, avg(r, g, bl, a, n))
		}
	}
	out := image.NewRGBA(b)
	// vertical
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			var r, g, bl, a, n int
			for dy := -radius; dy <= radius; dy++ {
				yy := y + dy
				if yy < b.Min.Y || yy >= b.Max.Y {
					continue
				}
				c := tmp.RGBAAt(x, yy)
				r += int(c.R)
				g += int(c.G)
				bl += int(c.B)
				a += int(c.A)
				n++
			}
			out.SetRGBA(x, y, avg(r, g, bl, a, n))
		}
	}
	return out
}

func avg(r, g, bl, a, n int) color.RGBA {
	if n == 0 {
		return color.RGBA{}
	}
	return color.RGBA{uint8(r / n), uint8(g / n), uint8(bl / n), uint8(a / n)}
}
```

- [ ] **Step 4: Run glow test**

Run: `go test ./internal/tui/render/ -run TestGlowImage`
Expected: PASS.

- [ ] **Step 5: Write the failing test (kitty)**

```go
package render

import (
	"encoding/base64"
	"image"
	"image/png"
	"bytes"
	"strings"
	"testing"
)

func TestKittyImageEnvelope(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 4))
	out := KittyImage(img, 8, 2)
	if !strings.HasPrefix(out, "\x1b_G") {
		t.Errorf("missing Kitty APC introducer")
	}
	if !strings.HasSuffix(out, "\x1b\\") {
		t.Errorf("missing string terminator")
	}
	if !strings.Contains(out, "f=100") {
		t.Errorf("expected PNG format key f=100")
	}
}

func TestKittyPayloadDecodesToPNG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 4))
	out := KittyImage(img, 8, 2)
	// Concatenate all base64 chunk payloads (between ';' and the ESC \).
	var b64 strings.Builder
	for _, chunk := range strings.Split(out, "\x1b_G") {
		if chunk == "" {
			continue
		}
		semi := strings.IndexByte(chunk, ';')
		end := strings.Index(chunk, "\x1b\\")
		if semi < 0 || end < 0 {
			continue
		}
		b64.WriteString(chunk[semi+1 : end])
	}
	raw, err := base64.StdEncoding.DecodeString(b64.String())
	if err != nil {
		t.Fatalf("payload not valid base64: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(raw)); err != nil {
		t.Fatalf("payload is not a valid PNG: %v", err)
	}
}
```

- [ ] **Step 6: Run kitty test to verify it fails**

Run: `go test ./internal/tui/render/ -run TestKitty`
Expected: FAIL — `undefined: KittyImage`.

- [ ] **Step 7: Implement kitty**

```go
package render

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"strings"
)

// KittyFlag gates the Kitty graphics path. DEFAULT OFF: the APC payload can be
// corrupted by bubbletea's line-diff renderer, so it must be verified on a real
// Kitty/Ghostty client before enabling. The half-block path is the shipping
// default regardless.
var KittyFlag = false

// kittyChunk is the max base64 bytes per APC chunk per the Kitty protocol.
const kittyChunk = 4096

// KittyImage encodes img as PNG and emits the Kitty graphics protocol escape
// to transmit-and-display it, scaled to cols×rows terminal cells. Payloads
// over 4096 base64 bytes are split into m=1 continuation chunks.
func KittyImage(img image.Image, cols, rows int) string {
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	payload := base64.StdEncoding.EncodeToString(buf.Bytes())

	var b strings.Builder
	first := true
	for len(payload) > 0 {
		n := kittyChunk
		if n > len(payload) {
			n = len(payload)
		}
		chunk := payload[:n]
		payload = payload[n:]
		more := 0
		if len(payload) > 0 {
			more = 1
		}
		if first {
			// a=T transmit+display, f=100 PNG, c/r = display cell box.
			fmt.Fprintf(&b, "\x1b_Ga=T,f=100,c=%d,r=%d,m=%d;%s\x1b\\", cols, rows, more, chunk)
			first = false
		} else {
			fmt.Fprintf(&b, "\x1b_Gm=%d;%s\x1b\\", more, chunk)
		}
	}
	return b.String()
}

// kittyImageBounds is a tiny helper so hero.go can size the cell box from a
// bitmap without re-deriving the scale math.
func kittyCellBox(bm Bitmap) (cols, rows int) {
	// Match the half-block footprint: width in cells == bitmap width, height
	// in cells == ceil(H/2). Keeps layout identical whichever backend wins.
	return bm.W, (bm.H + 1) / 2
}

var _ = image.Rect // image used via the signature; keep import honest
```

(Delete the `var _ = image.Rect` line — `image.Image` in the signature already uses the import. It is shown only to flag the dependency.)

- [ ] **Step 8: Run kitty tests**

Run: `go test ./internal/tui/render/ -run TestKitty`
Expected: PASS.

- [ ] **Step 9: Add the Kitty arm to Logo**

Modify `internal/tui/render/hero.go` `Logo`:

```go
func Logo(caps Caps, w int) string {
	bm := Wordmark("console")
	if caps.KittyGraphics && KittyFlag {
		cols, rows := kittyCellBox(bm)
		img := GlowImage(bm, color.RGBA{122, 162, 247, 255}, 6) // #7aa2f7
		return KittyImage(img, cols, rows) + "\n"
	}
	if caps.Truecolor {
		return HalfBlock(bm, HalfBlockOpts{Top: "#7aa2f7", Bottom: "#bb9af7", Glow: "#3b4261"})
	}
	return strings.Join(asciiLogo, "\n") + "\n"
}
```

Add `"image/color"` to `hero.go` imports.

- [ ] **Step 10: Build + full test**

Run: `go build ./... && go test ./...`
Expected: all PASS. (Kitty stays inert because `KittyFlag == false`; `Logo` behaviour is unchanged from Task 4.)

- [ ] **Step 11: Commit**

```bash
git add internal/tui/render/glow.go internal/tui/render/glow_test.go \
        internal/tui/render/kitty.go internal/tui/render/kitty_test.go \
        internal/tui/render/hero.go
git commit -m "feat(render): kitty-graphics glow logo backend (flag-default-off, half-block fallback)"
```

---

## Task 6: Motion polish — faster tick + eased tracking

**Files:**
- Modify: `internal/tui/app.go` (tick interval; locate the `tea.Tick(...)` and the `tickMsg` constant)

The current tick is 110ms (~9fps). Drop to 60ms (~16fps) for liquid spinner/route motion without flooding the SSH pipe. Frame-derived cadences (boot streaming, tracking, blink) are currently expressed in raw frame counts — halving the interval doubles their speed, so re-scale the divisors that gate them.

- [ ] **Step 1: Find the tick interval and frame divisors**

Run: `grep -n 'tea.Tick\|/15\|/9\|frame/\|110\|time.Millisecond' internal/tui/app.go`
Record every `m.frame/<N>` and time literal — these are the cadence knobs.

- [ ] **Step 2: Write/adjust a test for the tick interval**

If a test asserts the interval, update it; otherwise add one. Example (adapt the constant name to the actual code):

```go
func TestTickInterval(t *testing.T) {
	if tickInterval != 60*time.Millisecond {
		t.Errorf("tickInterval = %v, want 60ms", tickInterval)
	}
}
```

- [ ] **Step 3: Halve the interval, double the gating divisors**

- Change the tick duration literal from `110 * time.Millisecond` to `60 * time.Millisecond` (introduce a named `const tickInterval = 60 * time.Millisecond` if not present and use it in the `tea.Tick`).
- For each `m.frame/N` that controls a *perceived* cadence (tagline rotation, status-hint rotation, boot-line reveal, tracking step advance), multiply `N` by ~1.8 (110/60) and round, so those animations keep their real-time speed while the spinner itself runs smoother. Example: tagline `m.frame/15` → `m.frame/27`; tracking advance similarly.
- Leave the spinner frame index (`spinFrames[m.frame%len]`) as-is — it *should* speed up; that is the point.

- [ ] **Step 4: Build, test, verify live**

Run: `go build ./... && go test ./...`
Expected: PASS.

Then watch the spinner cadence:

```bash
pkill -f 'go run ./cmd/sshd'; sleep 1; nohup go run ./cmd/sshd/ > /tmp/csd.log 2>&1 & sleep 4
tmux kill-session -t mot 2>/dev/null; tmux new-session -d -s mot -x 100 -y 30
tmux send-keys -t mot 'ssh -o StrictHostKeyChecking=no localhost -p 2222' Enter; sleep 2
for i in 1 2 3; do tmux capture-pane -t mot -p | grep -m1 'establishing\|warming\|fetching'; sleep 0.2; done
```

Expected: spinner glyph advances visibly between captures; tagline does NOT rotate faster than before.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat(tui): smoother motion — 60ms tick with re-scaled animation cadences"
```

---

## Task 7 (optional): Confirmed-order art via the render package

**Files:**
- Modify: `internal/tui/screens/checkout.go` (`confirmView`, the coffee-cup block)
- Modify: `internal/tui/screens/checkout.go` constructor path or `app.go` to pass `caps` into `Checkout`

Only do this once Tasks 1–5 are merged and the splash logo is confirmed crisp. The coffee-cup art (`╭────────╮` … `╰────────╯`) is a candidate for the same half-block/Kitty treatment, but it is small and already readable, so the ROI is lower. If pursued:

- [ ] **Step 1:** Add `caps render.Caps` to `Checkout` (mirror `Splash.WithCaps`), passed from `app.go` where `NewCheckout` is called (two call sites: `internal/tui/app.go` food + instamart checkout).
- [ ] **Step 2:** Author a small `Bitmap` for a coffee cup (hand-built `[]bool` grid, ~16×12) in `render/hero.go` as `func CupArt(caps Caps) string`, rendered via `HalfBlock` with a gold gradient (`#e0af68`→`#c0caf5`).
- [ ] **Step 3:** Test it renders `▀` on truecolor and the existing box-art on fallback (mirror `TestLogoFallbackIsAscii`).
- [ ] **Step 4:** Build, test, drive the confirmed screen, commit.

---

## Self-Review

**Spec coverage:**
- Wide-range crisp logo → Tasks 2–4 (half-block, works on any truecolor terminal). ✓
- Glow (the design's signature) → coarse halo in half-block (Task 3/4); real bloom in Kitty (Task 5). ✓
- Kitty "if best, go for it" but wide-range priority → Task 5 behind default-off flag with guaranteed half-block fallback. ✓
- Smoothness/crispness of motion → Task 6. ✓
- Capability detection / no breakage on plain terminals → Task 1 + ASCII fallback arm. ✓

**Placeholder scan:** No "TBD"/"handle errors"/"similar to" — every code step is complete. The two `var _ = ...` lines are explicitly flagged for deletion with the reason. ✓

**Type consistency:** `Bitmap{W,H,on}`, `Bitmap.At`, `Wordmark`, `HalfBlock(bm, HalfBlockOpts{Top,Bottom,Glow})`, `Caps{Truecolor,KittyGraphics}`, `DetectCaps(term, env, truecolor)`, `Logo(caps, w)`, `GlowImage(bm, tint, scale)`, `KittyImage(img, cols, rows)`, `KittyFlag`, `kittyCellBox(bm)` — names are consistent across Tasks 1–5. `New(caps)` signature change is propagated to `cmd/sshd/main.go` in Task 4 Step 7. ✓

**Known risks documented:** Kitty/bubbletea renderer interaction (Task 5 risk note, flag-default-off); basicfont width math drives both backends so layout stays identical; existing splash test may assert the old logo (Task 4 Step 8 calls this out). ✓
