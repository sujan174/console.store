package datasource

import (
	"sort"
	"strings"

	"console.store/internal/broker/api"
	"console.store/internal/catalog"
)

// cleanAddrLine strips a short, digit-free "<name>: " prefix that Swiggy prepends
// to the formatted address (e.g. "Sujan: FD 46 HAL…" → "FD 46 HAL…"). A delivery
// line should show the place, not the recipient's name. Real address parts start
// with a number or are longer/comma-bearing, so they're left intact.
func cleanAddrLine(s string) string {
	if i := strings.Index(s, ": "); i > 0 && i <= 24 {
		if prefix := s[:i]; !strings.ContainsAny(prefix, "0123456789,") {
			return strings.TrimSpace(s[i+2:])
		}
	}
	return s
}

func toAddresses(in []api.Address) []catalog.Address {
	out := make([]catalog.Address, len(in))
	for i, a := range in {
		out[i] = catalog.Address{ID: a.ID, Label: a.Label, City: a.City, Line: cleanAddrLine(a.Line), Full: a.Full, Lat: a.Lat, Lng: a.Lng}
	}
	return out
}

func toPlaces(in []api.Restaurant, section catalog.Section) []catalog.Place {
	out := make([]catalog.Place, len(in))
	for i, r := range in {
		out[i] = catalog.Place{
			ID: r.ID, SwiggyID: r.ID, Name: r.Name, City: r.City,
			Section: section, ETA: r.ETA, Rating: r.Rating, Description: r.Description,
			Offer: r.Offer,
		}
	}
	return out
}

func toOptionGroups(in []api.OptionGroup) []catalog.OptionGroup {
	out := make([]catalog.OptionGroup, len(in))
	for i, g := range in {
		choices := make([]catalog.Choice, len(g.Choices))
		for j, ch := range g.Choices {
			choices[j] = catalog.Choice{ID: ch.ID, Name: ch.Name, Price: ch.Price, InStock: ch.InStock}
		}
		// Sort choices cheapest-first WITHIN the group (stable, so equal-price
		// choices keep Swiggy's order). Group order is untouched. Selection +
		// cart-send key off choice ID, so re-ordering is display-only safe; the
		// required single-choice default (Choices[0] / first in-stock) becomes
		// the cheapest, which is the sensible default.
		sort.SliceStable(choices, func(a, b int) bool { return choices[a].Price < choices[b].Price })
		out[i] = catalog.OptionGroup{ID: g.ID, Name: g.Name, Min: g.Min, Max: g.Max, Variant: g.Variant, Absolute: g.Absolute, Choices: choices}
	}
	return out
}

func toMenuPlace(m api.Menu) catalog.Place {
	items := make([]catalog.Item, len(m.Items))
	for i, it := range m.Items {
		items[i] = catalog.Item{
			ID: it.ID, SwiggyID: it.ID, Name: it.Name, Price: it.Price,
			Veg: it.Veg, Desc: it.Description, Rating: it.Rating,
			Customizable: it.Customizable, Category: it.Category,
		}
	}
	return catalog.Place{ID: m.RestaurantID, SwiggyID: m.RestaurantID, Items: items}
}
