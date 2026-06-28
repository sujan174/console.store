package broker

import (
	"math"
	"strconv"
	"strings"

	"console.store/internal/broker/api"
	"console.store/internal/swiggy"
)

func mapAddresses(in []swiggy.Address) []api.Address {
	out := make([]api.Address, len(in))
	for i, a := range in {
		label := a.Tag
		if label == "" {
			label = a.Category
		}
		// Swiggy gives one formatted line (no separate city/lat/lng); use it for
		// both the short label line and the full address Swiggy needs back.
		out[i] = api.Address{ID: a.ID, Label: label, Line: a.Line, Full: a.Line}
	}
	return out
}

func mapRestaurants(in []swiggy.Restaurant) []api.Restaurant {
	out := make([]api.Restaurant, len(in))
	for i, r := range in {
		desc := strings.Join(r.Cuisines, ", ")
		if r.CostForTwo != "" {
			if desc != "" {
				desc += " · "
			}
			desc += r.CostForTwo
		}
		out[i] = api.Restaurant{
			ID: r.ID, Name: r.Name, City: r.AreaName,
			ETA: r.DeliveryTimeRange, Description: desc, Rating: r.AvgRating,
			Offer: r.Offer, Unavailable: unavailableStatus(r.Availability),
		}
	}
	return out
}

// unavailableStatus reports whether Swiggy's availabilityStatus marks the
// restaurant as not currently deliverable (closed / unserviceable). The exact
// string set is not fully harvested, so this matches defensively on known
// negative markers and treats an empty/unknown status as deliverable — better to
// keep a good restaurant than hide one. Raw values are visible in the
// search_restaurants response logged under CONSOLE_DEBUG_SWIGGY for refinement.
func unavailableStatus(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	if s == "" {
		return false
	}
	for _, bad := range []string{"close", "unserv", "unavail", "not_deliver", "not deliver", "notdeliver", "out of", "temporarily"} {
		if strings.Contains(s, bad) {
			return true
		}
	}
	return false
}

func mapMenu(in swiggy.Menu) api.Menu {
	items := make([]api.MenuItem, len(in.Items))
	for i, m := range in.Items {
		rating, _ := strconv.ParseFloat(m.Rating, 64) // "4.6" -> 4.6; "" -> 0
		items[i] = api.MenuItem{ID: m.ID, Name: m.Name, Price: int(math.Round(m.Price)), Veg: m.Veg, Description: m.Desc, Rating: rating, Customizable: m.HasVariants || m.HasAddons, Category: m.Category, InStock: m.InStock > 0}
	}
	return api.Menu{RestaurantID: in.RestaurantID, Items: items}
}

func mapCart(in swiggy.Cart) api.Cart {
	lines := make([]api.CartLine, len(in.Items))
	for i, l := range in.Items {
		lines[i] = api.CartLine{ItemID: l.ItemID, Name: l.Name, Quantity: l.Quantity, Price: l.Price}
	}
	return api.Cart{
		CartID: in.CartID, Restaurant: in.Restaurant, ItemTotal: in.ItemTotal, Delivery: in.Delivery,
		Taxes: in.Taxes, Total: in.Total, Lines: lines,
		ValidAddons: mapOptions(in.ValidAddons),
	}
}

func mapOrder(in swiggy.Order) api.Order {
	return api.Order{ID: string(in.ID), Status: in.Status, Restaurant: in.Restaurant, Total: in.Total, ETA: in.ETA}
}

func mapCartItems(in []api.CartItem) []swiggy.CartItem {
	out := make([]swiggy.CartItem, len(in))
	for i, c := range in {
		// api.CartItem.ItemID carries the Swiggy menu item id (catalog SwiggyID);
		// update_food_cart wants it as menu_item_id.
		ci := swiggy.CartItem{MenuItemID: c.ItemID, Quantity: c.Quantity}
		for _, v := range c.VariantsV2 {
			ci.VariantsV2 = append(ci.VariantsV2, swiggy.CartVariant{GroupID: v.GroupID, VariationID: v.VariationID})
		}
		for _, v := range c.VariantsLegacy {
			ci.Variants = append(ci.Variants, swiggy.CartVariant{GroupID: v.GroupID, VariationID: v.VariationID})
		}
		for _, a := range c.Addons {
			ci.Addons = append(ci.Addons, swiggy.CartAddon{GroupID: a.GroupID, ChoiceID: a.ChoiceID})
		}
		out[i] = ci
	}
	return out
}

func mapTracking(t swiggy.Tracking) api.Tracking {
	return api.Tracking{OrderID: t.OrderID, Status: t.Status, ETA: t.ETA, Active: t.Active}
}

func mapOptions(in []swiggy.OptionGroup) []api.OptionGroup {
	out := make([]api.OptionGroup, len(in))
	for i, g := range in {
		choices := make([]api.OptionChoice, len(g.Choices))
		for j, ch := range g.Choices {
			choices[j] = api.OptionChoice{ID: ch.ID, Name: ch.Name, Price: ch.Price, InStock: ch.InStock}
		}
		out[i] = api.OptionGroup{ID: g.ID, Name: g.Name, Min: g.Min, Max: g.Max, Variant: g.Variant, Absolute: g.Absolute, Choices: choices}
	}
	return out
}
