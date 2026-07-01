package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- JSON key-path writer (OpenClaw / Zed / VS Code) ----

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
	p := filepath.Join(dir, "openclaw.json")
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

func TestJSONVSCodeTypedEntry(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "mcp.json")
	changed, err := writeJSONServerAt(p, []string{"servers"}, "console", serverEntryTyped("/bin/console", []string{"mcp"}, "stdio"))
	if err != nil || !changed {
		t.Fatalf("write: changed=%v err=%v", changed, err)
	}
	entry := readJSONObj(t, p)["servers"].(map[string]any)["console"].(map[string]any)
	if entry["type"] != "stdio" {
		t.Errorf("VS Code entry type = %v, want stdio", entry["type"])
	}
	if entry["command"] != "/bin/console" {
		t.Errorf("command = %v", entry["command"])
	}
}

// ---- Hermes YAML writer ----

func TestHermesEmptyFileCreatesBlock(t *testing.T) {
	got := upsertHermesServer("", "console", "/bin/console", []string{"mcp"})
	want := "mcp_servers:\n  console:\n    command: \"/bin/console\"\n    args: [\"mcp\"]\n"
	if got != want {
		t.Fatalf("empty create:\n got %q\nwant %q", got, want)
	}
}

func TestHermesAppendsWhenNoMcpServersKey(t *testing.T) {
	in := "model: hermes-4\napi_key: xxx\n"
	got := upsertHermesServer(in, "console", "/bin/console", []string{"mcp"})
	if !strings.HasPrefix(got, "model: hermes-4\napi_key: xxx\n") {
		t.Errorf("existing top-level keys not preserved:\n%s", got)
	}
	if !strings.Contains(got, "mcp_servers:\n  console:\n") {
		t.Errorf("mcp_servers block not appended:\n%s", got)
	}
}

func TestHermesInsertsAlongsideSibling(t *testing.T) {
	in := "mcp_servers:\n  github:\n    command: \"npx\"\n    args: [\"-y\", \"srv\"]\n"
	got := upsertHermesServer(in, "console", "/bin/console", []string{"mcp"})
	if !strings.Contains(got, "  github:") {
		t.Errorf("sibling github dropped:\n%s", got)
	}
	if !strings.Contains(got, "  console:\n    command: \"/bin/console\"\n    args: [\"mcp\"]") {
		t.Errorf("console not inserted at 2-space indent:\n%s", got)
	}
}

func TestHermesReplacesExistingAndIsIdempotent(t *testing.T) {
	in := "mcp_servers:\n  console:\n    command: \"/old/console\"\n    args: [\"mcp\"]\n  other:\n    command: \"x\"\n    args: []\n"
	got := upsertHermesServer(in, "console", "/new/console", []string{"mcp"})
	if strings.Contains(got, "/old/console") {
		t.Errorf("old console entry not replaced:\n%s", got)
	}
	if !strings.Contains(got, "command: \"/new/console\"") {
		t.Errorf("new console entry missing:\n%s", got)
	}
	if !strings.Contains(got, "  other:") {
		t.Errorf("sibling 'other' dropped:\n%s", got)
	}
	// Idempotent: re-running yields the same content.
	again := upsertHermesServer(got, "console", "/new/console", []string{"mcp"})
	if again != got {
		t.Errorf("not idempotent:\n first %q\nsecond %q", got, again)
	}
}

func TestHermesHonorsFourSpaceIndent(t *testing.T) {
	in := "mcp_servers:\n    github:\n        command: \"npx\"\n        args: []\n"
	got := upsertHermesServer(in, "console", "/bin/console", []string{"mcp"})
	if !strings.Contains(got, "    console:\n        command: \"/bin/console\"") {
		t.Errorf("console not aligned to existing 4-space indent:\n%s", got)
	}
}

func TestHermesStripLeavesSiblings(t *testing.T) {
	in := "mcp_servers:\n  console:\n    command: \"/bin/console\"\n    args: [\"mcp\"]\n  github:\n    command: \"npx\"\n    args: []\n"
	got := stripHermesServer(in, "console")
	if strings.Contains(got, "console:") {
		t.Errorf("console not stripped:\n%s", got)
	}
	if !strings.Contains(got, "  github:") {
		t.Errorf("sibling github dropped on strip:\n%s", got)
	}
}

// TestWireMCPPerNewAgent exercises the full wireMCP composition (key path,
// EntryType, YAML dispatch) for each newly added agent.
func TestWireMCPPerNewAgent(t *testing.T) {
	dir := t.TempDir()
	bin := "/usr/local/bin/console"
	cases := []struct {
		a     Agent
		file  string
		check func(t *testing.T, path string)
	}{
		{
			a: Agent{Name: "windsurf", Kind: KindJSON, ConfigPath: filepath.Join(dir, "windsurf.json")},
			check: func(t *testing.T, p string) {
				e := readJSONObj(t, p)["mcpServers"].(map[string]any)["console"].(map[string]any)
				if e["command"] != bin {
					t.Errorf("windsurf command=%v", e["command"])
				}
			},
		},
		{
			a: Agent{Name: "openclaw", Kind: KindJSON, ConfigPath: filepath.Join(dir, "openclaw.json"), JSONKey: []string{"mcp", "servers"}},
			check: func(t *testing.T, p string) {
				if readJSONObj(t, p)["mcp"].(map[string]any)["servers"].(map[string]any)["console"] == nil {
					t.Error("openclaw console missing under mcp.servers")
				}
			},
		},
		{
			a: Agent{Name: "zed", Kind: KindJSON, ConfigPath: filepath.Join(dir, "zed.json"), JSONKey: []string{"context_servers"}},
			check: func(t *testing.T, p string) {
				if readJSONObj(t, p)["context_servers"].(map[string]any)["console"] == nil {
					t.Error("zed console missing under context_servers")
				}
			},
		},
		{
			a: Agent{Name: "vscode", Kind: KindJSON, ConfigPath: filepath.Join(dir, "vscode.json"), JSONKey: []string{"servers"}, EntryType: "stdio"},
			check: func(t *testing.T, p string) {
				e := readJSONObj(t, p)["servers"].(map[string]any)["console"].(map[string]any)
				if e["type"] != "stdio" {
					t.Errorf("vscode type=%v want stdio", e["type"])
				}
			},
		},
		{
			a: Agent{Name: "hermes", Kind: KindYAML, ConfigPath: filepath.Join(dir, "hermes.yaml")},
			check: func(t *testing.T, p string) {
				raw, _ := os.ReadFile(p)
				if !strings.Contains(string(raw), "mcp_servers:") || !strings.Contains(string(raw), "console:") {
					t.Errorf("hermes yaml missing block:\n%s", raw)
				}
			},
		},
	}
	for _, c := range cases {
		t.Run(c.a.Name, func(t *testing.T) {
			changed, err := wireMCP(c.a, bin)
			if err != nil || !changed {
				t.Fatalf("wireMCP: changed=%v err=%v", changed, err)
			}
			c.check(t, c.a.ConfigPath)
			// Idempotent.
			if changed, err := wireMCP(c.a, bin); err != nil || changed {
				t.Fatalf("second wireMCP not idempotent: changed=%v err=%v", changed, err)
			}
			// unwire removes it.
			if changed, err := unwireMCP(c.a); err != nil || !changed {
				t.Fatalf("unwireMCP: changed=%v err=%v", changed, err)
			}
		})
	}
}
