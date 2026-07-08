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

// A saved AddrPref pointing at an address id the account no longer has (the
// user deleted it in the Swiggy app) must not resolve to a dead id: open_store
// reconciles against the live address list and falls back to list[0], clearing
// the stale pref so it doesn't keep resolving to nothing on every call.
func TestOpenStoreReconcilesStaleAddress(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_ = localstore.SaveAddrPref(localstore.AddrPref{}.SetActive("dead1", "Old Place"))
	be := &fakeBackend{addrs: []api.Address{{ID: "fresh1", Label: "New Home"}}}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleOpenStore(context.Background(), nil, OpenStoreIn{})
	if err != nil {
		t.Fatalf("handleOpenStore: %v", err)
	}
	if out.Address.ID != "fresh1" {
		t.Fatalf("address id = %q, want fresh1 (stale pref reconciled to first live address)", out.Address.ID)
	}
	ap, err := localstore.LoadAddrPref()
	if err != nil {
		t.Fatalf("LoadAddrPref: %v", err)
	}
	if ap.LastAddrID == "dead1" {
		t.Fatalf("stale address id dead1 still recorded in addrpref: %+v", ap)
	}
}
