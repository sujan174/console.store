---
name: console-card
description: Use when the user asks what consolestore remembers about them, or wants to view or change their default delivery address, favorite restaurants, or dietary preferences.
---

# The consolestore taste card

consolestore keeps a small local "card" so ordering needs less back-and-forth. It
is built automatically from real orders — the user never has to set it up — and you
read or adjust it with `get_card` and `update_card`.

## Showing what's remembered

- `get_card` returns `default_address_id` + label, `favorites` (restaurants ranked
  by how often they're ordered), and `prefs` (free-form notes like "vegetarian" or
  "no onion"). It also reconciles against live addresses; relay any `warnings` (e.g.
  a saved default deleted on Swiggy) and offer to set a new default.
- **Empty card** (blank address, no favorites, no prefs — a new user): say so
  plainly rather than implying there's data. Explain it fills in on its own — every
  order sets the default address and repeat orders promote a favorite, so the fastest
  seed is to place an order. Then offer to jump-start now: set a default address,
  and/or record any dietary prefs.

## Changing the card

- **Set a default address:** `list_addresses`, let the user pick (present them
  clearly; if there are none, they add one in the Swiggy app first), then
  `update_card` with that `default_address_id`.
- **Editing prefs — always read-modify-write.** `update_card` with `prefs` REPLACES
  the whole list. So before every prefs write: call `get_card`, take its CURRENT
  `prefs`, apply the change, and send the full result. Never rebuild the list from
  memory of an earlier turn — the card can change between turns (another order, the
  app, the CLI), so a fresh `get_card` is the only safe source.
  - Add ("also no garlic"): current prefs **plus** "no garlic".
  - Remove ("I eat onion now"): current prefs **minus** the matching note.
  - A note already present → leave it; don't create duplicates.
  - Echo the resulting full list back so the user can see nothing was dropped.
- There is nothing to "create" — the card exists already and grows on its own.
  `update_card` only records explicit choices the user states.
- Prefs are advisory hints the ordering flow may honor (e.g. steering a veg choice —
  see the console-order skill); they don't hard-filter menus. Record them, but don't
  promise they auto-enforce.

## How it fills in over time

- Every placed order — from this agent, the app, or the CLI — refreshes the default
  address. Orders through an agent or from a saved preset also bump the ordered
  restaurant in `favorites`. More orders → better suggestions, no manual upkeep.
