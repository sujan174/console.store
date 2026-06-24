package swiggy

import (
	"context"
	"encoding/json"
	"testing"
)

func TestCallToolDispatches(t *testing.T) {
	srv := newFakeMCP(t, map[string]toolFn{
		"echo": func(args map[string]any) (any, error) {
			return map[string]any{"said": args["msg"]}, nil
		},
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	raw, err := c.CallTool(context.Background(), "echo", map[string]any{"msg": "hi"})
	if err != nil {
		t.Fatal(err)
	}
	var got struct{ Said string }
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Said != "hi" {
		t.Fatalf("said=%q", got.Said)
	}
}

func TestCallToolSurfacesToolError(t *testing.T) {
	srv := newFakeMCP(t, map[string]toolFn{
		"boom": func(map[string]any) (any, error) { return nil, &MCPError{Code: 1, Message: "kaboom"} },
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	_, err := c.CallTool(context.Background(), "boom", nil)
	if err == nil {
		t.Fatal("expected tool error")
	}
}
