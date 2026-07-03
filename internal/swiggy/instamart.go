package swiggy

import (
	"context"
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

// Instamart (mcp.swiggy.com/im) tool wrappers. The product/search AND
// cart shapes were harvested live (2026-07-03) — the primary struct tags match
// the real payloads; alternate spellings remain as fallbacks for drift, and
// CallTool's debug logger keeps the raw JSON so misses stay harvestable.
// Still unharvested: checkout + get_orders(non-empty) + track_order (need a
// real order). See memory: instamart-mcp-schemas.

// IMVariant is one SKU-level variation of a product (a pack size). spinId is
// the id sent to update_cart — carts hold variants, never parent products.
type IMVariant struct {
	SpinID  string  `json:"spinId"`
	QtyDesc string  `json:"quantityDescription"` // "250 ml x 4"
	Name    string  `json:"displayName"`
	Brand   string  `json:"brandName"`
	Price   imPrice `json:"price"`
	InStock bool    `json:"isInStockAndAvailable"`
}

type imPrice struct {
	MRP        float64 `json:"mrp"`
	OfferPrice float64 `json:"offerPrice"`
}

// Rupees is the effective price: the offer price when present, else MRP.
func (p imPrice) Rupees() int {
	if p.OfferPrice > 0 {
		return int(math.Round(p.OfferPrice))
	}
	return int(math.Round(p.MRP))
}

// IMProduct matches search_products / your_go_to_items items.
type IMProduct struct {
	ID       string      `json:"productId"`
	Name     string      `json:"displayName"`
	Brand    string      `json:"brand"`
	InStock  bool        `json:"inStock"`
	Avail    bool        `json:"isAvail"`
	Variants []IMVariant `json:"variations"`
}

// imProductsEnvelope wraps both product tools. nextOffset arrives as a STRING
// ("1"); json.Number tolerates either.
type imProductsEnvelope struct {
	NextOffset json.Number `json:"nextOffset"`
	Products   []IMProduct `json:"products"`
}

// SearchIMProducts searches the Instamart catalog at an address.
func (c *Client) SearchIMProducts(ctx context.Context, addressID, query string, offset int) ([]IMProduct, error) {
	env, err := decodeResult[imProductsEnvelope](c.CallTool(ctx, "search_products", map[string]any{
		"addressId": addressID, "query": query, "offset": offset,
	}))
	return env.Products, err
}

// IMGoToItems returns the account's "Your Go To Items" (frequent buys) for an
// address. An account with no Instamart history makes the upstream API fail
// ("Failed to fetch Your Go To Items: API returned invalid response") — that is
// mapped to an empty list, not an error, so a fresh account still gets a
// working browse screen.
func (c *Client) IMGoToItems(ctx context.Context, addressID string) ([]IMProduct, error) {
	env, err := decodeResult[imProductsEnvelope](c.CallTool(ctx, "your_go_to_items", map[string]any{
		"addressId": addressID, "offset": 0,
	}))
	if err != nil && strings.Contains(err.Error(), "Your Go To Items") {
		return nil, nil
	}
	return env.Products, err
}

// IMCartItem is the SENT shape for update_cart items (which REPLACES the whole
// cart server-side).
type IMCartItem struct {
	SpinID   string `json:"spinId"`
	Quantity int    `json:"quantity"`
}

// IMCartLine is one typed cart line from get_cart/update_cart.
type IMCartLine struct {
	SpinID    string
	Name      string
	Quantity  int
	Price     int // per-unit rupees
	Available bool
}

// IMCart is the typed Instamart cart. Handling is Instamart's handling fee —
// a bill row Food doesn't have; the TUI folds it into "taxes & charges".
// AddrLat/AddrLng come from selectedAddressDetails — the ONLY place Swiggy
// exposes the delivery coordinates that track_order later requires (harvested
// 2026-07-03: get_addresses and get_orders both omit them), so callers persist
// them at placement time.
type IMCart struct {
	CartID         string
	AddrID         string
	AddrLat        float64
	AddrLng        float64
	ItemTotal      int
	Delivery       int
	Handling       int
	Taxes          int
	Total          int
	Items          []IMCartLine
	PaymentMethods []string
}

// flexID decodes a JSON id that may arrive as a string OR a number (Swiggy
// mixes both across payloads). json.Number alone rejects non-numeric strings.
type flexID string

func (f *flexID) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*f = flexID(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*f = flexID(n.String())
	return nil
}

// val returns the id, normalizing JSON null/zero to "".
func (f flexID) val() string {
	if f == "0" {
		return ""
	}
	return string(f)
}

// imCartEmptyError reports whether an MCP error means "no cart exists" —
// Instamart returns an ERROR for an empty cart rather than an empty payload.
func imCartEmptyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Cart not found") || strings.Contains(msg, "CART_EXPIRED")
}

