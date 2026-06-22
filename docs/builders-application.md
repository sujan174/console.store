# Swiggy Builders Club — Application

**Goal:** get `console.store` reviewed and approved for Swiggy Builders Club access.
**Scope:** the *application itself*. The security approach it commits to lives in
[security.md](security.md); backend implementation happens later, at the staging
gate.

## How the program gates access

```
apply ──▶ quick review ──▶ get access (staging) ──▶ build & ship ──▶ demo
```

- **Apply:** submit project concept, integration details, redirect URI(s), server
  selection, and a **demo link** showing the agent end-to-end. Review judges *use
  case, security approach, and program fit*.
- **Staging first:** build on localhost / staging with real DCR-issued credentials;
  production access is granted only after the staging integration is green.
- **No `client_id` to request:** the client self-registers via Dynamic Client
  Registration (RFC 7591) once it has access.

Program is invite-led and reviewed in phases. Portal:
<https://mcp.swiggy.com/builders/> · contact: builders@swiggy.in

Consequence: the application needs **documents + a recording, zero backend code**.
The hardened backend is the *next* gate, built against real (staging) endpoints.

## The application package — three deliverables

### 1. Security-approach document

→ [security.md](security.md). This is the core reviewed artifact: threat model,
delegated auth (OAuth 2.1 + PKCE + DCR), token storage (KMS + Postgres RLS),
transport/logging, auth-failure handling, order idempotency, least privilege,
SSH-layer isolation, data handling, and secrets management — stated as commitments.

### 2. Demo video (asciinema)

- **Tool:** [asciinema](https://asciinema.org) — records the SSH session to a
  lightweight `.cast`, uploaded for a shareable link (the "demo link" the form
  wants). Optionally convert to mp4.
- **Run-sheet** (single clean take, drives the existing TUI end-to-end):
  1. `ssh localhost -p 2222` → splash boot sequence
  2. Menu → browse sections (coffee / food / snacks)
  3. Open a restaurant → add items (qty up/down)
  4. `c` → cart review → checkout (COD notice)
  5. Confirm → order placed → `t` tracking
  6. `:` command palette → Instamart → add → cart → checkout
- **Caption track** explaining the curation concept and that fulfilment will run
  through Swiggy MCP.
- **Mock data is acceptable** — at apply time there is no API access; the video
  demonstrates the product concept and UX, not live Swiggy calls.

**Demo link:** _TBD — paste the asciinema.org URL here once recorded._

### 3. Integration details (for the form)

| Field | Value |
|-------|-------|
| **Servers** | Food (`/food`) + Instamart (`/im`). No Dineout. |
| **Scope** | `mcp:tools` |
| **Auth** | OAuth 2.1 + PKCE (S256), DCR (RFC 7591), `state` CSRF |
| **Redirect URI** | _Open item #1 — decide before submission_ |
| **Payment** | Cash-on-Delivery only (v1) |
| **Concept blurb** | Curated, per-city, terminal-native (SSH) ordering over Swiggy MCP; COD; staging-first. The TUI never talks to Swiggy directly. |

## Open items

1. **Redirect URI:** own-domain HTTPS (`https://auth.console.store/cb`) vs
   `http://localhost` for dev. Decide before form submission; register via
   builders@swiggy.in.
2. **`phone` claim:** confirm it is present on the staging Builders JWT (fallback:
   key on `sub` + one console.store OTP).
3. **Tool-count gap:** confirm the documented Food/Instamart tools unused by v1
   flows are unneeded during staging.

## Explicitly out of scope (now)

- Any backend code (built at the staging gate — see [security.md](security.md) §11).
- Production hosting / domain procurement (tied to open item #1).
- CI (buildable today, but not an approval gate).

## Success criteria

- A reviewable security-approach document covering the threat model and reqs in
  [security.md](security.md).
- A shareable asciinema demo link of the end-to-end TUI flow.
- Integration details + redirect URI decided.
- Application submitted at the portal (or via builders@swiggy.in).
