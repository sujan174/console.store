package swiggy

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// PaymentMethod is one selectable payment method from get_payment_options.
// Kind is "qr" (desktop scan-to-pay), "intent" (a mobile UPI app deep link), or
// "cod". PaymentCode is Swiggy's internal code (e.g. "UPI"), when exposed.
type PaymentMethod struct {
	ID          string
	DisplayName string
	Kind        string
	PaymentCode string
}

// PaymentOptions is the live payment picker for the current cart. QR is the
// terminal-friendly scan-to-pay method (nil when the user isn't UPI-eligible);
// Intents are mobile UPI apps; CODAvailable mirrors Swiggy's (optimistic) cod
// flag — note place_food_order may still reject Cash even when this is true.
type PaymentOptions struct {
	QR      *PaymentMethod
	Intents []PaymentMethod
	// COD is the pay-on-delivery method when Swiggy offers it (nil otherwise). Its
	// ID is what to send as place_food_order.paymentMethod — Swiggy renamed the
	// method id to "COD" (the legacy "Cash" token is what our earlier place sent
	// and got "cash temporarily disabled" back), so we send whatever id the live
	// options report.
	COD          *PaymentMethod
	CODAvailable bool
}

type rawPayMethod struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Kind        string `json:"kind"`
	Raw         struct {
		PaymentCode string `json:"payment_code"`
	} `json:"raw"`
}

// paymentOptionsEnvelope decodes get_payment_options. Methods live under
// platforms.<surface>.methods; allMethods is a flat list that additionally
// carries raw.payment_code. Tolerant: unknown platforms/fields are ignored.
type paymentOptionsEnvelope struct {
	Platforms map[string]struct {
		Methods []rawPayMethod `json:"methods"`
	} `json:"platforms"`
	COD struct {
		Available   bool   `json:"available"`
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
	} `json:"cod"`
	AllMethods []rawPayMethod `json:"allMethods"`
}

// PaymentOptions fetches the live payment methods for the current food cart at
// addressID. QR is the first kind=="qr" method; Intents collect kind=="intent".
func (c *Client) PaymentOptions(ctx context.Context, addressID string) (PaymentOptions, error) {
	env, err := decodeResult[paymentOptionsEnvelope](c.CallTool(ctx, "get_payment_options", map[string]any{
		"addressId": addressID,
	}))
	if err != nil {
		return PaymentOptions{}, err
	}

	// id -> payment_code, harvested from the flat allMethods list (the per-
	// platform method objects don't carry it).
	codes := map[string]string{}
	for _, m := range env.AllMethods {
		if m.Raw.PaymentCode != "" {
			codes[m.ID] = m.Raw.PaymentCode
		}
	}

	out := PaymentOptions{CODAvailable: env.COD.Available}
	if env.COD.Available {
		id := env.COD.ID
		if id == "" {
			id = "COD"
		}
		out.COD = &PaymentMethod{ID: id, DisplayName: env.COD.DisplayName, Kind: "cod"}
	}
	seen := map[string]bool{}
	add := func(m rawPayMethod) {
		if m.ID == "" || seen[m.ID] {
			return
		}
		seen[m.ID] = true
		pm := PaymentMethod{ID: m.ID, DisplayName: m.DisplayName, Kind: strings.ToLower(m.Kind), PaymentCode: m.Raw.PaymentCode}
		if pm.PaymentCode == "" {
			pm.PaymentCode = codes[m.ID]
		}
		switch pm.Kind {
		case "qr":
			if out.QR == nil {
				q := pm
				out.QR = &q
			}
		case "intent":
			out.Intents = append(out.Intents, pm)
		}
	}
	// Platform methods first (they carry the reliable kind), then allMethods.
	for _, p := range env.Platforms {
		for _, m := range p.Methods {
			add(m)
		}
	}
	for _, m := range env.AllMethods {
		add(m)
	}
	return out, nil
}

