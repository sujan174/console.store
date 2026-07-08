package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"consolestore/internal/broker/api"
	"consolestore/internal/localstore"
)

func TestOpenStoreToolDeclaresUI(t *testing.T) {
	tool := openStoreTool()
	ui, ok := tool.Meta["ui"].(map[string]any)
	if !ok || ui["resourceUri"] != appResourceURI {
		t.Fatalf("tool missing _meta.ui.resourceUri: %+v", tool.Meta)
	}
	if tool.Meta["ui/resourceUri"] != appResourceURI {
		t.Fatalf("tool missing legacy ui/resourceUri key")
	}
}

func TestAppResourceServesBundle(t *testing.T) {
	res, contents := appResource()
	if res.MIMEType != appResourceMIME {
		t.Fatalf("mime = %q, want %q", res.MIMEType, appResourceMIME)
	}
	out, err := contents(context.Background(), &mcp.ReadResourceRequest{})
	if err != nil || len(out.Contents) == 0 || !strings.Contains(out.Contents[0].Text, "<html") {
		t.Fatalf("resource did not serve bundle html: %v", err)
	}
}

func TestOpenStoreEchoesRestaurantName(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{
		addrs: []api.Address{{ID: "a1", Label: "Home", Full: "12 Main St"}},
		menu:  api.Menu{RestaurantID: "r1", Items: []api.MenuItem{{ID: "i1", Name: "Burger", Price: 200, InStock: true}}},
	}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleOpenStore(context.Background(), nil, OpenStoreIn{
		RestaurantID:   "r1",
		RestaurantName: "Burger King",
	})
	if err != nil {
		t.Fatalf("handleOpenStore: %v", err)
	}
	if out.Restaurant["id"] != "r1" {
		t.Fatalf("restaurant id = %q, want r1", out.Restaurant["id"])
	}
	if out.Restaurant["name"] != "Burger King" {
		t.Fatalf("restaurant name = %q, want Burger King", out.Restaurant["name"])
	}
}

func TestOpenStoreHomeScreen(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_ = localstore.SaveAddrPref(localstore.AddrPref{}.SetActive("a1", "Home"))
	s := NewServer(&fakeBackend{}, &fakeAuth{token: true})
	_, out, err := s.handleOpenStore(context.Background(), nil, OpenStoreIn{})
	if err != nil || out.Screen != "home" || len(out.Categories) == 0 || out.Address.ID != "a1" {
		t.Fatalf("home out=%+v err=%v", out, err)
	}
	// A bare open (no query) is a shell with nothing to fetch — not loading,
	// and no seeded restaurants (the widget never asked for a search).
	if out.Loading {
		t.Fatalf("bare home must not be loading: %+v", out)
	}
	if len(out.Restaurants) != 0 {
		t.Fatalf("bare home must not carry restaurants, got %d", len(out.Restaurants))
	}
}

func TestOpenStoreRestaurantScreen(t *testing.T) {
	s := NewServer(&fakeBackend{}, &fakeAuth{token: true})
	_, out, err := s.handleOpenStore(context.Background(), nil, OpenStoreIn{RestaurantID: "r1", AddressID: "a1"})
	if err != nil || out.Screen != "restaurant" {
		t.Fatalf("restaurant screen=%q err=%v", out.Screen, err)
	}
	// A restaurant open is a shell too — the widget fetches the menu itself,
	// so open_store must not seed one, but must flag loading=true.
	if out.Menu != nil {
		t.Fatalf("restaurant shell must not carry a menu, got %+v", out.Menu)
	}
	if !out.Loading {
		t.Fatalf("restaurant shell must set loading=true")
	}
}

// A fresh user (empty AddrPref) who never called set_address must still get a
// real address: open_store falls back to the account's first Swiggy address so
// be.Menu never receives an empty addressId.
func TestOpenStoreFallsBackToFirstAddress(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{addrs: []api.Address{{ID: "fb1", Label: "Home"}}}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleOpenStore(context.Background(), nil, OpenStoreIn{RestaurantID: "r1"})
	if err != nil {
		t.Fatalf("handleOpenStore: %v", err)
	}
	if out.Address.ID != "fb1" {
		t.Fatalf("address id = %q, want fb1 (fell back to first Swiggy address)", out.Address.ID)
	}
	if out.Entry["address_id"] != "fb1" {
		t.Fatalf("entry address_id = %q, want fb1 (empty addressId would go to be.Menu)", out.Entry["address_id"])
	}
}

