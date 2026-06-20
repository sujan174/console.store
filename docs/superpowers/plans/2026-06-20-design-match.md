# Design-Match Rebuild Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Rebuild the TUI presentation layer to match the approved Claude Design mock (`docs/design/console.store.reference.html`) as faithfully as a truecolor terminal allows — exact palette, layout, copy, all 8 screens + chrome, full motion.

**Architecture:** Keep the `catalog` data seam untouched (its mock data already matches the design). Overhaul `theme` to the exact design palette + surfaces. Add a tick-driven animation loop to the root `Model`. Restyle every screen and add four new surfaces: splash (boot+logo), bottom status bar, `:` command palette, and order tracking. Keymap matches the design exactly.

**Tech Stack:** Go 1.22+, bubbletea (with `tea.Tick`), lipgloss (truecolor + `lipgloss.Width`).

**Source of truth:** `docs/design/console.store.reference.html`. Every screen's exact markup, colors, copy, and selected-row styling is in there — implementers READ the cited line ranges, they are not re-transcribed here. The embedded `<script>` (lines 481-949) is the complete state machine + data; port its logic directly.

---

## Terminal-fidelity rules (apply everywhere)

The design uses CSS effects a terminal cannot render. Translate them consistently:

| Design CSS | Terminal translation |
|---|---|
| `text-shadow:0 0 Npx …` (glow) | drop — just the base color, bold for emphasis |
| `linear-gradient(90deg,#1f2335,#1c2030)` selected row | solid `Background(SelRowBg)` = `#1f2335` |
| `border-left:2px solid <c>` full-bleed row | a `▌` left-bar in color `<c>`, row spans inner width |
| scanlines / vignette / `flick` / blur | drop entirely |
| `box-shadow` modal | a lipgloss `RoundedBorder()` in `Div2` color |
| emoji (☕ 🛵 ⌂ 🎉) | keep — modern terminals render them; they are in the design |
| braille spinner `⠋⠙⠹…` | keep — animate via tick |

Selected-row contract (used by menu, restaurant, cart, address, tracking-none): full inner width (60 cols), `Background(SelRowBg)`, `▌` left bar in the row's accent (blue cursor `#7aa2f7` for selection; green `#9ece6a` for in-cart-not-selected), text `Bright`. Non-selected: 2-space indent, `Text` color, a space where the bar would be.

---

### Task 1: Theme overhaul — exact design palette + surfaces

**Files:**
- Modify: `internal/tui/theme/tokyonight.go` (full rewrite)
- Modify: `internal/tui/theme/tokyonight_test.go` (update colour assertions)

Read `docs/design/console.store.reference.html` lines 176-189 (root style) and scan the inline `color:#…`/`background:#…` usages across 191-477 to confirm each token's meaning.

- [ ] **Step 1: Rewrite the palette + styles**

```go
package theme

import "github.com/charmbracelet/lipgloss"

// Tokyo Night — exact hexes from the approved design (docs/design/console.store.reference.html).
const (
	BgDeep   = "#0a0a10" // html/body, overlay base
	Bg       = "#15161f" // app background
	PanelHi  = "#191a24" // address modal surface
	PanelLo  = "#10111a" // status bar / cmd output surface
	PanelCmd = "#0e0f17" // command palette surface
	SelRowBg = "#1f2335" // selected row background (gradient start; solid here)
	Div      = "#232539" // section dividers / screen top borders
	Div2     = "#2c2e44" // dashed bill rules, modal border
	Text     = "#a9b1d6" // default
	Bright   = "#c0caf5" // headings, selected text
	Dim      = "#565f89" // secondary / labels
	Faint    = "#3b3b5a" // deep hints / idle bullets
	Cursor   = "#7aa2f7" // blue — cursor, nav, links-back
	Price    = "#7dcfff" // cyan — prices
	Green    = "#9ece6a" // eta / new / success / in-cart
	Gold     = "#e0af68" // cart chip / active category / cup
	Fav      = "#f7768e" // red — errors, decrement, cancel
	Purple   = "#bb9af7" // "the usual", command prompt
)

func fg(hex string) lipgloss.Style { return lipgloss.NewStyle().Foreground(lipgloss.Color(hex)) }

var (
	BrandStyle  = fg(Bright).Bold(true)
	TextStyle   = fg(Text)
	ItemStyle   = fg(Text)
	BrightStyle = fg(Bright)
	DimStyle    = fg(Dim)
	FaintStyle  = fg(Faint)
	CursorStyle = fg(Cursor).Bold(true)
	PriceStyle  = fg(Price)
	EtaStyle    = fg(Green)
	NewStyle    = fg(Green)
	GreenStyle  = fg(Green)
	CartStyle   = fg(Gold)
	GoldStyle   = fg(Gold)
	CatOnStyle  = fg(Gold)
	CatOffStyle = fg(Dim)
	FavStyle    = fg(Fav)
	PurpleStyle = fg(Purple)
	KeyHintStyle = fg(Faint)
	HintKeyStyle = fg(Dim) // the key glyph inside a hint line (↑↓, ↵, esc)

	// SelRowStyle: selected full-bleed row background.
	SelRowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(Bright)).Background(lipgloss.Color(SelRowBg))
)
```

