# Running console.store live (local, single-user)

This brings up the **whole stack on your own machine** and lets you order from
**your own Swiggy account** through the SSH TUI. It is local-only: the OAuth
redirect stays on `http://localhost`, which Swiggy whitelists. A public
deployment at `consolestore.in` additionally needs Swiggy to whitelist that
domain's redirect URI and to lift the third-party-app pause — neither is
required for this local runbook.

## What "no KMS" means here

Tokens are still encrypted at rest with **AES-256-GCM** using a local 32-byte
master key (`CONSOLE_KMS_PROVIDER=local`). "No KMS" means no AWS — not
plaintext. A real Swiggy access token can place non-cancellable COD orders, so
it is never stored in the clear.

## Prerequisites

- Docker (Postgres runs in a container)
- Go 1.26
- An SSH client
- A Swiggy account you can log into in a browser

## One-time setup

```bash
make keygen      # writes a fresh local-KMS master key into .env.local (gitignored)
make db-up       # starts Postgres, applies schema (tables + RLS + console_broker role)
```

`.env.local` holds your master key and DB DSN. It is gitignored — never commit it.

## Run it (three terminals)

```bash
# Terminal A — the privileged broker (holds tokens, the only thing that calls Swiggy)
make broker

# Terminal B — the SSH-facing TUI in live mode
make sshd

# Terminal C — connect
make ssh           # == ssh localhost -p 2222
```

## First connection: authorize once

1. On first `ssh`, the TUI shows an **authorize gate** with a URL.
2. Open that URL in a browser, log into Swiggy, approve access.
   The browser redirects to `http://localhost:8765/cb` (the broker's callback
   listener) and shows "console.store authorized. Return to your terminal."
3. Back in the TUI, press `r` to retry. Your Swiggy addresses load. You're in.

Your SSH public key is now linked to your Swiggy account in Postgres, so future
connections skip the gate until the token expires.

> ⚠️ Keep the Swiggy app closed while using this — simultaneous sessions can
> conflict (Swiggy's own guidance).

## Browsing vs. placing real orders

- **Default (safe):** browse restaurants, open menus, build a cart. The cart
  syncs to Swiggy, but `place order` is **refused** — `CONSOLE_LIVE_ORDERS` is
  unset.
- **Arm real orders:** uncomment `CONSOLE_LIVE_ORDERS=1` in `.env.local`, then
  restart `make broker`. Now the checkout screen places a **real, non-cancellable
  COD order**. Review the cart first. (`make live-orders` prints this reminder.)

## Reset / teardown

```bash
make db-down     # stop Postgres, keep data (stays logged in)
make db-reset    # DESTROY the db volume and re-init schema (forces re-login)
```

## How the pieces fit

```
ssh client ──> cmd/sshd (TUI, CONSOLE_BACKEND=live)
                   │ Unix socket /tmp/console-broker.sock
                   ▼
              cmd/broker ──> Postgres (tokens, RLS-scoped per account)
                   │
                   ▼
            mcp.swiggy.com/food + /im   (OAuth, cart, COD orders)
```

- `accountID` is derived from your **SSH public key** via `AccountForPubkey` —
  never from client input. The broker pins it; a session can only act as itself.
- The broker is the only component with Swiggy tokens or DB access. The TUI
  talks to it solely over the local 0600 Unix socket.