// open_store deliberately does NOT reconcile a cached AddrPref against the
// live address list — that cost a Swiggy round trip on every single call
// just to guard the rare case of a since-deleted address. A stale cached id
// is trusted and passed straight through; a deleted address now surfaces as
// whatever error Menu/SearchOrganic give for an unknown addressId, instead
// of being silently caught upfront.
func TestOpenStoreRestaurantShellNoMenuFetch(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_ = localstore.SaveAddrPref(localstore.AddrPref{}.SetActive("a1", "Home"))
	be := &fakeBackend{menu: api.Menu{RestaurantID: "r1"}}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleOpenStore(context.Background(), nil, OpenStoreIn{RestaurantID: "r1", RestaurantName: "Truffles", Query: "burger"})
	if err != nil {
		t.Fatalf("handleOpenStore: %v", err)
	}
	if out.Screen != "restaurant" {
		t.Fatalf("screen = %q, want restaurant", out.Screen)
	}
	if out.Menu != nil {
		t.Fatalf("shell must not carry a menu, got %+v", out.Menu)
	}
	if !out.Loading {
		t.Fatalf("restaurant shell must set loading=true")
	}
	if be.menuCalls != 0 {
		t.Fatalf("open_store must NOT fetch the menu server-side; menuCalls=%d", be.menuCalls)
	}
	if out.Restaurant["id"] != "r1" || out.Restaurant["name"] != "Truffles" {
		t.Fatalf("restaurant meta = %+v", out.Restaurant)
	}
	if out.Entry["search"] != "burger" || out.Entry["address_id"] != "a1" {
		t.Fatalf("entry = %+v", out.Entry)
	}
}

// Level C: open_store{restaurant_name} with NO restaurant_id returns a
// name-only restaurant shell — the widget searches for the restaurant itself,
// picks the match, and loads its menu, all under the loader. open_store makes
// zero backend calls; the shell carries the name (no id) + the item query.
func TestOpenStoreNameShellResolvesInWidget(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_ = localstore.SaveAddrPref(localstore.AddrPref{}.SetActive("a1", "Home"))
	be := &fakeBackend{}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleOpenStore(context.Background(), nil, OpenStoreIn{RestaurantName: "Truffles", Query: "burger"})
	if err != nil {
		t.Fatalf("handleOpenStore: %v", err)
	}
	if out.Screen != "restaurant" {
		t.Fatalf("screen = %q, want restaurant", out.Screen)
	}
	if !out.Loading {
		t.Fatalf("name shell must set loading=true")
	}
	if out.Menu != nil {
		t.Fatalf("name shell must not carry a menu, got %+v", out.Menu)
	}
	if out.Restaurant["name"] != "Truffles" {
		t.Fatalf("restaurant name = %q, want Truffles", out.Restaurant["name"])
	}
	if out.Restaurant["id"] != "" {
		t.Fatalf("name shell must have an EMPTY id (widget resolves it); got %q", out.Restaurant["id"])
	}
	if out.Entry["search"] != "burger" || out.Entry["address_id"] != "a1" {
		t.Fatalf("entry = %+v", out.Entry)
	}
	if be.menuCalls != 0 || be.searchPageCalls != 0 {
		t.Fatalf("open_store must make zero backend calls; menuCalls=%d searchPageCalls=%d", be.menuCalls, be.searchPageCalls)
	}
}

func TestOpenStoreHomeQueryShellNoSearchFetch(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_ = localstore.SaveAddrPref(localstore.AddrPref{}.SetActive("a1", "Home"))
	be := &fakeBackend{search: []api.Restaurant{{ID: "r1", Name: "Oven Story"}}, searchMore: true, searchNext: 8}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleOpenStore(context.Background(), nil, OpenStoreIn{Query: "pizza"})
	if err != nil {
		t.Fatalf("handleOpenStore: %v", err)
	}
	if out.Screen != "home" {
		t.Fatalf("screen = %q, want home", out.Screen)
	}
	if len(out.Restaurants) != 0 {
		t.Fatalf("home shell must not carry restaurants, got %d", len(out.Restaurants))
	}
	if !out.Loading {
		t.Fatalf("home query shell must set loading=true")
	}
	if out.Query != "pizza" {
		t.Fatalf("query = %q", out.Query)
	}
	if be.searchPageCalls != 0 {
		t.Fatalf("open_store must NOT search server-side; searchPageCalls=%d", be.searchPageCalls)
	}
	if len(out.Categories) == 0 {
		t.Fatalf("home shell should still carry the cuisine categories")
	}
}

func TestOpenStoreBareHomeNotLoading(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_ = localstore.SaveAddrPref(localstore.AddrPref{}.SetActive("a1", "Home"))
	be := &fakeBackend{}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleOpenStore(context.Background(), nil, OpenStoreIn{})
	if err != nil {
		t.Fatalf("handleOpenStore: %v", err)
	}
	if out.Screen != "home" || out.Loading {
		t.Fatalf("bare open_store must be home, not loading: screen=%q loading=%v", out.Screen, out.Loading)
	}
	if be.searchPageCalls != 0 {
		t.Fatalf("bare open_store must not search; searchPageCalls=%d", be.searchPageCalls)
	}
}

func TestOpenStoreTrustsCachedAddressWithoutReconcile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_ = localstore.SaveAddrPref(localstore.AddrPref{}.SetActive("dead1", "Old Place"))
	be := &fakeBackend{addrs: []api.Address{{ID: "fresh1", Label: "New Home"}}}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleOpenStore(context.Background(), nil, OpenStoreIn{})
	if err != nil {
		t.Fatalf("handleOpenStore: %v", err)
	}
	if out.Address.ID != "dead1" {
		t.Fatalf("address id = %q, want dead1 (cached address trusted, no live reconcile)", out.Address.ID)
	}
}
