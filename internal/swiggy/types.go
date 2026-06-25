package swiggy

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
	Price      float64 `json:"price"`
	Veg        bool    `json:"isVeg"`
	Rating     string  `json:"rating"`
	InStock    int     `json:"inStock"`
	Bestseller bool    `json:"isBestseller"`
}

// menuEnvelope matches get_restaurant_menu: {"categories":[{"items":[...]}]}.
type menuEnvelope struct {
	Categories []struct {
		Items []MenuItem `json:"items"`
	} `json:"categories"`
}

// Menu is the flattened (category-merged) item list for one restaurant.
type Menu struct {
	RestaurantID string
	Items        []MenuItem
}

type CartItem struct {
	ItemID   string `json:"itemId"`
	Quantity int    `json:"quantity"`
}

type Cart struct {
	CartID string `json:"cartId"`
	Total  int    `json:"total"`
	Items  []struct {
		ItemID   string `json:"itemId"`
		Name     string `json:"name"`
		Quantity int    `json:"quantity"`
		Price    int    `json:"price"`
	} `json:"items"`
}

type Coupon struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Amount      int    `json:"amount"`
}

type Order struct {
	ID         string `json:"orderId"`
	Status     string `json:"status"`
	Restaurant string `json:"restaurantName"`
	Total      int    `json:"total"`
	PlacedAt   string `json:"placedAt"`
}

type Tracking struct {
	OrderID string `json:"orderId"`
	Status  string `json:"status"`
	ETA     string `json:"eta"`
}

type Product struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Price int    `json:"price"`
}
