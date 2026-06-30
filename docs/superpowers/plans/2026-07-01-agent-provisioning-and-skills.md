# Agent Provisioning + Skills Implementation Plan (Plan 2 of 2)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** After install, auto-wire the `console mcp` server into every detected local agent (Claude Desktop, Claude Code, Cursor, Codex) and drop two `SKILL.md` bundles into the agents that support skills — idempotently, reversibly, and without touching the network or auth.

**Architecture:** A new `internal/agents` package detects installed agents by their config-file locations, merges a `console` MCP server entry into each (JSON for Claude/Cursor, TOML for Codex — both hand-rolled to avoid a second dependency), and copies two embedded `SKILL.md` bundles into each agent's skills directory. A new `console agents install|list|remove` subcommand is **early-dispatched in `main()`** (like `help`) so it runs with no OAuth/keyring/network. The curl/irm installer calls `console agents install --quiet` as its last step.

**Tech Stack:** Go 1.26, stdlib only (`encoding/json`, `embed`, `os`, `path/filepath`, `runtime`, `strings`) — **no new dependency** in this plan. Shell (`install.sh`) + PowerShell (`install.ps1`).

## Global Constraints

- Go 1.26; **stdlib only in this plan** (no TOML library — hand-roll Codex's append/strip). Run `go vet ./...` + `gofmt -w` on every changed file.
- `internal/agents` must NOT import `tui`, `mcp`, `broker`, `auth`, or `swiggy` — it only writes config files + copies embedded skill files. No network, no keyring.
- Provisioning must be **idempotent** (safe to re-run), **never clobber** other servers/keys in a config, and **reversible** (`console agents remove`).
- Opt-out: if `CONSOLE_NO_AGENT_SETUP=1`, `console agents install` is a no-op that prints one line and exits 0.
- The MCP server entry invokes the **absolute path of the running binary** (`os.Executable()`) with arg `mcp`.
- v1 skill targets: Claude Code, Claude Desktop, Codex get `SKILL.md`; Cursor is MCP-only.
- Tests isolate the filesystem with `t.Setenv("HOME", t.TempDir())` (and on the Claude-Desktop macOS path, also rely on `HOME`); never read or write the developer's real agent configs.

---

### Task 1: `internal/agents` package — detection + agent registry

**Files:**
- Create: `internal/agents/agents.go`
- Create: `internal/agents/agents_test.go`

**Interfaces:**
- Produces: `type Agent struct{ Name, Title string; Kind Kind; ConfigPath, SkillsDir string }`; `type Kind int` (`KindJSON`, `KindTOML`); `Detect() []Agent`; `ServerName = "console"`; `consoleBinary() string`.
- Consumes: stdlib only.

- [ ] **Step 1: Write the failing test** — `internal/agents/agents_test.go`

```go
package agents

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectFindsClaudeCodeByConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // windows homedir fallback
	// Claude Code is detected by ~/.claude.json presence.
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	found := Detect()
	var got *Agent
	for i := range found {
		if found[i].Name == "claude-code" {
			got = &found[i]
		}
	}
	if got == nil {
		t.Fatalf("claude-code not detected; found %+v", found)
	}
	if got.Kind != KindJSON || got.SkillsDir == "" {
		t.Fatalf("claude-code agent = %+v", *got)
	}
}

func TestDetectIgnoresAbsentAgents(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if len(Detect()) != 0 {
		t.Fatalf("expected no agents in empty home, got %+v", Detect())
	}
}
```

- [ ] **Step 2: Run it — must fail**

Run: `go test ./internal/agents/ -run TestDetect`
Expected: FAIL (undefined).

- [ ] **Step 3: Write the implementation** — `internal/agents/agents.go`

```go
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
	KindJSON Kind = iota // Claude Desktop, Claude Code, Cursor
	KindTOML             // Codex (~/.codex/config.toml)
)

// Agent is one detected local agent and where its MCP config + skills live.
type Agent struct {
	Name       string // stable id: claude-desktop, claude-code, cursor, codex
	Title      string // human label for summaries
	Kind       Kind
	ConfigPath string // absolute path to the MCP config file
	SkillsDir  string // absolute skills dir, or "" when the agent has no skill support
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

// candidates returns every agent we know how to provision, with its paths.
func candidates(h string) []Agent {
	claudeSkills := filepath.Join(h, ".claude", "skills")
	return []Agent{
		{Name: "claude-desktop", Title: "Claude Desktop", Kind: KindJSON, ConfigPath: claudeDesktopConfig(h), SkillsDir: claudeSkills},
		{Name: "claude-code", Title: "Claude Code", Kind: KindJSON, ConfigPath: filepath.Join(h, ".claude.json"), SkillsDir: claudeSkills},
		{Name: "cursor", Title: "Cursor", Kind: KindJSON, ConfigPath: filepath.Join(h, ".cursor", "mcp.json"), SkillsDir: ""},
		{Name: "codex", Title: "Codex", Kind: KindTOML, ConfigPath: filepath.Join(h, ".codex", "config.toml"), SkillsDir: filepath.Join(h, ".codex", "skills")},
	}
}

// present reports whether an agent is installed: its config file exists, OR the
// directory that would hold its config exists (the agent is installed but has no
// MCP config yet — we should still wire it).
func present(a Agent) bool {
	if _, err := os.Stat(a.ConfigPath); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Dir(a.ConfigPath)); err == nil {
		return true
	}
	return false
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
```

Note on `claude-desktop` detection: on macOS `~/Library/Application Support/Claude` only exists once Desktop has run; `TestDetectIgnoresAbsentAgents` relies on a clean temp `HOME` so neither the Claude dir nor `.codex`/`.cursor` exist. `present` checks `filepath.Dir(ConfigPath)` — for claude-code that's `home` itself (always exists), so claude-code is detected ONLY via the `.claude.json` file branch. Confirm by re-reading: for `claude-code`, `ConfigPath = ~/.claude.json`, `Dir = ~` which always exists → that would wrongly detect claude-code in an empty home. **Fix:** claude-code must be detected by file OR by `~/.claude/` dir, not by `~`. Adjust `present` to special-case: when `ConfigPath`'s parent is `home`, require the file itself (or a sibling marker dir). Implement this precisely in Step 4.

- [ ] **Step 4: Refine `present` so a bare home dir doesn't false-positive** — replace `present` in `agents.go`:

```go
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
	}
	return nil
}
```

- [ ] **Step 5: Run the tests — must pass**

Run: `go test ./internal/agents/ -run TestDetect`
Expected: PASS (claude-code detected via `.claude.json`; empty home → none).

- [ ] **Step 6: Commit**

```bash
gofmt -w internal/agents/
git add internal/agents/
git commit -m "feat(agents): detect installed local agents + config/skills registry"
```

---

### Task 2: JSON MCP config writer (Claude Desktop/Code, Cursor)

**Files:**
- Create: `internal/agents/json_config.go`
- Create: `internal/agents/json_config_test.go`

**Interfaces:**
- Produces: `writeJSONServer(path, name, command string, args []string) (changed bool, err error)`; `removeJSONServer(path, name string) (changed bool, err error)`.
- Consumes: stdlib `encoding/json`.

- [ ] **Step 1: Write the failing test** — `internal/agents/json_config_test.go`

```go
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
```

- [ ] **Step 2: Run it — must fail**

Run: `go test ./internal/agents/ -run 'TestWriteJSON|TestRemoveJSON'`
Expected: FAIL.

- [ ] **Step 3: Write the implementation** — `internal/agents/json_config.go`

```go
package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
)

// serverEntry is the MCP server shape Claude/Cursor expect.
func serverEntry(command string, args []string) map[string]any {
	return map[string]any{"command": command, "args": toAnySlice(args)}
}

func toAnySlice(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

// loadJSONObject reads path into a generic map. A missing file yields an empty
// object; a present-but-unparseable file is an error (we refuse to clobber it).
func loadJSONObject(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

func saveJSONObject(path string, m map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

// writeJSONServer merges our server entry under "mcpServers"[name], preserving
// every other key. Returns changed=false when the file already has the exact
// entry (idempotent).
func writeJSONServer(path, name, command string, args []string) (bool, error) {
	m, err := loadJSONObject(path)
	if err != nil {
		return false, err
	}
	servers, _ := m["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	entry := serverEntry(command, args)
	if existing, ok := servers[name]; ok && reflect.DeepEqual(existing, any(entry)) {
		return false, nil
	}
	servers[name] = entry
	m["mcpServers"] = servers
	return true, saveJSONObject(path, m)
}

// removeJSONServer deletes "mcpServers"[name] if present.
func removeJSONServer(path, name string) (bool, error) {
	m, err := loadJSONObject(path)
	if err != nil {
		return false, err
	}
	servers, _ := m["mcpServers"].(map[string]any)
	if servers == nil {
		return false, nil
	}
	if _, ok := servers[name]; !ok {
		return false, nil
	}
	delete(servers, name)
	m["mcpServers"] = servers
	return true, saveJSONObject(path, m)
}
```

Note on the idempotency test: `serverEntry` builds `args` as `[]any{"mcp"}`, and JSON round-trips arrays to `[]any`, so `reflect.DeepEqual` matches on the second call. Verify `TestWriteJSONServerIdempotent` passes; if the first write's in-memory `[]any` vs the reloaded `[]any` differ in element type, the comparison still holds because both are `[]any{string}`.

- [ ] **Step 4: Run the tests — must pass**

Run: `go test ./internal/agents/ -run 'TestWriteJSON|TestRemoveJSON'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/agents/
git add internal/agents/
git commit -m "feat(agents): idempotent JSON MCP config writer (Claude/Cursor)"
```

---

### Task 3: TOML MCP config writer (Codex)

**Files:**
- Create: `internal/agents/toml_config.go`
- Create: `internal/agents/toml_config_test.go`

**Interfaces:**
- Produces: `writeTOMLServer(path, name, command string, args []string) (bool, error)`; `removeTOMLServer(path, name string) (bool, error)`.
- Consumes: stdlib `strings`, `os`.

- [ ] **Step 1: Write the failing test** — `internal/agents/toml_config_test.go`

```go
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
```

- [ ] **Step 2: Run it — must fail**

Run: `go test ./internal/agents/ -run 'TestWriteTOML|TestRemoveTOML'`
Expected: FAIL.

- [ ] **Step 3: Write the implementation** — `internal/agents/toml_config.go`

We hand-roll a minimal merge: Codex's `mcp_servers` are TOML tables keyed `[mcp_servers.<name>]`. We append our table if absent and strip it on remove. We never rewrite the rest of the file, so other tables/keys are preserved byte-for-byte.

```go
package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func tomlHeader(name string) string { return "[mcp_servers." + name + "]" }

// tomlBlock renders our server table. args is rendered as a TOML string array.
func tomlBlock(name, command string, args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = fmt.Sprintf("%q", a)
	}
	var b strings.Builder
	b.WriteString(tomlHeader(name) + "\n")
	b.WriteString(fmt.Sprintf("command = %q\n", command))
	b.WriteString("args = [" + strings.Join(quoted, ", ") + "]\n")
	return b.String()
}

func readFileOrEmpty(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// writeTOMLServer appends our table if the file lacks it. Idempotent: if the
// exact block is already present, returns changed=false.
func writeTOMLServer(path, name, command string, args []string) (bool, error) {
	cur, err := readFileOrEmpty(path)
	if err != nil {
		return false, err
	}
	block := tomlBlock(name, command, args)
	if strings.Contains(cur, strings.TrimRight(block, "\n")) {
		return false, nil
	}
	// If an older/different console block exists, strip it first so we don't dup.
	if strings.Contains(cur, tomlHeader(name)) {
		cur = stripTOMLTable(cur, name)
	}
	if cur != "" && !strings.HasSuffix(cur, "\n") {
		cur += "\n"
	}
	if cur != "" {
		cur += "\n"
	}
	cur += block
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, []byte(cur), 0o644)
}

// removeTOMLServer strips our table. Returns changed=false when absent.
func removeTOMLServer(path, name string) (bool, error) {
	cur, err := readFileOrEmpty(path)
	if err != nil {
		return false, err
	}
	if !strings.Contains(cur, tomlHeader(name)) {
		return false, nil
	}
	out := stripTOMLTable(cur, name)
	return true, os.WriteFile(path, []byte(out), 0o644)
}

// stripTOMLTable removes the [mcp_servers.<name>] header and the lines under it,
// up to (but not including) the next table header or EOF. Other content is
// preserved. This is a line-oriented strip — adequate for the simple, flat
// config.toml Codex generates; it does not parse arbitrary TOML.
func stripTOMLTable(content, name string) string {
	header := tomlHeader(name)
	lines := strings.Split(content, "\n")
	var out []string
	skipping := false
	for _, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if trimmed == header {
			skipping = true
			continue
		}
		if skipping {
			// A new table header ends our block.
			if strings.HasPrefix(trimmed, "[") {
				skipping = false
			} else {
				continue // drop lines belonging to our table
			}
		}
		out = append(out, ln)
	}
	// Collapse any run of blank lines left behind into a single one and trim trailing.
	joined := strings.Join(out, "\n")
	for strings.Contains(joined, "\n\n\n") {
		joined = strings.ReplaceAll(joined, "\n\n\n", "\n\n")
	}
	return strings.TrimRight(joined, "\n") + "\n"
}
```

- [ ] **Step 4: Run the tests — must pass**

Run: `go test ./internal/agents/ -run 'TestWriteTOML|TestRemoveTOML'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/agents/
git add internal/agents/
git commit -m "feat(agents): hand-rolled TOML MCP writer for Codex (no toml dep)"
```

---

### Task 4: Embed + install the two SKILL.md bundles

**Files:**
- Create: `internal/agents/bundles/console-order/SKILL.md`
- Create: `internal/agents/bundles/console-card/SKILL.md`
- Create: `internal/agents/skills.go`
- Create: `internal/agents/skills_test.go`

**Interfaces:**
- Produces: `installSkills(skillsDir string) (installed []string, err error)`; `removeSkills(skillsDir string) (removed []string, err error)`; `bundleNames() []string`.
- Consumes: stdlib `embed`, `io/fs`.

- [ ] **Step 1: Author `internal/agents/bundles/console-order/SKILL.md`** (clear, task-focused; references the exact MCP tool names from Plan 1)

```markdown
---
name: console-order
description: Order food on Swiggy through consolestore's MCP tools — search, build a cart, show the real bill, and place only after the user confirms. Use when the user wants to order food, get a meal, or reorder a usual.
---

# Ordering food with consolestore

consolestore exposes Swiggy ordering as MCP tools. Orders cost real money and
**cannot be cancelled**, so placing always takes two steps and an explicit user
confirmation.

## First: make sure you can act

1. Call `auth_status`. If `signed_in` is false, call `sign_in`, show the user the
   returned `authorize_url` (their browser usually opens on its own), and poll
   `auth_status` until it reports `signed_in: true`. Then continue.
2. Call `get_card` to load what consolestore remembers (default address, favorite
   restaurants, dietary prefs). If it returns `warnings`, tell the user plainly —
   for example, a saved address that no longer exists — and ask how to proceed.

## Choosing the address

- If the card has a `default_address_id`, use it without asking.
- Otherwise call `list_addresses` and ask the user which one to use.
- Never invent an address id. There is no GPS; the address always comes from the
  card or `list_addresses`.

## Finding the food

- Direct request ("get me McDonald's"): call `search_restaurants` with the card's
  address and the user's words. Don't list addresses first.
- Reorder ("the usual"): call `list_usuals`, or `list_presets` for saved carts.
- A preset is the fastest path — see "Ordering a preset" below.
- Pick a restaurant, then `get_menu`. For an item marked `customizable`, call
  `get_item_options` to get its variant/add-on group and choice ids.

## Building the cart

- Call `update_cart` with the FULL set of lines you want (it replaces the cart
  for that restaurant). Each line needs the menu `item_id` and quantity; pass
  variant/add-on selections using the ids from `get_item_options`.
- If a cross-restaurant change is rejected, tell the user the cart already holds a
  different restaurant and ask whether to replace it (call `clear_cart` first).

## Placing the order (two steps, always)

1. Call `prepare_order` with the address id. It returns the real Swiggy `bill`
   and a `confirmation_id`. **Show the bill to the user** — item total, delivery,
   taxes, and to-pay — and ask them to confirm.
2. Only after the user clearly says yes, call `place_order` with that
   `confirmation_id`. If the cart changed since `prepare_order`, the call is
   refused — re-run `prepare_order` and confirm the new bill.
3. Never call `place_order` on your own initiative, and never retry it. If it
   returns an error, the order may still have been placed — call
   `list_active_orders` before doing anything else.

## Ordering a preset

- Call `order_preset` with the preset `name` (and `index` when several share a
  name). It loads the preset into the cart and returns a `bill` + `confirmation_id`
  — then follow the same confirm-then-`place_order` step above.

## Tracking

- After placing, or any time the user asks "where's my order", call
  `list_active_orders` and then `track_order` for the live status and ETA.
```

- [ ] **Step 2: Author `internal/agents/bundles/console-card/SKILL.md`**

```markdown
---
name: console-card
description: View and tune the consolestore taste card — the local memory of the user's default address, favorite restaurants, and dietary preferences. Use when the user asks what consolestore remembers, or wants to change their default address or food preferences.
---

# The consolestore taste card

consolestore keeps a small local "card" so ordering needs less back-and-forth.
It is built automatically from real orders — the user never has to set it up — and
you can read or adjust it with two tools.

## Showing what's remembered

- Call `get_card`. It returns the `default_address_id` and label, `favorites`
  (restaurants ranked by how often they're ordered), and any `prefs` (free-form
  notes like "vegetarian" or "no onion").
- `get_card` also reconciles against live addresses. If it returns `warnings` — for
  example, the saved default address was deleted on Swiggy — relay them and offer
  to set a new default.

## Changing the card

- To set a default address: call `list_addresses`, let the user pick, then call
  `update_card` with that `default_address_id`.
- To record preferences: call `update_card` with a `prefs` list. Providing `prefs`
  replaces the whole list, so include everything that should remain.
- There is nothing to "create" — the card already exists and grows on its own each
  time an order is placed. `update_card` only records explicit choices the user
  states.

## How it fills in over time

- Every placed order (from this agent, the consolestore app, or its CLI) bumps the
  ordered restaurant in `favorites` and refreshes the default address. So the more
  the user orders, the better the card's suggestions — no manual upkeep needed.
```

- [ ] **Step 3: Write the failing test** — `internal/agents/skills_test.go`

```go
package agents

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallSkillsCopiesBundles(t *testing.T) {
	dir := t.TempDir()
	installed, err := installSkills(dir)
	if err != nil {
		t.Fatalf("installSkills: %v", err)
	}
	if len(installed) != 2 {
		t.Fatalf("installed = %v", installed)
	}
	for _, name := range []string{"console-order", "console-card"} {
		p := filepath.Join(dir, name, "SKILL.md")
		fi, err := os.Stat(p)
		if err != nil || fi.Size() == 0 {
			t.Fatalf("missing/empty %s: %v", p, err)
		}
	}
}

func TestRemoveSkillsDeletesOnlyOurs(t *testing.T) {
	dir := t.TempDir()
	// A foreign skill must survive removal.
	other := filepath.Join(dir, "someone-else")
	_ = os.MkdirAll(other, 0o755)
	_ = os.WriteFile(filepath.Join(other, "SKILL.md"), []byte("x"), 0o644)

	_, _ = installSkills(dir)
	removed, err := removeSkills(dir)
	if err != nil {
		t.Fatalf("removeSkills: %v", err)
	}
	if len(removed) != 2 {
		t.Fatalf("removed = %v", removed)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("foreign skill was deleted")
	}
	if _, err := os.Stat(filepath.Join(dir, "console-order")); !os.IsNotExist(err) {
		t.Fatalf("console-order not removed")
	}
}
```

- [ ] **Step 4: Write the implementation** — `internal/agents/skills.go`

```go
package agents

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed bundles
var bundlesFS embed.FS

// bundleNames are the skill bundle directory names under bundles/.
func bundleNames() []string {
	entries, err := fs.ReadDir(bundlesFS, "bundles")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

// installSkills copies each embedded bundle into skillsDir/<name>/. It overwrites
// only our own bundle dirs. Returns the bundle names installed.
func installSkills(skillsDir string) ([]string, error) {
	var installed []string
	for _, name := range bundleNames() {
		srcDir := "bundles/" + name
		entries, err := fs.ReadDir(bundlesFS, srcDir)
		if err != nil {
			return installed, err
		}
		dstDir := filepath.Join(skillsDir, name)
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return installed, err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			data, err := bundlesFS.ReadFile(srcDir + "/" + e.Name())
			if err != nil {
				return installed, err
			}
			if err := os.WriteFile(filepath.Join(dstDir, e.Name()), data, 0o644); err != nil {
				return installed, err
			}
		}
		installed = append(installed, name)
	}
	return installed, nil
}

// removeSkills deletes only our bundle dirs from skillsDir.
func removeSkills(skillsDir string) ([]string, error) {
	var removed []string
	for _, name := range bundleNames() {
		dst := filepath.Join(skillsDir, name)
		if _, err := os.Stat(dst); err == nil {
			if err := os.RemoveAll(dst); err != nil {
				return removed, err
			}
			removed = append(removed, name)
		}
	}
	return removed, nil
}
```

- [ ] **Step 5: Run the tests — must pass**

Run: `go test ./internal/agents/ -run TestInstallSkills`
Expected: PASS. Then `go test ./internal/agents/ -run TestRemoveSkills`.

- [ ] **Step 6: Commit**

```bash
gofmt -w internal/agents/
git add internal/agents/
git commit -m "feat(agents): embed + install/remove console-order & console-card skills"
```

---

### Task 5: Orchestrator — `Install`, `List`, `Remove` with summary + opt-out

**Files:**
- Create: `internal/agents/provision.go`
- Create: `internal/agents/provision_test.go`

**Interfaces:**
- Produces: `Install(out io.Writer) error`; `List(out io.Writer) error`; `Remove(out io.Writer) error`.
- Consumes: `Detect`, `consoleBinary`, `writeJSONServer`/`removeJSONServer`, `writeTOMLServer`/`removeTOMLServer`, `installSkills`/`removeSkills`, env `CONSOLE_NO_AGENT_SETUP`.

- [ ] **Step 1: Write the failing test** — `internal/agents/provision_test.go`

```go
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
```

- [ ] **Step 2: Run it — must fail**

Run: `go test ./internal/agents/ -run TestInstall`
Expected: FAIL.

- [ ] **Step 3: Write the implementation** — `internal/agents/provision.go`

```go
package agents

import (
	"fmt"
	"io"
	"os"
)

func optedOut() bool { return os.Getenv("CONSOLE_NO_AGENT_SETUP") == "1" }

// wireMCP writes our server entry into one agent's config (JSON or TOML).
func wireMCP(a Agent, bin string) (bool, error) {
	switch a.Kind {
	case KindTOML:
		return writeTOMLServer(a.ConfigPath, ServerName, bin, []string{"mcp"})
	default:
		return writeJSONServer(a.ConfigPath, ServerName, bin, []string{"mcp"})
	}
}

func unwireMCP(a Agent) (bool, error) {
	switch a.Kind {
	case KindTOML:
		return removeTOMLServer(a.ConfigPath, ServerName)
	default:
		return removeJSONServer(a.ConfigPath, ServerName)
	}
}

// Install wires the `console mcp` server + skills into every detected agent.
// Idempotent. Honors CONSOLE_NO_AGENT_SETUP=1. Best-effort per agent: a failure
// on one agent is reported but does not abort the others.
func Install(out io.Writer) error {
	if optedOut() {
		fmt.Fprintln(out, "agent setup skipped (CONSOLE_NO_AGENT_SETUP=1)")
		return nil
	}
	agents := Detect()
	if len(agents) == 0 {
		fmt.Fprintln(out, "no local agents detected — nothing to set up.")
		return nil
	}
	bin := consoleBinary()
	for _, a := range agents {
		changed, err := wireMCP(a, bin)
		if err != nil {
			fmt.Fprintf(out, "  %-16s mcp: error: %v\n", a.Title, err)
			continue
		}
		status := "already current"
		if changed {
			status = "wired"
		}
		skillNote := ""
		if a.SkillsDir != "" {
			if names, serr := installSkills(a.SkillsDir); serr != nil {
				skillNote = fmt.Sprintf(" · skills: error: %v", serr)
			} else if len(names) > 0 {
				skillNote = fmt.Sprintf(" · skills: %d", len(names))
			}
		}
		fmt.Fprintf(out, "  %-16s mcp: %s%s\n", a.Title, status, skillNote)
	}
	fmt.Fprintln(out, "done. (Claude Desktop must be restarted to load the new MCP server.)")
	return nil
}

// List prints which agents are detected and whether console is wired.
func List(out io.Writer) error {
	agents := Detect()
	if len(agents) == 0 {
		fmt.Fprintln(out, "no local agents detected.")
		return nil
	}
	for _, a := range agents {
		wired := "not wired"
		if raw, err := os.ReadFile(a.ConfigPath); err == nil {
			if containsServer(string(raw), a.Kind) {
				wired = "wired"
			}
		}
		fmt.Fprintf(out, "  %-16s %s  (%s)\n", a.Title, wired, a.ConfigPath)
	}
	return nil
}

func containsServer(content string, kind Kind) bool {
	if kind == KindTOML {
		return stringsContains(content, tomlHeader(ServerName))
	}
	return stringsContains(content, "\""+ServerName+"\"")
}

// Remove unwires the server + skills from every detected agent.
func Remove(out io.Writer) error {
	agents := Detect()
	bin := "" // unused for removal
	_ = bin
	for _, a := range agents {
		changed, err := unwireMCP(a)
		if err != nil {
			fmt.Fprintf(out, "  %-16s mcp: error: %v\n", a.Title, err)
			continue
		}
		status := "not present"
		if changed {
			status = "removed"
		}
		if a.SkillsDir != "" {
			_, _ = removeSkills(a.SkillsDir)
		}
		fmt.Fprintf(out, "  %-16s mcp: %s\n", a.Title, status)
	}
	fmt.Fprintln(out, "done.")
	return nil
}
```

Add a tiny local helper to avoid importing `strings` twice across files if preferred, or just import `strings` and use `strings.Contains` directly (replace `stringsContains`):

```go
// at top: import "strings"; then use strings.Contains and delete stringsContains.
```

Use `strings.Contains` directly — do not define `stringsContains`. (The snippet above names it only to flag the choice; in the real file import `strings` and call `strings.Contains`.)

- [ ] **Step 4: Run the tests — must pass**

Run: `go test ./internal/agents/ -run TestInstall`
Expected: PASS.

- [ ] **Step 5: Full package test + vet**

Run: `go test ./internal/agents/ && go vet ./internal/agents/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -w internal/agents/
git add internal/agents/
git commit -m "feat(agents): Install/List/Remove orchestrator + summary + opt-out"
```

---

### Task 6: `console agents` subcommand + early dispatch in `main()`

**Files:**
- Create: `internal/agents/dispatch.go`
- Create: `internal/agents/dispatch_test.go`
- Modify: `cmd/store/main.go` (early-route `agents` before `run()`)
- Modify: `internal/cli/help.go` (mention `agents` in usage)

**Interfaces:**
- Produces: `Dispatch(args []string, out io.Writer) int` (args is everything after `agents`).
- Consumes: `Install`, `List`, `Remove`.

- [ ] **Step 1: Write the failing test** — `internal/agents/dispatch_test.go`

```go
package agents

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDispatchInstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	_ = os.WriteFile(filepath.Join(home, ".claude.json"), []byte("{}"), 0o600)

	var buf bytes.Buffer
	code := Dispatch([]string{"install", "--quiet"}, &buf)
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	raw, _ := os.ReadFile(filepath.Join(home, ".claude.json"))
	if !strings.Contains(string(raw), "console") {
		t.Fatalf("install did not wire: %s", raw)
	}
}

func TestDispatchUnknown(t *testing.T) {
	var buf bytes.Buffer
	if code := Dispatch([]string{"frobnicate"}, &buf); code == 0 {
		t.Fatalf("unknown subcommand should be non-zero")
	}
}
```

- [ ] **Step 2: Run it — must fail**

Run: `go test ./internal/agents/ -run TestDispatch`
Expected: FAIL.

- [ ] **Step 3: Write the implementation** — `internal/agents/dispatch.go`

```go
package agents

import (
	"fmt"
	"io"
)

// Dispatch runs `console agents <sub>`. args is the slice after "agents".
// --quiet is accepted (and ignored beyond suppressing the usage hint) so the
// installer can call it non-interactively.
func Dispatch(args []string, out io.Writer) int {
	sub := ""
	for _, a := range args {
		if a == "--quiet" || a == "-q" {
			continue
		}
		if sub == "" {
			sub = a
		}
	}
	switch sub {
	case "", "install", "setup":
		if err := Install(out); err != nil {
			fmt.Fprintf(out, "agents: %v\n", err)
			return 1
		}
		return 0
	case "list", "ls", "status":
		if err := List(out); err != nil {
			fmt.Fprintf(out, "agents: %v\n", err)
			return 1
		}
		return 0
	case "remove", "rm", "uninstall":
		if err := Remove(out); err != nil {
			fmt.Fprintf(out, "agents: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(out, "usage: console agents [install|list|remove]\n")
		return 2
	}
}
```

- [ ] **Step 4: Run the tests — must pass**

Run: `go test ./internal/agents/ -run TestDispatch`
Expected: PASS.

- [ ] **Step 5: Early-route `agents` in `cmd/store/main.go`**

In `main()`, alongside the existing `help` short-circuit (main.go:50-53), add an `agents` branch BEFORE `run(args)` so provisioning needs no auth/network:

```go
	if len(args) > 0 && args[0] == "agents" {
		os.Exit(agents.Dispatch(args[1:], os.Stdout))
	}
```

Add import `"consolestore/internal/agents"` to `cmd/store/main.go`.

- [ ] **Step 6: Mention `agents` in CLI usage** — `internal/cli/help.go`

Add a line to `printUsage` describing the command (match the file's existing formatting; place it near the other management commands). Example line to include in the printed usage text:

```
  console agents [install|list|remove]   wire console into your AI agents (MCP + skills)
```

- [ ] **Step 7: Build, vet, full test**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
gofmt -w internal/agents/ cmd/store/main.go internal/cli/help.go
git add internal/agents cmd/store/main.go internal/cli/help.go
git commit -m "feat(agents): console agents subcommand + early dispatch + usage"
```

---

### Task 7: Installer hook + docs

**Files:**
- Modify: `scripts/install.sh`
- Modify: `landing page/public/install.sh` (if a bundled copy exists — mirror it)
- Modify: `scripts/install.ps1`
- Modify: `landing page/public/install.ps1`
- Modify: `CLAUDE.md` (composition note), `RELEASING.md` (one line)

**Interfaces:**
- Produces: installers that call `console agents install --quiet` after placing the binary.
- Consumes: the `console agents` subcommand (Task 6).

- [ ] **Step 1: Read the current installers**

Run: `sed -n '1,200p' scripts/install.sh` and `sed -n '40,70p' scripts/install.ps1`
Identify the success point: in `install.sh`, just after the binary is moved into place and `chmod +x` and the "run: console" message; in `install.ps1`, after the `Write-Host "run: console"` line.

- [ ] **Step 2: Add the hook to `scripts/install.sh`**

After the binary is installed and the PATH note is printed, before the final exit, add (use the installed binary path variable the script already defines — shown here as `$out`/`$BIN`; match the script's actual variable):

```sh
# Wire console into the user's local AI agents (Claude Desktop/Code, Cursor,
# Codex): register the MCP server + drop skills. Best-effort and idempotent; an
# opt-out is honored inside the binary (CONSOLE_NO_AGENT_SETUP=1).
if [ -x "$out" ]; then
  "$out" agents install --quiet || true
fi
```

(Replace `$out` with the script's real path-to-installed-binary variable.)

- [ ] **Step 3: Mirror the hook to `landing page/public/install.sh`** if that file exists (the served copy). Use the same snippet adapted to its variable names. If only one canonical `install.sh` exists, skip.

- [ ] **Step 4: Add the hook to `scripts/install.ps1`**

After the final `Write-Host "run: console"` line, append:

```powershell
# Wire console into local AI agents (MCP + skills). Best-effort + idempotent;
# CONSOLE_NO_AGENT_SETUP=1 opts out (handled inside the binary).
try { & $out agents install --quiet } catch { }
```

(`$out` is the installed binary path the script already computed.)

- [ ] **Step 5: Mirror to `landing page/public/install.ps1`** (the served copy — required, since the landing serves this file). Use the same snippet.

- [ ] **Step 6: Docs**

In `CLAUDE.md`, under the composition list, add a one-line entry:

```
internal/agents/      provisions local agents: registers `console mcp` in each
                      detected agent (Claude Desktop/Code JSON, Cursor JSON,
                      Codex TOML) + installs SKILL.md bundles. Run by the
                      installer (`console agents install`) and `console agents`.
```

In `RELEASING.md`, add one line noting that the served `install.sh`/`install.ps1` now call `console agents install` and must be deployed with the landing.

- [ ] **Step 7: Verify the binary path the installers pass is absolute**

Run: `go build -o /tmp/console ./cmd/store && /tmp/console agents list`
Expected: prints detected agents (or "no local agents detected"); exits 0. This confirms the subcommand the installers invoke works against a built binary.

- [ ] **Step 8: Commit**

```bash
git add scripts/install.sh scripts/install.ps1 "landing page/public/install.sh" "landing page/public/install.ps1" CLAUDE.md RELEASING.md
git commit -m "feat(install): wire local agents post-install (console agents install)"
```

(Skip any of the four installer paths that don't exist; commit the ones that do.)

---

## Plan 2 self-review

**Spec coverage:**
- Detect installed agents — Task 1. ✓
- MCP register, merge without clobber, idempotent, reversible — Tasks 2 (JSON), 3 (TOML), 5 (orchestrator), with remove paths. ✓
- Skills install into Claude Code/Desktop/Codex; Cursor MCP-only — Task 4 (`SkillsDir==""` skips Cursor) + Task 5. ✓
- `console agents install|list|remove`, summary, opt-out, Desktop-restart note — Tasks 5, 6. ✓
- Installer calls `console agents install --quiet`; all logic in Go — Task 7. ✓
- No second dependency (TOML hand-rolled) — Task 3. ✓
- Absolute binary path via `os.Executable()` — Task 1 (`consoleBinary`). ✓
- No network/auth (early dispatch in `main()`) — Task 6. ✓

**Placeholder scan:** No TBD/TODO. Every code step has full code. Two intentional "match the script's real variable" notes in Task 7 are necessary because `install.sh`/`install.ps1` are read at implementation time (Step 1 reads them) — these are adaptation instructions, not placeholders, and the snippet to insert is complete.

**Type consistency:** `Agent`/`Kind`/`KindJSON`/`KindTOML`/`ServerName` defined in Task 1, used in Tasks 2-6. `writeJSONServer`/`removeJSONServer` (Task 2) and `writeTOMLServer`/`removeTOMLServer` (Task 3) consumed by `wireMCP`/`unwireMCP` (Task 5). `installSkills`/`removeSkills` (Task 4) consumed by Task 5. `Install`/`List`/`Remove` (Task 5) consumed by `Dispatch` (Task 6). `tomlHeader` (Task 3) reused by `containsServer` (Task 5). `Detect`/`consoleBinary` (Task 1) consumed by Task 5.

**Known follow-ups for the implementer (not blockers):**
- In Task 5, use `strings.Contains` directly and import `strings`; do not define `stringsContains`.
- Task 7 Step 1 must read the actual installer variable names before inserting the hook; the `$out` placeholder in snippets stands for "the installed-binary path variable the script already defines."
- Confirm `internal/cli/help.go`'s `printUsage` formatting and match it when adding the `agents` line.
```
