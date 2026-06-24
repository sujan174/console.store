# Continue here

**Paste this into a new Claude session:**

> Read `docs/superpowers/HANDOFF.md`, then continue the Swiggy MCP integration. Recon is done; next is brainstorming the broker design.

## TL;DR

- console.store TUI is built (mock data). Goal: broker real orders via **Swiggy MCP**.
- **Recon complete this session.** Auth chain confirmed live; 26 real tool schemas harvested to `docs/superpowers/research/`.
- **Local dev needs no approval.** Builders Club gates production only.
- Dev redirect URI = `http://localhost:8765/cb`. Dineout excluded.

## Next step

Brainstorm the **broker** (`cmd/broker` + `internal/{auth,swiggy,account,session,store}` — none exist yet). Fill the existing `catalog.Repository` with a Swiggy-backed impl so screens change zero.

## Still open

- Confirm `phone` claim in the access-token JWT.
- Confirm with `builders@swiggy.in`: localhost dev = sandbox or live? (orders are real, COD, non-cancellable).
- Builders Club submission (after a local demo exists).

Full detail + tool→flow mapping: **`HANDOFF.md`**.
