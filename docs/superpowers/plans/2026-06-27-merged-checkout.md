# Merged Checkout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Collapse the cart + checkout screens into one review/checkout page with restaurant-style qty steppers that edit the live Swiggy cart in place and gate order placement on sync confirmation.

**Architecture:** The `Checkout` screen (`screens/checkout.go`) gains an editable item list (cursor + `− ×N +` stepper, reusing the restaurant style). The root (`app.go`) routes `c` straight to `scrCheckout`, drives the live list from the authoritative `m.lines` (not the flattened `liveCart.Lines`), and freezes all input while a reduce/delete sync is in flight (`cartMutating`), clearing it on `CartSyncedMsg`.

**Tech Stack:** Go 1.26, bubbletea/lipgloss, `go test`/`go vet`/`gofmt`. Worktree: `/Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live` (branch `worktree-swiggy-live`). Build via `./scripts/build.sh`.

## Global Constraints

- `internal/tui/screens` must NOT import `internal/tui`.
- Bill constants (`DeliveryFee=29`, `CouponAmount=50`) stay duplicated across `app.go` and `screens/cart.go`; keep in sync, don't cross-import.
- Live `+`/`−`/`delete` edit `m.lines` (authoritative, carries `AddOns`/`Selections`); `liveCart` is bill-only (`billFromLive()`).
- Freeze (block all input) applies ONLY to reduce/delete; `+` is optimistic (no freeze).
- No real order placed by implementation/tests — mock backends (`liveFake`) only.
- After each task: `gofmt -w` changed files, `go vet ./internal/tui/...`, `go test ./internal/tui/...` all green, then rebuild via `./scripts/build.sh` (invoke with the worktree-absolute path).
- All git/test/build commands use worktree-absolute paths, e.g. `git -C <worktree> …`, `go -C <worktree> test …`, `<worktree>/scripts/build.sh` — the shell cwd resets to the main repo between calls.

---

### Task 1: Checkout screen renders an editable item list with steppers

Add the item list + cursor + restaurant-style stepper to the `Checkout` summary view, plus the builders the root needs. Pure screen change; no behavior wired yet.

**Files:**
- Modify: `internal/tui/screens/checkout.go`
- Test: `internal/tui/screens/checkout_test.go`

**Interfaces:**
- Consumes: `CartLine` (`screens/cart.go`), `components.List`, `components.Row`, `theme.*`, `AddOnSummary` (`screens/cart.go`).
- Produces:
  - `func (c Checkout) WithCursor(i int) Checkout`
  - `func (c Checkout) WithLiveSync(live bool, syncErr string) Checkout`
  - `func (c Checkout) WithMutating(m bool) Checkout`
  - `func (c Checkout) Cursor() int`
  - `func (c Checkout) Up() Checkout`
  - `func (c Checkout) Down() Checkout`
  - The summary view now renders one row per line with a `− ×N +` stepper on the focused row and the line total right-aligned.

- [ ] **Step 1: Write the failing test**

Add to `internal/tui/screens/checkout_test.go`:

```go
func TestCheckoutRendersStepperOnFocusedLine(t *testing.T) {
	lines := []CartLine{
		{Item: catalog.Item{ID: "i1", Name: "Iced Americano", Price: 169}, Qty: 2},
		{Item: catalog.Item{ID: "i2", Name: "Cold Brew", Price: 260}, Qty: 1},
	}
	c := NewCheckout("Blue Tokai", catalog.Address{Line: "HSR", Label: "home"}, lines, "~30 min").
		WithLiveSync(true, "").WithCursor(0)
	v := c.View(0)
	if !strings.Contains(v, "Iced Americano") || !strings.Contains(v, "Cold Brew") {
		t.Fatalf("checkout must list every line:\n%s", v)
	}
	// Focused line (cursor 0) shows the − ×N + stepper and its line total.
	if !strings.Contains(v, "×2") || !strings.Contains(v, "−") || !strings.Contains(v, "+") {
		t.Fatalf("focused line missing − ×N + stepper:\n%s", v)
	}
	if !strings.Contains(v, "₹338") { // 169 × 2
		t.Fatalf("focused line missing line total ₹338:\n%s", v)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go -C /Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live test ./internal/tui/screens/ -run TestCheckoutRendersStepperOnFocusedLine -v`
Expected: FAIL — `WithLiveSync`/`WithCursor` undefined (compile error) or stepper absent.

