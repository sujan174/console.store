package swiggy

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// These structs decode the fields console.store uses; unknown fields are
// ignored. They intentionally mirror catalog shapes so the catalog/swiggy
// adapter (a later slice) maps them with minimal translation.

// Address matches Swiggy's get_addresses response items. The response wraps
// them: {"addresses":[...],"total":N} — see addressesEnvelope.
type Address struct {
	ID       string `json:"id"`
	Tag      string `json:"addressTag"`      // "Home", "Work", "Basketball Court"
	Category string `json:"addressCategory"` // "Home", "Work", "Other"
	Line     string `json:"addressLine"`     // full formatted address text
}

type addressesEnvelope struct {
	Addresses []Address `json:"addresses"`
}

// Restaurant matches Swiggy's search_restaurants response items, wrapped in
// {"query":...,"restaurants":[...]} — see restaurantsEnvelope.
type Restaurant struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	AreaName          string   `json:"areaName"`
	AvgRating         float64  `json:"avgRating"`
	CostForTwo        string   `json:"costForTwo"`
	Cuisines          []string `json:"cuisines"`
	DeliveryTimeRange string   `json:"deliveryTimeRange"`
	Offer             string   `json:"offer"`
	Availability      string   `json:"availabilityStatus"`
}

type restaurantsEnvelope struct {
	Restaurants []Restaurant `json:"restaurants"`
}

// MenuItem matches an item inside a get_restaurant_menu category. Note Rating
// arrives as a STRING ("4.6"), not a number.
type MenuItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Desc string `json:"description"`
	// Price arrives in rupees and may be fractional (e.g. 1185.59 on coffee
	// bags). Keep it float here; the broker rounds to whole rupees for the TUI.
	// It MUST be float64 — an int field fails to unmarshal a decimal and would
	// drop the entire menu.
	Price       float64 `json:"price"`
	Veg         bool    `json:"isVeg"`
	Rating      string  `json:"rating"`
	InStock     int     `json:"inStock"`
	Bestseller  bool    `json:"isBestseller"`
	HasVariants bool    `json:"hasVariants"`
	HasAddons   bool    `json:"hasAddons"`
	Category    string  `json:"-"` // filled by collect(); not from JSON
}

// menuEnvelope matches get_restaurant_menu. A category EITHER holds items
// directly OR nests them under subcategories (e.g. "Summer Specials" →
// "Summer Special Beverages" → items). Both levels use the same item shape, so
// menuCategory is recursive and collect() flattens the whole tree. Missing the
// subcategory branch silently drops items (that hid Blue Tokai's Iced Mocha).
type menuEnvelope struct {
	Categories []menuCategory `json:"categories"`
}

type menuCategory struct {
	Title         string         `json:"title"`
	Items         []MenuItem     `json:"items"`
	Subcategories []menuCategory `json:"subcategories"`
}

// collect flattens the category tree, tagging each item with the most specific
// (sub)category title it belongs to.
func (c menuCategory) collect() []MenuItem {
	out := make([]MenuItem, 0, len(c.Items))
	for _, it := range c.Items {
		if it.Category == "" {
			it.Category = c.Title
		}
		out = append(out, it)
	}
	for _, sub := range c.Subcategories {
		out = append(out, sub.collect()...)
	}
	return out
}

// Menu is the flattened (category-merged) item list for one restaurant.
type Menu struct {
	RestaurantID string
	Items        []MenuItem
}

// CartItem is the SENT shape for update_food_cart's cartItems entries. Swiggy
// requires the snake_case "menu_item_id" — "itemId" yields
// INVALID_ITEM_IDS_IN_REQUEST. VariantsV2/Addons carry the customization
// selections (omitted for simple items).
type CartItem struct {
	MenuItemID string        `json:"menu_item_id"`
	Quantity   int           `json:"quantity"`
	VariantsV2 []CartVariant `json:"variantsV2,omitempty"`
	Variants   []CartVariant `json:"variants,omitempty"` // legacy variations channel
	Addons     []CartAddon   `json:"addons,omitempty"`
}

// CartVariant selects one variation within a variant group.
type CartVariant struct {
	GroupID     string `json:"group_id"`
	VariationID string `json:"variation_id"`
}

// CartAddon selects one choice within an addon group.
type CartAddon struct {
	GroupID  string `json:"group_id"`
	ChoiceID string `json:"choice_id"`
}

// CartLine is the typed, TUI-facing shape for one cart item (post-conversion).
type CartLine struct {
	ItemID   string
	Name     string
	Quantity int
	Price    int // whole rupees
}

// Cart is the typed, TUI-facing cart (converted from cartEnvelope). The pricing
// fields carry Swiggy's real bill breakdown (whole rupees) for an accurate
// checkout split instead of mock delivery/coupon math.
type Cart struct {
	CartID    string
	ItemTotal int // pricing.item_total
	Delivery  int // pricing.delivery_charge
	Taxes     int // pricing.taxes_and_charges
	Total     int // pricing.to_pay
	Items     []CartLine
	// ValidAddons are the add-on groups Swiggy reports as valid for the current
	// variant/add-on selection (the server-driven customization mechanism). Used
	// by the customize wizard to render the next page. Empty for simple carts.
	ValidAddons []OptionGroup
}

