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
	RestaurantID   string `json:"restaurant_id,omitempty"`
	RestaurantName string `json:"restaurant_name,omitempty"`
	Category       string `json:"category,omitempty"`
	ItemID         string `json:"item_id,omitempty"`
	Query          string `json:"query,omitempty"`
	Vertical       string `json:"vertical,omitempty"`
}

// CategoryDTO is one dev-curated cuisine chip on the store home.
type CategoryDTO struct {
	Label string `json:"label"`
	Query string `json:"query"`
}

// OpenStoreOut seeds the app so it paints without a second round-trip. Screen
// discriminates the two shapes the app can open into: "home" (categories +
// optional search results + recent orders), "restaurant" (a menu), or
// "instamart" (the grocery vertical).
type OpenStoreOut struct {
	Screen   string `json:"screen"`             // "home" | "restaurant" | "instamart"
	Vertical string `json:"vertical,omitempty"` // "instamart" when Screen is the instamart shell (or carried through signed_out)
	// AuthorizeURL is set ONLY on a "signed_out" shell — the browser OAuth URL
	// the widget's Sign-in button opens (via app.openLink). Empty otherwise.
	AuthorizeURL string            `json:"authorize_url,omitempty"`
	Address      AddrRefDTO        `json:"address"`
	Restaurant   map[string]string `json:"restaurant,omitempty"`
	Entry        map[string]string `json:"entry,omitempty"`
	Menu         *GetMenuOut       `json:"menu,omitempty"`
	// Categories is ALWAYS the food cuisine chips; IMCategories is ALWAYS the
	// Instamart rail. Both ride on every home-class shell (home AND instamart)
	// so the widget's vertical tab can switch with full data either way — a
	// shell that seeded only its own vertical left the other one blank.
	Categories   []CategoryDTO            `json:"categories,omitempty"`
	IMCategories []CategoryDTO            `json:"im_categories,omitempty"`
	Restaurants  []RestaurantDTO          `json:"restaurants,omitempty"`
	RecentOrders []localstore.PlacedOrder `json:"recent_orders,omitempty"`
	Query        string                   `json:"query,omitempty"`
	// NextOffset/HasMore paginate Restaurants (query-seeded home only — a
	// bare open_store{} has no results to page through yet). The app calls
	// search_restaurants{offset: NextOffset} for "load more"; HasMore false
	// means don't bother.
	NextOffset int  `json:"next_offset,omitempty"`
	HasMore    bool `json:"has_more,omitempty"`
	// Loading marks a shell result the widget resolves itself (fetches the
	// menu / runs the search under its loading animation). Set on a restaurant
	// open and on a home open that carries a query; absent on a bare home.
	Loading bool `json:"loading,omitempty"`
}

func openStoreTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "open_store",
		Description: "Open the interactive ordering app — render it ONCE per turn. The app resolves " +
			"and loads everything itself under a loading animation, so this is normally the ONLY " +
			"call you make for an ordering intent (no pre-search needed). Shapes: nothing → the " +
			"store home; query alone → home search for a cuisine/dish (e.g. \"pizza\"); " +
			"restaurant_name (a specific restaurant the user named, e.g. \"Truffles\") → the app " +
			"searches for it, opens its menu, and prefills the in-menu search with `query` if given " +
			"(the item/dish); restaurant_id → open that restaurant's menu directly when you already " +
			"hold the id (e.g. a reorder). Prefer restaurant_name over resolving the id yourself. " +
			"Never call this twice in one turn. For GROCERIES (Instamart — milk, snacks, " +
			"drinks, essentials) pass vertical:\"instamart\" with an optional product query; " +
			"the same app opens on the grocery vertical.",
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

// resolveAddress picks the delivery address for a tool call, in precedence
// order: an explicit id the caller passed → the locally-preferred active
// address (AddrPref) → the account's first saved address.
//
// This is the seam that lets EVERY resolution tool (open_store,
// search_restaurants, get_menu, get_previous_orders) accept an OPTIONAL
// address_id and fill it itself — so the agent never has to spend an
// initialize/list_addresses round trip just to obtain an id to hand back.
// For a returning user (AddrPref set) it is a pure local read, zero network;
// only a fresh user who has never run set_address pays the one Addresses()
// call — the same cost list_addresses would have, minus the extra agent hop.
//
// No live reconcile against the address list on the happy path: catching a
// since-deleted cached id would cost a Swiggy round trip on every call to
// guard a rare edge case. Trust the cached address; a dead id surfaces as
// whatever error the downstream Menu/Search call returns.
func (s *Server) resolveAddress(explicit string) (id, label string) {
	if explicit != "" {
		return explicit, ""
	}
	pref, _ := localstore.LoadAddrPref()
	if id, label = pref.Active(); id != "" {
		return id, label
	}
	if list, err := s.be.Addresses(); err == nil && len(list) > 0 {
		return list[0].ID, list[0].Label
	}
	return "", ""
}

// requireAddress resolves the active address for a resolution tool, returning a
// typed no_address error when none can be resolved. Forwarding an empty
// addressId to Swiggy instead hard-fails ("addressId is required") and makes
// agents retry in a loop — the behavior that burned through the rate limit
// (live 2026-07-09). A clean, actionable error tells the agent to pick an
// address (list_addresses + set_address, or open_store) first.
func (s *Server) requireAddress(explicit string) (id, label string, err error) {
	id, label = s.resolveAddress(explicit)
	if id == "" {
		return "", "", codedErr(codeNoAddress, "no delivery address is set — call list_addresses then set_address (or open_store) to choose one before searching")
	}
	return id, label, nil
}

