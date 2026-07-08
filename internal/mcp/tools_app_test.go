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
}

func TestOpenStoreRestaurantScreen(t *testing.T) {
	s := NewServer(&fakeBackend{}, &fakeAuth{token: true})
	_, out, err := s.handleOpenStore(context.Background(), nil, OpenStoreIn{RestaurantID: "r1", AddressID: "a1"})
	if err != nil || out.Screen != "restaurant" {
		t.Fatalf("restaurant screen=%q err=%v", out.Screen, err)
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
