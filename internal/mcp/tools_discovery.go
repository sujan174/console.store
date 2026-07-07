package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"consolestore/internal/broker/api"
	"consolestore/internal/localstore"
)

// requireAuth gates every data/order tool. The agent is told to call sign_in.
func (s *Server) requireAuth(ctx context.Context) error {
	if s.auth == nil || !s.auth.TokenPresent(ctx) {
		return errors.New("not signed in — call the sign_in tool to authorize, then retry")
	}
	return nil
}

// --- DTOs (lean projections of api.* so the agent gets stable, documented shapes) ---

type AddressDTO struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Full  string `json:"full"`
}
type RestaurantDTO struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	ETA         string  `json:"eta"`
	Rating      float64 `json:"rating"`
	Offer       string  `json:"offer,omitempty"`
	Unavailable bool    `json:"unavailable"`
}
type MenuItemDTO struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Price        int    `json:"price"`
	Veg          bool   `json:"veg"`
	InStock      bool   `json:"in_stock"`
	Customizable bool   `json:"customizable"`
	Category     string `json:"category,omitempty"`
}

func toAddressDTOs(in []api.Address) []AddressDTO {
	out := make([]AddressDTO, 0, len(in))
	for _, a := range in {
		out = append(out, AddressDTO{ID: a.ID, Label: a.Label, Full: a.Full})
	}
	return out
}
func toRestaurantDTOs(in []api.Restaurant) []RestaurantDTO {
	out := make([]RestaurantDTO, 0, len(in))
	for _, r := range in {
		out = append(out, RestaurantDTO{ID: r.ID, Name: r.Name, ETA: r.ETA, Rating: r.Rating, Offer: r.Offer, Unavailable: r.Unavailable})
	}
	return out
}
func toMenuItemDTOs(in []api.MenuItem) []MenuItemDTO {
	out := make([]MenuItemDTO, 0, len(in))
	for _, m := range in {
		out = append(out, MenuItemDTO{ID: m.ID, Name: m.Name, Price: m.Price, Veg: m.Veg, InStock: m.InStock, Customizable: m.Customizable, Category: m.Category})
	}
	return out
}

// --- list_addresses ---

type ListAddressesIn struct{}
type ListAddressesOut struct {
	Addresses []AddressDTO `json:"addresses"`
}

func (s *Server) handleListAddresses(ctx context.Context, _ *mcp.CallToolRequest, _ ListAddressesIn) (*mcp.CallToolResult, ListAddressesOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, ListAddressesOut{}, err
	}
	addrs, err := s.be.Addresses()
	if err != nil {
		return nil, ListAddressesOut{}, err
	}
	return nil, ListAddressesOut{Addresses: toAddressDTOs(addrs)}, nil
}

// --- search_restaurants ---

type SearchRestaurantsIn struct {
	AddressID string `json:"address_id" jsonschema:"the delivery address id from list_addresses"`
	Query     string `json:"query" jsonschema:"restaurant or dish to search for"`
}
type SearchRestaurantsOut struct {
	Restaurants []RestaurantDTO `json:"restaurants"`
	Corrected   string          `json:"corrected_query,omitempty"`
}

func (s *Server) handleSearchRestaurants(ctx context.Context, _ *mcp.CallToolRequest, in SearchRestaurantsIn) (*mcp.CallToolResult, SearchRestaurantsOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, SearchRestaurantsOut{}, err
	}
	res, effective, err := s.be.SearchOrganic(in.AddressID, in.Query)
	if err != nil {
		return nil, SearchRestaurantsOut{}, err
	}
	out := SearchRestaurantsOut{Restaurants: toRestaurantDTOs(res)}
	if effective != in.Query {
		out.Corrected = effective
	}
	return nil, out, nil
}

// --- list_usuals ---

type ListUsualsIn struct {
	AddressID string `json:"address_id"`
}
type ListUsualsOut struct {
	Restaurants []RestaurantDTO `json:"restaurants"`
}

