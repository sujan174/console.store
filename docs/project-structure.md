# Project structure

Go module: `console.store`

```
console.store/
├── cmd/
│   ├── sshd/
│   │   └── main.go            # wish SSH server entrypoint; wires middleware + bubbletea
│   └── broker/
│       └── main.go            # HTTPS OAuth callback + internal API (can be same binary)
├── internal/
│   ├── tui/
│   │   ├── app.go             # root bubbletea model; screen router; global keymap
│   │   ├── theme/
│   │   │   ├── tokyonight.go  # lipgloss styles + palette constants
│   │   │   └── ascii.go       # logo / splash / confirm ASCII art
│   │   ├── components/
│   │   │   ├── header.go      # brand + address + cart chip (top bar)
│   │   │   ├── list.go        # single-column selectable list (❯ cursor, selrow)
│   │   │   ├── cartbar.go     # bottom cart summary
│   │   │   ├── qrcode.go      # render auth QR + short link
│   │   │   └── keyhints.go    # footer key hints
│   │   └── screens/
│   │       ├── splash.go      # "fetching your grub" loader
│   │       ├── onboard.go     # QR/link link-account screen + poll
│   │       ├── menu.go        # places list (category tabs)
│   │       ├── restaurant.go  # items list for a restaurant
│   │       ├── cart.go        # cart review
│   │       ├── checkout.go    # confirm + COD notice
│   │       ├── confirmed.go   # ascii order-confirmed
│   │       ├── track.go       # live ETA tracking
│   │       ├── instamart.go   # flat curated grocery list + cart
│   │       ├── address.go     # address switcher overlay
│   │       └── errors.go      # empty / error / reauth states
│   ├── account/
│   │   ├── service.go         # resolve pubkey↔account↔phone; device binding
│   │   └── session.go         # 30-day session validity
│   ├── auth/
│   │   ├── broker.go          # PKCE gen, authorize URL, callback, token exchange
│   │   ├── pkce.go            # verifier/challenge (S256)
│   │   ├── jwt.go             # decode phone/sub claims (no signature trust)
│   │   └── pending.go         # pending-auth session store + TUI poll signaling
│   ├── swiggy/
│   │   ├── client.go          # MCP client, Bearer injection, backoff, 401 hook
│   │   ├── food.go            # 14 Food tool wrappers (typed)
│   │   ├── instamart.go       # 13 Instamart tool wrappers (typed)
│   │   ├── types.go           # request/response structs
│   │   └── idempotency.go     # post-failure order verification
│   ├── curation/
│   │   ├── store.go           # per-city whitelist load
│   │   ├── filter.go          # intersect live Swiggy results with whitelist + tags
│   │   └── usual.go           # "the usual" from order history
│   ├── session/
│   │   └── state.go           # per-SSH cart/address/restaurant/screen state
│   └── store/
│       ├── db.go              # Postgres/SQLite access
│       ├── crypto.go          # token encryption at rest
│       └── migrate.go         # migrations runner
├── migrations/                # SQL migrations
├── config/
│   └── config.go              # env config (staging/prod Swiggy, DB, KMS)
├── docs/
├── go.mod
└── go.sum
```

## Package responsibilities & key types

### `internal/tui`
Elm-architecture (`bubbletea`). Root `app.Model` holds the active screen and global state handle; each screen is its own `tea.Model` returning `tea.Cmd`s. Backend calls are dispatched as `tea.Cmd`s that emit result messages — the TUI stays non-blocking.

```go
type Screen int
const ( ScreenSplash Screen = iota; ScreenOnboard; ScreenMenu; ScreenRestaurant; ScreenCart; ScreenCheckout; ScreenConfirmed; ScreenTrack; ScreenInstamart; ScreenAddress; ScreenError )

type Model struct {
    screen   Screen
    session  *session.State
    account  *account.Account
    width, height int
    // sub-models per screen
}
```

### `internal/account`
```go
type Account struct {
    Phone     string      // primary key
    SwiggySub string      // JWT sub (fallback key)
    Devices   []PubKey    // bound SSH public keys
    TokenRef  string      // pointer to encrypted token in store
}
func (s *Service) ResolveByPubKey(pk string) (*Account, bool)
func (s *Service) BindDevice(acc *Account, pk string) error
```

### `internal/auth`
Completion is signaled via a **shared store (DB/Redis) that the TUI polls** — NOT a Go channel. A channel would not cross processes if `broker` and `sshd` run as separate binaries/replicas (the SSH session and the OAuth callback are different requests, possibly on different hosts).
```go
type AuthStatus int // Pending | Done | Expired
type PendingAuth struct { ID, Verifier, State string; Status AuthStatus }
func (b *Broker) Begin() (authURL, pendingID string, err error)   // persists PendingAuth
func (b *Broker) Callback(code, state string) error               // exchanges, stores token, sets Status=Done
func (b *Broker) Poll(pendingID string) (AuthStatus, *TokenRef)   // TUI polls this
func DecodeClaims(jwt string) (phone, sub string, exp time.Time, err error)
```

### `internal/swiggy`
```go
type Client struct { base string; http *http.Client; onUnauth func(userID string) }
func (c *Client) SearchRestaurants(ctx, tok, addrID, query string) ([]Restaurant, error)
func (c *Client) UpdateFoodCart(ctx, tok string, req CartReq) (Cart, error)
func (c *Client) PlaceFoodOrder(ctx, tok string, req OrderReq) (Order, error) // COD
// ... one method per tool; see swiggy-integration.md
```

### `internal/curation`
```go
type Whitelist struct { City string; Restaurants map[string]CuratedRestaurant }
func (w *Whitelist) Filter(live []swiggy.Restaurant) []CuratedRestaurant   // intersect + tag
func Usual(history []swiggy.Order) (item CuratedItem, ok bool)
```

### `internal/session`
```go
type State struct {
    Address      swiggy.Address
    Category     string
    Restaurant   *swiggy.Restaurant   // nil until chosen
    FoodCart     swiggy.Cart
    InstaCart    swiggy.Cart
    Screen       tui.Screen
}
```

## Conventions
- One package = one clear responsibility; files stay focused (split when a file does too much).
- All Swiggy calls go through `internal/swiggy` — no other package imports the MCP SDK.
- All Swiggy `/auth/*` calls go through `internal/auth` — no other package touches OAuth.
- TUI imports `session` + result message types only; never `swiggy` or `auth` directly.
- Secrets never logged; tokens only ever in `store` (encrypted) and `swiggy` (in-memory per call).
```
