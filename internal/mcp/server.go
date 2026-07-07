// Package mcp serves consolestore's Swiggy ordering tools to local agents over
// a stdio MCP server. It is a second front-end over broker.Service (alongside
// the TUI and the headless CLI) and MUST NOT import internal/tui.
package mcp

import (
	"context"
	"errors"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"consolestore/internal/broker/api"
	"consolestore/internal/version"
)

var errAuthUnavailable = errors.New("sign-in is unavailable in this build")

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

	// Instamart (grocery) vertical — separate cart, address-bound not
	// restaurant-bound, keyed by spinId.
	IMSearch(addressID, query string) ([]api.IMProduct, error)
	IMGetCart() (api.IMCart, error)
	IMUpdateCart(addressID string, items []api.IMCartItem) (api.IMCart, error)
	IMClearCart() error
	IMPlaceOrder(addressID string) (api.Order, error)
	IMOrders(activeOnly bool) ([]api.IMOrder, error)
	IMTrack(orderID string, lat, lng float64) (api.Tracking, error)
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

	// mu guards optNames and lastCart, the process-local memory caches that
	// bridge get_item_options / update_cart / order_preset into a taste
	// observation on place_order and into save_preset. See memcache.go.
	mu       sync.Mutex
	optNames map[string]namedChoice
	lastCart *cartWrite
}