func (s *Server) handleListUsuals(ctx context.Context, _ *mcp.CallToolRequest, in ListUsualsIn) (*mcp.CallToolResult, ListUsualsOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, ListUsualsOut{}, err
	}
	res, err := s.be.Usuals(in.AddressID)
	if err != nil {
		return nil, ListUsualsOut{}, err
	}
	return nil, ListUsualsOut{Restaurants: toRestaurantDTOs(res)}, nil
}

// --- get_menu ---

type GetMenuIn struct {
	AddressID    string `json:"address_id"`
	RestaurantID string `json:"restaurant_id"`
}
type GetMenuOut struct {
	RestaurantID string        `json:"restaurant_id"`
	Items        []MenuItemDTO `json:"items"`
}

func (s *Server) handleGetMenu(ctx context.Context, _ *mcp.CallToolRequest, in GetMenuIn) (*mcp.CallToolResult, GetMenuOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, GetMenuOut{}, err
	}
	m, err := s.be.Menu(in.AddressID, in.RestaurantID)
	if err != nil {
		return nil, GetMenuOut{}, err
	}
	return nil, GetMenuOut{RestaurantID: m.RestaurantID, Items: toMenuItemDTOs(m.Items)}, nil
}

// --- get_item_options ---

type GetItemOptionsIn struct {
	AddressID    string `json:"address_id"`
	RestaurantID string `json:"restaurant_id"`
	ItemName     string `json:"item_name"`
	MenuItemID   string `json:"menu_item_id"`
}
type OptionChoiceDTO struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Price   int    `json:"price"`
	InStock bool   `json:"in_stock"`
}
type OptionGroupDTO struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Min      int               `json:"min"`
	Max      int               `json:"max"`
	Variant  bool              `json:"variant"`
	Absolute bool              `json:"absolute"`
	Choices  []OptionChoiceDTO `json:"choices"`
}
type GetItemOptionsOut struct {
	Groups []OptionGroupDTO `json:"groups"`
}

func (s *Server) handleGetItemOptions(ctx context.Context, _ *mcp.CallToolRequest, in GetItemOptionsIn) (*mcp.CallToolResult, GetItemOptionsOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, GetItemOptionsOut{}, err
	}
	groups, err := s.be.ItemOptions(in.AddressID, in.RestaurantID, in.ItemName, in.MenuItemID)
	if err != nil {
		return nil, GetItemOptionsOut{}, err
	}
	s.rememberOptions(groups)
	out := GetItemOptionsOut{Groups: make([]OptionGroupDTO, 0, len(groups))}
	for _, g := range groups {
		dg := OptionGroupDTO{ID: g.ID, Name: g.Name, Min: g.Min, Max: g.Max, Variant: g.Variant, Absolute: g.Absolute}
		for _, c := range g.Choices {
			dg.Choices = append(dg.Choices, OptionChoiceDTO{ID: c.ID, Name: c.Name, Price: c.Price, InStock: c.InStock})
		}
		out.Groups = append(out.Groups, dg)
	}
	return nil, out, nil
}

// --- list_active_orders ---

type ListActiveOrdersIn struct {
	AddressID string `json:"address_id"`
}
type OrderDTO struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	Restaurant string `json:"restaurant"`
	Total      int    `json:"total"`
	ETA        string `json:"eta"`
	Vertical   string `json:"vertical"` // "food" or "instamart"
}
type ListActiveOrdersOut struct {
	Orders  []OrderDTO `json:"orders"`
	Warning string     `json:"warning,omitempty"` // set when the instamart query failed; food orders above are still complete
}

func toOrderDTO(o api.Order) OrderDTO {
	return OrderDTO{ID: o.ID, Status: o.Status, Restaurant: o.Restaurant, Total: o.Total, ETA: o.ETA, Vertical: "food"}
}

func toIMOrderDTO(o api.IMOrder) OrderDTO {
	restaurant := "Instamart"
	return OrderDTO{ID: o.ID, Status: o.Status, Restaurant: restaurant, Total: o.Total, ETA: o.ETA, Vertical: "instamart"}
}

