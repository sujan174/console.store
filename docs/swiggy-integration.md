# Swiggy Builders Club integration

## Servers

| Server | URL | v1 use |
|--------|-----|--------|
| Food | `https://mcp.swiggy.com/food` | coffee + restaurants |
| Instamart | `https://mcp.swiggy.com/im` | snacks / grocery (separate cart) |
| Dineout | `https://mcp.swiggy.com/dineout` | **skipped** |

Transport: MCP over Streamable HTTP. Each call carries `Authorization: Bearer <per-user JWT>` and the `resource` parameter identifying the server. Access control is **user-level** in v1 — the one token grants the full tool set.

## Food server — 13 enumerated (Swiggy documents 14)

| Tool | Purpose | Used in |
|------|---------|---------|
| `search_restaurants` | restaurants near an address; returns `deliveryTimeRange` (~30-60 min, **standard** delivery) (filter `OPEN`) | menu (places) |
| `get_restaurant_menu` | items, categories, variants, add-ons | restaurant screen |
| `search_menu` | keyword search within a menu | `/` search |
| `update_food_cart` | add/modify items (binds to ONE restaurant) | add item |
| `get_food_cart` | server-truth cart + bill | cart, pre-checkout |
| `flush_food_cart` | clear cart before switching restaurant | "start new cart?" |
| `fetch_food_coupons` | available codes | cart |
| `apply_food_coupon` | apply a coupon (no COD pre-filter; handle rejection inline) | cart |
| `place_food_order` | checkout with `paymentMethod: "COD"` | checkout |
| `get_food_orders` | order history | "the usual", verify-on-failure |
| `track_food_order` | live status + ETA (poll ≥10s) | tracking |
| `get_addresses` | saved delivery locations | address bar/switcher |
| `report_error` | diagnostics for support | error handler |

## Instamart server — 11 enumerated (Swiggy documents 13)

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
| `create_address` | add address — **Instamart-only** (Food has none) | address add |
| `report_error` | diagnostics | error handler |

(Dineout's 8 tools intentionally unused in v1.)

> **Count gap:** Swiggy documents 14 Food / 13 Instamart tools; the tables above enumerate the ones our flows use (13 / 11). The remaining few (likely product-detail / category-browse helpers) are uncaptured and **not relied on** by any v1 flow — confirm during staging integration.

## Not available — no tool exists (design around these)

| Missing capability | Reality | Handling |
|--------------------|---------|----------|
| Cancel / modify an order | Orders are **final** once placed | UI never offers cancel; show "can't be cancelled" at checkout/confirm; idempotency guard prevents accidental doubles |
| Online payment | COD only | no payment UI; pay rider on delivery |
| Bolt / 10-min restaurant delivery | Food is standard **~30-60 min** | honest delivery windows; **Instamart** is the only fast lane (~10-20 min) |
| Fetch Instamart product by `spinId` | can't render a curated SKU list in one call | `search_products` per curated name → match `spinId` → cache |
| Food `create_address` | can't add an address from the Food flow | use Instamart `create_address` (same account) or the Swiggy app |
| Scheduled / future delivery | immediate only | no scheduling UI |

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
tracking           → track_food_order / track_order (status+ETA; rider/steps best-effort, poll ≥10s)
the usual          → get_food_orders / your_go_to_items → re-add → checkout (re-fetch live price; skip if closed)
instamart list     → search_products per curated SKU → match spinId → cache (no fetch-by-id tool)
address switch     → get_addresses ; create_address is Instamart-only ; flush_food_cart / clear_cart on change
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
| **No cancellation** | no cancel/modify tool — orders final; UI shows this, never offers cancel |
| Honest ETAs | Food shows `deliveryTimeRange` (~30-60 min); Instamart ~10-20 min |
| Always read before mutate | call `get_*_cart` at turn start (server truth) |

## Idempotency & errors

`place_food_order` and `checkout` are **non-idempotent** — and orders **cannot be cancelled**, so a wrongful retry creates an un-undoable double order. The verify-before-retry guard below is **mandatory, not advisory**.

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
