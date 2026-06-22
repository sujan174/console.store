# console.store — Swiggy Builders Club application

A terminal-native (SSH) curated food & snack ordering shop for developers in India,
Tokyo Night themed. Users `ssh console.store`, browse a hand-picked catalog in a
`bubbletea` TUI, and order. Fulfilment runs through **Swiggy's Builders Club MCP
servers** (Food + Instamart); console.store never talks to Swiggy from the client.

> `ssh console.store`

## Status

**Applying to the Swiggy Builders Club.** Today only the TUI + an in-memory mock
data layer exist (`internal/tui`, `internal/catalog`). The backend that brokers
Swiggy auth and orders is **design/commitment**, not built — it is constructed
later, at the program's staging gate. This is expected: Builders Club reviews a
builder's *concept and security approach* at apply time, not shipped backend code.

v1 scope: **Food + Instamart, Cash-on-Delivery only, staging-first.**

## What this folder is

`docs/` holds **only the Builders Club application package** — the documents Swiggy
review reads, plus the security plan they judge.

| Doc | What it covers |
|-----|----------------|
| [security.md](security.md) | **The detailed security plan** — delegated auth, token storage, order integrity, data handling. The core reviewed artifact. |
| [builders-application.md](builders-application.md) | The submission itself — servers/scope/redirect URI, demo (asciinema) run-sheet, open items, where to apply. |

## The one-paragraph concept

Users `ssh console.store`. A Charm/`wish` SSH server renders a `bubbletea` TUI
themed Tokyo Night. First run links the user's Swiggy account via a link/QR they
open on their phone (Builders Club delegated OAuth 2.1 + PKCE); the **phone number**
from the returned JWT becomes their console.store account key. The backend stores a
per-user Swiggy token and brokers every order through Swiggy's Food and Instamart
MCP servers. console.store's own value is **curation** — a hand-picked, per-city
whitelist of restaurants/items surfaced cleanly in the terminal. Payment is
Cash-on-Delivery (v1 limitation). The TUI never talks to Swiggy directly.

## Tech stack

- **Language:** Go (Golang) 1.26
- **SSH server:** [`charmbracelet/wish`](https://github.com/charmbracelet/wish)
- **TUI framework:** [`charmbracelet/bubbletea`](https://github.com/charmbracelet/bubbletea) (Elm architecture)
- **Styling:** [`charmbracelet/lipgloss`](https://github.com/charmbracelet/lipgloss) — Tokyo Night palette
- **MCP client (planned):** Go MCP SDK over Streamable HTTP
- **Persistence (planned):** Postgres (prod) / SQLite (dev); tokens encrypted at rest
- **QR in terminal (planned):** `skip2/go-qrcode` + half-block rendering

## Apply

Program portal: <https://mcp.swiggy.com/builders/> · contact: builders@swiggy.in
