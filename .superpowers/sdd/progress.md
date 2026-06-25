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