// cartEnvelope decodes the real get_food_cart / update_food_cart response. The
// cart lives under "data" (null when empty); statusCode 0 == success.
type cartEnvelope struct {
	Data          *cartData `json:"data"`
	StatusCode    int       `json:"statusCode"`
	StatusMessage string    `json:"statusMessage"`
	ErrorCodes    []string  `json:"errorCodes"`
	Successful    *bool     `json:"successful"`
}

type cartData struct {
	CartID    json.Number `json:"cart_id"`
	ItemCount int         `json:"item_count"`
	Items     []struct {
		MenuItemID json.Number `json:"menu_item_id"`
		Name       string      `json:"name"`
		Quantity   int         `json:"quantity"`
		FinalPrice float64     `json:"final_price"`
		Total      float64     `json:"total"`
		// valid_addons is nested PER ITEM (not at the data root). It carries the
		// authoritative per-choice availability (inStock) that search_menu omits.
		ValidAddons []validAddonGroup `json:"valid_addons"`
	} `json:"items"`
	Pricing struct {
		ItemTotal       float64 `json:"item_total"`
		DeliveryCharge  float64 `json:"delivery_charge"`
		TaxesAndCharges float64 `json:"taxes_and_charges"`
		ToPay           float64 `json:"to_pay"`
	} `json:"pricing"`
	Restaurant struct {
		Name string `json:"name"`
	} `json:"restaurant"`
}

// validAddonGroup decodes one entry of a cart item's valid_addons array. Field
// names + numeric ids match the LIVE response (group_id / id are numbers;
// inStock / minAddons / maxAddons are camelCase) — NOT search_menu's shape.
type validAddonGroup struct {
	GroupID   json.Number `json:"group_id"`
	GroupName string      `json:"group_name"`
	MinAddons int         `json:"minAddons"`
	MaxAddons int         `json:"maxAddons"`
	Choices   []struct {
		ID      json.Number `json:"id"`
		Name    string      `json:"name"`
		Price   float64     `json:"price"`
		InStock int         `json:"inStock"`
	} `json:"choices"`
}

// cartError returns a non-nil error when Swiggy rejected the cart operation
// (error codes present, an explicit successful:false, or a non-success status
// with no cart data). A successful empty cart (statusCode 0, data null, no
// error codes) is NOT an error. Used so cart-sync failures surface to the user
// instead of silently falling back to the placeholder bill.
func (e cartEnvelope) cartError() error {
	rejected := len(e.ErrorCodes) > 0 || (e.Successful != nil && !*e.Successful)
	if !rejected && e.StatusCode != 0 && e.Data == nil {
		rejected = true
	}
	if !rejected {
		return nil
	}
	msg := e.StatusMessage
	if msg == "" {
		msg = "cart update rejected"
	}
	if len(e.ErrorCodes) > 0 {
		return fmt.Errorf("swiggy: %s (%s)", msg, strings.Join(e.ErrorCodes, ", "))
	}
	return fmt.Errorf("swiggy: %s", msg)
}

// validAddons converts the cart response's valid_addons into typed OptionGroups
// (always add-on groups — Variant=false, additive prices). Empty when the cart
// reports none.
func (e cartEnvelope) validAddons() []OptionGroup {
	if e.Data == nil {
		return nil
	}
	var out []OptionGroup
	seen := map[string]bool{} // dedup groups shared across cart items
	for _, it := range e.Data.Items {
		for _, g := range it.ValidAddons {
			gid := g.GroupID.String()
			if seen[gid] {
				continue
			}
			seen[gid] = true
			og := OptionGroup{ID: gid, Name: g.GroupName, Min: g.MinAddons, Max: g.MaxAddons}
			for _, ch := range g.Choices {
				og.Choices = append(og.Choices, OptionChoice{
					ID: ch.ID.String(), Name: ch.Name, Price: int(math.Round(ch.Price)), InStock: ch.InStock == 1,
				})
			}
			if len(og.Choices) > 0 {
				out = append(out, og)
			}
		}
	}
	return out
}

// toCart converts a decoded cartEnvelope into the typed Cart. An empty cart
// (data null) yields a zero Cart. Prices arrive as floats; round to rupees.
func (e cartEnvelope) toCart() Cart {
	if e.Data == nil {
		return Cart{}
	}
	d := e.Data
	c := Cart{
		CartID:    d.CartID.String(),
		ItemTotal: int(math.Round(d.Pricing.ItemTotal)),
		Delivery:  int(math.Round(d.Pricing.DeliveryCharge)),
		Taxes:     int(math.Round(d.Pricing.TaxesAndCharges)),
		Total:     int(math.Round(d.Pricing.ToPay)),
	}
	for _, it := range d.Items {
		c.Items = append(c.Items, CartLine{
			ItemID:   it.MenuItemID.String(),
			Name:     it.Name,
			Quantity: it.Quantity,
			Price:    int(math.Round(it.FinalPrice)),
		})
	}
	return c
}

type Coupon struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Amount      int    `json:"amount"`
}

// Order matches place_food_order / get_food_orders. orderId is a NUMBER in the
// API (e.g. 241351408816590) — an int/string field fails to decode it and the
// order silently looks failed even when CONFIRMED. json.Number accepts both.
type Order struct {
	ID         json.Number `json:"orderId"`
	Status     string      `json:"status"`
	Restaurant string      `json:"restaurantName"`
	Total      int         `json:"totalAmount"`
	ETA        string      `json:"estimatedDelivery"`
}

type Tracking struct {
	OrderID string
	Status  string
	ETA     string
	Active  bool
}

type Product struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Price int    `json:"price"`
}
