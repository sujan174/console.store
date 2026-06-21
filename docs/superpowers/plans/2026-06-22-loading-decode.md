# Glitch-Decode Loading Screen Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the fake boot-log splash with a snappy glitch-decode reveal of the `console.store` block wordmark, settling into the existing home landing.

**Architecture:** A pure, stateless `render.DecodeWordmark(caps, step, frame)` returns the block wordmark mid-decode (left columns resolved, right columns frame-seeded glitch glyphs). The existing 60ms tick drives `decodeStep` 0→`DecodeSteps`; the splash boot branch renders the decode, then shows the unchanged settled home (cup + `go to shop`) which holds for a keypress.

**Tech Stack:** Go 1.26, bubbletea, lipgloss, go-colorful. Tests: `go test`, inline substring assertions.

**Spec:** `docs/superpowers/specs/2026-06-22-loading-decode-design.md`

---

## File Structure

- **Create** `internal/tui/render/decode.go` — `DecodeWordmark` + `DecodeSteps` + glitch charset. Pure render logic.
- **Create** `internal/tui/render/decode_test.go` — unit tests for the decode function.
- **Modify** `internal/tui/screens/splash.go` — delete fake-log machinery (`bootLines`, `BootLineCount`, `Taglines`, `spin`/`tagline`/`bootStep` fields, `WithBoot`); add `decodeStep` field + `WithDecode`; render decode in the boot branch.
- **Modify** `internal/tui/screens/splash_test.go` — boot-phase test now asserts the decode/subtitle, not the fake logs.
- **Modify** `internal/tui/app.go` — rename `bootStep`→`decodeStep`; tick increments every frame to `DecodeSteps`; key during decode skips to home; `View` calls `WithDecode`.
- **Modify** `internal/tui/app_test.go` — update splash tests to decode semantics.
- **Modify** `internal/tui/flow_test.go` — add the extra keypress now needed to pass the home landing.

---

## Task 1: `render.DecodeWordmark`

**Files:**
- Create: `internal/tui/render/decode.go`
- Test: `internal/tui/render/decode_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/tui/render/decode_test.go`:

```go
package render

import (
	"strings"
	"testing"
)

func TestDecodeStartHasGlitch(t *testing.T) {
	out := DecodeWordmark(Caps{Truecolor: true}, 0, 0)
	if !strings.ContainsAny(out, glitchChars) {
		t.Fatalf("step 0 should contain glitch chars:\n%s", out)
	}
	if got := len(strings.Split(strings.TrimRight(out, "\n"), "\n")); got != len(asciiLogo) {
		t.Fatalf("line count = %d, want %d", got, len(asciiLogo))
	}
}

func TestDecodeEndIsClean(t *testing.T) {
	out := DecodeWordmark(Caps{Truecolor: true}, DecodeSteps, 5)
	if strings.ContainsAny(out, glitchChars) {
		t.Fatalf("settled wordmark should have no glitch chars:\n%s", out)
	}
	if !strings.Contains(out, "█") {
		t.Fatalf("settled wordmark should contain block glyphs:\n%s", out)
	}
}

func TestDecodeMidHasBoth(t *testing.T) {
	out := DecodeWordmark(Caps{Truecolor: true}, DecodeSteps/2, 3)
	if !strings.ContainsAny(out, glitchChars) {
		t.Fatalf("mid-decode should still have glitch chars:\n%s", out)
	}
	if !strings.Contains(out, "█") {
		t.Fatalf("mid-decode should have resolved block glyphs:\n%s", out)
	}
}

func TestDecodeDeterministic(t *testing.T) {
	a := DecodeWordmark(Caps{Truecolor: true}, 4, 7)
	b := DecodeWordmark(Caps{Truecolor: true}, 4, 7)
	if a != b {
		t.Fatal("same (step, frame) must produce identical output")
	}
}

func TestDecodeKittySettles(t *testing.T) {
	// On the Kitty bitmap path the bloom can't be glyph-decoded; it returns the
	// settled logo regardless of step.
	caps := Caps{KittyGraphics: true}
	prev := KittyFlag
	KittyFlag = true
	defer func() { KittyFlag = prev }()
	if DecodeWordmark(caps, 0, 0) != Logo(caps, 64) {
		t.Fatal("kitty path should return the settled Logo")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/render -run TestDecode`
