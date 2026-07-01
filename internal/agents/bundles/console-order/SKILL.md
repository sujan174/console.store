---
name: console-order
description: Order food on Swiggy through consolestore's MCP tools — search, build a cart, show the real bill, and place only after the user confirms. Use when the user wants to order food, get a meal, or reorder a usual.
---

# Ordering food with consolestore

consolestore exposes Swiggy ordering as MCP tools. Orders cost real money and
**cannot be cancelled**, so placing always takes two steps and an explicit user
confirmation.

## First: make sure you can act

1. Call `auth_status`. If `signed_in` is false, call `sign_in`, show the user the
   returned `authorize_url` (their browser usually opens on its own), and poll
   `auth_status` until it reports `signed_in: true`. Then continue.
2. Call `get_card` to load what consolestore remembers (default address, favorite
   restaurants, dietary prefs). If it returns `warnings`, tell the user plainly —
   for example, a saved address that no longer exists — and ask how to proceed.

## Choosing the address

- If the card has a `default_address_id`, use it without asking.
- Otherwise call `list_addresses` and ask the user which one to use.
- Never invent an address id. There is no GPS; the address always comes from the
  card or `list_addresses`.

## Finding the food

- Direct request ("get me McDonald's"): call `search_restaurants` with the card's
  address and the user's words. Don't list addresses first.
- Reorder ("the usual"): call `list_usuals`, or `list_presets` for saved carts.
- A preset is the fastest path — see "Ordering a preset" below.
- Pick a restaurant, then `get_menu`. For an item marked `customizable`, call
  `get_item_options` to get its variant/add-on group and choice ids.

## Building the cart

- Call `update_cart` with the FULL set of lines you want (it replaces the cart
  for that restaurant). Each line needs the menu `item_id` and quantity; pass
  variant/add-on selections using the ids from `get_item_options`.
- If a cross-restaurant change is rejected, tell the user the cart already holds a
  different restaurant and ask whether to replace it (call `clear_cart` first).

## Placing the order (two steps, always)

1. Call `prepare_order` with the address id. It returns the real Swiggy `bill`
   and a `confirmation_id`. **Show the bill to the user** — item total, delivery,
   taxes, and to-pay — and ask them to confirm.
2. Only after the user clearly says yes, call `place_order` with that
   `confirmation_id`. If the cart changed since `prepare_order`, the call is
   refused — re-run `prepare_order` and confirm the new bill.
3. Never call `place_order` on your own initiative, and never retry it. If it
   returns an error, the order may still have been placed — call
   `list_active_orders` before doing anything else.

## Ordering a preset

- Call `order_preset` with the preset `name` (and `index` when several share a
  name). It loads the preset into the cart and returns a `bill` + `confirmation_id`
  — then follow the same confirm-then-`place_order` step above.

## Tracking

- After placing, or any time the user asks "where's my order", call
  `list_active_orders` and then `track_order` for the live status and ETA.
