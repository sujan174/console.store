// Package api defines the broker's wire types, shared by the broker, the TUI
// datasource, and the headless CLI. It imports only stdlib — it must never pull
// in swiggy/auth, so these stay plain data types with no Swiggy capability or
// tokens attached.
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
	Offer       string
	// Unavailable is true when Swiggy reports the restaurant as closed or
	// unserviceable to the address (availabilityStatus). Zero value = deliverable,
	// so mock/test restaurants are kept by default.
	Unavailable bool
}

type MenuItem struct {
	ID           string
	Name         string
	Price        int
	Veg          bool
	Description  string
	Rating       float64
	Customizable bool
	Category     string
	// InStock reflects Swiggy's inStock flag (true = orderable). Mapped from the
	// menu's inStock>0; defaults true for mock items via the catalog mapping.
	InStock bool
}

type Menu struct {
	RestaurantID string
	Items        []MenuItem
}

type CartItem struct {
	ItemID         string
	Quantity       int
	VariantsV2     []CartVariantSel // variantsV2 channel (absolute-price variants)
	VariantsLegacy []CartVariantSel // legacy variations channel
	Addons         []CartAddonSel
}

// CartVariantSel / CartAddonSel are the user's customization selections sent
// with a cart line.
type CartVariantSel struct {
	GroupID     string
	VariationID string
}
type CartAddonSel struct {
	GroupID  string
	ChoiceID string
}

// OptionGroup / OptionChoice are an item's customization options (variant or
// addon group) returned by ItemOptions.
type OptionGroup struct {
	ID       string
	Name     string
	Min      int
	Max      int
	Variant  bool
	Absolute bool
	Choices  []OptionChoice
}
type OptionChoice struct {
	ID      string
	Name    string
	Price   int
	InStock bool
	Default bool // variantsV2 default variation — omitted from the cart wire (Swiggy pre-applies it)
}

type CartLine struct {
	ItemID    string
	Name      string
	Quantity  int
	Price     int
	Available bool // false when Swiggy reports the cart item as out of stock
}

// PaymentMethod is one selectable payment option (get_payment_options). Kind is
// "qr", "intent", or "cod".
type PaymentMethod struct {
	ID          string
	DisplayName string
	Kind        string
	PaymentCode string
}

// PaymentOptions is the live payment picker for the current cart. QR is the
// terminal scan-to-pay method (nil when the user isn't UPI-eligible).
type PaymentOptions struct {
	QR           *PaymentMethod
	Intents      []PaymentMethod
	CODAvailable bool
}

// PendingPayment is a placed-but-unpaid food order awaiting UPI payment. Carry
// it verbatim between place → poll → confirm; UPIString is what the TUI renders
// as a QR.
type PendingPayment struct {
	OrderID   string
	PaasID    string
	UPIString string
	BridgeURL string
	CartID    string
	AddressID string
	Lat, Lng  float64
	Amount    int
	ExpiresAt int64 // unix millis; payment window deadline (Swiggy's 5 min)
}

// PaymentStatus mirrors swiggy.PaymentStatus (same ordering).
type PaymentStatus int

const (
	PayPending PaymentStatus = iota
	PaySuccess
	PayFailed
)

type Cart struct {
	CartID string
	// Restaurant is the name of the restaurant the cart belongs to (from Swiggy's
	// cart.restaurant.name). Used to seed cartRestaurant when an existing cart is
	// pulled at launch, so the conflict modal fires on a cross-restaurant add.
	Restaurant string
	ItemTotal  int // Swiggy bill: item subtotal
	Delivery   int // Swiggy bill: delivery charge
	Taxes      int // Swiggy bill: taxes & charges
	Total      int // Swiggy bill: to-pay
	Lines      []CartLine
	// ValidAddons are the add-on groups Swiggy reports valid for the current
	// variant selection — drives the customize wizard's next page.
	ValidAddons []OptionGroup
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

// UpdateCartArgs is the argument bundle for a cart sync. It outlived the RPC
// transport: broker.Service.UpdateCart and the datasource both take it.
type UpdateCartArgs struct {
	AccountID      string
	AddressID      string
	RestaurantID   string
	RestaurantName string
	Items          []CartItem
}

// ---- Instamart (grocery) vertical ----
// The Instamart cart is a SEPARATE cart from the Food cart: it binds to the
// delivery address (not a restaurant), allows items from multiple dark stores,
// and is keyed by SKU-level spinIds instead of menu_item_ids.

// IMVariantSel is one purchasable variation (pack size) of a product.
type IMVariantSel struct {
	SpinID  string // variant id sent to update_cart
	SkuID   string // required alongside SpinID by update_cart
	Label   string // "250 ml x 4"
	Price   int    // effective rupees (offer price)
	MRP     int    // strike-through price; 0 or ==Price when no offer
	InStock bool
}

// IMProduct is one product from search_products / your_go_to_items.
type IMProduct struct {
	ID       string // productId
	Name     string
	Brand    string
	InStock  bool
	Variants []IMVariantSel
}

// IMCartLine is one line of the live Instamart cart.
type IMCartLine struct {
	SpinID    string
	Name      string
	Quantity  int
	Price     int // per-unit rupees
	Available bool
}

// IMCart carries Swiggy's real Instamart bill. Handling is Instamart's
// handling fee (a row Food doesn't have). AddrLat/AddrLng are the delivery
// coordinates from the cart's selectedAddressDetails — the only source for
// them (track_order requires coordinates; get_addresses/get_orders omit them),
// so they are captured here and persisted at placement.
type IMCart struct {
	AddrID         string
	AddrLat        float64
	AddrLng        float64
	ItemTotal      int
	Delivery       int
	Handling       int
	Taxes          int
	Total          int
	Lines          []IMCartLine
	PaymentMethods []string
}

// IMCartItem is the SENT shape for an Instamart cart sync (update_cart
// REPLACES the whole cart with these items).
type IMCartItem struct {
	SpinID   string
	SkuID    string
	Quantity int
}

// IMOrder is one Instamart order. Status is the human display state
// ("Order picked up"); Detail is the sub-line (rider updates). The live
// payload carries NO coordinates — Lat/Lng stay for drift tolerance, but
// tracking coordinates are persisted from the CART at placement time.
type IMOrder struct {
	ID     string
	Status string
	Detail string
	ETA    string
	Total  int
	Lat    float64
	Lng    float64
	Items  []string
	Active bool
}