// PendingPayment is a placed-but-unpaid food order awaiting UPI payment. The
// order is PENDING_PAYMENT until confirm_order finalizes it after the user pays.
// UPIString is the intent to render as a QR / open on a phone; the echo fields
// (CartID/Lat/Lng) are required by check_payment_status and confirm_order.
type PendingPayment struct {
	OrderID   string
	PaasID    string
	UPIString string // upiIntentUrl — the raw upi:// intent
	BridgeURL string // hosted mcp.swiggy.com/deeplink-redirect page (browser-friendly)
	CartID    string
	AddressID string // echoed from the place request; required by status/confirm
	Lat, Lng  float64
	Amount    int
	// ExpiresAt is the unix-millis deadline after which the payment window is
	// closed: Swiggy's maxTimeToPollForInMs (5 min) added to placement time. Past
	// it the order is marked FAILED and a late payment is refunded async, so every
	// surface (terminal, /pay page, widget) MUST stop offering the QR/link then.
	ExpiresAt int64
}

// PlaceUPIRequest places an online-payment food order at AddressID using Method
// (typically the "PayWithQR" method from PaymentOptions).
type PlaceUPIRequest struct {
	AddressID string
	Method    PaymentMethod
}

// pendingRaw tolerantly decodes the place_food_order (UPI) response. The exact
// shape was not harvestable at build time (a real place is required), so it
// accepts camelCase AND snake_case keys, several UPI-string spellings, and
// numbers-as-strings — always failing soft rather than placing a phantom order.
type pendingRaw struct {
	OrderID  string `json:"orderId"`
	OrderID2 string `json:"order_id"`
	PaasID   string `json:"paasId"`
	PaasID2  string `json:"paas_id"`
	// cartId arrives as a NUMBER in the live shape — flexID tolerates number|string.
	CartID  flexID `json:"cartId"`
	CartID2 flexID `json:"cart_id"`
	// The live UPI string field is `upiIntentUrl`; keep the other spellings as
	// tolerant fallbacks in case a future/alternate flow differs.
	UPIIntentURL string    `json:"upiIntentUrl"`
	UPIIntent    string    `json:"upiIntent"`
	UPIString    string    `json:"upiString"`
	QRString     string    `json:"qrString"`
	QR           string    `json:"qr"`
	IntentURL    string    `json:"intentUrl"`
	BridgeURL    string    `json:"bridgeUrl"`
	Lat          flexFloat `json:"lat"`
	Lng          flexFloat `json:"lng"`
	// Live amount is `paidAmount`; amount/to_pay are tolerant fallbacks.
	PaidAmount flexFloat `json:"paidAmount"`
	Amount     flexFloat `json:"amount"`
	ToPay      flexFloat `json:"to_pay"`
	// maxTimeToPollForInMs is the payment window Swiggy allows (5 min). 0 → default.
	MaxPollMs flexFloat `json:"maxTimeToPollForInMs"`
}

// defaultPayWindowMs is the fallback payment window when the response omits
// maxTimeToPollForInMs — Swiggy's observed value is 5 minutes.
const defaultPayWindowMs = 300000

func (r pendingRaw) pending() PendingPayment {
	amt := float64(r.PaidAmount)
	if amt == 0 {
		amt = float64(r.Amount)
	}
	if amt == 0 {
		amt = float64(r.ToPay)
	}
	return PendingPayment{
		OrderID:   firstNonEmpty(r.OrderID, r.OrderID2),
		PaasID:    firstNonEmpty(r.PaasID, r.PaasID2),
		UPIString: firstNonEmpty(r.UPIIntentURL, r.UPIIntent, r.UPIString, r.QRString, r.QR, r.IntentURL),
		BridgeURL: r.BridgeURL,
		CartID:    firstNonEmpty(r.CartID.val(), r.CartID2.val()),
		Lat:       float64(r.Lat),
		Lng:       float64(r.Lng),
		Amount:    int(math.Round(amt)),
	}
}