func (s *Server) handleListActiveOrders(ctx context.Context, _ *mcp.CallToolRequest, in ListActiveOrdersIn) (*mcp.CallToolResult, ListActiveOrdersOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, ListActiveOrdersOut{}, err
	}
	orders, err := s.be.ActiveOrders(in.AddressID)
	if err != nil {
		return nil, ListActiveOrdersOut{}, err
	}
	out := ListActiveOrdersOut{Orders: make([]OrderDTO, 0, len(orders))}
	for _, o := range orders {
		out.Orders = append(out.Orders, toOrderDTO(o))
	}
	// Instamart orders are merged in best-effort: a failure here (e.g. no
	// Instamart history/account restriction) must not break the food listing.
	imOrders, imErr := s.be.IMOrders(true)
	if imErr != nil {
		out.Warning = fmt.Sprintf("instamart orders unavailable: %v", imErr)
	} else {
		for _, o := range imOrders {
			out.Orders = append(out.Orders, toIMOrderDTO(o))
		}
	}
	return nil, out, nil
}

// --- track_order ---

type TrackOrderIn struct {
	OrderID string `json:"order_id"`
}
type TrackOrderOut struct {
	Status string `json:"status"`
	ETA    string `json:"eta"`
}

func (s *Server) handleTrackOrder(ctx context.Context, _ *mcp.CallToolRequest, in TrackOrderIn) (*mcp.CallToolResult, TrackOrderOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, TrackOrderOut{}, err
	}
	if lat, lng, ok := s.imTrackCoords(in.OrderID); ok {
		tr, err := s.be.IMTrack(in.OrderID, lat, lng)
		if err != nil {
			return nil, TrackOrderOut{}, err
		}
		return nil, TrackOrderOut{Status: tr.Status, ETA: tr.ETA}, nil
	}
	tr, err := s.be.TrackOrder(in.OrderID)
	if err != nil {
		return nil, TrackOrderOut{}, err
	}
	return nil, TrackOrderOut{Status: tr.Status, ETA: tr.ETA}, nil
}

// imTrackCoords resolves the lat/lng needed to poll IMTrack for orderID: first
// the locally saved ActiveOrder (cheap, no network), then a live IMOrders(true)
// lookup. Returns ok=false when orderID isn't a known Instamart order — the
// caller then falls back to food TrackOrder.
func (s *Server) imTrackCoords(orderID string) (lat, lng float64, ok bool) {
	if ao, found, err := localstore.LoadActiveOrder(); err == nil && found &&
		ao.OrderID == orderID && ao.Vertical == "instamart" {
		return ao.Lat, ao.Lng, true
	}
	orders, err := s.be.IMOrders(true)
	if err != nil {
		return 0, 0, false
	}
	for _, o := range orders {
		if o.ID == orderID {
			return o.Lat, o.Lng, true
		}
	}
	return 0, 0, false
}

// --- list_presets (local, no Swiggy call, still gated for consistency) ---

type ListPresetsIn struct{}
type PresetDTO struct {
	Name           string `json:"name"`
	RestaurantName string `json:"restaurant"`
	AddrLine       string `json:"address"`
	Lines          int    `json:"line_count"`
	Vertical       string `json:"vertical"` // "food" or "instamart"
}
type ListPresetsOut struct {
	Presets []PresetDTO `json:"presets"`
}

func (s *Server) handleListPresets(ctx context.Context, _ *mcp.CallToolRequest, _ ListPresetsIn) (*mcp.CallToolResult, ListPresetsOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, ListPresetsOut{}, err
	}
	ps, err := localstore.LoadPresets()
	if err != nil {
		return nil, ListPresetsOut{}, err
	}
	out := ListPresetsOut{Presets: make([]PresetDTO, 0, len(ps.Items))}
	for _, p := range ps.Items {
		v := "food"
		if p.IsInstamart() {
			v = "instamart"
		}
		out.Presets = append(out.Presets, PresetDTO{Name: p.Name, RestaurantName: p.RestaurantName, AddrLine: p.AddrLine, Lines: len(p.Lines), Vertical: v})
	}
	return nil, out, nil
}