// imNum decodes a JSON value that may be a number, a currency STRING
// ("₹385"), or an object carrying one of the known price/amount keys.
// Returns 0 when absent/unrecognized.
func imNum(raw json.RawMessage) float64 {
	if len(raw) == 0 {
		return 0
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return float64(rupeesFromLabel(str))
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return 0
	}
	for _, k := range []string{"offerPrice", "finalPrice", "final_price", "amount", "value", "total", "mrp"} {
		if v, ok := obj[k]; ok {
			var f float64
			if err := json.Unmarshal(v, &f); err == nil && f != 0 {
				return f
			}
		}
	}
	return 0
}

// imCartItemRaw decodes one cart item. The primary keys are the REAL
// get_cart/update_cart shape (harvested live 2026-07-03): itemName,
// discountedFinalPrice (per-unit rupees), mrp, isInStockAndAvailable.
// The alternates stay as fallbacks for payload drift.
type imCartItemRaw struct {
	SpinID      flexID          `json:"spinId"`
	SpinIDSnake flexID          `json:"spin_id"`
	ItemID      flexID          `json:"itemId"`
	ItemName    string          `json:"itemName"`
	Name        string          `json:"name"`
	DisplayName string          `json:"displayName"`
	Quantity    int             `json:"quantity"`
	FinalPrice  json.RawMessage `json:"discountedFinalPrice"`
	MRP         json.RawMessage `json:"mrp"`
	Price       json.RawMessage `json:"price"`
	PriceAlt    json.RawMessage `json:"finalPrice"`
	Total       json.RawMessage `json:"total"`
	InStockAvl  *bool           `json:"isInStockAndAvailable"`
	InStock     *bool           `json:"inStock"`
	IsAvailable *bool           `json:"isAvailable"`
}

func (r imCartItemRaw) toLine() IMCartLine {
	id := r.SpinID.val()
	if id == "" {
		if s := r.SpinIDSnake.val(); s != "" {
			id = s
		} else if s := r.ItemID.val(); s != "" {
			id = s
		}
	}
	name := r.ItemName
	if name == "" {
		name = r.Name
	}
	if name == "" {
		name = r.DisplayName
	}
	price := imNum(r.FinalPrice) // the discounted per-unit price the user pays
	if price == 0 {
		price = imNum(r.Price)
	}
	if price == 0 {
		price = imNum(r.PriceAlt)
	}
	if price == 0 && r.Quantity > 0 {
		price = imNum(r.Total) / float64(r.Quantity)
	}
	if price == 0 {
		price = imNum(r.MRP) // strike price — better than showing ₹0
	}
	avail := true
	if r.InStockAvl != nil {
		avail = *r.InStockAvl
	} else if r.InStock != nil {
		avail = *r.InStock
	} else if r.IsAvailable != nil {
		avail = *r.IsAvailable
	}
	return IMCartLine{
		SpinID: id, Name: name, Quantity: r.Quantity,
		Price: int(math.Round(price)), Available: avail,
	}
}

// imBillRaw tolerates the plausible bill-breakdown spellings.
type imBillRaw struct {
	ItemTotal    json.RawMessage `json:"itemTotal"`
	ItemTotalSnk json.RawMessage `json:"item_total"`
	Delivery     json.RawMessage `json:"deliveryFee"`
	DeliveryAlt  json.RawMessage `json:"deliveryCharge"`
	DeliverySnk  json.RawMessage `json:"delivery_charge"`
	Handling     json.RawMessage `json:"handlingFee"`
	HandlingSnk  json.RawMessage `json:"handling_fee"`
	Taxes        json.RawMessage `json:"taxesAndCharges"`
	TaxesSnk     json.RawMessage `json:"taxes_and_charges"`
	GrandTotal   json.RawMessage `json:"grandTotal"`
	Total        json.RawMessage `json:"total"`
	ToPay        json.RawMessage `json:"toPay"`
	ToPaySnk     json.RawMessage `json:"to_pay"`
}

