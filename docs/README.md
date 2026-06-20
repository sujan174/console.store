# console.store — documentationTokyo Night

A terminal-native (SSH) food & snack ordering shop for developers in India. Curated experience on top of Swiggy's Builders Club MCP APIs, with a Tokyo Night TUI.

> `ssh console.store`

## Status

Design / pre-implementation. v1 scope locked: **Food + Instamart, Cash-on-Delivery only, staging-first**.

## Document index

| Doc | What it covers |
|-----|----------------|
| [specs/2026-06-20-console-store-design.md](superpowers/specs/2026-06-20-console-store-design.md) | **Canonical design spec** — goals, scope, decisions, risks |
| [architecture.md](architecture.md) | The 6 components, data flow, deployment |
| [project-structure.md](project-structure.md) | Go module layout, every package, key types |
| [auth.md](auth.md) | Phone-key OAuth (broker auth-code + PKCE), token lifecycle |
| [swiggy-integration.md](swiggy-integration.md) | MCP client, all 35 tools, screen→tool mapping, idempotency |
| [data-model.md](data-model.md) | Entities, storage, encryption, schemas |
| [ui/ui-mocks.md](ui/ui-mocks.md) | Every screen mocked in Tokyo Night |

## The one-paragraph summary

Users `ssh console.store`. A Charm/`wish` SSH server renders a `bubbletea` TUI themed Tokyo Night. First run links the user's Swiggy account via a QR/link they open on their phone (broker-mediated OAuth 2.1 + PKCE); their **phone number** (read from the returned JWT) becomes their console.store account key. The backend stores a per-user Swiggy token and brokers every order through Swiggy's Food and Instamart MCP servers. console.store's own value is **curation** — a hand-picked, per-city whitelist of restaurants/items surfaced cleanly in the terminal. Payment is Cash-on-Delivery (Swiggy API v1 limitation). The TUI never talks to Swiggy directly.

## Tech stack

- **Language:** Go (Golang)
- **SSH server:** [`charmbracelet/wish`](https://github.com/charmbracelet/wish)
- **TUI framework:** [`charmbracelet/bubbletea`](https://github.com/charmbracelet/bubbletea) (Elm architecture)
- **Styling:** [`charmbracelet/lipgloss`](https://github.com/charmbracelet/lipgloss)
- **Components:** [`charmbracelet/bubbles`](https://github.com/charmbracelet/bubbles) (list, textinput, spinner, viewport)
- **MCP client:** Go MCP SDK (`mark3labs/mcp-go` or official `modelcontextprotocol/go-sdk`) over Streamable HTTP
- **Persistence:** Postgres (prod) / SQLite (dev); tokens encrypted at rest
- **QR in terminal:** `skip2/go-qrcode` + half-block rendering
```
