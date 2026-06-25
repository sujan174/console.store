# SDD progress — live Swiggy broker

Slice 1: internal/store (Postgres+RLS+KMS)
Plan: docs/superpowers/plans/2026-06-25-store-postgres-rls-kms.md
BASE: b7faf13c50aacaf4f5edff78efebec530a886a5c
Builders: Sonnet · Slice reviewer: Opus
- [x] Slice 1 implementer dispatched
- [x] Slice 1 Opus review clean (1 Critical search_path + Important FORCE RLS + Minor race fixed in 10218ac, re-verified green)
Slice1 review: 1 Critical (search_path), 1 Important (FORCE RLS), 1 Minor (ON CONFLICT race). Dispatching fixer.
Slice 1: complete (commits b7faf13..10218ac, review clean after fix wave 1)

Slice 2: internal/swiggy (MCP client)
Plan: docs/superpowers/plans/2026-06-25-swiggy-mcp-client.md
BASE: c251d35c262604b3295cf816a46abaa7602f58be
- [x] Slice 2 implementer dispatched
- [x] Slice 2 Opus review clean (2 Important money-safety gaps + minors fixed in 5e7f620, re-verified)
Slice 2: complete (commits c251d35..5e7f620, review clean after fix wave 1)

Slice 3: internal/auth (OAuth/PKCE/DCR/flow)
Plan: docs/superpowers/plans/2026-06-25-auth-oauth-pkce-dcr.md
BASE: 6aaffc9a6bb6cff22d6d9e38dc8e32e04ec2e864
- [x] Slice 3 implementer dispatched
- [x] Slice 3 Opus review clean (Important state-claim replay/race fixed in 3d8ca35; NOTE: broker must make pubkey+token binding transactional)
Slice 3: complete (commits 6aaffc9..3d8ca35, review clean after fix wave 1)
CARRY TO BROKER: binding ordering FindOrCreateAccount->LinkPubkey->PutToken can half-bind if PutToken fails; make store binding transactional in broker.

Slice 4: internal/broker + cmd/broker (RPC composition root)
Plan: docs/superpowers/plans/2026-06-25-broker-rpc.md
BASE: 11ef8821cc9585800fb63036ac2775ef9fa18a57
- [ ] Slice 4 implementer dispatched
- [ ] Slice 4 Opus review clean

Slice 4: built (commits 11ef882..d56c913), 5 tests -race pass, api-isolation verified (no swiggy/store/auth leak into TUI-shared api pkg).
  REVIEW DEFERRED to final whole-branch review (user rejected per-slice review).
  OPEN CONCERN to review later: broker RPC trusts AccountID as a plain arg from the socket caller. Acceptable ONLY if the TUI derives accountID from its OWN SSH pubkey via AccountForPubkey(session.PublicKey) and NEVER lets the user supply it. => Slice 5 MUST resolve accountID from the SSH session pubkey, not user input. Socket is local 0600; TUI is the trusted intermediary.
- [ ] Slice 4 review: DEFERRED to final

Slice 5: catalog/swiggy + tui/datasource + sshd live wiring
Plan: docs/superpowers/plans/2026-06-25-tui-live-wiring.md
BASE: 5dd2d0d8016a0bc7414c781423b32dcb575bdf98
- [x] Slice 5 implementer dispatched
- [ ] Slice 5 review: DEFERRED to final

Slice 5: built (commits 5dd2d0d..820b4bf), all tests pass (catalog/swiggy, tui/datasource, tui live_test), full repo green.
  Task 1: internal/catalog/swiggy — Snapshot (RW-mutex, map-keyed) + Repository (sync reads, empty on miss) ✓
  Task 2: internal/tui/datasource — Backend interface, LoadAddresses/LoadPlaces/LoadMenu Cmds, api→catalog mapping ✓
  Task 3: internal/tui/datasource/broker_backend.go — BrokerBackend pinned to session accountID (never from input), sectionQuery ✓
  Task 4: internal/tui/live.go + app.go — WithLiveBackend Option, variadic New, Init dispatches live loads, Update handles datasource Msgs + needsAuth gate, View authorize gate + r-retry ✓
            cmd/sshd/main.go — CONSOLE_BACKEND=live, liveModel resolves accountID from SSH pubkey via api.Client.AccountForPubkey, WithPublicKeyAuth(accept-all), falls back to mock on any error ✓
  OPEN CONCERN (slice-4 carry) CLOSED: broker RPC accountID is now derived from SSH pubkey via AccountForPubkey, never from user/client input ✓

Slice 6: config seed + nav load
Plan: docs/superpowers/plans/2026-06-25-config-seed-nav-load.md
BASE: 8c419a8279c6949bbe6036f996d99655f2d2e2ab
S6 Task 1 (internal/config JSON package): complete (commits 8c419a8..1c24356, review clean)
S6 Task 2 (New() addr-after-opts + WithSeededSnapshot): complete (commits 1c24356..71206cd, review clean)
S6 Task 3 (sshd config load + Snapshot seed): complete (commits 71206cd..a3dc2bd, review clean)
S6 Task 4 (scrMenu enter fires LoadMenu live): complete (commits a3dc2bd..b6b6dd9, review clean)
Slice 6 COMPLETE.