Expected: FAIL — `undefined: DecodeWordmark`, `undefined: glitchChars`, `undefined: DecodeSteps`.

- [ ] **Step 3: Write the implementation**

Create `internal/tui/render/decode.go`:

```go
package render

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// DecodeSteps is the number of animation ticks the decode runs before it locks.
// ~0.8s at the 60ms tick (13 * 60ms ≈ 0.78s).
const DecodeSteps = 13

// glitchChars are the noise glyphs shown for not-yet-resolved cells. None of
// these appear in asciiLogo, so their presence reliably signals "mid-decode".
const glitchChars = `01<>/\{}[]#%&$*+=`

const (
	decodeEdgeHex  = "#7aa2f7" // bright cyan-blue render head
	decodeGlitchHx = "#565f89" // dim glitch noise
)

// DecodeWordmark renders the block wordmark mid-decode. step is decode progress
// (0..DecodeSteps); frame is the global animation tick (drives glitch shimmer).
// Columns left of the resolve front show the real glyph (gradient-tinted on
// truecolor); the front column is a bright edge; columns to the right show a
// deterministic glitch glyph. At step==DecodeSteps the wordmark is fully clean.
//
// The Kitty graphics path renders a rasterized bloom that cannot be glyph-
// decoded, so it settles straight to the bloom logo.
func DecodeWordmark(caps Caps, step, frame int) string {
	if caps.KittyGraphics && KittyFlag {
		return Logo(caps, 64)
	}

	W := 0
	for _, ln := range asciiLogo {
		if r := len([]rune(ln)); r > W {
			W = r
		}
	}
	resolved := step * W / DecodeSteps
	if step >= DecodeSteps {
		resolved = W
	}

	top, _ := colorful.Hex("#7aa2f7")
	bot, _ := colorful.Hex("#bb9af7")
	edge := lipgloss.NewStyle().Foreground(lipgloss.Color(decodeEdgeHex))
	glitch := lipgloss.NewStyle().Foreground(lipgloss.Color(decodeGlitchHx))
	n := len(asciiLogo)

	var out strings.Builder
	for y, ln := range asciiLogo {
		runes := []rune(ln)
		frac := 0.0
		if n > 1 {
			frac = float64(y) / float64(n-1)
		}
		lineHex := top.BlendLab(bot, frac).Clamped().Hex()
		grad := lipgloss.NewStyle().Foreground(lipgloss.Color(lineHex))

		for x := 0; x < W; x++ {
			r := ' '
			if x < len(runes) {
				r = runes[x]
			}
			switch {
			case r == ' ':
				out.WriteByte(' ') // silhouette gaps stay empty
			case x < resolved:
				if caps.Truecolor {
					out.WriteString(grad.Render(string(r)))
				} else {
					out.WriteString(string(r))
				}
			case x == resolved:
				out.WriteString(edge.Render(string(r)))
			default:
				g := rune(glitchChars[(x*31+y*7+frame)%len(glitchChars)])
				out.WriteString(glitch.Render(string(g)))
			}
		}
		out.WriteByte('\n')
	}
	return out.String()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/render -run TestDecode -v`
Expected: PASS (all 5).

- [ ] **Step 5: Commit**

```bash
git add internal/tui/render/decode.go internal/tui/render/decode_test.go
git commit -m "feat(render): glitch-decode wordmark function

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 2: Splash renders the decode

**Files:**
- Modify: `internal/tui/screens/splash.go`
- Modify: `internal/tui/screens/splash_test.go`

- [ ] **Step 1: Update the boot-phase test to decode semantics**

In `internal/tui/screens/splash_test.go`, replace `TestSplashBootPhaseStreamsLines` and the logo-phase setup call. New file body for those two tests:

```go
func TestSplashDecodePhase(t *testing.T) {
	s := screens.NewSplash().WithDecode(2)
	v := s.View()
	if strings.Contains(v, "tls handshake") || strings.Contains(v, "devs online") {
		t.Errorf("decode phase must not show the old fake boot logs:\n%s", v)
	}
	if !strings.Contains(v, "coffee · food · snacks") {
		t.Errorf("decode phase should show the section subtitle:\n%s", v)
	}
}

func TestSplashLogoPhase(t *testing.T) {
	s := screens.NewSplash().WithDecode(99) // past DecodeSteps -> settled
	v := s.View()
	if !strings.Contains(v, "go to shop") {
		t.Errorf("logo phase should show the go-to-shop button:\n%s", v)
	}
	if !strings.Contains(v, "connected") {
		t.Errorf("settled splash should show the ✓ connected line:\n%s", v)
	}
	if !strings.Contains(v, "coffee · food · snacks") {
		t.Errorf("settled splash should show the section subtitle:\n%s", v)
	}
	if !strings.Contains(v, ".store") {
		t.Errorf("settled splash should show the gold .store suffix:\n%s", v)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/tui/screens -run TestSplash`
Expected: FAIL — `s.WithDecode undefined` (compile error).

- [ ] **Step 3: Rewrite splash.go boot machinery**

In `internal/tui/screens/splash.go`:

(a) Delete the `bootLines` var, the `BootLineCount` const, and the `Taglines` var entirely.

(b) Replace the `Splash` struct and its `WithBoot` builder. The struct becomes:

```go
type Splash struct {
	decodeStep int // decode progress (0..render.DecodeSteps)
	frame      int // animation frame (steam + glitch shimmer)
	sel        int // selected home item
	caps       render.Caps
	logoCache  string // render.Logo is constant per session; computed once here
}
```

(c) Remove the `WithBoot` method. Add `WithDecode`:

```go
// WithDecode returns a copy reflecting the current decode step.
func (s Splash) WithDecode(step int) Splash { s.decodeStep = step; return s }
```

(d) Replace the boot branch in `View`. The method top stays (`b.WriteString("\n\n")`), then:

```go
	// decode phase: glitch-resolve the wordmark, subtitle beneath.
	if s.decodeStep < render.DecodeSteps {
		art := render.DecodeWordmark(s.caps, s.decodeStep, s.frame)
		artLines := strings.Split(strings.TrimRight(art, "\n"), "\n")
		w := 0
		for _, l := range artLines {
			if x := lipgloss.Width(l); x > w {
				w = x
			}
		}
		for _, l := range artLines {
			b.WriteString("  " + l + "\n")
		}
		b.WriteString("\n")
		sub := theme.DimStyle.Render("coffee · food · snacks")
		pad := (w - lipgloss.Width(sub)) / 2
		if pad < 0 {
			pad = 0
		}
		b.WriteString("  " + strings.Repeat(" ", pad) + sub + "\n")
		return b.String()
	}
```

(e) The settled-phase code below this branch (connect line, logo, gold `.store`, subtitle, coffee+menu `JoinHorizontal`) is unchanged.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/screens -run TestSplash -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/screens/splash.go internal/tui/screens/splash_test.go
git commit -m "feat(splash): render glitch-decode, drop fake boot logs

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 3: Wire the decode into the app

**Files:**
- Modify: `internal/tui/app.go`
- Modify: `internal/tui/app_test.go`
- Modify: `internal/tui/flow_test.go`

- [ ] **Step 1: Update app_test.go to decode semantics**

Replace `TestStartsOnSplashThenKeyToMenu` and `TestSplashHoldsUntilKey` in `internal/tui/app_test.go`:

```go
func TestStartsOnSplashThenKeyToMenu(t *testing.T) {
	m := New(render.Caps{}, nil)
	if m.screen != scrSplash {
		t.Fatalf("app should start on splash, got screen %d", m.screen)
	}
	// a key during the decode skips it and settles the home landing (still splash)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = updated.(Model)
	if m.screen != scrSplash {
		t.Fatalf("key during decode should settle home, not leave splash; got %d", m.screen)
	}
	// from the settled home, activating go-to-shop -> menu
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = updated.(Model)
	if !strings.Contains(m.View(), "console.store") {
		t.Errorf("after activating, should be on menu:\n%s", m.View())
	}
}

func TestSplashHoldsUntilKey(t *testing.T) {
	m := New(render.Caps{}, nil)
	// Ticks resolve the decode but never leave the splash — it's a landing
	// screen now; the user must pick "go to shop".
	for i := 0; i < 200; i++ {
		updated, _ := m.Update(tickMsg(time.Now()))
		m = updated.(Model)
	}
	if m.screen != scrSplash {
		t.Errorf("splash should hold until a key, got screen %d", m.screen)
	}
	if m.decodeStep < render.DecodeSteps {
		t.Errorf("decode should have finished, decodeStep=%d", m.decodeStep)
	}
	// enter activates the selected home item -> menu
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.screen != scrMenu {
		t.Errorf("enter on settled splash should go to menu, got %d", m.screen)
	}
}
```

If `screens` is now unused in `app_test.go` imports, remove it; keep `render`.

- [ ] **Step 2: Update flow_test.go for the home landing**

In `internal/tui/flow_test.go`, the splash now settles to a home landing that needs one activation to reach the menu. Replace the splash-dismiss line:

```go
	// skip the decode -> home landing, then activate go-to-shop -> menu
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
```

(replaces the single `tm.Send(... "x")` "dismiss the splash -> menu" line; the rest of the flow is unchanged.)

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/tui -run 'TestStartsOnSplash|TestSplashHolds|TestFlow'`
Expected: FAIL — `m.decodeStep undefined`, `m.WithDecode` not wired (compile errors in app.go).

- [ ] **Step 4: Edit app.go**

(a) Rename the struct field `bootStep int` → `decodeStep int` (keep the `homeSel int` line below it).

(b) Replace the splash branch in `onTick`:

```go
	if m.screen == scrSplash {
		// Resolve the decode; then hold on the home landing for a keypress.
		if m.decodeStep < render.DecodeSteps {
			m.decodeStep++
		}
	}
```

(c) Replace the splash key block (currently the `if m.screen == scrSplash { switch ... }` that handles up/down/default):

```go
		if m.screen == scrSplash {
			// A key during the decode skips it and settles the home landing.
			if m.decodeStep < render.DecodeSteps {
				m.decodeStep = render.DecodeSteps
				return m, nil
			}
			// Settled home: arrows move the cursor; any other key activates the
			// selection (every item lands on the shop today).
			switch k.String() {
			case "up", "k":
				if m.homeSel > 0 {
					m.homeSel--
				}
			case "down", "j":
				if m.homeSel < screens.ItemCount()-1 {
					m.homeSel++
				}
			default:
				m.screen = scrMenu
			}
			return m, nil
		}
```

(d) Replace the splash `View` call:

```go
		sp := m.splash.WithDecode(m.decodeStep).WithFrame(m.frame).WithSelection(m.homeSel).View()
```

(remove the old `screens.Taglines[...]` indexing; it no longer exists.)

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/tui -run 'TestStartsOnSplash|TestSplashHolds|TestFlow' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go internal/tui/flow_test.go
git commit -m "feat(tui): drive glitch-decode on the splash tick

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 4: Full verification + restart

**Files:** none (verification only)

- [ ] **Step 1: Format**

Run: `gofmt -w internal/tui/render/decode.go internal/tui/screens/splash.go internal/tui/app.go`

- [ ] **Step 2: Vet + build**

Run: `go vet ./... && go build ./...`
Expected: no output, clean build.

- [ ] **Step 3: Full test suite**

Run: `go test ./...`
Expected: all packages `ok`.

- [ ] **Step 4: Restart the SSH server** (standing instruction — restart after every change)

```bash
lsof -ti :2222 | xargs kill -9 2>/dev/null; pkill -f cmd/sshd 2>/dev/null; sleep 1
go run ./cmd/sshd   # run in background
sleep 2; lsof -nP -iTCP:2222 -sTCP:LISTEN
```
Expected: a `LISTEN` line on `127.0.0.1:2222`.

- [ ] **Step 5: Manual smoke (user)**

`ssh localhost -p 2222` — confirm: glitch chars resolve L→R into the wordmark in ~0.8s, no fake logs, settles to cup + `go to shop`, a key during decode jumps straight to the home landing.

---

## Self-Review

- **Spec coverage:** concept (decode) → Task 1; pacing `DecodeSteps=13` → Task 1; degradation truecolor/flat/Kitty → Task 1 (`DecodeWordmark` branches) + Task 1 test `TestDecodeKittySettles`; delete fake logs → Task 2; settle-to-home holds + skip-on-key → Task 3; tests → Tasks 1-3. All covered.
- **Placeholder scan:** none — every code/test step is concrete.
- **Type consistency:** `DecodeWordmark(caps, step, frame)`, `DecodeSteps`, `glitchChars`, `Splash.decodeStep`, `WithDecode(step)`, `Model.decodeStep` used identically across tasks. Kitty test toggles the existing `KittyFlag`.
