# console.store — Security Plan

**Audience:** Swiggy Builders Club review.
**Date:** 2026-06-22.
**Status of this document:** This is the security *approach* console.store commits
to. Most of it is **design / commitment**, not shipped code — the backend that
brokers Swiggy auth and orders is built later, at the program's staging gate,
against real endpoints. Where something already exists in the running TUI it is
marked **[built]**; everything else is **[commitment]**.

console.store handles three things worth protecting: a per-user **Swiggy access
token** (which can place real orders and spend real money), the user's **phone
number / minimal PII**, and the **order-placement capability** itself. This
document states, requirement by requirement, how each is protected.

---

## 1. Threat model

**Assets**

| Asset | Why it matters |
|-------|----------------|
| Swiggy access token (per user) | Bearer credential that can browse, cart, and place real, non-cancellable orders |
| User phone number (from the Builders JWT) | PII; also the console.store account key |
| Order-placement capability | A bug or abuse can cost a user real money |
| SSH host key | Impersonation of the server would enable credential phishing |
| KMS data key / DB credentials | Compromise unlocks all stored tokens at once |

**Adversaries & what we assume they can do**

- **Another console.store user** — has a valid SSH session; must never read or use another user's token or data.
- **Network attacker (MITM)** — can observe/modify traffic between components; mitigated by HTTPS everywhere and SSH transport for the client.
- **Database-read attacker** — gains read access to the Postgres store (leaked backup, SQLi, over-broad role); must not obtain a usable token.
- **Compromised app process** — partial RCE / logic bug in the broker; blast radius limited by least privilege, RLS, and not holding plaintext tokens longer than a call.
- **Malicious SSH client** — sends hostile input/keys; mitigated by the TUI being a closed input surface and the client never holding Swiggy credentials.

**Trust boundaries**

```
[SSH client] --ssh--> [wish TUI server] --internal--> [broker] --https--> [Swiggy MCP]
                                              |
                                              +--> [Postgres + RLS] / [KMS]
```

The **TUI never crosses the Swiggy boundary** — only the broker holds tokens and
calls Swiggy. The client is never given a Swiggy token, JWT, or refresh material.

---

## 2. Delegated authentication (OAuth 2.1 + PKCE + DCR)

console.store uses Swiggy's delegated-auth flow. The user authenticates on Swiggy's
own phone surface; console.store never sees a password or OTP.

- **OAuth 2.1 + PKCE, `S256`.** A fresh code `verifier` and `challenge` are
  generated per pending-auth session and never reused. The verifier never leaves
  the broker. **[commitment]**
- **`state` CSRF token per session.** A random `state` is stored with the
  pending-auth record and verified on callback; any mismatch is rejected before a
  token exchange happens. **[commitment]**
- **Dynamic Client Registration (RFC 7591).** The broker self-registers its client
  identity (`POST /auth/register`); there is **no pre-shared `client_id` or client
  secret** committed anywhere. **[commitment]**
- **Exact-match HTTPS redirect URI.** The callback URI is registered exact-match and
  HTTPS-only (localhost permitted only in dev). See open item in
  [builders-application.md](builders-application.md). **[commitment]**
- **No credential handling.** console.store never touches the user's Swiggy
  password, OTP, or raw account PII — that flow lives entirely on Swiggy's phone
  surface. **[commitment]**

---

## 3. Token lifecycle & storage

The Swiggy token is the highest-value asset; it gets defense in depth.

- **Encrypted at rest.** Tokens are encrypted with a per-environment **KMS data
  key**; the database never stores plaintext token material. **[commitment]**
- **Postgres Row-Level Security.** RLS policies ensure a DB role can read only its
  own user's row — so even an app-logic bug or an over-scoped query cannot return
  another user's token. Encryption + RLS together mean a DB-read attacker gets
  ciphertext for one row at most. **[commitment]**
- **One token per account, never shared.** Strictly one Swiggy token per
  console.store account (keyed by phone). Tokens are never pooled or reused across
  users. **[commitment]**
- **In-memory only during a call.** Plaintext token exists only transiently, in the
  broker, for the duration of a single Swiggy call; it is never written to disk in
  plaintext and never returned to the TUI. **[commitment]**
