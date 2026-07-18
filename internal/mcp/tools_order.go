package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"consolestore/internal/broker/api"
	"consolestore/internal/localstore"
)

func nowUnix() int64 { return time.Now().Unix() }

// orderCapRupees is Swiggy's Builders Club limit: place_food_order is refused
// at ≥₹1000 (real COD). Enforced here so the failure lands at prepare time,
// with a clear message, instead of at the moment of placement.
const orderCapRupees = 1000

// cartRebuildWindowSeconds bounds how old a cached cart write may be and still
// seed an automatic rebuild (address switch / Swiggy-side expiry). Conversation
// scale: old enough to span a long ordering chat, young enough that yesterday's
// cart never resurrects.
const cartRebuildWindowSeconds = 2 * 60 * 60

// prepare syncs the cart, validates availability, stores a confirmation bound to
// the bill, and returns both. Shared by prepare_order and order_preset.
func (s *Server) prepare(addressID string, c api.Cart, ident orderIdentity) (string, CartDTO, error) {
	if len(c.Lines) == 0 {
		return "", CartDTO{}, errors.New("cart is empty — add items before preparing an order")
	}
	for _, l := range c.Lines {
		if !l.Available {
			return "", CartDTO{}, fmt.Errorf("%q is sold out — remove it before ordering", l.Name)
		}
	}
	if c.Total >= orderCapRupees {
		return "", CartDTO{}, codedErr(codeOverCap, "the bill is ₹%d — Swiggy refuses agent-placed orders of ₹%d or more; ask the user what to remove to get under the cap", c.Total, orderCapRupees)
	}
	id := s.pending.put(addressID, c, ident, nowUnix())
	return id, cartToDTO(c), nil
}

// rebuildCart re-syncs the cached last cart write at addressID. Returns the
// fresh cart and re-records the cache under the (possibly new) address, which
// also keeps taste observation working after an address switch.
func (s *Server) rebuildCart(addressID string, cw *cartWrite) (api.Cart, error) {
	c, err := s.be.UpdateCart(addressID, cw.RestaurantID, cw.RestaurantName, cartWriteItems(cw))
	if err != nil {
		return api.Cart{}, err
	}
	cp := *cw
	cp.AddressID = addressID
	cp.WrittenAt = nowUnix()
	s.recordCartWrite(&cp)
	return c, nil
}

type PrepareOrderIn struct {
	AddressID string `json:"address_id"`
}
type PrepareOrderOut struct {
	ConfirmationID string     `json:"confirmation_id"`
	Bill           CartDTO    `json:"bill"`
	Address        AddrRefDTO `json:"address"` // where this order delivers — show it with the bill
	// Rebuilt is set when the server re-synced the cart before preparing:
	// "address_change" (cart moved to this address) or "expired" (Swiggy had
	// dropped the cart). Mention it to the user in one line with the bill.
	Rebuilt string `json:"rebuilt,omitempty"`
	Note    string `json:"note"`
}

