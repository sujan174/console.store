package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"consolestore/internal/config"
	"consolestore/internal/localstore"
	"consolestore/internal/mcp/orderapp"
)

const (
	appResourceURI  = "ui://console/order"
	appResourceMIME = "text/html;profile=mcp-app"
)

// OpenStoreIn is what Claude sends after resolving the restaurant from chat
// (or nothing at all, for the store home).
type OpenStoreIn struct {
	AddressID      string `json:"address_id,omitempty"`
	RestaurantID   string `json:"restaurant_id"`
	RestaurantName string `json:"restaurant_name,omitempty"`
	Category       string `json:"category,omitempty"`
	ItemID         string `json:"item_id,omitempty"`
	Query          string `json:"query,omitempty"`
}

// CategoryDTO is one dev-curated cuisine chip on the store home.
type CategoryDTO struct {
	Label string `json:"label"`
	Query string `json:"query"`
}

// OpenStoreOut seeds the app so it paints without a second round-trip. Screen
// discriminates the two shapes the app can open into: "home" (categories +
// optional search results + recent orders) or "restaurant" (a menu).
type OpenStoreOut struct {
	Screen       string                   `json:"screen"` // "home" | "restaurant"
	Address      AddrRefDTO               `json:"address"`
	Restaurant   map[string]string        `json:"restaurant,omitempty"`
	Entry        map[string]string        `json:"entry,omitempty"`
	Menu         *GetMenuOut              `json:"menu,omitempty"`
	Categories   []CategoryDTO            `json:"categories,omitempty"`
	Restaurants  []RestaurantDTO          `json:"restaurants,omitempty"`
	RecentOrders []localstore.PlacedOrder `json:"recent_orders,omitempty"`
	Query        string                   `json:"query,omitempty"`
}

func openStoreTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "open_store",
		Description: "Open the interactive ordering app. With no restaurant_id → the store home " +
			"(categories + search + your restaurants); with query → search results on the home " +
			"screen; with restaurant_id → its menu; add item_id to deep-link straight to that item. " +
			"Pass restaurant_name with the display name you resolved so the app can label the store.",
		Meta: mcp.Meta{
			"ui":             map[string]any{"resourceUri": appResourceURI},
			"ui/resourceUri": appResourceURI,
		},
	}
}

func appResource() (*mcp.Resource, mcp.ResourceHandler) {
	res := &mcp.Resource{URI: appResourceURI, MIMEType: appResourceMIME, Name: "consolestore order app"}
	h := func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{URI: appResourceURI, MIMEType: appResourceMIME, Text: orderapp.HTML}},
		}, nil
	}
	return res, h
}

// registerApp wires the open_store tool + the UI resource onto srv.
func (s *Server) registerApp(srv *mcp.Server) {
	res, h := appResource()
	srv.AddResource(res, h)
	addTool(srv, openStoreTool(), s.handleOpenStore)
}

func (s *Server) handleOpenStore(ctx context.Context, _ *mcp.CallToolRequest, in OpenStoreIn) (*mcp.CallToolResult, OpenStoreOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, OpenStoreOut{}, err
	}
	addr, label := in.AddressID, ""
	if addr == "" {
		pref, _ := localstore.LoadAddrPref()
		addr, label = pref.Active()
	}

	if in.RestaurantID != "" {
		m, err := s.be.Menu(addr, in.RestaurantID)
		if err != nil {
			return nil, OpenStoreOut{}, err
		}
		menu := GetMenuOut{RestaurantID: m.RestaurantID, Items: toMenuItemDTOs(m.Items)}
		return nil, OpenStoreOut{
			Screen:     "restaurant",
			Address:    AddrRefDTO{ID: addr, Label: label},
			Restaurant: map[string]string{"id": in.RestaurantID, "name": in.RestaurantName},
			Entry:      map[string]string{"category": in.Category, "item_id": in.ItemID, "address_id": addr},
			Menu:       &menu,
		}, nil
	}

	cats := make([]CategoryDTO, 0)
	for _, c := range config.DefaultCategories() {
		cats = append(cats, CategoryDTO{Label: c.Label, Query: c.Query})
	}
	var rests []RestaurantDTO
	if in.Query != "" {
		res, _, err := s.be.SearchOrganic(addr, in.Query)
		if err != nil {
			return nil, OpenStoreOut{}, err
		}
		rests = toRestaurantDTOs(res)
	}
	recent, _ := localstore.LoadOrders(addr)
	return nil, OpenStoreOut{
		Screen:       "home",
		Address:      AddrRefDTO{ID: addr, Label: label},
		Categories:   cats,
		Restaurants:  rests,
		RecentOrders: recent,
		Query:        in.Query,
	}, nil
}
