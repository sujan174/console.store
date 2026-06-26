## Review-fix report

### F1 — Search results navigable and openable

**File:** `internal/tui/app.go`  
**Function:** `Update` → `scrMenu` search-mode key block (~line 1702)

**Changes:**
- Added `searchSubmitted string` field to `Model` (tracks last-submitted query).
- Restructured the search-mode key handler: printable runes (`k.Type == tea.KeyRunes`) always append to `searchQuery` — `j`/`k`/etc. are letters, not nav, in this path.
- `up`/`k` (named keys) move cursor up when `searchQuery == searchSubmitted` (results loaded).
- `down`/`j` (named keys) move cursor down when results loaded.
- `enter`: if `searchQuery == searchSubmitted` (results present), opens the selected place via the same `scrRestaurant` / `LoadMenu` path the Home/category list uses; otherwise submits the query and sets `searchSubmitted = searchQuery`.
- `esc` clears both `searchQuery` and `searchSubmitted`.

**Tests added (`internal/tui/menu_flow_test.go`):**
- `TestSearchResultsNavigableAndOpenable`: enter search → type "tokai" → Enter submits (sets `searchSubmitted`) → `↓` moves cursor (clamped on 1 result) → Enter opens `scrRestaurant`.
- `TestCategoryViewNoNearbyHeader` (covers F2, below).

**Commands run:**
```
go build ./...       # clean
go test ./...        # all green
gofmt -l <files>     # empty
go vet ./...         # clean
```

---

### F2 — Category view no longer shows "nearby" header

**File:** `internal/tui/app.go`  
**Function:** `buildMenu` (~line 291)

**Changes:**
- Removed `menu = menu.WithSections(nil, catPlaces)` for the category branch.
- Category view now falls through to `mainPlaces()` default (flat `places`), which `NewMenu` already receives as `viewPlaces` (= catPlaces). The `twoPaneView()` renders via the plain flat-list path with no section headers.
- Home view (`WithSections(usuals, nearby)`) is untouched.

**Test:** `TestCategoryViewNoNearbyHeader` asserts `"nearby"` is absent and `"Blue Tokai"` is present after selecting the Coffee category.

---

### F3 — Usuals load no longer fires redundantly

**File:** `internal/tui/app.go`  
**Function:** `Update` → `AddressesLoadedMsg` handler (~line 1214)

**Changes:**
- Changed `m.usualsLoaded = false` to `m.usualsLoaded = true` before dispatching `LoadUsuals`.
- The comment explains: `liveInitCmds` already fired `LoadUsuals` on startup; the `AddressesLoadedMsg` handler is the second (address-scoped) dispatch. Setting the flag to true prevents `ensureHomeLoaded` from firing a third time when the user navigates Home.
- On address change (the only legitimate re-load trigger), the handler now owns the dispatch and the flag stays true — no double-fire.

No new tests (behavioral: prevents extra network calls, not observable in unit tests without a spy).

---

### F4 — `a` / `tab` work when rail is focused

**File:** `internal/tui/app.go`  
**Function:** `Update` → rail-focused key block (~line 1763)

**Changes:**
- Added `case "a":` — unfocuses rail, opens address screen (`scrAddress`).
- Added `case "tab":` — unfocuses rail, switches vertical to Instamart.

Both are consistent with the main-list block's handling.

No new tests (single-case key routing, covered by the overall test suite's rail focus tests).

---

### F5 — Dead code removed

**Files:**
- `internal/tui/screens/menu_test.go`: removed unused `itoa` helper function.
- `internal/tui/app.go`: removed `railCatLabels()` method; its single call site (rail-focused block) now inlines the two-line label-build directly, matching `buildMenu`'s existing inline.

---

### Full suite result

```
go test ./...   →   all packages PASS (no failures)
go build ./...  →   clean (no errors)
gofmt -l        →   empty (no files need formatting)
go vet ./...    →   clean
```

Touched files: `internal/tui/app.go`, `internal/tui/menu_flow_test.go`, `internal/tui/screens/menu_test.go`.  
Not touched: `internal/tui/live.go`, `internal/tui/screens/menu.go` (no changes needed).
