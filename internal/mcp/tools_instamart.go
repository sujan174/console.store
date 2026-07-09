package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"consolestore/internal/broker/api"
	"consolestore/internal/localstore"
)

// imMinRupees is Instamart's MIN_ORDER_NOT_MET floor — checkout is refused
// below this. Enforced here so the failure lands at prepare time, mirroring
// the ₹1000 cap check for Food.
const imMinRupees = 99

// --- DTOs ---

type IMVariantDTO struct {
	SpinID  string `json:"spin_id"`
	SkuID   string `json:"sku_id" jsonschema:"required alongside spin_id when adding this variant via im_update_cart"`
	Label   string `json:"label"`
	Price   int    `json:"price"`
	MRP     int    `json:"mrp,omitempty"`
	InStock bool   `json:"in_stock"`
}
type IMProductDTO struct {
	ProductID string         `json:"product_id"`
	Name      string         `json:"name"`
	Brand     string         `json:"brand,omitempty"`
	InStock   bool           `json:"in_stock"`
	Variants  []IMVariantDTO `json:"variants"`
}

func toIMProductDTOs(in []api.IMProduct) []IMProductDTO {
	out := make([]IMProductDTO, 0, len(in))
	for _, p := range in {
		d := IMProductDTO{ProductID: p.ID, Name: p.Name, Brand: p.Brand, InStock: p.InStock}
		for _, v := range p.Variants {
			d.Variants = append(d.Variants, IMVariantDTO{
				SpinID: v.SpinID, SkuID: v.SkuID, Label: v.Label, Price: v.Price, MRP: v.MRP, InStock: v.InStock,
			})
		}
		out = append(out, d)
	}
	return out
}

type IMCartLineDTO struct {
	SpinID    string `json:"spin_id"`
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	Price     int    `json:"price"`
	Available bool   `json:"available"`
}
type IMCartDTO struct {
	Lines          []IMCartLineDTO `json:"lines"`
	ItemTotal      int             `json:"item_total"`
	Delivery       int             `json:"delivery"`
	Handling       int             `json:"handling"`
	Taxes          int             `json:"taxes"`
	ToPay          int             `json:"to_pay"`
	PaymentMethods []string        `json:"payment_methods,omitempty"`
	Message        string          `json:"message,omitempty"`
}

func imCartToDTO(c api.IMCart) IMCartDTO {
	d := IMCartDTO{
		ItemTotal: c.ItemTotal, Delivery: c.Delivery, Handling: c.Handling, Taxes: c.Taxes,
		ToPay: c.Total, PaymentMethods: c.PaymentMethods,
	}
	for _, l := range c.Lines {
		d.Lines = append(d.Lines, IMCartLineDTO{
			SpinID: l.SpinID, Name: l.Name, Quantity: l.Quantity, Price: l.Price, Available: l.Available,
		})
	}
	if len(d.Lines) == 0 {
		d.Message = "instamart cart is empty"
	}
	return d
}

// --- im_search_products ---

type IMSearchProductsIn struct {
	AddressID string `json:"address_id,omitempty"`
	Query     string `json:"query" jsonschema:"product or brand to search for"`
}
type IMSearchProductsOut struct {
	Products []IMProductDTO `json:"products"`
}

func (s *Server) handleIMSearchProducts(ctx context.Context, _ *mcp.CallToolRequest, in IMSearchProductsIn) (*mcp.CallToolResult, IMSearchProductsOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, IMSearchProductsOut{}, err
	}
	ps, err := s.be.IMSearch(in.AddressID, in.Query)
	if err != nil {
		return nil, IMSearchProductsOut{}, err
	}
	return nil, IMSearchProductsOut{Products: toIMProductDTOs(ps)}, nil
}

// --- im_get_cart ---

type IMGetCartIn struct{}
type IMGetCartOut struct {
	Cart IMCartDTO `json:"cart"`
}

func (s *Server) handleIMGetCart(ctx context.Context, _ *mcp.CallToolRequest, _ IMGetCartIn) (*mcp.CallToolResult, IMGetCartOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, IMGetCartOut{}, err
	}
	c, err := s.be.IMGetCart()
	if err != nil {
		return nil, IMGetCartOut{}, err
	}
	return nil, IMGetCartOut{Cart: imCartToDTO(c)}, nil
}

// --- im_update_cart ---

type IMCartItemIn struct {
	SpinID   string `json:"spin_id"`
	SkuID    string `json:"sku_id" jsonschema:"the skuId from the matching search result variant; required alongside spin_id"`
	Quantity int    `json:"quantity"`
}
type IMUpdateCartIn struct {
	AddressID string         `json:"address_id,omitempty"`
	Items     []IMCartItemIn `json:"items" jsonschema:"the full desired set of cart lines (this REPLACES the whole instamart cart)"`
}
type IMUpdateCartOut struct {
	Cart IMCartDTO `json:"cart"`
}