- [ ] **Step 3: Add fields + builders to Checkout**

In `internal/tui/screens/checkout.go`, extend the struct (after `bill Bill`):

```go
type Checkout struct {
	restaurant string
	addr       catalog.Address
	lines      []CartLine
	placed     bool
	orderID    string
	eta        string
	placing    bool
	bill       Bill
	cursor     int
	liveMode   bool
	syncErr    string
	mutating   bool
}
```

Add builders + cursor moves (place after `WithPlacing`):

```go
// WithCursor sets the focused line index (clamped in View).
func (c Checkout) WithCursor(i int) Checkout { c.cursor = i; return c }

// Cursor returns the focused line index.
func (c Checkout) Cursor() int { return c.cursor }

// WithLiveSync marks the page live and carries the last sync error (drives the
// bill's syncing/error state, same as the old cart screen).
func (c Checkout) WithLiveSync(live bool, syncErr string) Checkout {
	c.liveMode = live
	c.syncErr = syncErr
	return c
}

// WithMutating marks a reduce/delete sync as in flight (freezes the CTA + line).
func (c Checkout) WithMutating(m bool) Checkout { c.mutating = m; return c }

func (c Checkout) clampCursor() int {
	i := c.cursor
	if i >= len(c.lines) {
		i = len(c.lines) - 1
	}
	if i < 0 {
		i = 0
	}
	return i
}

// Up / Down move the line cursor.
func (c Checkout) Up() Checkout   { c.cursor--; c.cursor = c.clampCursor(); return c }
func (c Checkout) Down() Checkout { c.cursor++; c.cursor = c.clampCursor(); return c }
```

- [ ] **Step 4: Render the item list in summaryView**

In `summaryView()`, after the `from`/`pay` label block and BEFORE the bill block (`if c.bill.Live { … }`), insert the item list. Replace the existing bill-block lead-in so the list precedes it:

```go
	// Item list: one row per line, full-bleed cursor bar. The focused row shows
	// the restaurant-style − ×N + stepper; others show a plain ×N. Customized
	// lines keep a faint "+ <add-ons>" summary after the name.
	if len(c.lines) > 0 {
		cur := c.clampCursor()
		list := components.List{Cursor: cur}
		for i, l := range c.lines {
			name := theme.BrightStyle.Render(l.Item.Name)
			if s := AddOnSummary(l.AddOns); s != "" {
				name += theme.FaintStyle.Render("  + " + s)
			}
			total := theme.PriceStyle.Render(fmt.Sprintf("₹%d", l.UnitPrice()*l.Qty))
			var right string
			if i == cur {
				updating := ""
				if c.mutating {
					updating = "  " + theme.DimStyle.Render("updating…")
				}
				stepper := theme.FavStyle.Render("−") + " " +
					theme.GreenStyle.Render(fmt.Sprintf("×%d", l.Qty)) + " " +
					theme.GreenStyle.Render("+")
				right = stepper + updating + "    " + total
			} else {
				right = theme.DimStyle.Render(fmt.Sprintf("×%d", l.Qty)) + "    " + total
			}
			list.Rows = append(list.Rows, components.Row{Left: name, Right: right, BarGreen: i == cur})
		}
		b.WriteString(list.View())
	}
```

(The existing bill block — `if c.bill.Live { renderBill… } else { … }` — stays directly after this and is unchanged, EXCEPT add a live syncing/error branch mirroring the cart: change the `else` to handle `c.liveMode` like `cart.go` does. Replace the existing bill block with:)

```go
	switch {
	case c.bill.Live:
		b.WriteString(renderBill(w, c.bill))
	case c.liveMode:
		b.WriteString(components.DashRule())
		if c.syncErr != "" {
			b.WriteString("  " + theme.FavStyle.Render("couldn't sync — "+c.syncErr) + "\n")
		} else {
			b.WriteString("  " + theme.DimStyle.Render("syncing cart…") + "\n")
		}
		b.WriteString(components.DashRule())
	default:
		b.WriteString(components.DashRule())
		b.WriteString("  " + justify(
			theme.BrightStyle.Render("to pay (COD)"),
			theme.BrightStyle.Render(fmt.Sprintf("₹%d", c.toPay())), w) + "\n")
		b.WriteString(components.DashRule())
	}
```

