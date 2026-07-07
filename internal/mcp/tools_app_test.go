package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"consolestore/internal/broker/api"
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
