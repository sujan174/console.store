// Package agents provisions consolestore into the user's local AI agents: it
// registers the `console mcp` server in each detected agent's config and copies
// the SKILL.md bundles into agents that support skills. It writes only config
// files and skill files — no network, no keyring — so it can run from the
// installer with no auth. It MUST NOT import tui/mcp/broker/auth/swiggy.
package agents

import (
	"os"
	"path/filepath"
	"runtime"
)

// ServerName is the key consolestore owns in each agent's MCP server map. We only
// ever read/write THIS key — every other server is preserved untouched.
const ServerName = "console"

type Kind int

const (
	KindJSON Kind = iota // Claude Desktop/Code, Cursor, Windsurf, OpenClaw, Zed, VS Code
	KindTOML             // Codex (~/.codex/config.toml)
	KindYAML             // Hermes (~/.hermes/config.yaml)
)

// Agent is one detected local agent and where its MCP config + skills live.
type Agent struct {
	Name       string // stable id: claude-desktop, claude-code, cursor, codex, …
	Title      string // human label for summaries
	Kind       Kind
	ConfigPath string // absolute path to the MCP config file
	SkillsDir  string // absolute skills dir, or "" when the agent has no skill support
	// JSONKey is the nested key path to the servers map for KindJSON agents
	// (defaults to ["mcpServers"] when empty). EntryType, when set, adds a
	// "type" field to each server entry (VS Code needs "type": "stdio").
	JSONKey   []string
	EntryType string
}

// jsonKey returns the agent's servers key path, defaulting to ["mcpServers"].
func (a Agent) jsonKey() []string {
	if len(a.JSONKey) == 0 {
		return []string{"mcpServers"}
	}
	return a.JSONKey
}

func home() string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	return ""
}

// claudeDesktopConfig returns the per-OS Claude Desktop config path.
func claudeDesktopConfig(h string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(h, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	case "windows":
		if ad := os.Getenv("APPDATA"); ad != "" {
			return filepath.Join(ad, "Claude", "claude_desktop_config.json")
		}
		return filepath.Join(h, "AppData", "Roaming", "Claude", "claude_desktop_config.json")
	default:
		return filepath.Join(h, ".config", "Claude", "claude_desktop_config.json")
	}
}

// zedConfig returns Zed's settings.json path (Zed uses ~/.config even on macOS).
func zedConfig(h string) string {
	if runtime.GOOS == "windows" {
		if ad := os.Getenv("APPDATA"); ad != "" {
			return filepath.Join(ad, "Zed", "settings.json")
		}
		return filepath.Join(h, "AppData", "Roaming", "Zed", "settings.json")
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "zed", "settings.json")
	}
	return filepath.Join(h, ".config", "zed", "settings.json")
}

// vscodeConfig returns VS Code's user-profile mcp.json path.
func vscodeConfig(h string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(h, "Library", "Application Support", "Code", "User", "mcp.json")
	case "windows":
		if ad := os.Getenv("APPDATA"); ad != "" {
			return filepath.Join(ad, "Code", "User", "mcp.json")
		}
		return filepath.Join(h, "AppData", "Roaming", "Code", "User", "mcp.json")
	default:
		base := os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			base = filepath.Join(h, ".config")
		}
		return filepath.Join(base, "Code", "User", "mcp.json")
	}
}

// candidates returns every agent we know how to provision, with its paths.
func candidates(h string) []Agent {
	claudeSkills := filepath.Join(h, ".claude", "skills")
	return []Agent{
		{Name: "claude-desktop", Title: "Claude Desktop", Kind: KindJSON, ConfigPath: claudeDesktopConfig(h), SkillsDir: claudeSkills},
		{Name: "claude-code", Title: "Claude Code", Kind: KindJSON, ConfigPath: filepath.Join(h, ".claude.json"), SkillsDir: claudeSkills},
		{Name: "cursor", Title: "Cursor", Kind: KindJSON, ConfigPath: filepath.Join(h, ".cursor", "mcp.json"), SkillsDir: ""},
		{Name: "codex", Title: "Codex", Kind: KindTOML, ConfigPath: filepath.Join(h, ".codex", "config.toml"), SkillsDir: filepath.Join(h, ".codex", "skills")},
		// MCP-only agents (no skill-bundle support): wire the console server, skip skills.
		{Name: "windsurf", Title: "Windsurf", Kind: KindJSON, ConfigPath: filepath.Join(h, ".codeium", "windsurf", "mcp_config.json")},
		{Name: "openclaw", Title: "OpenClaw", Kind: KindJSON, ConfigPath: filepath.Join(h, ".openclaw", "openclaw.json"), JSONKey: []string{"mcp", "servers"}},
		{Name: "zed", Title: "Zed", Kind: KindJSON, ConfigPath: zedConfig(h), JSONKey: []string{"context_servers"}},
		{Name: "vscode", Title: "VS Code", Kind: KindJSON, ConfigPath: vscodeConfig(h), JSONKey: []string{"servers"}, EntryType: "stdio"},
		{Name: "hermes", Title: "Hermes", Kind: KindYAML, ConfigPath: filepath.Join(h, ".hermes", "config.yaml")},
	}
}

// present reports whether an agent is installed. To avoid false positives from
// the home dir itself (e.g. ~/.claude.json's parent is ~), we require either the
// config file to exist, or a dedicated agent directory (never the home root).
func present(a Agent) bool {
	if _, err := os.Stat(a.ConfigPath); err == nil {
		return true
	}
	for _, dir := range agentDirs(a) {
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return true
		}
	}
	return false
}

// agentDirs are dedicated directories whose existence proves the agent is
// installed (excludes the home root).
func agentDirs(a Agent) []string {
	h := home()
	switch a.Name {
	case "claude-desktop":
		return []string{filepath.Dir(a.ConfigPath)} // .../Claude
	case "claude-code":
		return []string{filepath.Join(h, ".claude")}
	case "cursor":
		return []string{filepath.Join(h, ".cursor")}
	case "codex":
		return []string{filepath.Join(h, ".codex")}
	case "windsurf", "openclaw", "zed", "vscode", "hermes":
		// The parent of the config file is the agent's own dedicated dir
		// (…/windsurf, ~/.openclaw, …/zed, …/Code/User, ~/.hermes).
		return []string{filepath.Dir(a.ConfigPath)}
	}
	return nil
}

// Detect returns the installed agents on this machine.
func Detect() []Agent {
	h := home()
	if h == "" {
		return nil
	}
	var out []Agent
	for _, a := range candidates(h) {
		if present(a) {
			out = append(out, a)
		}
	}
	return out
}

// consoleBinary is the absolute path the agent config should launch for `mcp`.
func consoleBinary() string {
	if p, err := os.Executable(); err == nil && p != "" {
		if rp, rerr := filepath.EvalSymlinks(p); rerr == nil {
			return rp
		}
		return p
	}
	return "console"
}
