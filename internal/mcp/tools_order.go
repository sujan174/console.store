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
	id := s.pending.put(addressID, c, ident, nowUnix())
	return id, cartToDTO(c), nil
}

type PrepareOrderIn struct {
	AddressID string `json:"address_id"`
}
type PrepareOrderOut struct {
	ConfirmationID string     `json:"confirmation_id"`
	Bill           CartDTO    `json:"bill"`
	Address        AddrRefDTO `json:"address"` // where this order delivers — show it with the bill
	Note           string     `json:"note"`
}

func (s *Server) handlePrepareOrder(ctx context.Context, _ *mcp.CallToolRequest, in PrepareOrderIn) (*mcp.CallToolResult, PrepareOrderOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, PrepareOrderOut{}, err
	}
	c, err := s.be.GetCart(in.AddressID, "")
	if err != nil {
		return nil, PrepareOrderOut{}, err
	}
	// Ad-hoc order: the cart carries only a restaurant name, no Swiggy id, so
	// restaurantID stays empty (bumpFavorite will skip — no name-keyed favorite).
	id, bill, err := s.prepare(in.AddressID, c, orderIdentity{restaurantName: c.Restaurant})
	if err != nil {
		return nil, PrepareOrderOut{}, err
	}
	card, _ := localstore.LoadCard()
	return nil, PrepareOrderOut{
		ConfirmationID: id, Bill: bill,
		Address: AddrRefDTO{ID: in.AddressID, Label: addrLabelFor(card, in.AddressID)},
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
		return nil, PlaceOrderOut{}, errors.New("unknown or expired confirmation_id — call prepare_order again")
	}
	// Re-fetch and verify the cart still matches what the user confirmed.
	c, err := s.be.GetCart(p.addressID, "")
	if err != nil {
		return nil, PlaceOrderOut{}, err
	}
	if cartHash(p.addressID, c) != p.hash || c.Total != p.total {
		return nil, PlaceOrderOut{}, errors.New("cart changed since prepare_order — call prepare_order again to re-confirm")
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
	return nil, PlaceOrderOut{Order: toOrderDTO(order)}, nil
}

type OrderPresetIn struct {
	Name  string `json:"name"`
	Index int    `json:"index,omitempty" jsonschema:"0-based pick among presets sharing a name; default 0"`
}
type OrderPresetOut struct {
	ConfirmationID string     `json:"confirmation_id"`
	Bill           CartDTO    `json:"bill"`
	Address        AddrRefDTO `json:"address"` // where this order delivers — show it with the bill
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
	return nil, OrderPresetOut{ConfirmationID: id, Bill: bill,
		Address: AddrRefDTO{ID: p.AddrID, Label: p.AddrLine},
		Note:    "show the user the full bill breakdown AND the delivery address; call place_order with this confirmation_id ONLY after they confirm."}, nil
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