// PlaceFoodOrderUPI places a non-idempotent online-payment food order. It gates
// on CONSOLE_LIVE_ORDERS. It does NOT use placeWithVerify (that path is
// Order-typed; a UPI place returns a pending-payment, and an unpaid pending
// order simply expires — so the CallTool no-retry policy for place_food_order is
// the sufficient duplicate guard). Rejects a phantom response (no order/paas id).
func (c *Client) PlaceFoodOrderUPI(ctx context.Context, req PlaceUPIRequest) (PendingPayment, error) {
	if !liveOrdersEnabled() {
		return PendingPayment{}, ErrOrdersDisabled
	}
	// Swiggy's contract (place_food_order / get_payment_options docs): paymentMethod
	// is the payment CODE ("UPI"), NOT the method id ("PayWithQR" is rejected as an
	// "Unsupported payment method"). Then a desktop QR method uses generateUPIQR;
	// a mobile app method echoes its id byte-for-byte into intentApp.
	code := req.Method.PaymentCode
	if code == "" {
		code = "UPI"
	}
	args := map[string]any{
		"addressId":     req.AddressID,
		"paymentMethod": code,
	}
	if req.Method.Kind == "intent" && req.Method.ID != "" {
		args["intentApp"] = req.Method.ID
	} else {
		args["generateUPIQR"] = true // desktop scan-to-pay QR
	}
	raw, err := c.CallTool(ctx, "place_food_order", args)
	if err != nil {
		return PendingPayment{}, err
	}
	pr, err := decodeResult[pendingRaw](raw, nil)
	if err != nil {
		return PendingPayment{}, err
	}
	p := pr.pending()
	p.AddressID = req.AddressID
	window := int64(pr.MaxPollMs)
	if window <= 0 {
		window = defaultPayWindowMs
	}
	p.ExpiresAt = time.Now().UnixMilli() + window
	if p.OrderID == "" && p.PaasID == "" {
		return PendingPayment{}, fmt.Errorf("swiggy: place returned no order id or paas id (payment not started)")
	}
	return p, nil
}

// PaymentStatus is the coarse state of an in-flight UPI payment.
type PaymentStatus int

const (
	PayPending PaymentStatus = iota // still waiting for the user to pay
	PaySuccess                      // paid — ready to confirm_order
	PayFailed                       // failed / cancelled / timed out
)

type statusRaw struct {
	Status        string `json:"status"`
	PaymentStatus string `json:"paymentStatus"`
	State         string `json:"state"`
}

func (s statusRaw) status() PaymentStatus {
	v := strings.ToUpper(firstNonEmpty(s.Status, s.PaymentStatus, s.State))
	switch {
	case strings.Contains(v, "SUCCESS"), strings.Contains(v, "PAID"), v == "COMPLETED", v == "PLACED":
		return PaySuccess
	case strings.Contains(v, "FAIL"), strings.Contains(v, "TIMEOUT"), strings.Contains(v, "CANCEL"), strings.Contains(v, "ERROR"):
		return PayFailed
	default:
		return PayPending
	}
}

// CheckPaymentStatus reads the current state of the pending UPI payment. It is
// read-only and idempotent (safe to poll), so it is NOT arming-gated. An error
// (transient) is surfaced as PayPending so the caller keeps polling.
func (c *Client) CheckPaymentStatus(ctx context.Context, p PendingPayment) (PaymentStatus, error) {
	raw, err := c.CallTool(ctx, "check_payment_status", map[string]any{
		"paasId": p.PaasID, "orderId": p.OrderID, "addressId": p.AddressID,
		"cartId": p.CartID, "lat": p.Lat, "lng": p.Lng,
	})
	if err != nil {
		return PayPending, err
	}
	st, derr := decodeResult[statusRaw](raw, nil)
	if derr != nil {
		return PayPending, derr
	}
	return st.status(), nil
}

// ConfirmOrder finalizes a paid pending order to PLACED. Gated by
// CONSOLE_LIVE_ORDERS (it completes a real, paid order). Fired once on the first
// payment SUCCESS, and once on timeout to mark a pending order failed.
func (c *Client) ConfirmOrder(ctx context.Context, p PendingPayment) (Order, error) {
	if !liveOrdersEnabled() {
		return Order{}, ErrOrdersDisabled
	}
	raw, err := c.CallTool(ctx, "confirm_order", map[string]any{
		"orderId": p.OrderID, "addressId": p.AddressID,
		"cartId": p.CartID, "lat": p.Lat, "lng": p.Lng,
	})
	if err != nil {
		return Order{}, err
	}
	return decodeResult[Order](raw, nil)
}

// flexFloat decodes a JSON number OR a numeric string (Swiggy mixes both). A
// non-numeric/empty value decodes to 0 rather than erroring — tolerant by design.
type flexFloat float64

func (f *flexFloat) UnmarshalJSON(b []byte) error {
	var n json.Number
	if err := json.Unmarshal(b, &n); err == nil {
		v, _ := n.Float64()
		*f = flexFloat(v)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return nil // null/object/etc → leave 0
	}
	if s == "" {
		return nil
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		*f = flexFloat(v)
	}
	return nil
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