func firstNum(raws ...json.RawMessage) int {
	for _, r := range raws {
		if f := imNum(r); f != 0 {
			return int(math.Round(f))
		}
	}
	return 0
}

// imBillBreakdown is the REAL bill shape (harvested live): label/value string
// pairs like {"Item Total","₹384.00"}, {"Handling Fee","₹1.00"},
// {"Delivery Partner Fee","FREE"}, plus a toPay {"To Pay","₹385"}.
type imBillBreakdown struct {
	LineItems []struct {
		Label string `json:"label"`
		Value string `json:"value"`
	} `json:"lineItems"`
	ToPay struct {
		Value string `json:"value"`
	} `json:"toPay"`
}

// rupeesFromLabel parses a bill value string ("₹384.00", "₹1,299", "FREE",
// "Add a tip") into whole rupees. Non-numeric values (FREE, tip prompts) are 0.
func rupeesFromLabel(s string) int {
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == '.' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return 0
	}
	f, err := strconv.ParseFloat(b.String(), 64)
	if err != nil {
		return 0
	}
	return int(math.Round(f))
}

// imCartData is the cart payload body, whether nested under "data" or flat.
type imCartData struct {
	CartID      flexID           `json:"cartId"`
	CartIDSnk   flexID           `json:"cart_id"`
	Items       []imCartItemRaw  `json:"items"`
	CartItems   []imCartItemRaw  `json:"cartItems"`
	BillBrk     *imBillBreakdown `json:"billBreakdown"`
	CartTotal   string           `json:"cartTotalAmount"` // "₹385" at the root
	Bill        *imBillRaw       `json:"bill"`
	Pricing     *imBillRaw       `json:"pricing"`
	imBillRaw                    // flat bill keys at the root (fallback)
	PayMethods  json.RawMessage  `json:"availablePaymentMethods"`
	AddrDetails *struct {
		ID  flexID  `json:"id"`
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	} `json:"selectedAddressDetails"`
}

type imCartEnvelope struct {
	Data *imCartData `json:"data"`
	imCartData
}

func (e imCartEnvelope) toCart() IMCart {
	d := e.imCartData
	if e.Data != nil {
		d = *e.Data
	}
	c := IMCart{CartID: d.CartID.val()}
	if c.CartID == "" {
		c.CartID = d.CartIDSnk.val()
	}
	if d.AddrDetails != nil {
		c.AddrID = d.AddrDetails.ID.val()
		c.AddrLat = d.AddrDetails.Lat
		c.AddrLng = d.AddrDetails.Lng
	}
	switch {
	case d.BillBrk != nil:
		// The REAL shape: label/value string rows. Labels route by keyword —
		// "Item Total" / "Handling Fee" / "Delivery Partner Fee" (often "FREE") /
		// "Delivery Partner Tip" (a prompt, not a charge; parses to 0). Unknown
		// charge rows fold into Taxes so the to-pay math still lines up.
		for _, li := range d.BillBrk.LineItems {
			v := rupeesFromLabel(li.Value)
			label := strings.ToLower(li.Label)
			switch {
			case strings.Contains(label, "item total"):
				c.ItemTotal = v
			case strings.Contains(label, "handling"):
				c.Handling = v
			case strings.Contains(label, "tip"):
				// prompt row, not a charge
			case strings.Contains(label, "delivery"):
				c.Delivery = v
			default:
				c.Taxes += v
			}
		}
		c.Total = rupeesFromLabel(d.BillBrk.ToPay.Value)
	default:
		bill := d.imBillRaw
		for _, alt := range []*imBillRaw{d.Bill, d.Pricing} {
			if alt != nil {
				bill = *alt
				break
			}
		}
		c.ItemTotal = firstNum(bill.ItemTotal, bill.ItemTotalSnk)
		c.Delivery = firstNum(bill.Delivery, bill.DeliveryAlt, bill.DeliverySnk)
		c.Handling = firstNum(bill.Handling, bill.HandlingSnk)
		c.Taxes = firstNum(bill.Taxes, bill.TaxesSnk)
		c.Total = firstNum(bill.GrandTotal, bill.ToPay, bill.ToPaySnk, bill.Total)
	}
	if c.Total == 0 {
		c.Total = rupeesFromLabel(d.CartTotal) // root "cartTotalAmount":"₹385"
	}
	items := d.Items
	if len(items) == 0 {
		items = d.CartItems
	}
	approx := 0
	for _, it := range items {
		l := it.toLine()
		if l.SpinID == "" && l.Name == "" {
			continue
		}
		c.Items = append(c.Items, l)
		approx += l.Price * l.Quantity
	}
	// Unrecognized bill breakdown: approximate from the lines so downstream
	// gates (₹99 minimum, ₹1000 cap) and the displayed subtotal never act on a
	// bogus ₹0 while items are present.
	if c.ItemTotal == 0 {
		c.ItemTotal = approx
	}
	if c.Total == 0 && len(c.Items) > 0 {
		c.Total = c.ItemTotal + c.Delivery + c.Handling + c.Taxes
	}
	c.PaymentMethods = decodePaymentMethods(d.PayMethods)
	return c
}