func (s *Server) handleOpenStore(ctx context.Context, _ *mcp.CallToolRequest, in OpenStoreIn) (*mcp.CallToolResult, OpenStoreOut, error) {
	// Signed out: open the app on a Sign-in screen instead of erroring (which
	// left the widget unopened and forced the agent to hand the user a link).
	// auth.Start begins/resumes the loopback OAuth flow, brings up the callback
	// listener, and returns the browser authorize URL. The widget renders a
	// Sign-in button wired to it, polls auth_status until signed in, then
	// resumes the carried intent (restaurant/query below).
	if s.auth == nil {
		return nil, OpenStoreOut{}, errAuthUnavailable
	}
	if !s.auth.TokenPresent(ctx) {
		url, _, err := s.auth.Start(ctx)
		if err != nil {
			return nil, OpenStoreOut{}, err
		}
		rest := map[string]string{}
		if in.RestaurantID != "" {
			rest["id"] = in.RestaurantID
		}
		if in.RestaurantName != "" {
			rest["name"] = in.RestaurantName
		}
		out := OpenStoreOut{
			Screen:       "signed_out",
			Vertical:     in.Vertical,
			AuthorizeURL: url,
			Entry:        map[string]string{"item_id": in.ItemID, "search": in.Query, "category": in.Category, "vertical": in.Vertical},
			Query:        in.Query,
			Categories:   foodCategoryDTOs(),
			IMCategories: imCategoryDTOs(),
		}
		if len(rest) > 0 {
			out.Restaurant = rest
		}
		return nil, out, nil
	}
	addr, label := s.resolveAddress(in.AddressID)

	if in.Vertical == "instamart" {
		// Instamart shell: the widget loads products itself (query if given,
		// else the first curated category) under its loader — same instant-open
		// pattern as the restaurant shell. Loading is therefore always true.
		// Food categories + recent orders ride along so the in-app vertical
		// tab can switch back to a fully-working food home (both local/cheap).
		recent, _ := localstore.LoadOrders(addr)
		return nil, OpenStoreOut{
			Screen:       "instamart",
			Vertical:     "instamart",
			Address:      AddrRefDTO{ID: addr, Label: label},
			Categories:   foodCategoryDTOs(),
			IMCategories: imCategoryDTOs(),
			RecentOrders: recent,
			Query:        in.Query,
			Loading:      true,
		}, nil
	}

	if in.RestaurantID != "" {
		// Instant-open: return a shell with NO menu — the widget fetches the
		// menu itself under its loading animation (one get_menu, the same read
		// this used to do server-side). resolveAddress stays local/cheap.
		return nil, OpenStoreOut{
			Screen:     "restaurant",
			Address:    AddrRefDTO{ID: addr, Label: label},
			Restaurant: map[string]string{"id": in.RestaurantID, "name": in.RestaurantName},
			// "search" prefills the restaurant's in-menu search box. This is the
			// `query` overload: WITH a restaurant_id, `query` means "search
			// inside this restaurant" — used for the ambiguous-item case (open
			// the store already showing the matches so the user picks). WITHOUT a
			// restaurant_id, `query` is the home search instead (below).
			Entry:   map[string]string{"category": in.Category, "item_id": in.ItemID, "address_id": addr, "search": in.Query},
			Loading: true,
		}, nil
	}

	cats := foodCategoryDTOs()

	if in.RestaurantName != "" {
		// Level C — a named restaurant with NO id yet: return a name-only
		// restaurant shell. The widget searches for the restaurant itself
		// (search_restaurants), picks the match, and loads its menu, all under
		// the loader — so the agent opens the app in ONE call, no pre-search.
		// `search` carries the item/dish query to prefill the in-menu search on
		// the confident match. Categories ride along so that if the widget
		// falls to the disambiguation chooser (home screen), its rail is intact.
		return nil, OpenStoreOut{
			Screen:     "restaurant",
			Address:    AddrRefDTO{ID: addr, Label: label},
			Restaurant: map[string]string{"name": in.RestaurantName}, // no id — widget resolves it
			Entry:      map[string]string{"address_id": addr, "search": in.Query},
			Categories: cats,
			Loading:    true,
		}, nil
	}
	recent, _ := localstore.LoadOrders(addr)
	// Instant-open home: a query returns a loading shell (widget runs the
	// search itself); a bare open is the plain home with no fetch.
	return nil, OpenStoreOut{
		Screen:       "home",
		Address:      AddrRefDTO{ID: addr, Label: label},
		Categories:   cats,
		IMCategories: imCategoryDTOs(),
		RecentOrders: recent,
		Query:        in.Query,
		Loading:      in.Query != "",
	}, nil
}

// foodCategoryDTOs / imCategoryDTOs are each vertical's curated rail chips.
// Both ride on every home-class shell so the widget's vertical tab always has
// full data for the side it switches TO.
func foodCategoryDTOs() []CategoryDTO {
	out := make([]CategoryDTO, 0)
	for _, c := range config.DefaultCategories() {
		out = append(out, CategoryDTO{Label: c.Label, Query: c.Query})
	}
	return out
}

func imCategoryDTOs() []CategoryDTO {
	out := make([]CategoryDTO, 0)
	for _, c := range config.DefaultIMCategories() {
		out = append(out, CategoryDTO{Label: c.Label, Query: c.Query})
	}
	return out
}