func (s *Server) handlePrepareOrder(ctx context.Context, _ *mcp.CallToolRequest, in PrepareOrderIn) (*mcp.CallToolResult, PrepareOrderOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, PrepareOrderOut{}, err
	}
	c, err := s.be.GetCart(in.AddressID, "")
	if err != nil {
		return nil, PrepareOrderOut{}, err
	}
	// The identity defaults to ad-hoc: the live cart carries only a restaurant
	// name, no Swiggy id, so restaurantID stays empty (bumpFavorite skips).
	ident := orderIdentity{restaurantName: c.Restaurant}
	rebuilt := ""
	if cw, ok := s.lastCartWrite(); ok && !cw.Placed && len(cw.Lines) > 0 &&
		nowUnix()-cw.WrittenAt <= cartRebuildWindowSeconds {
		switch {
		case cw.AddressID != in.AddressID:
			// Address switch: re-sync the same lines at the new address so
			// serviceability and the bill are recomputed for where the food
			// actually goes. Same outlet only — never silently switch outlets.
			c, err = s.rebuildCart(in.AddressID, cw)
			if err != nil {
				return nil, PrepareOrderOut{}, codedErr(codeUnserviceable,
					"%s can't deliver to this address (%v) — offer search_restaurants near the new address for the same brand, or keep the original address",
					cartName(cw.RestaurantName), err)
			}
			rebuilt = "address_change"
		case len(c.Lines) == 0:
			// Same address but the cart vanished — Swiggy expired it server-side.
			c, err = s.rebuildCart(in.AddressID, cw)
			if err != nil {
				return nil, PrepareOrderOut{}, codedErr(codeCartExpired,
					"the cart expired on Swiggy and rebuilding it failed (%v) — re-add the items with update_cart", err)
			}
			rebuilt = "expired"
		}
		if rebuilt != "" {
			// The rebuild came from our own cache, so the real Swiggy identity
			// is known — record it (favorites/taste work like a preset order).
			ident = orderIdentity{restaurantID: cw.RestaurantID, restaurantName: cw.RestaurantName}
		}
	}
	id, bill, err := s.prepare(in.AddressID, c, ident)
	if err != nil {
		return nil, PrepareOrderOut{}, err
	}
	card, _ := localstore.LoadCard()
	return nil, PrepareOrderOut{
		ConfirmationID: id, Bill: bill,
		Address: AddrRefDTO{ID: in.AddressID, Label: addrLabelFor(card, in.AddressID)},
		Rebuilt: rebuilt,
		Note:    "show the user the full bill breakdown AND the delivery address; call place_order with this confirmation_id ONLY after they confirm.",
	}, nil
}

type PlaceOrderIn struct {
	ConfirmationID string `json:"confirmation_id"`
	Method         string `json:"method,omitempty" jsonschema:"payment method preference: \"upi\" (scan-to-pay) or \"cod\" (cash on delivery); omit for the default (UPI when available, else COD)"`
}
type PlaceOrderOut struct {
	// Exactly one of Order / Payment is set. Order = a placed order (instamart, or
	// a legacy no-UPI food user). Payment = a UPI scan-to-pay handoff: show the QR
	// and/or PayURL, poll check_payment, then confirm_order once paid.
	Order   *OrderDTO   `json:"order,omitempty"`
	Payment *PaymentDTO `json:"payment,omitempty"`
}

// PaymentDTO is a UPI scan-to-pay handoff for a PENDING_PAYMENT food order. The
// client renders QRSVG inline (self-contained SVG) or offers PayURL (the hosted
// /pay page, which also renders the QR) as a fallback, shows the ExpiresAt
// countdown, then polls check_payment and calls confirm_order when paid.
type PaymentDTO struct {
	PaymentID string `json:"payment_id"`
	Amount    int    `json:"amount"`
	UPIString string `json:"upi_string"` // the raw upi:// intent (deep-links a UPI app)
	QRSVG     string `json:"qr_svg"`     // inline SVG of upi_string, ready to embed
	PayURL    string `json:"pay_url"`    // hosted /pay page (QR + open-in-app), a fallback
	ExpiresAt int64  `json:"expires_at"` // unix millis; the payment window closes here
	Note      string `json:"note"`
}

// payURLBase is where the hosted scan-to-pay page lives (CONSOLE_PAY_URL overrides
// for local testing). Mirrors the TUI's payBaseURL so both surfaces point at the
// same page.
func payURLBase() string {
	if v := strings.TrimSpace(os.Getenv("CONSOLE_PAY_URL")); v != "" {
		return v
	}
	return "https://consolestore.in/pay"
}

func toPaymentDTO(id string, p api.PendingPayment) *PaymentDTO {
	payURL := ""
	if p.UPIString != "" {
		payURL = fmt.Sprintf("%s?upi=%s&exp=%d", payURLBase(), url.QueryEscape(p.UPIString), p.ExpiresAt)
	}
	return &PaymentDTO{
		PaymentID: id, Amount: p.Amount, UPIString: p.UPIString,
		QRSVG: qrSVG(p.UPIString), PayURL: payURL, ExpiresAt: p.ExpiresAt,
		Note: "show the user the QR to scan (or the pay link) and the amount; poll check_payment every couple of seconds, then call confirm_order once it reports paid. If the window closes, tell them nothing was charged and to place again.",
	}
}

