# console.store — Live Swiggy Broker · Design Spec

**Date:** 2026-06-25
**Status:** Design / for approval. Supersedes the "planned backend" framing in
`CLAUDE.md` and the `[commitment]` items in `docs/security.md` with a concrete,
buildable architecture.
**Scope:** Take console.store from "TUI + in-memory mock" to a production-grade
app that brokers **real** Swiggy orders over the live MCP APIs, with Postgres +
RLS + KMS token storage and a full GitHub Actions CI/CD pipeline.

---

## 0. Locked decisions (from brainstorming)

| Decision | Choice |
|----------|--------|
| Order write-path | Build fully; a **real** order call fires only when `CONSOLE_LIVE_ORDERS=1` **and** the existing COD confirm is accepted. Default build/CI/dev never spends money. |
| Token store | **Real Postgres + Row-Level Security + KMS envelope encryption** now (not a deferred commitment). |
| CI/CD | GitHub Actions: `gofmt` check, `go vet`, `golangci-lint`, `go test ./...` (race), build matrix, release artifact. No live deploy job. |
| Live verification | Driven by the **user** — the assistant is harness-blocked from completing a Swiggy login (it mints a real user token). I build + write tests with mocks/fakes; the user runs the one interactive authorize and we verify live together. |
| Servers | Food + Instamart only. Dineout excluded. Scope `mcp:tools`. |

---

## 1. The central constraint and how we resolve it

`catalog.Repository` is **synchronous, returns no `error`, takes no `context`**,
and is called *inside the bubbletea render/Update path* (e.g.
`screens.NewMenu(m.repo.Places(...))` runs on a keystroke). A live impl is
network I/O: it can block, fail (401/419/403/5xx), and is per-user
authenticated. We cannot put a blocking HTTPS call behind that interface without
freezing the UI, and we cannot return errors through it.

**Resolution — split read into "snapshot cache (sync)" + "fetch (async tea.Cmd)":**

- The TUI keeps reading the **same `catalog.Repository` interface, unchanged**,
  backed by a per-session **in-memory snapshot** that the Swiggy impl populates.
- A new, small **async layer** (bubbletea `tea.Cmd`s + `tea.Msg`s) fetches from
  the broker and writes results into the snapshot, then triggers a re-render.
- Cache miss returns the zero value (empty list / `ok=false`) — exactly what the
  screens already handle as "nothing here yet" — and kicks a background fetch.

This means **`catalog.Repository`'s signature does not change**, but the root
`Model` gains async-load message handling and lightweight loading/empty states.
The handoff's aspiration of "screens change zero" holds for the *data seam*; the
root model (`app.go`) does gain async wiring and a few loading indicators. That
is unavoidable and is called out here explicitly rather than pretended away.

**Writes** (cart mutate, coupon, place order, tracking) are *actions*, already
naturally asynchronous in a TUI, so they are plain `tea.Cmd`s returning result
messages — no interface contortion needed. They get a **new seam** (see §3.4);
they were never part of `catalog.Repository`.

---

## 2. Process topology & trust boundary

security.md requires the **TUI to never hold a Swiggy token**. Two viable
shapes:

- **(A) Separate broker process** — TUI ↔ broker over a local internal RPC
  (Unix socket / gRPC); broker is the sole token holder + Postgres/KMS client.
  Strongest isolation; matches the security.md diagram literally.
- **(B) In-process broker package** — broker is a Go package the `sshd` server
  calls directly; the token never leaves server memory and never reaches the
  *client*. Simpler; one binary.