// decodePaymentMethods accepts ["Cash", ...] or [{"name":...}/{"method":...}].
func decodePaymentMethods(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var strs []string
	if err := json.Unmarshal(raw, &strs); err == nil {
		return strs
	}
	var objs []map[string]any
	if err := json.Unmarshal(raw, &objs); err != nil {
		return nil
	}
	var out []string
	for _, o := range objs {
		for _, k := range []string{"name", "method", "type", "id", "displayName"} {
			if s, ok := o[k].(string); ok && s != "" {
				out = append(out, s)
				break
			}
		}
	}
	return out
}

// GetIMCart returns the current Instamart cart. An empty cart is (IMCart{},
// nil) — the server signals it with a "Cart not found" error, not a payload.
func (c *Client) GetIMCart(ctx context.Context) (IMCart, error) {
	env, err := decodeResult[imCartEnvelope](c.CallTool(ctx, "get_cart", nil))
	if imCartEmptyError(err) {
		return IMCart{}, nil
	}
	if err != nil {
		return IMCart{}, err
	}
	return env.toCart(), nil
}

// UpdateIMCart REPLACES the whole Instamart cart with items at an address.
func (c *Client) UpdateIMCart(ctx context.Context, addressID string, items []IMCartItem) (IMCart, error) {
	if items == nil {
		items = []IMCartItem{}
	}
	env, err := decodeResult[imCartEnvelope](c.CallTool(ctx, "update_cart", map[string]any{
		"selectedAddressId": addressID, "items": items,
	}))
	if err != nil {
		return IMCart{}, err
	}
	return env.toCart(), nil
}

// ClearIMCart removes every item from the Instamart cart. A "Cart not found"
// (already empty) is success.
func (c *Client) ClearIMCart(ctx context.Context) error {
	_, err := c.CallTool(ctx, "clear_cart", nil)
	if imCartEmptyError(err) {
		return nil
	}
	return err
}

// IMOrder is one Instamart order from get_orders. Status carries the
// human-friendly currentStatus ("Order picked up") when present, falling back
// to the raw state ("CONFIRMED"); Detail is the sub-line (rider updates).
// Lat/Lng stay for compatibility but the REAL payload carries NO coordinates
// (harvested 2026-07-03 — the docs were wrong): tracking coordinates must be
// captured from the cart's selectedAddressDetails at placement time instead.
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

// imOrderRaw matches the REAL get_orders order shape (harvested live
// 2026-07-03): raw state in status, display state in currentStatus, rider
// sub-line in statusMessage, ETA in estimatedDeliveryTime; deliveryAddress
// carries id/addressLine/phone but NO coordinates.
type imOrderRaw struct {
	OrderID       flexID          `json:"orderId"`
	Status        string          `json:"status"`        // "CONFIRMED"
	CurrentStatus string          `json:"currentStatus"` // "Order picked up"
	StatusMessage string          `json:"statusMessage"` // "SANJAY J has picked up your order"
	ETA           string          `json:"estimatedDeliveryTime"`
	Total         json.RawMessage `json:"totalAmount"`
	IsActive      *bool           `json:"isActive"`
	Lat           float64         `json:"lat"` // never seen live; kept for drift
	Lng           float64         `json:"lng"`
	Items         json.RawMessage `json:"items"`
}