type CheckPaymentIn struct {
	PaymentID string `json:"payment_id"`
}
type CheckPaymentOut struct {
	Status  string `json:"status"` // "pending" | "paid" | "failed" | "expired"
	Paid    bool   `json:"paid"`
	Failed  bool   `json:"failed"`
	Expired bool   `json:"expired"`
	Note    string `json:"note"`
}

func (s *Server) handleCheckPayment(ctx context.Context, _ *mcp.CallToolRequest, in CheckPaymentIn) (*mcp.CallToolResult, CheckPaymentOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, CheckPaymentOut{}, err
	}
	e, ok := s.payments.get(in.PaymentID)
	if !ok {
		return nil, CheckPaymentOut{}, codedErr(codeConfirmationExpired, "unknown or expired payment_id — place the order again")
	}
	// The payment window closed: paying now would only be refunded, so report it
	// as expired without polling and drop the entry (nothing can confirm it).
	if paymentExpired(e.pay) {
		s.payments.remove(in.PaymentID)
		return nil, CheckPaymentOut{Status: "expired", Expired: true,
			Note: "the payment window closed — nothing was charged. place the order again for a fresh QR."}, nil
	}
	// Route the poll to the vertical that minted the pending: an Instamart
	// pending polls the IM client, food the food client.
	poll := s.be.PollPayment
	if e.ord.vertical == "instamart" {
		poll = s.be.IMPollPayment
	}
	st, err := poll(e.pay)
	if err != nil {
		return nil, CheckPaymentOut{}, err
	}
	switch st {
	case api.PaySuccess:
		return nil, CheckPaymentOut{Status: "paid", Paid: true, Note: "payment received — call confirm_order to finalize the order."}, nil
	case api.PayFailed:
		s.payments.remove(in.PaymentID)
		return nil, CheckPaymentOut{Status: "failed", Failed: true, Note: "the payment failed — nothing was charged. place the order again."}, nil
	default:
		return nil, CheckPaymentOut{Status: "pending", Note: "still waiting for the payment — keep polling until paid or expired."}, nil
	}
}

func (s *Server) handleConfirmOrder(ctx context.Context, _ *mcp.CallToolRequest, in CheckPaymentIn) (*mcp.CallToolResult, PlaceOrderOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, PlaceOrderOut{}, err
	}
	// Atomically claim the payment for this confirm so two concurrent
	// confirm_order calls can't both fire the non-idempotent ConfirmOrder.
	e, ok := s.payments.claimConfirm(in.PaymentID)
	if !ok {
		return nil, PlaceOrderOut{}, codedErr(codeConfirmationExpired, "unknown, expired, or already-confirming payment_id — place the order again")
	}
	if paymentExpired(e.pay) {
		s.payments.remove(in.PaymentID)
		return nil, PlaceOrderOut{}, codedErr(codeConfirmationExpired, "the payment window closed — nothing was charged. place the order again")
	}
	if e.ord.vertical == "instamart" {
		// Instamart UPI: finalize via the IM client. The broker's IMConfirmOrder
		// already force-clears the server cart in-service, so we don't clear here.
		order, err := s.be.IMConfirmOrder(e.pay) // never retried
		if err != nil {
			s.payments.releaseConfirm(in.PaymentID) // failed — allow a manual retry
			return nil, PlaceOrderOut{}, fmt.Errorf("confirm failed: %w — run list_active_orders before retrying in case it was placed", err)
		}
		if order.Restaurant == "" {
			order.Restaurant = "Instamart"
		}
		s.payments.remove(in.PaymentID)
		// Coords ride on the pending (stamped at place time from the cart) —
		// track_order needs them and the cart is gone by now.
		s.recordIMPlacement(e.ord, order, e.pay.Lat, e.pay.Lng)
		return nil, PlaceOrderOut{Order: toOrderDTOPtr(order)}, nil
	}
	order, err := s.be.ConfirmOrder(e.pay) // never retried
	if err != nil {
		s.payments.releaseConfirm(in.PaymentID) // failed — allow a manual retry
		return nil, PlaceOrderOut{}, fmt.Errorf("confirm failed: %w — run list_active_orders before retrying in case it was placed", err)
	}
	s.payments.remove(in.PaymentID)
	s.recordFoodPlacement(e.ord, order)
	return nil, PlaceOrderOut{Order: toOrderDTOPtr(order)}, nil
}

