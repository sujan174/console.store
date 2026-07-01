package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"consolestore/internal/broker/api"
)

type CartLineDTO struct {
	ItemID    string `json:"item_id"`
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	Price     int    `json:"price"`
	Available bool   `json:"available"`
}
type CartDTO struct {
	CartID     string        `json:"cart_id,omitempty"`
	Restaurant string        `json:"restaurant"`
	ItemTotal  int           `json:"item_total"`
	Delivery   int           `json:"delivery"`
	Taxes      int           `json:"taxes"`
	Total      int           `json:"total"`
	Lines      []CartLineDTO `json:"lines"`
}

// cartToDTO projects the agent-facing cart. api.Cart.ValidAddons is intentionally
// omitted — it's an internal TUI customize-wizard field, not order-relevant.
func cartToDTO(c api.Cart) CartDTO {
	d := CartDTO{CartID: c.CartID, Restaurant: c.Restaurant, ItemTotal: c.ItemTotal, Delivery: c.Delivery, Taxes: c.Taxes, Total: c.Total}
	for _, l := range c.Lines {
		d.Lines = append(d.Lines, CartLineDTO{ItemID: l.ItemID, Name: l.Name, Quantity: l.Quantity, Price: l.Price, Available: l.Available})
	}
	return d
}

// CartItemIn mirrors api.CartItem with snake_case selection groups.
type CartVariantSelIn struct {
	GroupID     string `json:"group_id"`
	VariationID string `json:"variation_id"`
}
type CartAddonSelIn struct {
	GroupID  string `json:"group_id"`
	ChoiceID string `json:"choice_id"`
}
type CartItemIn struct {
	ItemID         string             `json:"item_id"`
	Quantity       int                `json:"quantity"`
	VariantsV2     []CartVariantSelIn `json:"variants_v2,omitempty"`
	VariantsLegacy []CartVariantSelIn `json:"variants_legacy,omitempty"`
	Addons         []CartAddonSelIn   `json:"addons,omitempty"`
}

func toCartItems(in []CartItemIn) []api.CartItem {
	out := make([]api.CartItem, 0, len(in))
	for _, ci := range in {
		item := api.CartItem{ItemID: ci.ItemID, Quantity: ci.Quantity}
		for _, v := range ci.VariantsV2 {
			item.VariantsV2 = append(item.VariantsV2, api.CartVariantSel{GroupID: v.GroupID, VariationID: v.VariationID})
		}
		for _, v := range ci.VariantsLegacy {
			item.VariantsLegacy = append(item.VariantsLegacy, api.CartVariantSel{GroupID: v.GroupID, VariationID: v.VariationID})
		}
		for _, a := range ci.Addons {
			item.Addons = append(item.Addons, api.CartAddonSel{GroupID: a.GroupID, ChoiceID: a.ChoiceID})
		}
		out = append(out, item)
	}
	return out
}

// --- get_cart ---

type GetCartIn struct {
	AddressID string `json:"address_id"`
}
type GetCartOut struct {
	Cart CartDTO `json:"cart"`
}

func (s *Server) handleGetCart(ctx context.Context, _ *mcp.CallToolRequest, in GetCartIn) (*mcp.CallToolResult, GetCartOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, GetCartOut{}, err
	}
	c, err := s.be.GetCart(in.AddressID, "")
	if err != nil {
		return nil, GetCartOut{}, err
	}
	return nil, GetCartOut{Cart: cartToDTO(c)}, nil
}

// --- update_cart ---

type UpdateCartIn struct {
	AddressID      string       `json:"address_id"`
	RestaurantID   string       `json:"restaurant_id"`
	RestaurantName string       `json:"restaurant_name,omitempty"`
	Items          []CartItemIn `json:"items" jsonschema:"the full desired set of cart lines (this replaces the cart for the restaurant)"`
}
type UpdateCartOut struct {
	Cart CartDTO `json:"cart"`
}

func (s *Server) handleUpdateCart(ctx context.Context, _ *mcp.CallToolRequest, in UpdateCartIn) (*mcp.CallToolResult, UpdateCartOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, UpdateCartOut{}, err
	}
	c, err := s.be.UpdateCart(in.AddressID, in.RestaurantID, in.RestaurantName, toCartItems(in.Items))
	if err != nil {
		return nil, UpdateCartOut{}, err
	}
	s.recordCartWrite(cartWriteFromUpdate(s, in, c))
	return nil, UpdateCartOut{Cart: cartToDTO(c)}, nil
}

// cartWriteFromUpdate projects an update_cart call + its resulting cart into a
// cartWrite for the memory caches: item names are resolved from the returned
// cart lines (the input only carries ids), and selection ids are named via the
// server's option-name cache when available.
func cartWriteFromUpdate(s *Server, in UpdateCartIn, c api.Cart) *cartWrite {
	names := make(map[string]string, len(c.Lines))
	for _, l := range c.Lines {
		names[l.ItemID] = l.Name
	}
	cw := &cartWrite{AddressID: in.AddressID, RestaurantID: in.RestaurantID, RestaurantName: in.RestaurantName}
	for _, ci := range in.Items {
		ln := memLine{ItemID: ci.ItemID, ItemName: names[ci.ItemID], Qty: ci.Quantity}
		for _, v := range ci.VariantsV2 {
			sel := memSel{GroupID: v.GroupID, ChoiceID: v.VariationID, Variant: true, Absolute: true}
			s.nameSel(&sel)
			ln.Sels = append(ln.Sels, sel)
		}
		for _, v := range ci.VariantsLegacy {
			sel := memSel{GroupID: v.GroupID, ChoiceID: v.VariationID, Variant: true, Absolute: false}
			s.nameSel(&sel)
			ln.Sels = append(ln.Sels, sel)
		}
		for _, a := range ci.Addons {
			sel := memSel{GroupID: a.GroupID, ChoiceID: a.ChoiceID, Variant: false, Absolute: false}
			s.nameSel(&sel)
			ln.Sels = append(ln.Sels, sel)
		}
		cw.Lines = append(cw.Lines, ln)
	}
	return cw
}

// --- clear_cart ---

type ClearCartIn struct{}
type ClearCartOut struct {
	Cleared bool `json:"cleared"`
}

func (s *Server) handleClearCart(ctx context.Context, _ *mcp.CallToolRequest, _ ClearCartIn) (*mcp.CallToolResult, ClearCartOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, ClearCartOut{}, err
	}
	if err := s.be.ClearCart(); err != nil {
		return nil, ClearCartOut{}, err
	}
	return nil, ClearCartOut{Cleared: true}, nil
}
