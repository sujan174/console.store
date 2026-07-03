package mcp

import (
	"context"
	"errors"
	"fmt"
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
}
type PlaceOrderOut struct {
	Order OrderDTO `json:"order"`
}

func (s *Server) handlePlaceOrder(ctx context.Context, _ *mcp.CallToolRequest, in PlaceOrderIn) (*mcp.CallToolResult, PlaceOrderOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, PlaceOrderOut{}, err
	}
	p, ok := s.pending.take(in.ConfirmationID, nowUnix())
	if !ok {
		return nil, PlaceOrderOut{}, codedErr(codeConfirmationExpired, "unknown or expired confirmation_id — call prepare_order again")
	}
	if p.vertical == "instamart" {
		return s.placeIMOrder(p)
	}
	// Re-fetch and verify the cart still matches what the user confirmed.
	c, err := s.be.GetCart(p.addressID, "")
	if err != nil {
		return nil, PlaceOrderOut{}, err
	}
	if cartHash(p.addressID, c) != p.hash || c.Total != p.total {
		return nil, PlaceOrderOut{}, codedErr(codeCartChanged, "cart changed since prepare_order — call prepare_order again to re-confirm")
	}
	order, err := s.be.PlaceOrder(p.addressID) // never retried
	if err != nil {
		return nil, PlaceOrderOut{}, fmt.Errorf("order failed: %w — run list_active_orders before retrying in case it was placed", err)
	}
	// Persist for `console status`/tracking and accrete the taste card.
	etaLo, etaHi := localstore.ParseETAMinutes(order.ETA)
	_ = localstore.SaveActiveOrder(localstore.ActiveOrder{
		OrderID: order.ID, Restaurant: order.Restaurant, ETALoMin: etaLo, ETAHiMin: etaHi,
		Total: order.Total, PlacedAt: nowUnix(),
	})
	// Record real identity: restaurantID is a Swiggy id for presets, "" for ad-hoc
	// orders (then bumpFavorite skips). Never a name in the id slot.
	_ = localstore.RecordOrder(p.addressID, p.addrLabel, p.restaurantID, p.restaurantName, nowUnix())
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
	return nil, PlaceOrderOut{Order: toOrderDTO(order)}, nil
}

// placeIMOrder re-verifies and places an Instamart order from a confirmation
// minted by im_prepare_order/order_preset. Mirrors handlePlaceOrder's food
// path: re-fetch, hash/total re-check, place once (never retried), persist
// ActiveOrder (Vertical "instamart"), best-effort stamp Lat/Lng from IMOrders.
func (s *Server) placeIMOrder(p pendingOrder) (*mcp.CallToolResult, PlaceOrderOut, error) {
	c, err := s.be.IMGetCart()
	if err != nil {
		return nil, PlaceOrderOut{}, err
	}
	if imCartHash(p.addressID, c) != p.hash || c.Total != p.total {
		return nil, PlaceOrderOut{}, codedErr(codeCartChanged, "cart changed since im_prepare_order — call im_prepare_order again to re-confirm")
	}
	order, err := s.be.IMPlaceOrder(p.addressID) // never retried
	if err != nil {
		return nil, PlaceOrderOut{}, fmt.Errorf("order failed: %w — run list_active_orders before retrying in case it was placed", err)
	}
	etaLo, etaHi := localstore.ParseETAMinutes(order.ETA)
	active := localstore.ActiveOrder{
		OrderID: order.ID, Restaurant: order.Restaurant, ETALoMin: etaLo, ETAHiMin: etaHi,
		Total: order.Total, PlacedAt: nowUnix(), Vertical: "instamart",
		// The cart just re-fetched above is the ONLY source of the delivery
		// coordinates track_order requires — get_addresses and get_orders both
		// omit them (harvested 2026-07-03).
		Lat: c.AddrLat, Lng: c.AddrLng,
	}
	_ = localstore.SaveActiveOrder(active)
	_ = localstore.RecordOrder(p.addressID, p.addrLabel, p.restaurantID, p.restaurantName, nowUnix())
	s.markCartWritePlaced()
	return nil, PlaceOrderOut{Order: toOrderDTO(order)}, nil
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
