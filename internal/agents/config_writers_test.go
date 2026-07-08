package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ---- generic JSON key-path writer (used by the Claude agents under
// "mcpServers"; exercised here with a nested path to prove the merge/preserve
// logic is independent of the key depth) ----

func readJSONObj(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return m
}

func TestJSONNestedKeyPathPreservesSiblings(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.json")
	// Pre-existing config with an unrelated server + top-level key.
	seed := `{"gateway":{"port":8080},"mcp":{"servers":{"time":{"command":"uvx","args":["mcp-server-time"]}}}}`
	if err := os.WriteFile(p, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := writeJSONServerAt(p, []string{"mcp", "servers"}, "console", serverEntry("/bin/console", []string{"mcp"}))
	if err != nil || !changed {
		t.Fatalf("write: changed=%v err=%v", changed, err)
	}
	m := readJSONObj(t, p)
	if m["gateway"] == nil {
		t.Error("top-level gateway key was dropped")
	}
	servers := m["mcp"].(map[string]any)["servers"].(map[string]any)
	if servers["time"] == nil {
		t.Error("sibling server 'time' was dropped")
	}
	if servers["console"] == nil {
		t.Fatal("console entry not written")
	}
	// Idempotent second run.
	changed, err = writeJSONServerAt(p, []string{"mcp", "servers"}, "console", serverEntry("/bin/console", []string{"mcp"}))
	if err != nil || changed {
		t.Fatalf("second write should be no-op: changed=%v err=%v", changed, err)
	}
	// Remove.
	changed, err = removeJSONServerAt(p, []string{"mcp", "servers"}, "console")
	if err != nil || !changed {
		t.Fatalf("remove: changed=%v err=%v", changed, err)
	}
	servers = readJSONObj(t, p)["mcp"].(map[string]any)["servers"].(map[string]any)
	if servers["console"] != nil {
		t.Error("console not removed")
	}
	if servers["time"] == nil {
		t.Error("remove dropped sibling 'time'")
	}
}