Update the place-order bar label (the `barLabel` block) to also reflect mutating:

```go
	barLabel := " > place order "
	switch {
	case c.placing:
		barLabel = " placing order… "
	case c.mutating:
		barLabel = " syncing… "
	}
```

Update the hint line at the end of `summaryView()` from the old `↵ place order · esc back` to include editing keys:

```go
	b.WriteString(components.Hint("↑↓", "move", "←→", "qty", "⌫", "remove", "↵", "place order", "esc", "back"))
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go -C /Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live test ./internal/tui/screens/ -run TestCheckoutRendersStepperOnFocusedLine -v`
Expected: PASS

- [ ] **Step 6: Run the screens suite + vet**

Run: `gofmt -w /Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live/internal/tui/screens/checkout.go && go -C /Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live vet ./internal/tui/screens/ && go -C /Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live test ./internal/tui/screens/`
Expected: `ok  console.store/internal/tui/screens`

- [ ] **Step 7: Commit**

```bash
git -C /Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live add internal/tui/screens/checkout.go internal/tui/screens/checkout_test.go
git -C /Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live commit -m "feat(tui): checkout renders editable item list with steppers

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 2: Route `c` to the merged checkout; retire the cart screen from nav

Make `openCartCmd` open `scrCheckout` (built with lines/addr/eta/bill/live state), point `esc` from checkout back to the menu, and rebuild checkout on `CartLoadedMsg`. The `scrCart` enum stays defined but is no longer navigated to.

**Files:**
- Modify: `internal/tui/app.go` (`openCartCmd`, `CartLoadedMsg` handler, `scrCheckout` `esc`, `buildCheckout` helper)
- Test: `internal/tui/app_test.go`, `internal/tui/statusbar_keybinds_test.go`

**Interfaces:**
- Consumes: `Checkout.WithCursor/WithLiveSync/WithMutating` (Task 1), `m.lines`, `m.cartHeader()`, `m.addr`, `m.cartEta()`, `m.billFromLive()`, `m.cartSyncErr`, `datasource.LoadCart`.
- Produces:
  - `func (m Model) buildCheckout() screens.Checkout`
  - `openCartCmd` now sets `m.screen = scrCheckout` and fetches the live cart.

- [ ] **Step 1: Write the failing test**

Add to `internal/tui/app_test.go` (live model helper pattern from `live_test.go`):

```go
func TestRestaurantCGoesStraightToCheckout(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	place := catalog.Place{ID: "r1", SwiggyID: "r1", Name: "Blue Tokai",
		Items: []catalog.Item{{ID: "i1", SwiggyID: "i1", Name: "Latte", Price: 250}}}
	snap.SetMenu(place)
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "acct-1", ""), WithSeededSnapshot())
	m.w, m.h = 100, 40
	m.screen = scrRestaurant
	m.rest = screens.NewRestaurant(place, map[string]int{}, "").WithAddr(m.addr)
	m.lines = []screens.CartLine{{Item: catalog.Item{ID: "i1", Name: "Latte", Price: 250}, Qty: 1}}
	m.cartRestaurant = "Blue Tokai"

	nm, _ := m.Update(key("c"))
	m = nm.(Model)
	if m.screen != scrCheckout {
		t.Fatalf("`c` must open the merged checkout directly, got screen %v", m.screen)
	}
}
```

(`key` helper exists in the package tests; if `app_test.go` lacks the live imports, add `swiggysnap "console.store/internal/catalog/swiggy"`, `"console.store/internal/tui/render"`, `"console.store/internal/tui/screens"`, `"console.store/internal/catalog"`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go -C /Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live test ./internal/tui/ -run TestRestaurantCGoesStraightToCheckout -v`
Expected: FAIL — screen is `scrCart`, not `scrCheckout`.

- [ ] **Step 3: Add buildCheckout + repoint openCartCmd**

In `internal/tui/app.go`, add near `buildCart`:

```go
// buildCheckout assembles the merged checkout screen. The live item list is
// driven by the authoritative m.lines (carries add-on/variant selections);
// liveCart feeds only the bill via billFromLive().
func (m Model) buildCheckout() screens.Checkout {
	return screens.NewCheckout(m.cartHeader(), m.addr, m.cartScreenLines(), m.cartEta()).
		WithBill(m.billFromLive()).
		WithLiveSync(m.live, m.cartSyncErr).
		WithMutating(m.cartMutating).
		WithCursor(m.checkout.Cursor())
}
```