// paymentExpired reports whether a pending payment is past its window. ExpiresAt
// unset (0) means Swiggy gave no deadline — treat as never-expired here and let the
// live poll decide (a missing deadline shouldn't hard-fail an in-progress payment).
func paymentExpired(p api.PendingPayment) bool {
	return p.ExpiresAt > 0 && nowUnixMilli() >= p.ExpiresAt
}

func nowUnixMilli() int64 { return time.Now().UnixMilli() }

func (s *Server) handlePlaceOrder(ctx context.Context, _ *mcp.CallToolRequest, in PlaceOrderIn) (*mcp.CallToolResult, PlaceOrderOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, PlaceOrderOut{}, err
	}
	// Validate the tender enum BEFORE consuming the confirmation: only "",
	// "upi", "cod" are meaningful. An unknown value ("card", a typo) must be
	// REJECTED, not silently treated as the UPI default (the agent would
	// believe it selected a method we ignored) — and rejecting it here, before
	// pending.take, means a pure input error doesn't burn the confirmation and
	// force a redundant prepare_order.
	method := strings.ToLower(strings.TrimSpace(in.Method))
	switch method {
	case "", "upi", "cod":
	default:
		return nil, PlaceOrderOut{}, codedErr(codeValidation, "unknown payment method — use \"upi\", \"cod\", or leave it empty for the default")
	}
	p, ok := s.pending.take(in.ConfirmationID, nowUnix())
	if !ok {
		return nil, PlaceOrderOut{}, codedErr(codeConfirmationExpired, "unknown or expired confirmation_id — call prepare_order again")
	}
	if p.vertical == "instamart" {
		return s.placeIMOrder(p, method)
	}
	// Food is UPI-only (Swiggy disabled COD for the food vertical), so an explicit
	// cod request can't be honored — say so instead of silently placing UPI.
	if method == "cod" {
		return nil, PlaceOrderOut{}, codedErr(codeValidation, "food orders are UPI-only right now")
	}
	// Re-fetch and verify the cart still matches what the user confirmed.
	c, err := s.be.GetCart(p.addressID, "")
	if err != nil {
		return nil, PlaceOrderOut{}, err
	}
	if cartHash(p.addressID, c) != p.hash || c.Total != p.total {
		return nil, PlaceOrderOut{}, codedErr(codeCartChanged, "cart changed since prepare_order — call prepare_order again to re-confirm")
	}
	// Swiggy disabled COD, so the default path is an online UPI payment: place a
	// PENDING_PAYMENT order, hand the client a scan-to-pay QR + link, and let it
	// poll (check_payment) and finalize (confirm_order). The bookkeeping runs at
	// confirm — the order isn't real until the money clears. A legacy no-UPI user
	// (ok == false) still gets the immediate COD path.
	pend, ok, err := s.be.PlaceUPI(p.addressID) // never retried
	if err != nil {
		return nil, PlaceOrderOut{}, fmt.Errorf("order failed: %w — run list_active_orders before retrying in case it was placed", err)
	}
	if ok {
		payID := s.payments.put(pend, p)
		return nil, PlaceOrderOut{Payment: toPaymentDTO(payID, pend)}, nil
	}
	order, err := s.be.PlaceCOD(p.addressID) // never retried
	if err != nil {
		return nil, PlaceOrderOut{}, fmt.Errorf("order failed: %w — run list_active_orders before retrying in case it was placed", err)
	}
	s.recordFoodPlacement(p, order)
	return nil, PlaceOrderOut{Order: toOrderDTOPtr(order)}, nil
}

