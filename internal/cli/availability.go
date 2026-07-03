package cli

import "consolestore/internal/localstore"

// availability is the result of a read-only stock probe against a preset's
// lines — used ONLY to decorate the pick list / `alias list --check` before a
// cart is touched. It never blocks ordering by itself: the existing order-time
// gate (the live cart's Available flags, checked in placePreset/placeIMPreset)
// is still the authoritative, final check.
type availability struct {
	known       bool   // false = probe failed or was skipped; render unmarked
	unavailable bool   // true = at least one line is confirmed sold out / gone
	itemName    string // the (first) unavailable line's display name, for the suffix
}

// maxProbeLines caps how many of a preset's lines are checked per probe, so a
// large preset (many lines) can't spray a burst of read calls just to render a
// pick list. Presets are small in practice (a "usual" order); checking a
// handful of lines is enough to catch the common case, and a probe failure
// never blocks the flow anyway.
const maxProbeLines = 4

// probeAvailability checks a preset's lines against the live menu (food) or a
// per-line search (instamart) WITHOUT touching any cart. It never returns an
// error to the caller — a probe failure (network, auth, whatever) just yields
// an "unknown" result so the flow is never blocked on this being best-effort.
func probeAvailability(be Backend, p localstore.Preset) availability {
	if p.IsInstamart() {
		return probeIMAvailability(be, p)
	}
	return probeFoodAvailability(be, p)
}

// probeFoodAvailability fetches the restaurant's live menu once and checks each
// preset line's ItemID against it. A line whose id isn't present in the menu is
// treated as unavailable ("no longer on the menu") — Swiggy drops items from
// the menu payload once they're delisted, not just flagged out of stock.
func probeFoodAvailability(be Backend, p localstore.Preset) availability {
	menu, err := be.Menu(p.AddrID, p.RestaurantID)
	if err != nil {
		return availability{} // unknown — never block on a probe failure
	}
	byID := make(map[string]bool, len(menu.Items)) // itemID -> InStock
	present := make(map[string]bool, len(menu.Items))
	for _, it := range menu.Items {
		byID[it.ID] = it.InStock
		present[it.ID] = true
	}
	lines := p.Lines
	if len(lines) > maxProbeLines {
		lines = lines[:maxProbeLines]
	}
	for _, l := range lines {
		if !present[l.ItemID] {
			return availability{known: true, unavailable: true, itemName: l.Name}
		}
		if !byID[l.ItemID] {
			return availability{known: true, unavailable: true, itemName: l.Name}
		}
	}
	return availability{known: true}
}

// probeIMAvailability searches for each preset line's product (by its saved
// Name, since Instamart has no single "get by spinId" tool) and checks whether
// the line's spinId still appears among the results as in-stock. Capped at
// maxProbeLines separate search calls — sequential, since downstream calls are
// already rate-limited (no benefit to firing them concurrently here).
func probeIMAvailability(be Backend, p localstore.Preset) availability {
	lines := p.Lines
	if len(lines) > maxProbeLines {
		lines = lines[:maxProbeLines]
	}
	for _, l := range lines {
		products, err := be.IMSearch(p.AddrID, l.Name)
		if err != nil {
			continue // unknown for this line — don't let one bad search block the rest
		}
		found := false
		inStock := false
		for _, prod := range products {
			for _, v := range prod.Variants {
				if v.SpinID == l.ItemID {
					found = true
					inStock = v.InStock
				}
			}
		}
		if !found || !inStock {
			return availability{known: true, unavailable: true, itemName: l.Name}
		}
	}
	return availability{known: true}
}
