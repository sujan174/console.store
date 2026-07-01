package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteTOMLServerAppendsAndPreserves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	seed := "model = \"gpt\"\n\n[mcp_servers.other]\ncommand = \"x\"\n"
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := writeTOMLServer(path, "console", "/usr/local/bin/console", []string{"mcp"})
	if err != nil || !changed {
		t.Fatalf("write changed=%v err=%v", changed, err)
	}
	raw, _ := os.ReadFile(path)
	s := string(raw)
	if !strings.Contains(s, "model = \"gpt\"") || !strings.Contains(s, "[mcp_servers.other]") {
		t.Fatalf("existing content lost:\n%s", s)
	}
	if !strings.Contains(s, "[mcp_servers.console]") || !strings.Contains(s, `command = "/usr/local/bin/console"`) {
		t.Fatalf("console block missing:\n%s", s)
	}
	if !strings.Contains(s, `args = ["mcp"]`) {
		t.Fatalf("args missing:\n%s", s)
	}
}

func TestWriteTOMLServerIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	_, _ = writeTOMLServer(path, "console", "/c", []string{"mcp"})
	changed, err := writeTOMLServer(path, "console", "/c", []string{"mcp"})
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatalf("second identical write should be changed=false")
	}
}

func TestRemoveTOMLServer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	seed := "model = \"gpt\"\n"
	_ = os.WriteFile(path, []byte(seed), 0o600)
	_, _ = writeTOMLServer(path, "console", "/c", []string{"mcp"})
	changed, err := removeTOMLServer(path, "console")
	if err != nil || !changed {
		t.Fatalf("remove changed=%v err=%v", changed, err)
	}
	raw, _ := os.ReadFile(path)
	s := string(raw)
	if strings.Contains(s, "[mcp_servers.console]") {
		t.Fatalf("console block not removed:\n%s", s)
	}
	if !strings.Contains(s, "model = \"gpt\"") {
		t.Fatalf("unrelated content lost:\n%s", s)
	}
}
