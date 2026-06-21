# Swiggy Builders Club — Application Package

**Date:** 2026-06-21
**Goal:** Get `console.store` applied to, and approved by, the Swiggy Builders Club program.
**Scope of this spec:** the *application*. Backend implementation is explicitly out of scope here (see "Post-approval milestone").

## Context

`console.store` is a terminal-native (SSH) curated food/snack ordering shop, Tokyo Night themed, that will broker orders through Swiggy's Builders Club MCP servers (Food + Instamart). **Today only the TUI + an in-memory mock data layer exist** (`internal/tui`, `internal/catalog`). None of the planned backend (`auth`, `swiggy`, `account`, `session`, `store`) is built.

This is not a blocker for applying. Swiggy gates access in stages and reviews a builder's *security approach and concept*, not shipped backend code.

## How Swiggy gates access (verified against the live program docs)

```
apply ──▶ quick review ──▶ staging access ──▶ (staging integration green) ──▶ production
```

- **Apply:** submit project concept, integration details, redirect URI(s), server selection, and **a demo video link** showing the agent working end-to-end. Review judges *use case, security approach, program fit*.
- **DCR:** no `client_id` to apply for — the client self-registers via Dynamic Client Registration (RFC 7591) once it has access.
- **Staging first:** production is granted only after the staging integration is "green."

Consequence: the application requires **zero backend code**. It requires documents + a recording. The hardened backend is the *next* gate, built against real (staging) endpoints — not blind, now.

Source: https://mcp.swiggy.com/builders/ , `/builders/docs/start/developer/`, `/builders/docs/start/authenticate/`, `/builders/docs/start/enterprise/delegated-auth/`. Repo design intent: `docs/auth.md`, `docs/swiggy-integration.md`.

## The application package — three deliverables

### 1. Security-approach document (the core reviewed artifact)

A written statement of how console.store will meet every Swiggy delegated-auth obligation. These are **commitments / design**, not code to build now. Built later at the staging gate.

| # | Swiggy requirement | console.store commitment |
|---|---|---|
| 1 | OAuth 2.1 + PKCE **S256**, fresh verifier/challenge per session | Server-side broker generates a fresh verifier+challenge and `state` per pending-auth session; never reused. |
| 2 | DCR (`POST /auth/register`, RFC 7591) | Broker self-registers its client identity once; console.store has no pre-shared `client_id`. |
| 3 | `state` CSRF token per session | Random `state` stored with the pending-auth record; verified on callback; mismatch rejected. |
| 4 | Tokens: **secure per-user storage, never shared, never plaintext beyond lifetime** | Encrypted at rest (per-env **KMS** data key) **+ Postgres Row-Level Security** so a DB role can only read its own user's row — defense in depth: even an app-logic bug cannot leak another user's token. Strictly one token per account; never shared. |
| 5 | Never log tokens / JWTs / PII; **HTTPS only** | Redaction in the logging layer from day one; tokens live only in the encrypted store and in-memory per call; all transport HTTPS. |
| 6 | Error handling: **401** expired → re-auth; **419** session revoked → full re-auth; **403** insufficient scope | Swiggy client maps all three. (Repo docs previously covered only 401/-32001 — 419 + 403 added here.) |
| 7 | No refresh tokens in v1 — re-run authorize on expiry; proactively re-auth ~60s before `expires_in` | Re-auth pushes the authorize link to the user's phone; silent inside Swiggy's 30-day session; proactive trigger before the 5-day token expiry. |
| 8 | Drop tokens on logout + `POST /auth/logout` | Explicit logout path revokes the Swiggy session and purges the stored token immediately. |
| 9 | Redirect URI: exact-match **HTTPS** (localhost allowed for dev) | **OPEN ITEM** — URI to be decided (own-domain `https://auth.console.store/cb` vs `http://localhost` for dev). Registered via builders@swiggy.in. |
| 10 | Don't request servers you don't need | Request **Food + Instamart only**; Dineout explicitly skipped in v1. |
| 11 | Never touch password/OTP/raw PII; orders are non-cancellable | console.store never sees credentials (auth is on Swiggy's phone flow). Non-idempotent `place_food_order`/`checkout` guarded by **verify-before-retry** (`get_*_orders` fingerprint match) so a 5xx can never create a double order. |

### 2. Demo video (asciinema)

- **Tool:** asciinema — records the SSH session to a lightweight `.cast`, uploaded to asciinema.org for a shareable link (the "demo video link" the form wants). Optionally convert to mp4.
- **Run-sheet** (single clean take) — drives the existing TUI end-to-end:
  1. `ssh localhost -p 2222` → splash boot sequence
  2. Menu → browse sections (coffee / food / snacks)
  3. Open a restaurant → add items (qty up/down)
  4. `c` → cart review → checkout (COD notice)
  5. Confirm → order placed → `t` tracking
  6. `:` command palette → Instamart → add → cart → checkout
- Narration/caption track explaining the curation concept + that fulfillment will run through Swiggy MCP.
- Mock data is acceptable: at application time there is no API access yet; the video demonstrates the *product concept and UX*.

### 3. Integration details (for the form)

- **Servers:** Food (`/food`) + Instamart (`/im`). No Dineout.
- **Scope:** `mcp:tools`.
- **Redirect URI:** open item #9.
- **Concept blurb:** curated, per-city terminal-native ordering over Swiggy MCP; COD; staging-first.

## Post-approval milestone (OUT OF SCOPE for this spec — parked)

Built only after staging access is granted, against real endpoints:

- `internal/auth` — PKCE broker, DCR, callback, token exchange, pending-auth store + TUI poll.
- `internal/swiggy` — MCP client over Streamable HTTP; per-call Bearer; 401/419/403 handling; backoff; idempotency guard.
- `internal/account` + `internal/session` — pubkey↔account↔phone; per-SSH state.
- `internal/store` — Postgres + **RLS** + KMS-encrypted token store; migrations.
- Swap `catalog/mem` for a Postgres+Swiggy `catalog.Repository` impl (zero screen changes by design).
- Staging verification checklist (`docs/auth.md`): cross-device authorize, `phone` claim present, silent re-auth <30d, DCR returns usable identity.

## Explicitly out of scope (now)

- **CI** — buildable today but not an approval gate; deferred (separate task).
- Any backend code.
- Production hosting / domain procurement (tied to open item #9).

## Open items

1. **Redirect URI** (req #9): own-domain HTTPS vs localhost-dev. Decide before form submission.
2. Confirm `phone` claim is present on the Builders JWT (staging) — fallback is keying on `sub` + one console.store OTP.
3. Tool-count gap: Swiggy documents 14 Food / 13 Instamart tools; v1 flows use 13 / 11. Confirm the remainder are unneeded during staging.

## Success criteria

- A reviewable security-approach document covering reqs 1–11.
- A shareable asciinema demo link of the end-to-end TUI flow.
- Integration details + redirect URI decided.
- Application submitted at `/builders/access/` (or builders@swiggy.in).
