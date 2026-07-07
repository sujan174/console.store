package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"consolestore/internal/mcp/orderapp"
)

const (
	appResourceURI  = "ui://console/order"
	appResourceMIME = "text/html;profile=mcp-app"
)

// OpenStoreIn is what Claude sends after resolving the restaurant from chat.
type OpenStoreIn struct {
	AddressID      string `json:"address_id,omitempty"`
	RestaurantID   string `json:"restaurant_id"`
	RestaurantName string `json:"restaurant_name,omitempty"`
	Category       string `json:"category,omitempty"`
	ItemID         string `json:"item_id,omitempty"`
}

// OpenStoreOut seeds the app so it paints without a second round-trip.
type OpenStoreOut struct {
	Restaurant map[string]string `json:"restaurant"`
	Entry      map[string]string `json:"entry"`
	Menu       GetMenuOut        `json:"menu"`
}

func openStoreTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "open_store",
		Description: "Open the interactive ordering app for a known restaurant, optionally deep-linked to a category or item. Call after the user has named a restaurant. Pass restaurant_name with the display name you resolved so the app can label the store.",
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
	addr := in.AddressID
	if addr == "" {
		list, err := s.be.Addresses()
		if err != nil {
			return nil, OpenStoreOut{}, err
		}
		if len(list) > 0 {
			addr = list[0].ID
		}
	}
	m, err := s.be.Menu(addr, in.RestaurantID)
	if err != nil {
		return nil, OpenStoreOut{}, err
	}
	return nil, OpenStoreOut{
		Restaurant: map[string]string{"id": in.RestaurantID, "name": in.RestaurantName},
		Entry:      map[string]string{"category": in.Category, "item_id": in.ItemID, "address_id": addr},
		Menu:       GetMenuOut{RestaurantID: m.RestaurantID, Items: toMenuItemDTOs(m.Items)},
	}, nil
}