func toIMCartItems(in []IMCartItemIn) []api.IMCartItem {
	out := make([]api.IMCartItem, 0, len(in))
	for _, ci := range in {
		out = append(out, api.IMCartItem{SpinID: ci.SpinID, SkuID: ci.SkuID, Quantity: ci.Quantity})
	}
	return out
}

func (s *Server) handleIMUpdateCart(ctx context.Context, _ *mcp.CallToolRequest, in IMUpdateCartIn) (*mcp.CallToolResult, IMUpdateCartOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, IMUpdateCartOut{}, err
	}
	c, err := s.be.IMUpdateCart(in.AddressID, toIMCartItems(in.Items))
	if err != nil {
		// Swiggy's store-closed message is already user-appropriate — pass it
		// through verbatim rather than wrapping it.
		return nil, IMUpdateCartOut{}, err
	}
	s.recordIMCartWrite(in.AddressID, in.Items)
	return nil, IMUpdateCartOut{Cart: imCartToDTO(c)}, nil
}

// --- im_clear_cart ---

type IMClearCartIn struct{}
type IMClearCartOut struct {
	Cleared bool `json:"cleared"`
}

func (s *Server) handleIMClearCart(ctx context.Context, _ *mcp.CallToolRequest, _ IMClearCartIn) (*mcp.CallToolResult, IMClearCartOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, IMClearCartOut{}, err
	}
	if err := s.be.IMClearCart(); err != nil {
		return nil, IMClearCartOut{}, err
	}
	s.clearCartWrite()
	return nil, IMClearCartOut{Cleared: true}, nil
}

// --- im_prepare_order ---

// imPrepare validates an Instamart cart and mints a confirmation. Shared by
// im_prepare_order and order_preset's instamart path.
func (s *Server) imPrepare(addressID string, c api.IMCart, ident orderIdentity) (string, IMCartDTO, error) {
	if len(c.Lines) == 0 {
		return "", IMCartDTO{}, fmt.Errorf("instamart cart is empty — add items before preparing an order")
	}
	for _, l := range c.Lines {
		if !l.Available {
			return "", IMCartDTO{}, fmt.Errorf("%q is sold out — remove it before ordering", l.Name)
		}
	}
	if c.Total >= orderCapRupees {
		return "", IMCartDTO{}, codedErr(codeOverCap, "the bill is ₹%d — Swiggy refuses agent-placed orders of ₹%d or more; ask the user what to remove to get under the cap", c.Total, orderCapRupees)
	}
	if c.Total < imMinRupees {
		return "", IMCartDTO{}, codedErr(codeUnderMin, "the bill is ₹%d — ₹%d minimum on instamart; ask the user to add more items", c.Total, imMinRupees)
	}
	ident.vertical = "instamart"
	id := s.pending.putIM(addressID, c, ident, nowUnix())
	return id, imCartToDTO(c), nil
}

type IMPrepareOrderIn struct {
	AddressID string `json:"address_id,omitempty"`
}
type IMPrepareOrderOut struct {
	ConfirmationID string     `json:"confirmation_id"`
	Bill           IMCartDTO  `json:"bill"`
	Address        AddrRefDTO `json:"address"`
	Note           string     `json:"note"`
}

func (s *Server) handleIMPrepareOrder(ctx context.Context, _ *mcp.CallToolRequest, in IMPrepareOrderIn) (*mcp.CallToolResult, IMPrepareOrderOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, IMPrepareOrderOut{}, err
	}
	c, err := s.be.IMGetCart()
	if err != nil {
		return nil, IMPrepareOrderOut{}, err
	}
	id, bill, err := s.imPrepare(in.AddressID, c, orderIdentity{restaurantName: "Instamart"})
	if err != nil {
		return nil, IMPrepareOrderOut{}, err
	}
	card, _ := localstore.LoadCard()
	return nil, IMPrepareOrderOut{
		ConfirmationID: id, Bill: bill,
		Address: AddrRefDTO{ID: in.AddressID, Label: addrLabelFor(card, in.AddressID)},
		Note:    "COD only — show the user the full bill breakdown AND the delivery address; call place_order with this confirmation_id ONLY after they confirm.",
	}, nil
}

// recordIMCartWrite stores a minimal cart-write for an Instamart update_cart
// call so save_preset can snapshot it. Instamart lines carry no name (the DTO
// only sends spinId/qty) — names are filled from the returned cart lines
// where possible; save_preset re-reads the live cart directly instead, so
// this is best-effort context only, kept for symmetry with the food path.
func (s *Server) recordIMCartWrite(addressID string, items []IMCartItemIn) {
	cw := &cartWrite{AddressID: addressID, RestaurantName: "Instamart"}
	for _, it := range items {
		cw.Lines = append(cw.Lines, memLine{ItemID: it.SpinID, Qty: it.Quantity})
	}
	s.recordCartWrite(cw)
}
