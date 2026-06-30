// Package mcp serves consolestore's Swiggy ordering tools to local agents over
// a stdio MCP server. It is a second front-end over broker.Service (alongside
// the TUI and the headless CLI) and MUST NOT import internal/tui.
package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"consolestore/internal/broker/api"
	"consolestore/internal/version"
)

// Backend is the account-pinned slice of broker capability the tools need.
// *datasource.BrokerBackend satisfies it.
type Backend interface {
	Addresses() ([]api.Address, error)
	SearchOrganic(addressID, query string) ([]api.Restaurant, string, error)
	PlacesQuery(addressID, query string) ([]api.Restaurant, error)
	Usuals(addressID string) ([]api.Restaurant, error)
	Menu(addressID, restaurantID string) (api.Menu, error)
	ItemOptions(addressID, restaurantID, itemName, menuItemID string) ([]api.OptionGroup, error)
	GetCart(addressID, restaurantName string) (api.Cart, error)
	UpdateCart(addressID, restaurantID, restaurantName string, items []api.CartItem) (api.Cart, error)
	ClearCart() error
	PlaceOrder(addressID string) (api.Order, error)
	TrackOrder(orderID string) (api.Tracking, error)
	ActiveOrders(addressID string) ([]api.Order, error)
}

// Authenticator drives first-run sign-in without exposing the token. Implemented
// in package main over the OAuth manager + loopback callback server.
type Authenticator interface {
	TokenPresent(ctx context.Context) bool
	// Start begins (or resumes) the loopback OAuth flow and returns the browser
	// authorize URL plus a flow id to poll. It also ensures the loopback callback
	// server is listening.
	Start(ctx context.Context) (authorizeURL, flowID string, err error)
	Authorized(flowID string) bool
}

type Server struct {
	be      Backend
	auth    Authenticator
	pending *confirmStore
}

func NewServer(be Backend, auth Authenticator) *Server {
	return &Server{be: be, auth: auth, pending: newConfirmStore()}
}

type ServerInfoIn struct{}
type ServerInfoOut struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (s *Server) handleServerInfo(_ context.Context, _ *mcp.CallToolRequest, _ ServerInfoIn) (*mcp.CallToolResult, ServerInfoOut, error) {
	return nil, ServerInfoOut{Name: "consolestore", Version: version.Version}, nil
}

// Serve registers all tools and runs the stdio server until ctx is done.
func Serve(ctx context.Context, s *Server) error {
	srv := mcp.NewServer(&mcp.Implementation{Name: "consolestore", Version: version.Version}, nil)
	s.register(srv)
	return srv.Run(ctx, &mcp.StdioTransport{})
}

// register wires every tool. Later tasks append to it.
func (s *Server) register(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{Name: "server_info", Description: "consolestore server name and version"}, s.handleServerInfo)
	mcp.AddTool(srv, &mcp.Tool{Name: "list_addresses", Description: "the user's saved Swiggy delivery addresses"}, s.handleListAddresses)
	mcp.AddTool(srv, &mcp.Tool{Name: "search_restaurants", Description: "search restaurants/dishes deliverable to an address"}, s.handleSearchRestaurants)
	mcp.AddTool(srv, &mcp.Tool{Name: "list_usuals", Description: "the user's frequently ordered restaurants for an address"}, s.handleListUsuals)
	mcp.AddTool(srv, &mcp.Tool{Name: "get_menu", Description: "menu items for a restaurant at an address"}, s.handleGetMenu)
	mcp.AddTool(srv, &mcp.Tool{Name: "get_item_options", Description: "variant/add-on groups for a customizable item"}, s.handleGetItemOptions)
	mcp.AddTool(srv, &mcp.Tool{Name: "list_active_orders", Description: "live (in-progress) orders for an address"}, s.handleListActiveOrders)
	mcp.AddTool(srv, &mcp.Tool{Name: "track_order", Description: "live status + ETA for an order id"}, s.handleTrackOrder)
	mcp.AddTool(srv, &mcp.Tool{Name: "list_presets", Description: "saved order presets (named cart snapshots)"}, s.handleListPresets)
	mcp.AddTool(srv, &mcp.Tool{Name: "get_cart", Description: "the current cart with the authoritative Swiggy bill"}, s.handleGetCart)
	mcp.AddTool(srv, &mcp.Tool{Name: "update_cart", Description: "set the cart lines for a restaurant (replaces the cart)"}, s.handleUpdateCart)
	mcp.AddTool(srv, &mcp.Tool{Name: "clear_cart", Description: "empty the cart"}, s.handleClearCart)
}
