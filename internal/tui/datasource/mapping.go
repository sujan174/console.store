package datasource

import (
	"sort"
	"strings"

	"consolestore/internal/broker/api"
	"consolestore/internal/catalog"
	swiggysnap "consolestore/internal/catalog/swiggy"
	"consolestore/internal/localstore"
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
	out := make([]catalog.Place, 0, len(in))
	for _, r := range in {
		// Hide restaurants Swiggy reports as closed / unserviceable — they can't
		// take an order, so opening them only leads to a failed add.
		if r.Unavailable {
			continue
		}
		out = append(out, catalog.Place{
			ID: r.ID, SwiggyID: r.ID, Name: r.Name, City: r.City,
			Section: section, ETA: r.ETA, Rating: r.Rating, Description: r.Description,
			Offer: r.Offer,
		})
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

// toIMItems maps Instamart products (search_products / your_go_to_items) to
// catalog.Items. Each product's default variant (first in-stock, else first)
// sets SwiggyID/Price; a product with more than one variant is Customizable
// with a synthesized single-choice "pack size" group so the TUI never needs a
// network round-trip to open the picker.
func toIMItems(ps []api.IMProduct) []catalog.Item {
	out := make([]catalog.Item, len(ps))
	for i, p := range ps {
		def, hasInStock := defaultIMVariant(p)
		desc := joinNonEmpty(" · ", p.Brand, def.Label)

		item := catalog.Item{
			ID:         p.ID,
			SwiggyID:   def.SpinID,
			Name:       p.Name,
			Price:      def.Price,
			Desc:       desc,
			Section:    catalog.SectionInstamart,
			OutOfStock: !p.InStock || !hasInStock,
		}

		if len(p.Variants) > 1 {
			choices := make([]catalog.Choice, len(p.Variants))
			for j, v := range p.Variants {
				choices[j] = catalog.Choice{ID: v.SpinID, Name: v.Label, Price: v.Price, InStock: v.InStock}
			}
			item.Customizable = true
			item.Options = []catalog.OptionGroup{{
				ID: "im-size", Name: "pack size", Min: 1, Max: 1,
				Variant: true, Absolute: true, Choices: choices,
			}}
		}

		out[i] = item
	}
	return out
}

// defaultIMVariant picks a product's default purchasable variant: the first
// in-stock one, else the first overall (so an out-of-stock product still shows
// a representative price). hasInStock reports whether any variant was in
// stock — toIMItems uses it to mark the item OutOfStock when none are.
// toCachedIM projects live products into the disk-cache shape (mirrors
// api.IMProduct so the reverse round-trips through toIMItems).
func toCachedIM(ps []api.IMProduct) []localstore.CachedIMProduct {
	out := make([]localstore.CachedIMProduct, len(ps))
	for i, p := range ps {
		cp := localstore.CachedIMProduct{ID: p.ID, Name: p.Name, Brand: p.Brand, InStock: p.InStock}
		for _, v := range p.Variants {
			cp.Variants = append(cp.Variants, localstore.CachedIMVariant{
				SpinID: v.SpinID, Label: v.Label, Price: v.Price, MRP: v.MRP, InStock: v.InStock,
			})
		}
		out[i] = cp
	}
	return out
}

// fromCachedIM rebuilds live products from the disk cache so the SAME toIMItems
// synthesis reconstructs identical catalog rows.
func fromCachedIM(cs []localstore.CachedIMProduct) []api.IMProduct {
	out := make([]api.IMProduct, len(cs))
	for i, c := range cs {
		p := api.IMProduct{ID: c.ID, Name: c.Name, Brand: c.Brand, InStock: c.InStock}
		for _, v := range c.Variants {
			p.Variants = append(p.Variants, api.IMVariantSel{
				SpinID: v.SpinID, Label: v.Label, Price: v.Price, MRP: v.MRP, InStock: v.InStock,
			})
		}
		out[i] = p
	}
	return out
}

// SeedIMFromCache paints the last-known Instamart product list for
// (addressID, query) from disk into the snapshot, so a relaunched browse shows
// results instantly instead of "loading…" while the live fetch streams over it
// (stale-while-revalidate). Returns true when it seeded. Best-effort: a miss or
// stale entry is a silent no-op. The live cart sync at prepare/checkout is
// always the money authority, so a stale cached price/stock can never mis-bill.
func SeedIMFromCache(snap *swiggysnap.Snapshot, addressID, query string) bool {
	if snap == nil || addressID == "" {
		return false
	}
	cached, ok := localstore.LoadCachedInstamart(addressID, query)
	if !ok {
		return false
	}
	snap.SetInstamart(addressID, query, toIMItems(fromCachedIM(cached)))
	return true
}

func defaultIMVariant(p api.IMProduct) (v api.IMVariantSel, hasInStock bool) {
	for _, cand := range p.Variants {
		if cand.InStock {
			return cand, true
		}
	}
	if len(p.Variants) > 0 {
		return p.Variants[0], false
	}
	return api.IMVariantSel{}, false
}

// joinNonEmpty joins the non-empty parts with sep.
func joinNonEmpty(sep string, parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, sep)
}

func toMenuPlace(m api.Menu) catalog.Place {
	items := make([]catalog.Item, len(m.Items))
	for i, it := range m.Items {
		items[i] = catalog.Item{
			ID: it.ID, SwiggyID: it.ID, Name: it.Name, Price: it.Price,
			Veg: it.Veg, Desc: it.Description, Rating: it.Rating,
			Customizable: it.Customizable, Category: it.Category,
			OutOfStock: !it.InStock,
		}
	}
	return catalog.Place{ID: m.RestaurantID, SwiggyID: m.RestaurantID, Items: items}
}
