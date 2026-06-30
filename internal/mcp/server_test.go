package mcp

import (
	"context"
	"testing"
)

func TestServerInfoReportsVersion(t *testing.T) {
	s := NewServer(nil, nil)
	res, out, err := s.handleServerInfo(context.Background(), nil, ServerInfoIn{})
	if err != nil {
		t.Fatalf("handleServerInfo: %v", err)
	}
	if res != nil && res.IsError {
		t.Fatalf("unexpected error result")
	}
	if out.Name != "consolestore" {
		t.Fatalf("Name = %q, want consolestore", out.Name)
	}
}