Slice 7: live cart sync + order placement
Plan: docs/superpowers/plans/2026-06-25-live-cart-order.md
BASE: b6b6dd9
S7 Task 1 (datasource UpdateCart/PlaceOrder + SyncCart/PlaceOrderCmd): complete (commits b6b6dd9..18ed7bb, review clean after comment fix)
  NOTE: implementer also updated broker_backend.go + broker_backend_test.go + live_test.go to satisfy new interface; Task 2 BrokerBackend methods already landed here.
S7 Task 2 (BrokerBackend account-pinning tests for UpdateCart/PlaceOrder): complete (commits 18ed7bb..2702564, review clean)
S7 Task 3 (Checkout.WithPlacing builder): complete (commits 2702564..bc9dd6c, review clean)
S7 Task 4 (app.go eager cart sync + live PlaceOrder + Msg handlers): complete (commits bc9dd6c..2b385b9, review clean after test fix)
  Minor noted: last-item removal doesn't clear Swiggy cart (liveSyncCart guards len==0) — expected, spec silent, future slice.
Slice 7 COMPLETE.
Final whole-branch review: 0 Critical, 2 Important (I1: ErrNeedsAuth gate unreachable; I2: cart-screen edits not synced at checkout).
Final fix (843ac5d): wrapAuthErr seam in BrokerBackend + tea.Sequence re-sync at checkout chokepoint. Re-review: approved.
Branch READY TO MERGE.

Slice 8: Restaurants IA redesign (chips + category filter + vertical shell)
Plan: docs/superpowers/plans/2026-06-25-restaurants-ia-redesign.md
BASE: c9d5d43baa07db43be7963aa121581992fbfc962
Builders: Sonnet · Reviewers: Sonnet (scaled per diff)
S8 Task 1 (thread Category swiggy->api->catalog): complete (commits c9d5d43..70e14b8, review clean)
  Minor (defer to final): api/dto.go dropped the `// has variants or add-ons` comment on Customizable; Category field und­ocumented at api layer.
S8 Task 2 (config cuisine chips + defaults): complete (commits 70e14b8..ab3eb05, review clean)
S8 Task 3 (key places by chip query; PlacesQuery path): complete (commits ab3eb05..fdc0ba6, review clean — account-pinning verified)
  Minor (defer): datasource.go SectionCoffee display-placeholder intent comment lives in spec not code.
S8 Task 4 (restaurant category bar + veg + dish filter): complete (commits fdc0ba6..0963454, review clean — cursor/selection desync risk verified absent)
  Minor (defer): WithCategory/WithVegOnly clear active search (UX choice); stepCategory clamps not wraps (matches plan code).
S8 Task 5 (browse chips render): complete (commits 0963454..d8cc11e, review clean)
  Minor (defer): WithChips doesn't clamp active (safe by render-loop equality; Task 6 owns nav).
  NOTE for Task 6: chips render ADDITIVELY above the old section-tab row; live mode must not show both.
S8 Task 6 (vertical toggle + chip wiring + Instamart placeholder): complete (commits d8cc11e..5747c37, review clean after fix wave)
  Fix wave (5747c37): hideSectionTabs (live shows chips only, not section tabs); chip-nav guarded by !Searching(); nil-cfg ChipCategories confirmed nil-safe.
S8 Task 7 (verification): complete.
  - Full build green; go vet clean; go test ./... all 17 pkgs ok.
  - gofmt clean on all 24 slice-touched files. (Pre-existing gofmt violations in cmdbar.go/cmdbar_test.go/app_test.go are NOT from this slice — out of scope.)
  - Restarted local broker + sshd on new code (docker postgres healthy). Live UI smoke is the user's step.
  - No order placed; CONSOLE_LIVE_ORDERS untouched.
Slice 8 (Restaurants IA redesign) COMPLETE — pending final whole-branch review.

Final whole-branch review (c9d5d43..648a77a, Opus): READY TO MERGE. All 4 cross-task risks clean (Category path end-to-end, cache re-keying coherent, account-pinning upheld, no double-bar/panic). Logged Minors all deferred. One NEW Minor: vertical-toggle had no on-screen affordance (spec §1/§2).
Final fix (107fa09): rendered "Restaurants · Instamart soon  tab ·" toggle row at top of live browse (gated len(chipLabels)>0; mock unchanged); fixed stale NextCategory "wraps" comment to "clamps". Tests green, gofmt/vet clean. sshd restarted on latest code.
Slice 8 (Restaurants IA redesign) COMPLETE & READY TO MERGE. Pending: user live SSH smoke, then merge worktree-swiggy-live.