// recordFoodPlacement accretes the caches after a food order is truly placed —
// immediately for COD, and at confirm_order time for UPI (the order isn't real
// until the money clears). Persists the active order for `console status`/tracking,
// records the placement identity/address, auto-saves the placed lines for the app's
// previous-orders list, keeps addrpref current, and observes taste. Best-effort:
// none of it is allowed to fail the placement.
func (s *Server) recordFoodPlacement(p pendingOrder, order api.Order) {
	etaLo, etaHi := localstore.ParseETAMinutes(order.ETA)
	_ = localstore.SaveActiveOrder(localstore.ActiveOrder{
		OrderID: order.ID, Restaurant: order.Restaurant, ETALoMin: etaLo, ETAHiMin: etaHi,
		Total: order.Total, PlacedAt: nowUnix(),
	})
	// Record real identity: restaurantID is a Swiggy id for presets, "" for ad-hoc
	// orders (then bumpFavorite skips). Never a name in the id slot.
	_ = localstore.RecordOrder(p.addressID, p.addrLabel, p.restaurantID, p.restaurantName, nowUnix())
	// Auto-save the placed order for the app's previous-orders list. Best-effort
	// and only possible when we have a cached cart write to source lines from
	// (a pre-existing Swiggy cart never routed through update_cart has none).
	if cw, ok := s.cartWriteFor(p.addressID); ok {
		po := localstore.PlacedOrder{
			RestaurantID: cw.RestaurantID, RestaurantName: cw.RestaurantName,
			Total: p.total, PlacedUnix: nowUnix(),
		}
		for _, ln := range cw.Lines {
			pl := localstore.PlacedLine{ItemID: ln.ItemID, Name: ln.ItemName, Qty: ln.Qty}
			for _, sel := range ln.Sels {
				pl.Sels = append(pl.Sels, localstore.PresetSel{
					GroupID: sel.GroupID, ChoiceID: sel.ChoiceID,
					Variant: sel.Variant, Absolute: sel.Absolute, Name: sel.ChoiceName,
				})
			}
			po.Lines = append(po.Lines, pl)
		}
		_ = localstore.AppendOrder(p.addressID, po)
	}
	// Record the placement address in the app's addrpref (default/last/lock)
	// model. Unconditional (not gated on a cart write existing) — every
	// successful placement, food or ad-hoc, should keep addrpref current.
	if ap, err := localstore.LoadAddrPref(); err == nil {
		_ = localstore.SaveAddrPref(ap.RecordPlacement(p.addressID, p.addrLabel, nowUnix()))
	}
	// Best-effort taste observation from the cart that was actually placed.
	// Never blocks the order and is never allowed to duplicate it.
	if cw, ok := s.cartWriteFor(p.addressID); ok && cw.RestaurantID != "" {
		t, terr := localstore.LoadTaste()
		if terr == nil {
			changed := false
			for _, ln := range cw.Lines {
				picks := namedPicks(ln.Sels)
				if len(picks) == 0 {
					continue
				}
				t.Observe(cw.RestaurantID, cw.RestaurantName, ln.ItemName, ln.ItemID, picks, nowUnix())
				changed = true
			}
			if changed {
				_ = localstore.SaveTaste(t)
			}
		}
	}
	// The cart write was consumed by this order: keep it (save_preset and taste
	// still read it) but never let it seed a rebuild of a fresh cart.
	s.markCartWritePlaced()
}

// placeIMOrder re-verifies and places an Instamart order from a confirmation
// minted by im_prepare_order/order_preset. Mirrors handlePlaceOrder's food
// path: re-fetch, hash/total re-check, then place once (never retried). The
// method preference picks the tender: "cod" places immediately; "upi" or the
// default tries a UPI scan-to-pay handoff (IMPlaceOrderUPI) and, when the
// account has no scan-to-pay method, falls back to COD — unless "upi" was
// explicit, in which case it's a validation error rather than a silent COD.
func (s *Server) placeIMOrder(p pendingOrder, method string) (*mcp.CallToolResult, PlaceOrderOut, error) {
	c, err := s.be.IMGetCart()
	if err != nil {
		return nil, PlaceOrderOut{}, err
	}
	if imCartHash(p.addressID, c) != p.hash || c.Total != p.total {
		return nil, PlaceOrderOut{}, codedErr(codeCartChanged, "cart changed since im_prepare_order — call im_prepare_order again to re-confirm")
	}
	if method == "cod" {
		return s.placeIMCOD(p, c)
	}
	// UPI (explicit "upi" or the default): place a PENDING_PAYMENT order and hand
	// the client a scan-to-pay QR + link, to be polled (check_payment) and
	// finalized (confirm_order) — the bookkeeping runs at confirm, once the money
	// clears (mirrors the food UPI path).
	pend, ok, err := s.be.IMPlaceOrderUPI(p.addressID) // never retried
	if err != nil {
		return nil, PlaceOrderOut{}, fmt.Errorf("order failed: %w — run list_active_orders before retrying in case it was placed", err)
	}
	if ok {
		// The re-fetched cart is the ONLY source of the delivery coordinates
		// track_order requires (get_orders omits them); carry them on the pending
		// so confirm_order can stamp the ActiveOrder even after the cart is gone.
		if pend.Lat == 0 && pend.Lng == 0 {
			pend.Lat, pend.Lng = c.AddrLat, c.AddrLng
		}
		payID := s.payments.put(pend, p)
		return nil, PlaceOrderOut{Payment: toPaymentDTO(payID, pend)}, nil
	}
	if method == "upi" {
		return nil, PlaceOrderOut{}, codedErr(codeValidation, "UPI is not available for this account right now — offer cash on delivery instead")
	}
	// Default with no scan-to-pay method available → COD.
	return s.placeIMCOD(p, c)
}

