---
name: console-card
description: View and tune the consolestore taste card — the local memory of the user's default address, favorite restaurants, and dietary preferences. Use when the user asks what consolestore remembers, or wants to change their default address or food preferences.
---

# The consolestore taste card

consolestore keeps a small local "card" so ordering needs less back-and-forth.
It is built automatically from real orders — the user never has to set it up — and
you can read or adjust it with two tools.

## Showing what's remembered

- Call `get_card`. It returns the `default_address_id` and label, `favorites`
  (restaurants ranked by how often they're ordered), and any `prefs` (free-form
  notes like "vegetarian" or "no onion").
- `get_card` also reconciles against live addresses. If it returns `warnings` — for
  example, the saved default address was deleted on Swiggy — relay them and offer
  to set a new default.

## Changing the card

- To set a default address: call `list_addresses`, let the user pick, then call
  `update_card` with that `default_address_id`.
- To record preferences: call `update_card` with a `prefs` list. Providing `prefs`
  replaces the whole list, so include everything that should remain.
- There is nothing to "create" — the card already exists and grows on its own each
  time an order is placed. `update_card` only records explicit choices the user
  states.

## How it fills in over time

- Every placed order (from this agent, the consolestore app, or its CLI) bumps the
  ordered restaurant in `favorites` and refreshes the default address. So the more
  the user orders, the better the card's suggestions — no manual upkeep needed.
