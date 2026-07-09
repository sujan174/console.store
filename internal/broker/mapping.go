package broker

import (
	"math"
	"strconv"
	"strings"

	"consolestore/internal/broker/api"
	"consolestore/internal/swiggy"
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
		lines[i] = api.CartLine{ItemID: l.ItemID, Name: l.Name, Quantity: l.Quantity, Price: l.Price, Available: l.Available}
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

func mapPaymentMethod(m *swiggy.PaymentMethod) *api.PaymentMethod {
	if m == nil {
		return nil
	}
	return &api.PaymentMethod{ID: m.ID, DisplayName: m.DisplayName, Kind: m.Kind, PaymentCode: m.PaymentCode}
}

func mapPaymentOptions(o swiggy.PaymentOptions) api.PaymentOptions {
	out := api.PaymentOptions{CODAvailable: o.CODAvailable, QR: mapPaymentMethod(o.QR)}
	for i := range o.Intents {
		out.Intents = append(out.Intents, *mapPaymentMethod(&o.Intents[i]))
	}
	return out
}

func mapPending(p swiggy.PendingPayment) api.PendingPayment {
	return api.PendingPayment{
		OrderID: p.OrderID, PaasID: p.PaasID, UPIString: p.UPIString, BridgeURL: p.BridgeURL, CartID: p.CartID,
		AddressID: p.AddressID, Lat: p.Lat, Lng: p.Lng, Amount: p.Amount, ExpiresAt: p.ExpiresAt,
	}
}

func unmapPending(p api.PendingPayment) swiggy.PendingPayment {
	return swiggy.PendingPayment{
		OrderID: p.OrderID, PaasID: p.PaasID, UPIString: p.UPIString, BridgeURL: p.BridgeURL, CartID: p.CartID,
		AddressID: p.AddressID, Lat: p.Lat, Lng: p.Lng, Amount: p.Amount, ExpiresAt: p.ExpiresAt,
	}
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
	return api.Tracking{OrderID: t.OrderID, Status: t.Status, Detail: t.Detail, ETA: t.ETA, Active: t.Active, Known: t.Known}
}

func mapOptions(in []swiggy.OptionGroup) []api.OptionGroup {
	out := make([]api.OptionGroup, len(in))
	for i, g := range in {
		choices := make([]api.OptionChoice, len(g.Choices))
		for j, ch := range g.Choices {
			choices[j] = api.OptionChoice{ID: ch.ID, Name: ch.Name, Price: ch.Price, InStock: ch.InStock, Default: ch.Default}
		}
		out[i] = api.OptionGroup{ID: g.ID, Name: g.Name, Min: g.Min, Max: g.Max, Variant: g.Variant, Absolute: g.Absolute, Choices: choices}
	}
	return out
}

func mapIMProducts(in []swiggy.IMProduct) []api.IMProduct {
	out := make([]api.IMProduct, 0, len(in))
	for _, p := range in {
		mp := api.IMProduct{ID: p.ID, Name: p.Name, Brand: p.Brand, InStock: p.InStock && p.Avail}
		for _, v := range p.Variants {
			mp.Variants = append(mp.Variants, api.IMVariantSel{
				SpinID: v.SpinID, Label: v.QtyDesc, Price: v.Price.Rupees(),
				MRP: int(math.Round(v.Price.MRP)), InStock: v.InStock,
			})
		}
		if len(mp.Variants) == 0 {
			continue // a product without a purchasable SKU is dead weight
		}
		out = append(out, mp)
	}
	return out
}

func mapIMCart(in swiggy.IMCart) api.IMCart {
	lines := make([]api.IMCartLine, len(in.Items))
	for i, l := range in.Items {
		lines[i] = api.IMCartLine{SpinID: l.SpinID, Name: l.Name, Quantity: l.Quantity, Price: l.Price, Available: l.Available}
	}
	return api.IMCart{
		AddrID: in.AddrID, AddrLat: in.AddrLat, AddrLng: in.AddrLng,
		ItemTotal: in.ItemTotal, Delivery: in.Delivery, Handling: in.Handling,
		Taxes: in.Taxes, Total: in.Total, Lines: lines, PaymentMethods: in.PaymentMethods,
	}
}

func mapIMCartItems(in []api.IMCartItem) []swiggy.IMCartItem {
	out := make([]swiggy.IMCartItem, len(in))
	for i, c := range in {
		out[i] = swiggy.IMCartItem{SpinID: c.SpinID, Quantity: c.Quantity}
	}
	return out
}

func mapIMOrders(in []swiggy.IMOrder) []api.IMOrder {
	out := make([]api.IMOrder, len(in))
	for i, o := range in {
		out[i] = api.IMOrder{ID: o.ID, Status: o.Status, Detail: o.Detail, ETA: o.ETA, Total: o.Total, Lat: o.Lat, Lng: o.Lng, Items: o.Items, Active: o.Active}
	}
	return out
}