**Recommendation: (A) separate `cmd/broker` process, Unix-domain socket.** It is
what security.md commits to ("TUI process never holds Swiggy tokens and never
calls Swiggy — it talks only to the internal broker"), gives a real privilege
boundary (broker runs as its own user, owns the DB/KMS creds; the SSH-facing TUI
does not), and keeps the most-exposed component free of credentials. The socket
is local-only, file-permission-gated.

```
[ssh client] --ssh--> [cmd/sshd: wish TUI]  --unix socket (internal RPC)-->  [cmd/broker]
                       (no tokens, no DB)                                     |   |   |
                                                                  Swiggy MCP <+   |   +-> KMS (envelope key)
                                                                  (food/im)       +-> Postgres + RLS
```

---

## 3. Subsystems (build units, dependency order)

Each is an independently-testable package with one clear purpose. Build order is
top-to-bottom; each ships with its own tests before the next starts.

### 3.1 `internal/store` — Postgres + RLS + KMS token store
- Schema: `accounts(id, phone, created_at)`, `ssh_pubkeys(account_id, pubkey)`,
  `swiggy_tokens(account_id, ciphertext, nonce, dek_wrapped, expires_at)`.
- **RLS**: every read/write sets `SET LOCAL app.current_account = $id`; policies
  restrict rows to the current account. Broker connects with a **non-superuser,
  RLS-bound** role (RLS does not apply to superusers — enforced + tested).
- **KMS envelope encryption**: a `KMS` interface with `Encrypt(plaintext)`/
  `Decrypt(blob)` for the data-encryption key. Two impls:
  - `kms/aws` — AWS KMS (`GenerateDataKey` / `Decrypt`) for production.
  - `kms/local` — a local dev provider sealing the DEK with a master key from an
    env-injected secret (never in repo), so the stack runs locally without AWS.
  Token plaintext is encrypted with the DEK (AES-256-GCM); only the **wrapped**
  DEK + ciphertext + nonce are stored. Plaintext token exists in memory only
  during a single Swiggy call.
- Local stack via `docker-compose.yml` (Postgres) + migrations (`migrate` or
  embedded SQL). KMS defaults to `kms/local` in dev.

### 3.2 `internal/swiggy` — Swiggy MCP client
- Reuses the proven transport from `cmd/swiggyprobe` (DCR, MCP streamable-HTTP
  `initialize` → `notifications/initialized` → `tools/call`, SSE `data:` parse).
- Typed wrappers for the **15 Food + 11 Instamart** harvested tools (schemas in
  `docs/superpowers/research/`). `context.Context` + real `error` on every call.
- **Auth-failure mapping** (security.md §5): 401 → silent re-auth, 419 → full
  re-auth + purge, 403 → surface, no blind retry.
- **Verify-before-retry** (security.md §6): before retrying any non-idempotent
  call (`place_food_order`, Instamart `checkout`), fingerprint the order and
  check `get_food_orders` / `get_orders`; suppress the retry if a match exists.
- **`CONSOLE_LIVE_ORDERS` gate**: `place_food_order` / `checkout` refuse to call
  Swiggy unless the env flag is set; otherwise return a typed `ErrOrdersDisabled`
  the TUI renders as "live orders off".

### 3.3 `internal/auth` — OAuth 2.1 + PKCE + DCR + session binding
- DCR (public client, no secret), PKCE `S256`, per-session `state` CSRF.
- **Cross-device authorize UX**: broker generates the authorize URL; TUI shows it
  as a link **and a QR code** in-terminal; user logs in on their phone; the
  broker's local callback listener (`http://localhost:8765/cb`) completes the
  exchange; the terminal session continues (polls broker for "authorized").
- **Account binding**: SSH pubkey ↔ account ↔ phone. Phone comes from the access
  token JWT `phone` claim if present (OPEN ITEM — decode + confirm); fallback =
  key on `sub` + a one-time console.store OTP.
- **No refresh tokens in v1** — on expiry, re-run authorize (proactive, ~60s
  before `expires_in`). `refresh_token` is available from Swiggy but
  intentionally unused; recorded in security.md.

### 3.4 `internal/broker` + `cmd/broker` — the orchestrator & RPC server
- Owns `store`, `swiggy`, `auth`. Exposes an **internal RPC** over a Unix socket.
- RPC surface, two shapes:
  - **Reads** (snapshot fill): `GetAddresses`, `GetPlaces(section)`,
    `GetMenu(placeID)`, `GetUsual`, `GetTrending`, `GetInstamartItems` — map 1:1
    onto what the TUI snapshot needs, themselves backed by Swiggy tool calls +
    a short broker-side cache.
  - **Writes/actions**: `UpdateCart`, `ApplyCoupon`, `PlaceOrder`, `Track`,
    `GetOrders`, `Logout`.
- **Redaction** logging middleware from day one (tokens/JWT/phone never logged).

### 3.5 `internal/catalog/swiggy` + TUI async layer — fill the seam
- `swiggy.Repository` implements `catalog.Repository` by reading the **session
  snapshot** (sync, no error — per §1).
- New `internal/tui/datasource` (or methods on `Model`): `tea.Cmd`s that call the
  broker RPC, plus `tea.Msg` types (`addressesLoadedMsg`, `placesLoadedMsg`,
  `orderResultMsg`, `authStateMsg`, …) handled in `app.go` `Update` to fill the
  snapshot + drive loading/empty/error states.
- `mem` repo stays as the **offline/test** Repository (and CI default), selected
  by config — so `go test` and local dev work with zero network.

### 3.6 `cmd/sshd` wiring
- Add a broker-client; inject the Swiggy-backed Repository + datasource into the
  per-session `Model`. Preserve the existing color-profile / OSC 11 wiring.
- A config flag selects **mock vs live** Repository (`CONSOLE_BACKEND=mock|live`).

---

## 4. Security mapping (every security.md item → where it lands)

| security.md | Realized by |
|-------------|-------------|
| §2 Delegated auth, PKCE, DCR, state | `internal/auth` (3.3) |
| §3 Encrypted at rest, RLS, in-mem only, no refresh, logout purge | `internal/store` (3.1) + `internal/auth` |
| §4 HTTPS only, redaction | `internal/swiggy` (HTTPS) + broker log middleware |
| §5 401/419/403 recovery | `internal/swiggy` failure map (3.2) |
| §6 Verify-before-retry, COD-only, confirm gate | `internal/swiggy` (3.2) + existing TUI confirm `[built]` |
| §7 Least privilege (Food+IM, `mcp:tools`, scoped DB role) | `auth` scope + `store` RLS role |
| §8 SSH host key, per-session isolation, broker isolation | existing `[built]` + process split (§2) |
| §10 KMS, host key, nothing secret in repo | `store/kms` + `.gitignore` + redaction |

After build, `docs/security.md` items flip `[commitment]` → `[built]`.

---

## 5. CI/CD (GitHub Actions)

- **`ci.yml`** on PR + push to main: `gofmt -l` (fail if non-empty), `go vet`,
  `golangci-lint`, `go test -race ./...` with a **Postgres service container** so
  `store` RLS tests run for real; `kms/local` provider in CI (no AWS creds).
- **`release.yml`** on tag: cross-compiled `cmd/sshd` + `cmd/broker` build
  matrix, checksummed artifacts attached to the GitHub release.
- Live Swiggy is **never** called in CI (no token, real money). The `swiggy`
  package is tested against a **fake MCP server** (httptest) replaying the
  harvested schemas + canned tool responses.

---

## 6. Testing strategy

- `store`: integration tests vs a real Postgres (CI service container + local
  docker-compose); assert RLS actually blocks cross-account reads; KMS round-trip.
- `swiggy`: unit tests vs httptest fake MCP server (DCR, SSE, tool calls,
  401/419/403 paths, verify-before-retry suppression).
- `auth`: PKCE/state generation + callback handling vs fake authz server.
- `broker`: RPC contract tests over a socket pair.
- `tui`: existing `teatest` flows keep running against the `mem` repo; add flows
  for loading/empty/error states and the QR authorize screen.
- **No live calls in any automated test.** One manual, user-driven live smoke
  run (documented runbook) before the Builders Club demo.

---

## 7. Build order (each = its own plan + subagent execution)

1. **`store`** (Postgres + RLS + KMS) — foundation, no deps.
2. **`swiggy`** (MCP client + typed tools + failure map + retry guard).
3. **`auth`** (OAuth/PKCE/DCR + binding + cross-device UX).
4. **`broker` + `cmd/broker`** (wire 1–3, RPC server).
5. **`catalog/swiggy` + TUI async layer + `cmd/sshd` wiring** (fill the seam).
6. **CI/CD** (Actions, docker-compose, release).
7. **Docs**: flip security.md `[commitment]`→`[built]`; record decisions; runbook.

Each step: spec is this doc's section → `writing-plans` → subagent build → tests
green → review → commit. We do **not** one-shot all six.

---

## 8. Open risks / items to confirm during build

- **`phone` JWT claim** present? Decode a real access token (user-run) to confirm;
  fallback path designed (OTP on `sub`).
- **localhost = sandbox or live?** Unconfirmed (`builders@swiggy.in`). Until
  confirmed, treat every order as real → the `CONSOLE_LIVE_ORDERS` gate is the
  safety net.
- **Token expiry vs Swiggy 30-day session** — proactive re-auth timing needs a
  real token's `expires_in` to tune.
- **MCP `tools/call` response shapes** — harvested `tools/list` gives input
  schemas; *output* shapes confirmed only against live/fake during `swiggy` build.
- "Screens change zero" is **not** strictly true — `app.go` gains async load
  handling + loading states. Acknowledged, scoped to the root model.
