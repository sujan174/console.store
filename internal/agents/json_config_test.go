package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestWriteJSONServerPreservesOthers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "claude_desktop_config.json")
	seed := `{"mcpServers":{"other":{"command":"x"}},"theme":"dark"}`
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := writeJSONServer(path, "console", "/usr/local/bin/console", []string{"mcp"})
	if err != nil || !changed {
		t.Fatalf("writeJSONServer changed=%v err=%v", changed, err)
	}
	m := readJSON(t, path)
	if m["theme"] != "dark" {
		t.Fatalf("top-level key lost: %+v", m)
	}
	servers := m["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Fatalf("existing server lost: %+v", servers)
	}
	con := servers["console"].(map[string]any)
	if con["command"] != "/usr/local/bin/console" {
		t.Fatalf("console entry = %+v", con)
	}
}

func TestWriteJSONServerIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp.json")
	_, _ = writeJSONServer(path, "console", "/c", []string{"mcp"})
	changed, err := writeJSONServer(path, "console", "/c", []string{"mcp"})
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatalf("second identical write should report changed=false")
	}
}

func TestRemoveJSONServer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp.json")
	_, _ = writeJSONServer(path, "console", "/c", []string{"mcp"})
	changed, err := removeJSONServer(path, "console")
	if err != nil || !changed {
		t.Fatalf("remove changed=%v err=%v", changed, err)
	}
	m := readJSON(t, path)
	servers, _ := m["mcpServers"].(map[string]any)
	if _, ok := servers["console"]; ok {
		t.Fatalf("console not removed: %+v", servers)
	}
}

// A malformed existing config must error, never get silently overwritten.
func TestWriteJSONServerUnparseableRefusesToClobber(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	const corrupt = "{not valid json"
	if err := os.WriteFile(path, []byte(corrupt), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := writeJSONServer(path, "console", "/c", []string{"mcp"}); err == nil {
		t.Fatal("expected error on unparseable config, got nil")
	}
	got, _ := os.ReadFile(path)
	if string(got) != corrupt {
		t.Fatalf("unparseable config was overwritten: %q", got)
	}
}