// placeIMCOD places the Instamart cart immediately (COD) and records the
// placement. c is the just-re-verified cart (its AddrLat/AddrLng are the only
// source of the delivery coordinates track_order needs).
func (s *Server) placeIMCOD(p pendingOrder, c api.IMCart) (*mcp.CallToolResult, PlaceOrderOut, error) {
	order, err := s.be.IMPlaceOrder(p.addressID) // never retried
	if err != nil {
		return nil, PlaceOrderOut{}, fmt.Errorf("order failed: %w — run list_active_orders before retrying in case it was placed", err)
	}
	s.recordIMPlacement(p, order, c.AddrLat, c.AddrLng)
	// Force-clear the server cart after placement: checkout normally consumes
	// it, but leftovers have been seen live lingering in the Swiggy app cart.
	// Best-effort — clear_cart maps "Cart not found" (already empty) to success.
	_ = s.be.IMClearCart()
	return nil, PlaceOrderOut{Order: toOrderDTOPtr(order)}, nil
}

// recordIMPlacement accretes the localstore caches after an Instamart order is
// truly placed — immediately for COD, and at confirm_order time for UPI. Mirrors
// recordFoodPlacement's tail for the IM vertical: ActiveOrder with Vertical
// "instamart" plus the delivery coords track_order requires (get_addresses and
// get_orders both omit them, harvested 2026-07-03), the placement identity, and
// addrpref. Best-effort. It deliberately does NOT clear the IM cart: the COD path
// clears it itself, and the broker's IMConfirmOrder force-clears it in-service.
func (s *Server) recordIMPlacement(p pendingOrder, order api.Order, lat, lng float64) {
	etaLo, etaHi := localstore.ParseETAMinutes(order.ETA)
	_ = localstore.SaveActiveOrder(localstore.ActiveOrder{
		OrderID: order.ID, Restaurant: order.Restaurant, ETALoMin: etaLo, ETAHiMin: etaHi,
		Total: order.Total, PlacedAt: nowUnix(), Vertical: "instamart",
		Lat: lat, Lng: lng,
	})
	_ = localstore.RecordOrder(p.addressID, p.addrLabel, p.restaurantID, p.restaurantName, nowUnix())
	// Best-effort, mirrors the food path: keep addrpref current for Instamart
	// placements too (previously only food touched it).
	if ap, err := localstore.LoadAddrPref(); err == nil {
		_ = localstore.SaveAddrPref(ap.RecordPlacement(p.addressID, p.addrLabel, nowUnix()))
	}
	s.markCartWritePlaced()
}

type OrderPresetIn struct {
	Name  string `json:"name"`
	Index int    `json:"index,omitempty" jsonschema:"0-based pick among presets sharing a name; default 0"`
}
type OrderPresetOut struct {
	ConfirmationID string     `json:"confirmation_id"`
	Vertical       string     `json:"vertical"` // "food" or "instamart"
	Bill           CartDTO    `json:"bill,omitempty"`
	IMBill         IMCartDTO  `json:"im_bill,omitempty"` // set instead of bill when vertical is "instamart"
	Address        AddrRefDTO `json:"address"`           // where this order delivers — show it with the bill
	Note           string     `json:"note"`
}

