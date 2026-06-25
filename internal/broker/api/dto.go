// Package api defines the broker's wire types and a typed RPC client. It is
// shared by the broker (server) and the TUI (client) and imports only stdlib —
// it must never pull in swiggy/store/auth, so the Swiggy capability and tokens
// stay out of the SSH-facing TUI binary.
package api

type Address struct {
	ID    string
	Label string
	City  string
	Line  string
	Full  string
	Lat   float64
	Lng   float64
}

type Restaurant struct {
	ID          string
	Name        string
	City        string
	ETA         string
	Description string
	Rating      float64
}

type MenuItem struct {
	ID           string
	Name         string
	Price        int
	Veg          bool
	Description  string
	Rating       float64
	Customizable bool // has variants or add-ons (fetch options before adding)
}

type Menu struct {
	RestaurantID string
	Items        []MenuItem
}

type CartItem struct {
	ItemID   string
	Quantity int
}

type CartLine struct {
	ItemID   string
	Name     string
	Quantity int
	Price    int
}

type Cart struct {
	CartID    string
	ItemTotal int // Swiggy bill: item subtotal
	Delivery  int // Swiggy bill: delivery charge
	Taxes     int // Swiggy bill: taxes & charges
	Total     int // Swiggy bill: to-pay
	Lines     []CartLine
}

type Order struct {
	ID         string
	Status     string
	Restaurant string
	Total      int
	ETA        string // Swiggy's estimatedDelivery, e.g. "45-50 mins"
}

type AuthStart struct {
	FlowID       string
	AuthorizeURL string
}