- [ ] **Step 2: Update theme test** to assert the new constants exist with these exact values (e.g. `if Bg != "#15161f"`, `if Purple != "#bb9af7"`, `if SelRowBg != "#1f2335"`). Keep one test that `SelRowStyle` renders with a background. Run `go test ./internal/tui/theme/`.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/theme/
git commit -m "feat(theme): exact design palette + surfaces (splash/panel/cmd/status)"
```

---

### Task 2: Animation tick + cursor-blink infrastructure

The design animates a 10-frame braille spinner at 110ms, streams boot lines at 340ms, advances tracking every 4200ms, blinks cursors. Model this with one `tea.Tick` at 110ms driving a frame counter; derive slower cadences by modulo.

**Files:**
- Modify: `internal/tui/app.go` (add tick plumbing)
- Test: `internal/tui/app_test.go`

Reference: `<script>` lines 536 (spinFrames), 582-593 (intervals), 920 (trackPct).

- [ ] **Step 1: Add a tick message + command at top of app.go**

```go
import "time"

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(110*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// spinFrames is the braille spinner (design line 536).
var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
```

- [ ] **Step 2: Add `frame int` to Model; start the tick in Init; advance on tickMsg**

In `Model` add `frame int`. Change `Init`:
```go
func (m Model) Init() tea.Cmd { return tick() }
```
At the TOP of `Update`, before the key switch, handle the tick (and keep the loop alive):
```go
	if _, ok := msg.(tickMsg); ok {
		m.frame++
		m = m.onTick()
		return m, tick()
	}
```
Add a method `onTick()` that advances boot/tracking state (filled in by Tasks 4 and 10; for now `func (m Model) onTick() Model { return m }`). Add a helper:
```go
func (m Model) spin() string { return spinFrames[m.frame%len(spinFrames)] }
// blinkOn reports the on-phase of a ~1s cursor blink.
func (m Model) blinkOn() bool { return (m.frame/5)%2 == 0 }
```

- [ ] **Step 3: Test the spinner advances**

```go
func TestTickAdvancesFrame(t *testing.T) {
	m := New()
	f0 := m.frame
	updated, cmd := m.Update(tickMsg(time.Now()))
	m = updated.(Model)
	if m.frame != f0+1 {
		t.Errorf("frame = %d, want %d", m.frame, f0+1)
	}
	if cmd == nil {
		t.Error("tick must reschedule itself")
	}
}
```
(Add `"time"` to the test imports.) Run `go test ./internal/tui/ -run TestTick`.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go
git commit -m "feat(tui): tick-driven animation loop (spinner, blink, boot/track cadence)"
```

---

### Task 3: Shared chrome — divider, full-bleed selrow, status bar, hint line

**Files:**
- Create: `internal/tui/components/chrome.go`
- Modify: `internal/tui/components/list.go` (selrow → full-bleed with left bar + accent)
- Test: `internal/tui/components/chrome_test.go`, `list_test.go`

Reference: status bar lines 459-463; dividers `border-top:1px solid #232539` (e.g. line 241); selected-row style line 845.

- [ ] **Step 1: Add chrome.go**

```go
package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"console.store/internal/tui/theme"
)

// InnerWidth is the content column width all screens render to.
const InnerWidth = 60

// Divider is the full-width section rule under a screen header (design: 1px #232539).
func Divider() string {
	return theme.FaintStyle.Foreground(lipgloss.Color(theme.Div)).Render(strings.Repeat("─", InnerWidth)) + "\n"
}

// DashRule is the dashed bill separator (design: 1px dashed #2c2e44).
func DashRule() string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Div2)).Render(strings.Repeat("╌", InnerWidth)) + "\n"
}

// StatusBar renders the persistent bottom bar (design lines 459-463).
//   ⊙ linked · <addr> · home · <screen>            <hint> · ↑<lat>ms ▋
func StatusBar(addr, screen, hint, latency string, blink bool) string {
	left := theme.GreenStyle.Render("⊙ linked") +
		theme.FaintStyle.Render(" · ") + theme.DimStyle.Render(addr+" · home") +
		theme.FaintStyle.Render(" · ") + theme.DimStyle.Render(screen)
	cur := " "
	if blink {
		cur = theme.CursorStyle.Render("▋")
	}
	right := theme.DimStyle.Render(hint) + theme.FaintStyle.Render(" · ↑"+latency+"ms ") + cur
	gap := InnerWidth - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	bar := left + strings.Repeat(" ", gap) + right
	return lipgloss.NewStyle().Background(lipgloss.Color(theme.PanelLo)).Render(bar)
}

// Hint renders a footer hint line; pass alternating (keyGlyph, label) pairs.
// Keys render in Dim, labels in Faint (design lines 263-265).
func Hint(pairs ...string) string {
	var b strings.Builder
	b.WriteString("  ")
	for i := 0; i+1 < len(pairs); i += 2 {
		b.WriteString(theme.DimStyle.Render(pairs[i]) + " " + theme.FaintStyle.Render(pairs[i+1]))
		if i+2 < len(pairs) {
			b.WriteString("   ")
		}
	}
	return b.String()
}
```

- [ ] **Step 2: Make the List selected row full-bleed with an accent left-bar**

In `list.go`, add an `Accent` field to `List` (default blue) and rewrite the cursor branch of `View()` so the selected row is: `▌`(accent) + space + body padded to `InnerWidth`, with `SelRowStyle` background across the whole thing; non-selected rows: two spaces + body in `ItemStyle`. Add to `Row` an optional `BarGreen bool` (renders a green left-bar when not selected — the in-cart marker, design line 863).

```go
type Row struct {
	Left  string
	Right string
	Tag   string
	Fav   bool
	BarGreen bool // green left-bar when not the cursor row (in-cart)
}

type List struct {
	Rows   []Row
	Cursor int
	Width  int
	filter string
}
```
In `View()` replace the per-row render with:
```go
	width := l.Width
	if width == 0 {
		width = components_InnerWidth // use the package const InnerWidth
	}
	var b strings.Builder
	for i, r := range l.VisibleRows() {
		right := r.Right
		if r.Tag != "" {
			right += "  " + theme.NewStyle.Render(r.Tag)
		}
		if r.Fav {
			right += "  " + theme.FavStyle.Render("♥")
		}
		pad := width - lipgloss.Width(r.Left) - lipgloss.Width(right)
		if pad < 1 {
			pad = 1
		}
		body := r.Left + strings.Repeat(" ", pad) + right
		if i == l.Cursor {
			bar := theme.CursorStyle.Render("▌")
			bw := lipgloss.Width(body)
			if bw < width {
				body += strings.Repeat(" ", width-bw)
			}
			b.WriteString(bar + theme.SelRowStyle.Render(" "+body+" ") + "\n")
		} else {
			lead := "  "
			if r.BarGreen {
				lead = theme.GreenStyle.Render("▌") + " "
			}
			b.WriteString(lead + theme.ItemStyle.Render(body) + "\n")
		}
	}
	return b.String()
```
(Use the package's `InnerWidth` const directly — both are in `package components`.)

- [ ] **Step 3: Tests** — `chrome_test.go`: `StatusBar` contains "linked" and the addr; `Hint` contains its labels; `Divider` width == InnerWidth display cells. `list_test.go`: a selected row's rendered output contains the `▌` bar; keep the column-alignment test (update expected width to InnerWidth).

- [ ] **Step 4: Commit**

```bash
git add internal/tui/components/
git commit -m "feat(tui): shared chrome — divider, dash rule, status bar, full-bleed selrow, hints"
```

---

### Task 4: Splash screen — boot stream → ASCII logo → auto-connect

**Files:**
- Create: `internal/tui/screens/splash.go`
- Modify: `internal/tui/app.go` (scrSplash initial screen; onTick drives boot; any key → menu)
- Test: `internal/tui/screens/splash_test.go`, `app_test.go`

Reference: design lines 195-228 (splash markup), 211-216 (ASCII logo — COPY THE GLYPHS EXACTLY), 539-545 (bootLines), 219 taglines, 584-593 (timing: reveal a line every ~3 ticks, then logo, auto-connect).

- [ ] **Step 1: Implement splash.go**

A `Splash` struct rendering two phases by a `bootStep int` and a `spin`/`tagline` string. Boot phase (bootStep < 5): the streamed boot lines (design 539-545, with their colors) plus a spinner line. Logo phase: the 6-line ASCII `CONSOLE` block (design 211-216, EXACT) in `Cursor` blue, the tagline + cyan spinner, `bangalore · coffee, food & snacks · ` + green `247` + ` devs online`, and a `Faint` "press any key to connect" + blue `▋`. Signature:
```go
func NewSplash() Splash
func (s Splash) WithBoot(step int, spin, tagline string) Splash
func (s Splash) View() string
```
Boot lines data (design 539-545) — embed as a package var:
```go
var bootLines = []struct{ Text, Color string }{
	{"guest@laptop ~ % ssh console.store", theme.Text},
	{"  ⊙ resolving console.store … 12.4ms", theme.Dim},
	{"  ⊙ tls handshake … ed25519 ✓", theme.Dim},
	{"  ⊙ auth guest@hsr-layout … ok", theme.Green},
	{"  ⊙ 247 devs online · kitchen warm ☕", theme.Price},
}
```

- [ ] **Step 2: Wire into app.go** — add `scrSplash` as the FIRST const and make `New()` start there with `screen: scrSplash`, `bootStep: 0`. In `onTick()`: while on splash, every 3rd frame increment `bootStep` until `len(bootLines)`, then after a short hold advance to `scrMenu`. Model fields: `bootStep int`, `bootHold int`. In the key handler, when `screen == scrSplash`, ANY key jumps to `scrMenu` (design line 773). `View()` routes `scrSplash` → `m.splash...View()` built from current `bootStep`, `m.spin()`, tagline.

- [ ] **Step 3: Tests** — splash_test: boot phase shows the first boot line; logo phase (step≥5) contains "console.store"-era tagline text and "press any key". app_test: `New()` starts on splash; a key advances to menu; enough ticks auto-advance to menu.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/screens/splash.go internal/tui/app.go internal/tui/screens/splash_test.go internal/tui/app_test.go
git commit -m "feat(tui): animated splash — boot stream, ASCII logo, auto-connect"
```

---

### Task 5: Menu restyle to design

**Files:**
- Modify: `internal/tui/screens/menu.go`
- Modify: `internal/tui/app.go` (menu key handling already arrow-based; ensure usual line + 3 tabs)
- Test: `menu_test.go`

Reference: design lines 231-267 (menu markup), 838-853 (rows + tabs logic). NOTE: design shows the usual as a SEPARATE purple line above the tabs (lines 243-246), NOT a list row. The `u` key adds the usual to cart and stays on menu (design 783, 936) with a flash; do NOT jump to cart.

- [ ] **Step 1: Rewrite menu.go View** to match the design exactly:
  - header: `console.store` (BrandStyle) … right: `cart · ₹{total}` (CartStyle)
  - second line: `{addr.Line}` (DimStyle) … right `[a]` (FaintStyle)
  - `Divider()`
  - usual line (when present): `↵ the usual` (PurpleStyle) + 3 spaces (Faint) + `{usual.Label}` (Text) … right `₹{price}` (PriceStyle). Reference line 244.
  - tabs row: `coffee  food  snacks` with active in `CatOnStyle`, others `CatOffStyle`, gap of 3 spaces (reference 248-252, 853). THREE tabs only.
  - the places `List` (full-bleed selected rows, blue bar).
  - `Hint("↑↓","move","←→","category","↵","open","a","address","c","cart",":","cmd")` — note `:` should render Purple; pass it via a Purple-rendered glyph (simplest: append `PurpleStyle.Render(":")+" "+FaintStyle.Render("cmd")` manually after the Hint call).
  - Revert the usual-as-row change from the prior plan: `NewMenu` builds rows from places ONLY (no usual row); cursor starts at 0; `Selected()` returns `places[SelectedIndex()]`; remove `SelectedUsual()`.

- [ ] **Step 2: app.go menu keys** — match design (lines 775-787): `↑↓`/j/k move; `←/→` cycle coffee→food→snacks (wrap, design 779); `1/2/3` jump to a tab; `u` adds the usual to cart and stays (flash); `a` opens address; `c` cart; `enter` opens selected place; `:` opens command palette (Task 11). Remove the snacks→instamart `→` behaviour (Instamart now lives in the command palette). Keep wrap-around on `←/→` across the 3 tabs.

- [ ] **Step 3: Tests** — menu_test: header shows brand + cart chip; usual line shows "the usual" + label + price; exactly the active tab is gold; `←/→` wraps coffee↔snacks. app_test: update `TestSectionSwitchChangesPlaces` (right from coffee → food) and the usual test (`u` adds to cart, stays on menu, cart total rises — does NOT switch to cart screen).

- [ ] **Step 4: Commit**

```bash
git add internal/tui/screens/menu.go internal/tui/app.go internal/tui/screens/menu_test.go internal/tui/app_test.go
git commit -m "feat(tui): menu restyle — usual line, 3 tabs, full-bleed rows, design palette"
```

---

### Task 6: Restaurant restyle — qty steppers, check/in-cart marks

**Files:**
- Modify: `internal/tui/screens/restaurant.go`
- Test: `restaurant_test.go`

Reference: design lines 270-299 (markup), 855-877 (row logic). Each item row: cursor `❯`(blue, on selected) / space; a check `✓`(green) when in cart; name (Bright if in cart else Text); green tag; right side — when in cart, an inline stepper `−`(red) `×{qty}`(green) `+`(green); then price (cyan). Selected row: blue full-bleed bar. In-cart non-selected: green left-bar (`Row.BarGreen`).

- [ ] **Step 1: Rewrite restaurant.go** to take the current cart quantities so it can render checks/steppers. Change the constructor to `NewRestaurant(p catalog.Place, qtyByItemID map[string]int, cartTotal int)`; build rows with `Right` = stepper+price string when `qty>0` else just price, `Fav`/`Tag` as needed, `BarGreen: qty>0`. The qty stepper is rendered text (no separate click targets needed in TUI): `theme.FavStyle.Render("−") + " " + theme.GreenStyle.Render("×"+itoa(qty)) + " " + theme.GreenStyle.Render("+") + "   "`. Add a check `✓` prefix to the name when in cart.
  - header: `← {name}` (PriceStyle) … `cart · ₹{total}` (CartStyle); eta (EtaStyle); `Divider()`.
  - footer: `Hint("↑↓","move","↵/→","add","←","remove","esc","back","c","cart")`.
  - `Selected()` stays `(catalog.Item, bool)`.

- [ ] **Step 2: app.go** — restaurant keys already match design (enter/→ add, ← remove/dec, esc/← back?, c cart). Per design (790-799): `enter`/`→` add; `←` decrement; `esc` back; `c` cart; `↑↓` move. NOTE design uses `←` for DECREMENT here (not back). Update the scrRestaurant handler: `left` → decrement the highlighted item (not back); `esc` → back. After add/dec, rebuild the restaurant screen with fresh qty map and cart total. Keep the empty-selection guard.

- [ ] **Step 3: Tests** — restaurant_test: an in-cart item row shows `✓` and `×N` and the stepper `−`/`+`; selected row shows the blue bar. app_test: adding then pressing `←` decrements.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/screens/restaurant.go internal/tui/app.go internal/tui/screens/restaurant_test.go internal/tui/app_test.go
git commit -m "feat(tui): restaurant restyle — inline qty steppers, in-cart check + green bar"
```

---

### Task 7: Cart restyle — bill breakdown (delivery + DEVFRIDAY coupon)

**Files:**
- Modify: `internal/tui/screens/cart.go`
- Modify: `internal/tui/app.go` (bill math)
- Test: `cart_test.go`

Reference: design lines 301-334 (markup), 605-606 (`toPay = itemTotal>0 ? item+29-50 : 0`), 940 (bill strings). Header `cart · {place}` (BrandStyle) … `{cartEta}` (EtaStyle). Rows: `❯`+name+`    x{qty}`(Dim) … `₹{lineTotal}`(Price); selected blue bar. Bill (after `DashRule()`): `item total … ₹{item}`, `delivery … ₹29`, `DEVFRIDAY  −₹50`(green both sides) `applied`, `DashRule()`, `to pay (COD) … ₹{total}`(Bright). Empty: `your cart is empty — press esc to browse.` (Dim, with esc in blue). Footer `Hint("↑↓","move","←→","qty","↵","checkout","esc","back")`.

- [ ] **Step 1: Add bill constants + helpers to app.go**

```go
const (
	DeliveryFee  = 29
	CouponCode   = "DEVFRIDAY"
	CouponAmount = 50
)

// toPay applies the design's bill: item + delivery − coupon, or 0 when empty.
func toPay(itemTotal int) int {
	if itemTotal <= 0 {
		return 0
	}
	return itemTotal + DeliveryFee - CouponAmount
}
```
The menu/restaurant cart-chip total stays the ITEM total (design `cartTotal = itemTotal`). Only the cart/checkout "to pay" applies delivery+coupon.

- [ ] **Step 2: Rewrite cart.go View** to render the bill block when it has items, using a passed-in item total + computed to-pay. Change `Cart` to also hold `eta string`. Keep `Up/Down/Left/Right/Inc/Dec/Remove/Lines`. The `cartEta` is `~{second half of place eta}` (design 939) — pass it in from the router.

- [ ] **Step 3: Tests** — cart_test: with one ₹149 item, bill shows `item total ₹149`, `delivery ₹29`, `DEVFRIDAY  −₹50`, `to pay (COD)   ₹128` (149+29−50). Empty shows the empty line. app_test: `cartTotal()` (chip) stays item-only; checkout total uses `toPay`.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/screens/cart.go internal/tui/app.go internal/tui/screens/cart_test.go internal/tui/app_test.go
git commit -m "feat(tui): cart restyle — bill breakdown with delivery + DEVFRIDAY coupon"
```

---

### Task 8: Checkout restyle + Confirmed (coffee cup) restyle

**Files:**
- Modify: `internal/tui/screens/checkout.go`
- Test: `checkout_test.go`

Reference: checkout design 336-355; confirmed design 357-386 (steam ˜˷˜, coffee-cup `╭────────╮` block, `order placed ✓` box `╔══╗` — COPY GLYPHS EXACTLY), 646-651 (order id `#SW…`, eta `~40 min`).

- [ ] **Step 1: Rewrite checkout summary view** (design 336-355): title `checkout`(BrandStyle), `Divider()`, label rows `deliver to`/`from`/`pay` with `Dim` labels (width 8em ≈ 10 cols) and values — `pay` value is `Cash / UPI to rider on delivery` in Gold. `DashRule()`, `to pay (COD) … ₹{total}`(Bright), `DashRule()`, a place-order bar `❯ place order` (full-bleed: `▌`green + SelRow bg), then red `no online payment — pay the rider on delivery`, Dim `orders can't be cancelled once placed`, footer `Hint("↵","place order","esc","back")`.

- [ ] **Step 2: Rewrite confirmed view** (design 357-386): centered-ish block — steam line (3 green chars), the 4-line gold coffee cup, the 3-line green `order placed ✓` box, then `{place} · ETA {eta} · ` + Dim `{orderId}`, Dim `pay ₹{total} to rider (cash/UPI)`, red `can't be cancelled now`, footer `Hint("↵","track","esc","back to menu")` (↵ green, esc blue). Order id format `#SW{4hex}` derived deterministically from the cart (keep the existing deterministic `orderID`, just change the prefix/format to `#SW%04X`).

- [ ] **Step 3: Tests** — checkout_test: summary shows `Cash / UPI to rider`, `to pay (COD)`, `place order`, the non-cancellable + no-online-payment lines. Confirm view shows `order placed`, the cup, the id, `track`.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/screens/checkout.go internal/tui/screens/checkout_test.go
git commit -m "feat(tui): checkout + confirmed restyle — pay-to-rider, coffee-cup ASCII, order box"
```

---

### Task 9: Tracking screen (new) — animated route + steps

**Files:**
- Create: `internal/tui/screens/tracking.go`
- Modify: `internal/tui/app.go` (scrTracking, onTick advances trackStep, enter-on-confirm → tracking)
- Test: `tracking_test.go`, `app_test.go`

Reference: design 388-423 (markup), 637-644 (timing 4200ms/step, 3 steps), 894-901 (steps + marks), 919-922 (trackPct, quips). Steps: `order confirmed / preparing … / out for delivery / delivered`; past=`●`green, current=spinner gold, future=`○`faint. Route bar: `☕ {place}` … `{addr} ⌂`, a progress line with `●`→`⌂` and the 🛵 at `trackPct`. ETA `~32 min`, rider `rider · Imran · KA 05 1234 · {quip}`.

- [ ] **Step 1: Implement tracking.go** — `NewTracking(place, addr, orderID string)` + `View(trackStep int, spin string)`; render header `← tracking · {id}`(Price) … `{place}`(Dim), `Divider()`, the route endpoints line, a 60-col progress bar (`█`/`─` proportion = `min(trackStep,3)/3`, with 🛵 positioned), the 4 step rows with marks, ETA, rider line, footer `Hint("esc","back to menu")`.

- [ ] **Step 2: app.go** — add `scrTracking`, fields `trackStep int`, `trackTick int`. From `scrConfirm`, `enter`/`t` → `scrTracking` with `trackStep=1`; `onTick` advances `trackStep` up to 3 every ~38 frames (≈4.2s). `esc` on tracking → reset cart, back to menu (design 826-828). `View()` routes scrTracking.

- [ ] **Step 3: Tests** — tracking_test: shows the four step labels and the rider line; step marks reflect trackStep. app_test: confirm→enter reaches tracking; ticks advance trackStep (cap 3).

- [ ] **Step 4: Commit**

```bash
git add internal/tui/screens/tracking.go internal/tui/app.go internal/tui/screens/tracking_test.go internal/tui/app_test.go
git commit -m "feat(tui): live order tracking — animated route, rider, step timeline"
```

---

### Task 10: Status bar wired on every non-splash screen

**Files:**
- Modify: `internal/tui/app.go` (compose status bar under each screen's body)
- Test: `app_test.go`

Reference: design 459-463; rotating `statusHints` (line 925); `screenNames` (836); latency `12.4`.

- [ ] **Step 1: In `View()`**, for every screen except splash, append `"\n" + components.StatusBar(addr, screenName, hint, latency, m.blinkOn())`. Add a `screenName()` helper mapping the current screen → label (design 836) and a rotating hint from `["type : for commands","247 devs online","DEVFRIDAY −₹50","ssh console.store"]` indexed by `m.frame/15 % 4`. Latency constant `"12.4"`.

- [ ] **Step 2: Test** — app_test: the menu view contains `⊙ linked` and the address; the splash view does NOT contain the status bar.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go
git commit -m "feat(tui): persistent status bar on all in-app screens"
```

---

### Task 11: Command palette (`:`) + `:instamart`

**Files:**
- Create: `internal/tui/screens/cmdbar.go`
- Modify: `internal/tui/app.go` (cmd state, key capture, command dispatch)
- Test: `cmdbar_test.go`, `app_test.go`

Reference: design 446-457 (markup), 659-736 (runCmd + all commands), 743-751 (key capture). The palette is a bottom overlay (PanelCmd bg) showing prior output lines then `: {text}▋`. Commands: `help/?`, `neofetch`, `coffee/brew`, `whoami`, `streak`, `uptime`, `sudo`, `vim/vi`, `sl`, `42/answer`, `theme`, `exit/quit/:q`, `tip`, `clear/cls`. PLUS one addition not in the web design: `instamart` → opens the Instamart fast-lane screen (the agreed home for that feature).

- [ ] **Step 1: Implement cmdbar.go** — a `CmdBar` holding `text string` and `out []CmdLine{Text,Color string}`; `View(blink bool)` renders the PanelCmd-bg block (output lines, the `:`+text+cursor, and a `type help · ↵ run · esc close` hint). Port the command outputs verbatim from design 670-732 (keep the exact copy — it's the brand voice). Provide `Run(cmd string) (out []CmdLine, closeAfter bool, action string)` where `action` is `""` normally or `"instamart"`/`"clear"`/`"close"` for side-effects.

- [ ] **Step 2: app.go** — add `cmdOpen bool`, `cmd screens.CmdBar`. When NOT splash and key is `:`, open the palette. While `cmdOpen`: `esc` closes; `enter` runs (`action=="instamart"` → close palette + open Instamart; `"clear"` wipes; `"close"`/exit closes after a beat); `backspace` edits; printable runes append (design 743-751). The palette renders ABOVE the status bar. Konami buffer + coffee-rain are out of scope (note in NOTES, skip).

- [ ] **Step 3: Tests** — cmdbar_test: `Run("help")` lists commands; `Run("neofetch")` mentions `guest@console.store`; `Run("instamart")` returns action `instamart`; unknown → `command not found`. app_test: `:` then typing `instamart` then enter routes to the Instamart screen; `:` then `help` then enter shows output and stays.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/screens/cmdbar.go internal/tui/app.go internal/tui/screens/cmdbar_test.go internal/tui/app_test.go
git commit -m "feat(tui): : command palette (neofetch/coffee/whoami/… + :instamart)"
```

---

### Task 12: Instamart restyle to match + reachable only via `:instamart`

**Files:**
- Modify: `internal/tui/screens/instamart.go`
- Modify: `internal/tui/app.go` (remove any leftover `i`/4th-tab entry; entry is the cmd action)
- Test: `instamart_test.go`, `app_test.go`

- [ ] **Step 1: Restyle instamart.go** to the design idiom: header `← instamart` (PriceStyle) … `cart · ₹{imTotal}` (CartStyle); subtitle `~12 min · fast lane` (EtaStyle); `Divider()`; item rows with the same full-bleed selected style + inline steppers/checks as the restaurant (reuse the Row/List); footer `Hint("↑↓","move","↵/→","add","←","remove","c","cart","esc","back")`. Keep the ₹99 minimum + separate `imLines`/`imCart`.

- [ ] **Step 2: app.go** — ensure the ONLY entry to `scrInstamart` is the command action from Task 11 (no `i` key on menu, no 4th tab). The `imCart` checkout still reuses the shared checkout with label `Instamart` and enforces `InstamartMin`. The `←` on the Instamart list decrements (matching restaurant), `esc` → menu.

- [ ] **Step 3: Tests** — instamart_test: header shows `fast lane` + the new styling; app_test: update `TestInstamartSeparateCartAndMinimum` and `TestInstamartOrderIDDerivedFromItsOwnCart` to ENTER via the command palette (`:`, `instamart`, enter) instead of arrow-walking tabs.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/screens/instamart.go internal/tui/app.go internal/tui/screens/instamart_test.go internal/tui/app_test.go
git commit -m "feat(tui): instamart restyle; entry via :instamart command only"
```

---

### Task 13: Address overlay restyle + final pass

**Files:**
- Modify: `internal/tui/screens/address.go`
- Modify: `cmd/sshd/main.go` if needed (ensure truecolor: `tea.WithAltScreen` already set; nothing required)
- Test: `address_test.go`

Reference: design 425-442 (modal markup), 903-911 (rows). Render as a bordered panel (`RoundedBorder` in Div2, PanelHi bg): title `deliver to —` (BrightStyle), rows `❯`+`{line}` … `{label}`(Dim) with selected row highlighted, footer `↑↓ move · ↵ select & reload · esc cancel`.

- [ ] **Step 1: Restyle address.go** to the bordered-panel look (lipgloss `Border(lipgloss.RoundedBorder())` + `BorderForeground(Div2)` + `Background(PanelHi)` padding 1,2). Keep `NewAddress`/`Selected`.

- [ ] **Step 2: Test** — address_test still passes (title + entries + current cursor); add an assertion the output contains the rounded border glyph `╮`.

- [ ] **Step 3: Full verification + commit**

```bash
go build ./... && go vet ./... && go test ./... && gofmt -l internal/
git add internal/tui/screens/address.go
git commit -m "feat(tui): address overlay restyle — bordered panel"
```

---

## Self-Review

**Coverage vs the design (`docs/design/console.store.reference.html`):**
- splash boot+logo → Task 4 ✓ · menu (usual line, 3 tabs, full-bleed) → Task 5 ✓ · restaurant (steppers, checks) → Task 6 ✓ · cart (bill+coupon) → Task 7 ✓ · checkout+confirmed (cup) → Task 8 ✓ · tracking → Task 9 ✓ · status bar → Task 10 ✓ · command palette → Task 11 ✓ · address modal → Task 13 ✓ · exact palette → Task 1 ✓ · animation → Task 2 ✓.
- Instamart: not in the web design; preserved via `:instamart` (Task 11/12) per user decision.
- Dropped (terminal-impossible, documented in the fidelity table): glow, scanlines, vignette, flicker, blur, gradients→solid, konami coffee-rain (noted skip in Task 11).

**Type consistency:** `components.InnerWidth` is the single width source (theme/list/chrome all use it). `Row` gains `BarGreen`. `Restaurant`/`Instamart` constructors take a `qtyByItemID map[string]int`. `Cart` gains `eta`. `toPay` lives in app.go; the cart chip stays item-total. `orderID` format → `#SW%04X`. Animation: single `tickMsg`/`frame`; `onTick` is extended by Tasks 4 & 9.

**Placeholder scan:** none — visual exactness defers to the cited reference line ranges (the agreed source of truth), Go contracts are concrete.

**Note for implementers:** READ the cited `docs/design/console.store.reference.html` line ranges for your screen before coding — copy ASCII glyphs and copy text exactly. The `<script>` block (481-949) is the authoritative behaviour. Keep the `catalog`/`mem` data layer untouched.
