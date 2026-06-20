# Swiggy Builders Club integration

## Servers

| Server | URL | v1 use |
|--------|-----|--------|
| Food | `https://mcp.swiggy.com/food` | coffee + restaurants |
| Instamart | `https://mcp.swiggy.com/im` | snacks / grocery (separate cart) |
| Dineout | `https://mcp.swiggy.com/dineout` | **skipped** |

Transport: MCP over Streamable HTTP. Each call carries `Authorization: Bearer <per-user JWT>` and the `resource` parameter identifying the server. Access control is **user-level** in v1 — the one token grants the full tool set.

## Food server — 14 tools

| Tool | Purpose | Used in |
|------|---------|---------|
| `search_restaurants` | restaurants by cuisine/query near an address (filter `OPEN`) | menu (places) |
| `get_restaurant_menu` | items, categories, variants, add-ons | restaurant screen |
| `search_menu` | keyword search within a menu | `/` search |
| `update_food_cart` | add/modify items (binds to ONE restaurant) | add item |
| `get_food_cart` | server-truth cart + bill | cart, pre-checkout |
| `flush_food_cart` | clear cart before switching restaurant | "start new cart?" |
| `fetch_food_coupons` | available codes | cart |
| `apply_food_coupon` | apply a (COD-compatible) coupon | cart |
| `place_food_order` | checkout with `paymentMethod: "COD"` | checkout |
| `get_food_orders` | order history | "the usual", verify-on-failure |
| `track_food_order` | live status + ETA (poll ≥10s) | tracking |
| `get_addresses` | saved delivery locations | address bar/switcher |
| `report_error` | diagnostics for support | error handler |

## Instamart server — 13 tools

| Tool | Purpose | Used in |
|------|---------|---------|
| `search_products` | groceries/items by keyword | instamart search |
| `your_go_to_items` | quick-reorder frequent SKUs | instamart "usual" |
| `update_cart` | add/modify items (by `spinId`) | add item |
| `get_cart` | cart + bill + min-order | instamart cart |
| `clear_cart` | flush before address change | address switch |
| `checkout` | place order, `paymentMethod: "COD"` | instamart checkout |
| `get_orders` | history | reorder / verify |
| `track_order` | delivery progress | tracking |
| `get_addresses` | saved locations | address |
| `create_address` | add new delivery address | address add |
| `report_error` | diagnostics | error handler |

(Dineout's 8 tools intentionally unused in v1.)

## Screen → tool mapping

```
onboarding         → (auth broker, not MCP)
splash/menu load   → get_addresses → search_restaurants → curation.Filter
places list        → search_restaurants (status OPEN) ∩ whitelist
restaurant items   → get_restaurant_menu ∩ whitelist  ;  search_menu for /
add item           → update_food_cart        (Food)  | update_cart (Instamart)
cart chip / review → get_food_cart                    | get_cart
coupon             → fetch_food_coupons / apply_food_coupon
checkout           → get_food_cart → place_food_order(COD) | get_cart → checkout(COD)
confirmed          → (orderId from place response)
tracking           → track_food_order / track_order   (poll ≥10s)
the usual          → get_food_orders / your_go_to_items → re-add → checkout
address switch     → get_addresses / create_address ; flush_food_cart / clear_cart on change
```

## Constraints baked into the client

| Rule | Enforcement |
|------|-------------|
| Food cart cap **₹1000** | validate before `place_food_order` |
| Instamart min **₹99** | validate before `checkout` |
| One restaurant per Food cart | switching → `flush_food_cart` + user prompt |
| Cart binds to address | address change → flush + re-validate |
| **COD only** | always send `paymentMethod: "COD"`; no payment UI |
| No scheduling | orders execute immediately; no future-time field |
| Always read before mutate | call `get_*_cart` at turn start (server truth) |

## Idempotency & errors

`place_food_order`, `checkout`, `book_table` are **non-idempotent**.

```go
order, err := c.PlaceFoodOrder(ctx, tok, req)
if err != nil && is5xx(err) {
    // DO NOT blind-retry
    recent := c.GetFoodOrders(ctx, tok)
    if placedJustNow(recent, req) {
        return recent.latest, nil   // it actually went through
    }
    // else safe to retry once
}
```

- `401` / `-32001` → `onUnauth` → re-auth flow, then resume.
- `UPSTREAM_ERROR` → exponential backoff (upstream capacity shedding).
- v1.0 has **no symbolic error codes** — branch on `error.message` text + HTTP status. (`error.code` planned v1.1.)
- Log the Swiggy `session id` on every failure for support correlation. Never log tokens.

## MCP client sketch

```go
type Client struct {
    food, insta string                 // server URLs
    mcp         mcpclient.Client        // Go MCP SDK over Streamable HTTP
    onUnauth    func(userID string)
}

func (c *Client) call(ctx context.Context, server, tool, tok string, args any) (json.RawMessage, error) {
    res, err := c.mcp.CallTool(ctx, server, tool, args, mcpclient.WithBearer(tok), mcpclient.WithResource(server))
    switch {
    case isUnauth(err):      c.onUnauth(userIDFrom(ctx)); return nil, ErrReauth
    case isUpstream(err):    return nil, backoffRetry(...)
    }
    return res, err
}
```

## Reference

- [Authenticate](https://mcp.swiggy.com/builders/docs/start/authenticate/)
- [Delegated auth](https://mcp.swiggy.com/builders/docs/start/enterprise/delegated-auth/)
- [Developer quickstart](https://mcp.swiggy.com/builders/docs/start/developer/)
- [Builders Club](https://mcp.swiggy.com/builders/)
```