- **No refresh tokens in v1.** On expiry the broker re-runs the authorize flow
  rather than holding long-lived refresh material. Re-auth is triggered
  **proactively** (~60s before the token's `expires_in`) so the user rarely hits a
  hard failure, and stays silent inside Swiggy's 30-day session. **[commitment]**
- **Drop on logout.** Explicit logout calls Swiggy `POST /auth/logout`, revokes the
  session, and **purges the stored token immediately**. **[commitment]**

---

## 4. Transport & logging

- **HTTPS only.** All broker↔Swiggy traffic is HTTPS; the client↔server hop is the
  SSH transport. No plaintext token ever crosses a network. **[commitment]**
- **Redaction from day one.** The logging layer redacts tokens, JWTs, and PII
  before any line is written. Tokens/JWTs/phone numbers are never logged, not even
  at debug level. **[commitment]**
- **No plaintext beyond lifetime.** Token material is not persisted in logs, crash
  dumps, or traces; it lives only in the encrypted store and transiently in memory.
  **[commitment]**

---

## 5. Auth-failure handling

The Swiggy client maps every documented auth-failure to a deterministic recovery:

| Status | Meaning | Response |
|--------|---------|----------|
| **401** | Access token expired | Silent re-auth: push the authorize link to the user's phone, retry once authorized |
| **419** | Session revoked | Full re-auth from scratch; purge the stale token first |
| **403** | Insufficient scope | Surface a clear error; do not retry blindly; re-request authorize with the needed scope |

No auth failure is retried with the same dead credential, and none leaks token
detail to the user. **[commitment]**

---

## 6. Order integrity & idempotency

Swiggy orders are **non-cancellable once placed**, and `place_food_order` /
checkout are **not idempotent**. A naive retry on a 5xx could place a duplicate
order and charge the user twice.

- **Verify-before-retry.** Before retrying any non-idempotent order call, the broker
  fingerprints the intended order and checks `get_food_orders` / `get_im_orders`
  for a matching recent order. If a match exists, the original succeeded and the
  retry is suppressed. A 5xx can therefore **never create a double order**.
  **[commitment]**
- **Confirmation gate.** The TUI shows an explicit, non-cancellable confirmation
  (and the COD amount) before any order call is made. **[built]**
- **Cash-on-Delivery only in v1.** No card/UPI credentials flow through
  console.store at all, removing an entire class of payment risk. **[commitment]**

---

## 7. Least privilege

- **Only the servers we need.** Request **Food + Instamart** servers only; Dineout
  is explicitly **not** requested in v1. **[commitment]**
- **Minimal scope.** `mcp:tools` only. **[commitment]**
- **No unused tools.** v1 flows exercise a subset of the documented Food/Instamart
  tools; the remainder are not invoked. **[commitment]**
- **Scoped DB roles.** The broker's DB role is constrained by RLS (see §3) and has
  no rights beyond its own rows. **[commitment]**

---

## 8. SSH layer & isolation

- **Host key.** The server presents a persistent SSH host key
  (`.ssh/console_host_key`, generated on first run) so clients can pin identity and
  detect impersonation. **[built]**
- **Per-session isolation.** Each SSH connection gets its own TUI model instance and
  its own session state; no cross-session shared mutable cart/auth state. **[built]**
- **Broker isolation.** The TUI process never holds Swiggy tokens and never calls
  Swiggy — it talks only to the internal broker, which is the sole token holder.
  This keeps the credential out of the most-exposed (client-facing) component.
  **[commitment]**
- **Account binding.** Identity binds SSH pubkey ↔ console.store account ↔ phone
  (from the Builders JWT), so a session is unambiguously one user. **[commitment]**

---

## 9. Data handling

- **PII we touch:** the user's **phone number**, read from the Builders JWT, used as
  the account key. That is the minimum needed to identify a returning user and to
  match Swiggy's account. **[commitment]**
- **PII we never touch:** passwords, OTPs, card/UPI details, and raw Swiggy account
  PII — none of it flows through console.store. **[commitment]**
- **Retention:** the stored artifacts are the encrypted token and the account
  record (phone + pubkey). On logout the token is purged (§3). **[commitment]**
- **No third-party sharing:** user data is not sold, shared, or sent anywhere other
  than Swiggy (to fulfil the user's own orders). **[commitment]**

---

## 10. Secrets management

- **KMS** holds the per-environment data key used to encrypt tokens; the app holds
  no long-lived decryption key in plaintext config. **[commitment]**
- **Host key** is stored on the server, outside the repo, and never committed.
  **[built]**
- **Nothing secret in the repo or logs.** No `client_id`/secret (DCR removes the
  need), no tokens, no KMS keys, no DB credentials in source control; redaction
  keeps them out of logs (§4). **[commitment]**

---

## 11. Staged rollout & open items

console.store follows the program's staged model: **apply → quick review → staging
access → (staging green) → production.** Nothing in §§2–10 ships blind; the backend
is built and verified against **staging** first.

**Staging verification checklist** (run before requesting production):

- Cross-device authorize completes (link/QR on phone, session continues in terminal).
- `phone` claim present on the Builders JWT (fallback: key on `sub` + a one-time console.store OTP).
- Silent re-auth works within Swiggy's 30-day session window.
- DCR returns a usable client identity.
- 401 / 419 / 403 each drive the correct recovery (§5).
- Verify-before-retry suppresses a forced duplicate order (§6).

**Open items** (also tracked in [builders-application.md](builders-application.md)):

1. **Redirect URI** — own-domain HTTPS (`https://auth.console.store/cb`) vs
   `http://localhost` for dev. Decide before form submission.
2. **`phone` claim** — confirm it is present on the staging Builders JWT.
3. **Tool-count gap** — confirm the documented Food/Instamart tools not used by v1
   flows are genuinely unneeded during staging.
