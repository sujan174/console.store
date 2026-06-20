# Authentication & Identity

## Model in one line

**Phone number = account key**, obtained via a **broker-mediated OAuth 2.1 authorization-code + PKCE (S256)** flow against Swiggy, completed by the user **on their phone**.

> Verified against Swiggy Builders Club docs (delegated multi-tenant auth). Swiggy supports **only** authorization-code + PKCE — there is **no** device-grant (RFC 8628). The "terminal shows a link, phone finishes it" UX is achieved by a server-side broker + TUI polling, not by a device grant.

## Swiggy endpoints

Base: `https://mcp.swiggy.com`

| Endpoint | Use |
|----------|-----|
| `GET /auth/authorize` | Start flow; user does phone+OTP/app consent |
| `POST /auth/token` | Exchange `code` → JWT access token |
| `POST /auth/register` | Dynamic Client Registration (RFC 7591) |
| `POST /auth/logout` | Revoke session |
| `GET /.well-known/oauth-authorization-server` | AS metadata (RFC 8414) |
| `GET /.well-known/oauth-protected-resource` | Resource metadata (RFC 9728) |

**Token facts:** JWT, `expires_in: 432000` (5 days). Auth code: 120s, single-use. Session: 30-day idle sliding window. **No refresh tokens in v1** (rolling refresh planned v1.1). Scopes: `mcp:tools mcp:resources mcp:prompts`. Redirect URIs: HTTPS exact-match (localhost allowed for dev); register new ones via builders@swiggy.in.

## First link (terminal → phone → broker)

```
1. TUI: user picks "link account"
2. Broker.Begin():
     verifier = random(32 bytes) base64url
     challenge = base64url(sha256(verifier))
     state = random
     store PendingAuth{id, verifier, state}
     authURL = /auth/authorize?response_type=code
                 &client_id=<dcr>&redirect_uri=https://auth.console.store/cb
                 &code_challenge=<challenge>&code_challenge_method=S256
                 &state=<state>&scope=mcp:tools&resource=<server>
3. TUI: render authURL as QR + short link
4. User opens on phone → Swiggy app/OTP consent → Swiggy 302 → redirect_uri?code&state
5. Broker.Callback(code, state):
     verify state → POST /auth/token (grant_type=authorization_code, code, verifier, redirect_uri)
     receive JWT
     DecodeClaims → phone, sub, exp
     store token encrypted, keyed by phone (account = phone)
     signal PendingAuth.Done
6. TUI (polling PendingAuth): unlock → load menu
```

**No separate console.store OTP** — the verified `phone` claim from the JWT *is* proof of phone ownership.

## Returning — same device

`ssh` → `wish` exposes pubkey → `account.ResolveByPubKey` → token valid → straight to menu (skip auth).

## New device — existing phone

```
1. TUI: enter phone
2. account lookup by phone → found
3. send tap-link to that phone (SMS/WhatsApp) carrying a console.store verify token
4. user taps → device's pubkey bound to account
5. reuse stored Swiggy token (no Swiggy re-OAuth unless expired)
```

This is the only place console.store sends its own message (first link needs none).

## Re-auth (token expiry)

- Access token dies at 5 days. On any `401` / JSON-RPC `-32001`, the Swiggy client calls `onUnauth(userID)` → triggers re-auth.
- Re-auth = push the `/auth/authorize` link to the user's phone again.
- **Inside Swiggy's 30-day session, this is silent** — Swiggy still has the user's session, so consent completes with **no OTP**, just a tap.
- After ~30 days idle, full phone+OTP again.
- v1.1 rolling refresh tokens will let the broker refresh server-side → removes the tap entirely.

```
order action ──▶ 401 ──▶ pause ──▶ push reauth link to phone
                                      │ tap (silent if <30d)
                                      ▼
                              token refreshed ──▶ resume the paused action
```

## Security

- Token is a Bearer to a **real Swiggy account** that can place real COD orders (≤₹1000). Treat as top-secret.
- Encrypt at rest (`store/crypto.go`), per-env KMS data key. Never log tokens or full JWTs.
- We **do not** trust the JWT signature ourselves (Swiggy validates at the HAProxy edge); we only *read* claims for identity correlation after a successful exchange.
- Bind each token strictly to one account; never share across users (Swiggy's delegated-auth requirement: "secure per-user storage, never shared").
- Surface Swiggy's warning: don't use the Swiggy app concurrently with our session (conflict risk).
- `state` prevents CSRF on the callback; `verifier` never leaves the broker.

## Staging verification checklist

1. ☐ Cross-device: terminal-initiated authorize URL, completed on phone, redirects to our callback and `POST /auth/token` succeeds.
2. ☐ `phone` claim present on the Builders JWT (not only `sub`).
   - Fallback: key on `sub`; add one console.store OTP to capture + verify phone.
3. ☐ Re-auth inside the 30-day window is genuinely silent (no OTP).
4. ☐ DCR (`/auth/register`) returns usable client identity for our redirect URI.
```
