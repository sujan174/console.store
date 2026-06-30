package agents

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallWiresDetectedAgents(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	// Make Codex + Claude Code "present".
	_ = os.MkdirAll(filepath.Join(home, ".codex"), 0o755)
	_ = os.WriteFile(filepath.Join(home, ".claude.json"), []byte("{}"), 0o600)

	var buf bytes.Buffer
	if err := Install(&buf); err != nil {
		t.Fatalf("Install: %v", err)
	}
	// Claude Code JSON now has our server.
	raw, _ := os.ReadFile(filepath.Join(home, ".claude.json"))
	if !strings.Contains(string(raw), `"console"`) || !strings.Contains(string(raw), `"mcp"`) {
		t.Fatalf("claude.json not wired:\n%s", raw)
	}
	// Codex TOML now has our table.
	raw, _ = os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if !strings.Contains(string(raw), "[mcp_servers.console]") {
		t.Fatalf("codex config not wired:\n%s", raw)
	}
	// Claude skill installed.
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "console-order", "SKILL.md")); err != nil {
		t.Fatalf("claude skill not installed: %v", err)
	}
	if !strings.Contains(buf.String(), "Claude Code") {
		t.Fatalf("summary missing agent: %s", buf.String())
	}
}

func TestInstallRespectsOptOut(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CONSOLE_NO_AGENT_SETUP", "1")
	_ = os.WriteFile(filepath.Join(home, ".claude.json"), []byte("{}"), 0o600)

	var buf bytes.Buffer
	if err := Install(&buf); err != nil {
		t.Fatalf("Install: %v", err)
	}
	raw, _ := os.ReadFile(filepath.Join(home, ".claude.json"))
	if strings.Contains(string(raw), "console") {
		t.Fatalf("opt-out should not wire anything:\n%s", raw)
	}
}