func (r imOrderRaw) toOrder() IMOrder {
	o := IMOrder{
		ID:     r.OrderID.val(),
		Status: r.CurrentStatus,
		Detail: r.StatusMessage,
		ETA:    r.ETA,
		Lat:    r.Lat,
		Lng:    r.Lng,
	}
	if o.Status == "" {
		o.Status = r.Status
	}
	o.Active = !imDelivered(r.Status) && !imDelivered(r.CurrentStatus)
	if r.IsActive != nil {
		o.Active = *r.IsActive
	}
	o.Total = firstNum(r.Total)
	o.Items = decodeIMOrderItems(r.Items)
	return o
}

// decodeIMOrderItems accepts ["Milk x2", ...] or [{"name":..,"quantity":..}].
func decodeIMOrderItems(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var strs []string
	if err := json.Unmarshal(raw, &strs); err == nil {
		return strs
	}
	var objs []struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		Quantity    int    `json:"quantity"`
	}
	if err := json.Unmarshal(raw, &objs); err != nil {
		return nil
	}
	var out []string
	for _, o := range objs {
		n := o.Name
		if n == "" {
			n = o.DisplayName
		}
		if n == "" {
			continue
		}
		if o.Quantity > 1 {
			n = n + " ×" + strconv.Itoa(o.Quantity)
		}
		out = append(out, n)
	}
	return out
}

type imOrdersEnvelope struct {
	Orders  []imOrderRaw `json:"orders"`
	HasMore bool         `json:"hasMore"`
}

// GetIMOrders lists Instamart orders (last 15 days). activeOnly filters to
// live ones.
func (c *Client) GetIMOrders(ctx context.Context, count int, activeOnly bool) ([]IMOrder, error) {
	env, err := decodeResult[imOrdersEnvelope](c.CallTool(ctx, "get_orders", map[string]any{
		"count": count, "activeOnly": activeOnly,
	}))
	if err != nil {
		return nil, err
	}
	out := make([]IMOrder, 0, len(env.Orders))
	for _, r := range env.Orders {
		out = append(out, r.toOrder())
	}
	return out, nil
}

// imTrackRaw matches the REAL track_order payload (harvested live 2026-07-03
// during an actual delivery): status is an OBJECT carrying the display
// message, a rider sub-line, and the ETA in both minutes and text.
type imTrackRaw struct {
	OrderID flexID `json:"orderId"`
	Status  struct {
		StatusMessage    string  `json:"statusMessage"`    // "Out for delivery"
		SubStatusMessage string  `json:"subStatusMessage"` // "SANJAY J is on the way to deliver your order"
		EtaMinutes       float64 `json:"etaMinutes"`
		EtaText          string  `json:"etaText"` // "9 mins"
	} `json:"status"`
	PollSeconds float64 `json:"pollingIntervalSeconds"`
}

// TrackIMOrder polls real-time tracking. lat/lng are REQUIRED by the tool and
// come from GetIMOrders (get_addresses omits coordinates for privacy). The
// response may be structured JSON or (like Food's track tools) a text blob —
// both are handled; a text blob becomes the Status verbatim.
func (c *Client) TrackIMOrder(ctx context.Context, orderID string, lat, lng float64) (Tracking, error) {
	raw, err := c.CallTool(ctx, "track_order", map[string]any{
		"orderId": orderID, "lat": lat, "lng": lng,
	})
	if err != nil {
		return Tracking{}, err
	}
	var t imTrackRaw
	if uerr := json.Unmarshal(raw, &t); uerr != nil || t.Status.StatusMessage == "" {
		// Unrecognized payload — treat it as human-readable status text rather
		// than failing the poll (the raw is already debug-logged for harvesting).
		txt := strings.TrimSpace(strings.Trim(string(raw), `"`))
		return Tracking{OrderID: orderID, Status: txt, Active: !imDelivered(txt)}, nil
	}
	return Tracking{
		OrderID: orderID,
		Status:  t.Status.StatusMessage,
		Detail:  t.Status.SubStatusMessage,
		ETA:     t.Status.EtaText,
		Active:  !imDelivered(t.Status.StatusMessage),
	}, nil
}

// imDelivered reports a terminal order status (delivered/cancelled).
func imDelivered(status string) bool {
	s := strings.ToLower(status)
	return strings.Contains(s, "delivered") || strings.Contains(s, "cancelled") ||
		strings.Contains(s, "canceled")
}