Note: `m.cartScreenLines()` returns `liveCart.Lines` when `m.cartLoaded`. Change it so the merged page uses `m.lines` in live mode (the authoritative list). Edit `cartScreenLines` to drop the live-cart branch for the item LIST:

```go
func (m Model) cartScreenLines() []screens.CartLine { return m.lines }
```

(Delete the `if m.live && m.cartLoaded { … liveCart.Lines … }` block — the bill still comes from `liveCart` via `billFromLive`. The flattened display copy is no longer used for the list.)

Repoint `openCartCmd`:

```go
// openCartCmd opens the merged checkout screen and, in live mode, fetches the
// real Swiggy cart so the bill reflects exactly what Place Order will charge.
func (m *Model) openCartCmd() tea.Cmd {
	m.cartLoaded = false
	m.checkout = m.buildCheckout()
	m.screen = scrCheckout
	if !m.live {
		return nil
	}
	rest := m.cartRestaurant
	if rest == "" {
		rest = m.rest.PlaceData().Name
	}
	if rest == "" {
		return nil
	}
	return datasource.LoadCart(m.backend, m.addr.ID, rest)
}
```

Add the `cartMutating` field to the `Model` struct (near `cartLoaded bool`):

```go
	cartMutating bool // true while a reduce/delete cart sync is in flight (freezes input)
```

- [ ] **Step 4: Rebuild checkout on CartLoadedMsg + fix esc**

In the `CartLoadedMsg` handler, replace the `if m.screen == scrCart { m.cart = m.buildCart() }` with:

```go
		if m.screen == scrCheckout {
			m.checkout = m.buildCheckout()
		}
```

In the `scrCheckout` key switch, change the `esc` case target from `scrCart` to `scrMenu`:

```go
			case "esc":
				m.screen = scrMenu
				return m, nil
```

- [ ] **Step 5: Update the two tests that asserted the old scrCart step**

Run `grep -n "scrCart" internal/tui/app_test.go internal/tui/statusbar_keybinds_test.go` in the worktree. For each assertion that expected `c` to land on `scrCart`, change the expected screen to `scrCheckout`. (These are substring/enum equality checks; update the expected enum only — do not change unrelated cases.)

- [ ] **Step 6: Run test to verify it passes**

Run: `go -C /Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live test ./internal/tui/ -run TestRestaurantCGoesStraightToCheckout -v`
Expected: PASS

- [ ] **Step 7: Full tui suite + vet**

Run: `gofmt -w /Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live/internal/tui/app.go && go -C /Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live vet ./internal/tui/... && go -C /Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live test ./internal/tui/...`
Expected: all `ok`. If a flow test still drives through `scrCart` (e.g. sends an extra `enter` to reach checkout), remove that now-stale intermediate step so it lands on `scrCheckout` after `c`.

- [ ] **Step 8: Commit**

```bash
git -C /Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live add internal/tui/app.go internal/tui/app_test.go internal/tui/statusbar_keybinds_test.go
git -C /Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live commit -m "feat(tui): route c to merged checkout, retire cart screen from nav

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 3: Live +/-/delete editing on checkout with the freeze gate

Wire the stepper keys on `scrCheckout`: `↑↓` move, `+` optimistic increment, `−`/`delete` reduce/remove with a full-input freeze until `CartSyncedMsg`, and `enter` blocked while mutating.

**Files:**
- Modify: `internal/tui/app.go` (`scrCheckout` key handler, `CartSyncedMsg` handler)
- Test: `internal/tui/live_test.go`

**Interfaces:**
- Consumes: `m.lines`, `m.liveCartCmd()`, `m.buildCheckout()`, `m.checkout.Cursor/Up/Down`, `m.cartMutating`, `datasource.CartSyncedMsg`, `datasource.PlaceOrderCmd`, `m.liveSyncCart()`.
- Produces: editing behavior on `scrCheckout`; `cartMutating` lifecycle.

- [ ] **Step 1: Write the failing tests**

Add to `internal/tui/live_test.go`:

```go
func checkoutModel(t *testing.T) Model {
	t.Helper()
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "acct-1", ""), WithSeededSnapshot())
	m.w, m.h = 100, 40
	m.screen = scrCheckout
	m.cartRestaurant = "Blue Tokai"
	m.lines = []screens.CartLine{
		{Item: catalog.Item{ID: "i1", Name: "Latte", Price: 250}, Qty: 2},
	}
	m.checkout = m.buildCheckout()
	return m
}