func (s *Server) handleOrderPreset(ctx context.Context, _ *mcp.CallToolRequest, in OrderPresetIn) (*mcp.CallToolResult, OrderPresetOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, OrderPresetOut{}, err
	}
	ps, err := localstore.LoadPresets()
	if err != nil {
		return nil, OrderPresetOut{}, err
	}
	matches := ps.ByName(in.Name)
	if len(matches) == 0 {
		return nil, OrderPresetOut{}, fmt.Errorf("no preset named %q", in.Name)
	}
	if in.Index < 0 || in.Index >= len(matches) {
		return nil, OrderPresetOut{}, fmt.Errorf("preset %q has %d entries; index %d out of range", in.Name, len(matches), in.Index)
	}
	p := matches[in.Index]
	if p.IsInstamart() {
		return s.orderIMPreset(p)
	}
	c, err := s.be.UpdateCart(p.AddrID, p.RestaurantID, p.RestaurantName, localstore.PresetCartItems(p))
	if err != nil {
		return nil, OrderPresetOut{}, err
	}
	s.recordCartWrite(cartWriteFromPreset(s, p))
	// Preset carries the real Swiggy restaurant id + saved address label.
	id, bill, err := s.prepare(p.AddrID, c, orderIdentity{
		restaurantID: p.RestaurantID, restaurantName: p.RestaurantName, addrLabel: p.AddrLine,
	})
	if err != nil {
		return nil, OrderPresetOut{}, err
	}
	return nil, OrderPresetOut{ConfirmationID: id, Vertical: "food", Bill: bill,
		Address: AddrRefDTO{ID: p.AddrID, Label: p.AddrLine},
		Note:    "show the user the full bill breakdown AND the delivery address; call place_order with this confirmation_id ONLY after they confirm."}, nil
}

// orderIMPreset routes an Instamart preset through IMUpdateCart + the im
// prepare path (same refusals as im_prepare_order: empty/sold-out/cap/min).
func (s *Server) orderIMPreset(p localstore.Preset) (*mcp.CallToolResult, OrderPresetOut, error) {
	c, err := s.be.IMUpdateCart(p.AddrID, localstore.PresetIMCartItems(p))
	if err != nil {
		return nil, OrderPresetOut{}, err
	}
	// Record the write like the food path does: saveIMPreset recovers the
	// address binding from the last "Instamart" cart-write, so a follow-up
	// save_preset {vertical:"instamart"} must not save an address-less preset.
	s.recordCartWrite(&cartWrite{AddressID: p.AddrID, RestaurantName: "Instamart"})
	id, bill, err := s.imPrepare(p.AddrID, c, orderIdentity{restaurantName: "Instamart", addrLabel: p.AddrLine})
	if err != nil {
		return nil, OrderPresetOut{}, err
	}
	return nil, OrderPresetOut{ConfirmationID: id, Vertical: "instamart", IMBill: bill,
		Address: AddrRefDTO{ID: p.AddrID, Label: p.AddrLine},
		Note:    "COD only — show the user the full bill breakdown AND the delivery address; call place_order with this confirmation_id ONLY after they confirm."}, nil
}

// cartWriteFromPreset projects a preset into a cartWrite for the memory
// caches. Selection names come from the preset's own saved names; GroupName is
// left for the option-name cache to fill in via nameSel (it may still be
// empty, e.g. if get_item_options was never called this session).
func cartWriteFromPreset(s *Server, p localstore.Preset) *cartWrite {
	cw := &cartWrite{AddressID: p.AddrID, RestaurantID: p.RestaurantID, RestaurantName: p.RestaurantName}
	for _, pl := range p.Lines {
		ln := memLine{ItemID: pl.ItemID, ItemName: pl.Name, Qty: pl.Qty}
		for _, sel := range pl.Sels {
			ms := memSel{GroupID: sel.GroupID, ChoiceID: sel.ChoiceID, Variant: sel.Variant, Absolute: sel.Absolute, ChoiceName: sel.Name}
			s.nameSel(&ms)
			ln.Sels = append(ln.Sels, ms)
		}
		cw.Lines = append(cw.Lines, ln)
	}
	return cw
}
