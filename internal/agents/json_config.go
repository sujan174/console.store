package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
)

// serverEntry is the MCP server shape Claude expects.
func serverEntry(command string, args []string) map[string]any {
	return map[string]any{"command": command, "args": toAnySlice(args)}
}

// nestedMap walks/creates the object at keyPath under m and returns it. Any
// existing non-object value along the path is replaced with an object (we never
// expect a scalar where a server map belongs).
func nestedMap(m map[string]any, keyPath []string) map[string]any {
	cur := m
	for _, k := range keyPath {
		child, ok := cur[k].(map[string]any)
		if !ok || child == nil {
			child = map[string]any{}
			cur[k] = child
		}
		cur = child
	}
	return cur
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

// writeJSONServerAt merges entry under keyPath[name], preserving every other
// key. keyPath is the nested path to the servers map (["mcpServers"] for the
// Claude agents). Idempotent: changed=false when the exact entry already
// exists. Kept general (nested keyPath) so the JSON merge/preserve logic is
// unit-testable independently of the single call site.
func writeJSONServerAt(path string, keyPath []string, name string, entry map[string]any) (bool, error) {
	m, err := loadJSONObject(path)
	if err != nil {
		return false, err
	}
	servers := nestedMap(m, keyPath)
	if existing, ok := servers[name]; ok && reflect.DeepEqual(existing, any(entry)) {
		return false, nil
	}
	servers[name] = entry
	return true, saveJSONObject(path, m)
}

// removeJSONServerAt deletes keyPath[name] if present.
func removeJSONServerAt(path string, keyPath []string, name string) (bool, error) {
	m, err := loadJSONObject(path)
	if err != nil {
		return false, err
	}
	// Walk (without creating) to the servers map.
	cur := m
	for _, k := range keyPath {
		child, ok := cur[k].(map[string]any)
		if !ok || child == nil {
			return false, nil
		}
		cur = child
	}
	if _, ok := cur[name]; !ok {
		return false, nil
	}
	delete(cur, name)
	return true, saveJSONObject(path, m)
}

// writeJSONServer / removeJSONServer keep the original ["mcpServers"] +
// command/args shape for the agents that used them before generalization.
func writeJSONServer(path, name, command string, args []string) (bool, error) {
	return writeJSONServerAt(path, []string{"mcpServers"}, name, serverEntry(command, args))
}

func removeJSONServer(path, name string) (bool, error) {
	return removeJSONServerAt(path, []string{"mcpServers"}, name)
}
