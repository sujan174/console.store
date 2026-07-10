# Instamart by hand — the im_* tool flow (text fallback + recovery)

Use this only when the client can't render the MCP App, or you're recovering
a failed flow. Same safety invariants as SKILL.md: server bill only, human
confirmation before place_order, one update_cart per settled edit.

## The flow

1. `im_search_products {query, address_id?}` — products carry `variants`
   (pack sizes), each with `spin_id`, `sku_id`, `label`, `price`, `in_stock`.
   **Carts hold variants.** Several pack sizes → ask which one (or match the
   user's words) — never pick a size silently.
2. `im_update_cart {address_id?, items:[{spin_id, sku_id, quantity}]}` —
   **REPLACES the whole Instamart cart.** Every item MUST carry BOTH
   `spin_id` AND `sku_id` (from the same search variant) — Swiggy rejects the
   whole call otherwise ("Each item must include both spinId and skuId").
   Adding to an existing cart: `im_get_cart` first, resend existing lines
   (their `sku_id` comes back on each line) plus the new one.
3. `im_get_cart` — lines + the real bill (`item_total`, `delivery`,
   `handling`, `taxes`, `to_pay`) and payment methods.
4. `im_prepare_order {address_id?}` → authoritative bill + `confirmation_id`.
   Show the full bill + address; get an explicit yes.
5. `place_order {confirmation_id, method?}` — `method:"upi"` returns a
   `payment` (QR + pay_url; poll `check_payment`, then `confirm_order` once
   paid — same flow as food); `method:"cod"` or a non-UPI account returns the
   placed `order` directly. Never call uninvited, never retry; on error check
   `list_active_orders` first.

## Closed store / unserviceable address

There is NO proactive serviceability check — Swiggy only reveals it when a
WRITE fails: `im_update_cart` errors with "The store is currently unavailable
or closed. Please try again later or choose a different delivery address."
When you see it: stop retrying writes for that address (they will all fail),
tell the user, and offer a different address or trying later. A successful
`im_get_cart` does NOT mean the store is open — reads succeed while writes
fail.

## Limits & errors

- `under_min:` — bill < ₹99 (Instamart minimum). Ask what to add.
- `over_cap:` — bill ≥ ₹1000 (agent-order cap, same as food). Ask what to trim.
- Empty cart comes back from `im_get_cart` as an "empty" message, not lines.
- UPI when the account is enabled, else COD; typical delivery 10–20 min; no
  cancellation (customer care 080-67466729).
- The Food cart and Instamart cart never interact.