func TestCheckoutIncrementOptimistic(t *testing.T) {
	m := checkoutModel(t)
	nm, cmd := m.Update(key("+"))
	m = nm.(Model)
	if m.lines[0].Qty != 3 {
		t.Fatalf("+ should bump qty to 3, got %d", m.lines[0].Qty)
	}
	if m.cartMutating {
		t.Fatal("+ is optimistic and must NOT freeze input")
	}
	if cmd == nil {
		t.Fatal("+ must return a sync cmd")
	}
}

func TestCheckoutReduceFreezesUntilSynced(t *testing.T) {
	m := checkoutModel(t)
	// − reduces qty and freezes.
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("-")})
	m = nm.(Model)
	if m.lines[0].Qty != 1 {
		t.Fatalf("− should reduce qty to 1, got %d", m.lines[0].Qty)
	}
	if !m.cartMutating {
		t.Fatal("− must freeze input (cartMutating) until confirmed")
	}
	if cmd == nil {
		t.Fatal("− must return a sync cmd")
	}
	// While frozen, another key is ignored.
	nm, _ = m.Update(key("+"))
	m2 := nm.(Model)
	if m2.lines[0].Qty != 1 {
		t.Fatalf("input must be frozen while mutating; qty changed to %d", m2.lines[0].Qty)
	}
	// Sync confirmation clears the freeze.
	nm, _ = m.Update(datasource.CartSyncedMsg{})
	m = nm.(Model)
	if m.cartMutating {
		t.Fatal("CartSyncedMsg must clear cartMutating")
	}
}