func NewServer(be Backend, auth Authenticator) *Server {
	return &Server{be: be, auth: auth, pending: newConfirmStore(), optNames: map[string]namedChoice{}}
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

// denull rewrites the ["null", X] union that jsonschema-go emits for Go slices
// (and other nilable types) into the single type X. Some MCP clients can't
// coerce a union-typed parameter — they fall back to sending a JSON string,
// which the server then rejects ("has type string, want array"), so an agent
// could never fill a slice param like update_cart.items. Every slice/optional
// param must therefore advertise a plain, single type. Recurses through array
// element and object property schemas so nested slices are collapsed too.
func denull(s *jsonschema.Schema) {
	if s == nil {
		return
	}
	if len(s.Types) > 0 {
		rest := make([]string, 0, len(s.Types))
		for _, t := range s.Types {
			if t != "null" {
				rest = append(rest, t)
			}
		}
		if len(rest) == 1 {
			s.Type = rest[0]
			s.Types = nil
		} else {
			s.Types = rest
		}
	}
	denull(s.Items)
	denull(s.AdditionalProperties)
	for _, p := range s.Properties {
		denull(p)
	}
}

// toolInputSchema infers T's JSON schema and collapses slice null-unions so MCP
// clients can coerce every parameter. Returns nil if inference fails, in which
// case the caller leaves the schema unset and the SDK falls back to its own
// (union-typed) inference.
func toolInputSchema[T any]() *jsonschema.Schema {
	sc, err := jsonschema.For[T](nil)
	if err != nil {
		return nil
	}
	denull(sc)
	return sc
}

// addTool registers a tool with a client-friendly (de-nulled) input schema. Use
// this instead of mcp.AddTool directly so slice params work for agents.
func addTool[In, Out any](srv *mcp.Server, t *mcp.Tool, h mcp.ToolHandlerFor[In, Out]) {
	if sc := toolInputSchema[In](); sc != nil {
		t.InputSchema = sc
	}
	mcp.AddTool(srv, t, h)
}

// register wires every tool. Later tasks append to it.
func (s *Server) register(srv *mcp.Server) {
	addTool(srv, &mcp.Tool{Name: "server_info", Description: "consolestore server name and version"}, s.handleServerInfo)
	addTool(srv, &mcp.Tool{Name: "initialize", Description: "Fast readiness check for ordering: returns whether the user is signed in and their active delivery address (locked default, else last-used). Call this FIRST on any ordering intent. Does not fetch the address list."}, s.handleInitialize)
	addTool(srv, &mcp.Tool{Name: "list_addresses", Description: "the user's saved Swiggy delivery addresses"}, s.handleListAddresses)
	addTool(srv, &mcp.Tool{Name: "search_restaurants", Description: "search restaurants/dishes deliverable to an address"}, s.handleSearchRestaurants)
	addTool(srv, &mcp.Tool{Name: "list_usuals", Description: "the user's frequently ordered restaurants for an address"}, s.handleListUsuals)
	addTool(srv, &mcp.Tool{Name: "get_menu", Description: "menu items for a restaurant at an address"}, s.handleGetMenu)
	addTool(srv, &mcp.Tool{Name: "get_item_options", Description: "variant/add-on groups for a customizable item"}, s.handleGetItemOptions)
	addTool(srv, &mcp.Tool{Name: "list_active_orders", Description: "live (in-progress) orders for an address"}, s.handleListActiveOrders)
	addTool(srv, &mcp.Tool{Name: "track_order", Description: "live status + ETA for an order id"}, s.handleTrackOrder)
	addTool(srv, &mcp.Tool{Name: "list_presets", Description: "saved order presets (named cart snapshots)"}, s.handleListPresets)
	addTool(srv, &mcp.Tool{Name: "get_cart", Description: "the current cart with the authoritative Swiggy bill"}, s.handleGetCart)
	addTool(srv, &mcp.Tool{Name: "update_cart", Description: "set the cart lines for a restaurant (replaces the cart; a cart from another restaurant is auto-replaced and reported in replaced_cart)"}, s.handleUpdateCart)
	addTool(srv, &mcp.Tool{Name: "clear_cart", Description: "empty the cart"}, s.handleClearCart)
	addTool(srv, &mcp.Tool{Name: "prepare_order", Description: "sync the cart and return the real bill + a confirmation_id (does NOT place; auto-moves the cart when the address changed and rebuilds it if Swiggy expired it — see the rebuilt field)"}, s.handlePrepareOrder)
	addTool(srv, &mcp.Tool{Name: "place_order", Description: "place the order for a confirmation_id from prepare_order or im_prepare_order (real, charges COD; never call without user confirmation)"}, s.handlePlaceOrder)
	addTool(srv, &mcp.Tool{Name: "order_preset", Description: "load a saved preset (food or instamart) into the cart and return a bill + confirmation_id (does NOT place)"}, s.handleOrderPreset)
	addTool(srv, &mcp.Tool{Name: "sign_in", Description: "start Swiggy sign-in; returns a browser URL (opened automatically when possible)"}, s.handleSignIn)
	addTool(srv, &mcp.Tool{Name: "auth_status", Description: "whether the user is signed in — and, when signed in, the opening card snapshot (default/last address, favorites, taste, suggestions, policies) so no separate get_card is needed to start"}, s.handleAuthStatus)
	addTool(srv, &mcp.Tool{Name: "get_card", Description: "the user's saved personalization: default/last address, favorite restaurants, policies, per-item tastes, and pending suggestions"}, s.handleGetCard)
	addTool(srv, &mcp.Tool{Name: "remember", Description: "save an explicit preference — a per-restaurant-item taste, a cross-restaurant policy, or the default address; or confirm a suggestion"}, s.handleRemember)
	addTool(srv, &mcp.Tool{Name: "forget", Description: "remove a saved taste or policy"}, s.handleForget)
	addTool(srv, &mcp.Tool{Name: "save_preset", Description: "save the current cart as a named preset the user can reorder (pass vertical: \"instamart\" to save the instamart cart instead of food)"}, s.handleSavePreset)
	addTool(srv, &mcp.Tool{Name: "forget_preset", Description: "delete a saved preset"}, s.handleForgetPreset)
	addTool(srv, &mcp.Tool{Name: "get_previous_orders", Description: "List previously placed orders for an address (newest first) so the user can reorder."}, s.handleGetPreviousOrders)
	addTool(srv, &mcp.Tool{Name: "set_address", Description: "Persist the delivery address the user picked in the app; as_default also locks it as the sticky default."}, s.handleSetAddress)
	addTool(srv, &mcp.Tool{Name: "im_search_products", Description: "search instamart (grocery) products deliverable to an address; carts are keyed by spin_id, the variant/pack-size id"}, s.handleIMSearchProducts)
	addTool(srv, &mcp.Tool{Name: "im_get_cart", Description: "the current instamart cart with the authoritative bill"}, s.handleIMGetCart)
	addTool(srv, &mcp.Tool{Name: "im_update_cart", Description: "set the instamart cart lines (replaces the whole instamart cart)"}, s.handleIMUpdateCart)
	addTool(srv, &mcp.Tool{Name: "im_clear_cart", Description: "empty the instamart cart"}, s.handleIMClearCart)
	addTool(srv, &mcp.Tool{Name: "im_prepare_order", Description: "sync the instamart cart and return the real bill + a confirmation_id (does NOT place)"}, s.handleIMPrepareOrder)
	s.registerApp(srv)
}
