package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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