func TestCheckoutEnterBlockedWhileMutating(t *testing.T) {
	m := checkoutModel(t)
	m.cartMutating = true
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if m.placingOrder {
		t.Fatal("enter must be a no-op while mutating")
	}
	if cmd != nil {
		t.Fatal("enter must not start the place sequence while mutating")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go -C /Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live test ./internal/tui/ -run 'TestCheckout(IncrementOptimistic|ReduceFreezesUntilSynced|EnterBlockedWhileMutating)' -v`
Expected: FAIL — keys not handled on scrCheckout; qty unchanged.

- [ ] **Step 3: Implement the scrCheckout editing handler**

In `app.go`, replace the `scrCheckout` key `switch` block. The current block is:

```go
		case scrCheckout:
			switch k.String() {
			case "esc":
				m.screen = scrMenu
				return m, nil
			case "enter":
				if m.live && !m.placingOrder {
					m.placingOrder = true
					m.orderErr = ""
					return m, tea.Sequence(m.liveSyncCart(), datasource.PlaceOrderCmd(m.backend, m.snap, m.addr.ID))
				}
				if !m.live {
					m.checkout = m.checkout.Placed(orderID(m.checkout.Lines()), "~40 min")
					m.screen = scrConfirm
				}
				return m, nil
			}
```

Replace with:

```go
		case scrCheckout:
			// Freeze: while a reduce/delete sync is in flight, ignore all keys
			// until CartSyncedMsg clears cartMutating (race guard).
			if m.cartMutating {
				return m, nil
			}
			switch k.String() {
			case "esc":
				m.screen = scrMenu
				return m, nil
			case "up", "k":
				m.checkout = m.checkout.Up()
				return m, nil
			case "down", "j":
				m.checkout = m.checkout.Down()
				return m, nil
			case "+", "=", "right", "l":
				// Optimistic increment of the focused line — no freeze.
				i := m.checkout.Cursor()
				if i >= 0 && i < len(m.lines) {
					m.lines[i].Qty++
					m.menu = m.menu.WithCartChip(m.cartChip())
					m.checkout = m.buildCheckout()
					return m, m.liveCartCmd()
				}
				return m, nil
			case "-", "_", "left", "h":
				// Reduce (remove at qty 1) — freeze until confirmed.
				i := m.checkout.Cursor()
				if i < 0 || i >= len(m.lines) {
					return m, nil
				}
				if m.lines[i].Qty <= 1 {
					m.lines = append(m.lines[:i], m.lines[i+1:]...)
				} else {
					m.lines[i].Qty--
				}
				return m.afterCheckoutReduce()
			case "delete", "backspace":
				// Remove the whole line — freeze until confirmed.
				i := m.checkout.Cursor()
				if i < 0 || i >= len(m.lines) {
					return m, nil
				}
				m.lines = append(m.lines[:i], m.lines[i+1:]...)
				return m.afterCheckoutReduce()
			case "enter":
				if m.live && !m.placingOrder {
					m.placingOrder = true
					m.orderErr = ""
					return m, tea.Sequence(m.liveSyncCart(), datasource.PlaceOrderCmd(m.backend, m.snap, m.addr.ID))
				}
				if !m.live {
					m.checkout = m.checkout.Placed(orderID(m.checkout.Lines()), "~40 min")
					m.screen = scrConfirm
				}
				return m, nil
			}
```

Add the shared reduce helper near `buildCheckout`:

```go
// afterCheckoutReduce finalizes a reduce/delete on the checkout: releases the
// restaurant binding if the cart is now empty, sets the freeze, rebuilds the
// page, and fires the cart sync (UpdateCart, or flush when empty).
func (m Model) afterCheckoutReduce() (tea.Model, tea.Cmd) {
	if len(m.lines) == 0 {
		m.cartRestaurant = ""
		m.cartSection = ""
	}
	m.cartMutating = true
	m.menu = m.menu.WithCartChip(m.cartChip())
	m.checkout = m.buildCheckout()
	return m, m.liveCartCmd()
}
```

- [ ] **Step 4: Clear the freeze in the CartSyncedMsg handler**

In the `CartSyncedMsg` handler (the `dm.Err`/else block that sets `m.liveCart = dm.Cart`), clear the freeze and rebuild the checkout. Replace:

```go
			if dm.Err != nil {
				m.cartSyncErr = "cart sync: " + dm.Err.Error()
			} else {
				m.cartSyncErr = ""
				m.liveCart = dm.Cart // real Swiggy pricing for an accurate bill
			}
			return m, nil
```

with:

```go
			if dm.Err != nil {
				m.cartSyncErr = "cart sync: " + dm.Err.Error()
			} else {
				m.cartSyncErr = ""
				m.liveCart = dm.Cart // real Swiggy pricing for an accurate bill
			}
			m.cartMutating = false // confirmed — unfreeze (success or error)
			if m.screen == scrCheckout {
				m.checkout = m.buildCheckout()
			}
			return m, nil
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go -C /Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live test ./internal/tui/ -run 'TestCheckout(IncrementOptimistic|ReduceFreezesUntilSynced|EnterBlockedWhileMutating)' -v`
Expected: PASS (all three)

- [ ] **Step 6: Full suite + vet + build**

Run: `gofmt -w /Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live/internal/tui/app.go && go -C /Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live vet ./internal/tui/... && go -C /Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live test ./internal/tui/... && /Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live/scripts/build.sh`
Expected: all `ok`; build prints `✓ safestore` + `✓ store`.

- [ ] **Step 7: Commit**

```bash
git -C /Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live add internal/tui/app.go internal/tui/live_test.go
git -C /Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live commit -m "feat(tui): live +/-/delete editing on checkout with sync freeze gate

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Self-Review

**Spec coverage:**
- Merged page / nav retire scrCart → Task 2. ✓
- Item list with steppers → Task 1. ✓
- Live list from `m.lines`, bill from `liveCart` → Task 2 (`cartScreenLines` → `m.lines`; `billFromLive` unchanged). ✓
- `+` optimistic; `−`/`delete` freeze; `enter` blocked while mutating → Task 3. ✓
- Freeze cleared on `CartSyncedMsg` (ok + err) → Task 3 Step 4. ✓
- Empty-cart releases restaurant binding + flush → Task 3 `afterCheckoutReduce` + existing `liveCartCmd` flush. ✓
- Bill syncing/error state on the merged page → Task 1 Step 4. ✓
- Mock bill math preserved → Task 1 default branch. ✓
- Tests for stepper/edit/freeze/enter-block → Tasks 1 & 3. ✓

**Placeholder scan:** none — every code step has full code.

**Type consistency:** `WithCursor/WithLiveSync/WithMutating/Cursor/Up/Down` defined in Task 1 and consumed in Tasks 2–3; `buildCheckout`/`afterCheckoutReduce`/`cartMutating` defined in Tasks 2–3 consistently; `cartScreenLines` returns `[]screens.CartLine` throughout.
