package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Hermes (NousResearch/hermes-agent) stores MCP servers in ~/.hermes/config.yaml
// under a top-level `mcp_servers:` mapping. There is no YAML in the stdlib and
// the project avoids dependencies (see the hand-rolled TOML writer), so this is a
// narrow, line-oriented writer that only ever touches our own `console:` entry —
// it does NOT parse arbitrary YAML. It preserves every other line, aligns our
// entry to the block's existing indentation, and is idempotent.

// leadingSpaces counts the spaces (not tabs — Hermes configs use spaces) at the
// start of a line.
func leadingSpaces(s string) int {
	n := 0
	for _, r := range s {
		if r != ' ' {
			break
		}
		n++
	}
	return n
}

// hermesBlock renders our console entry indented at childIndent (the indent of
// the sibling servers under mcp_servers), with its fields two spaces deeper.
func hermesBlock(childIndent int, name, command string, args []string) string {
	ci := strings.Repeat(" ", childIndent)
	// mcp_servers sits at column 0, so the nesting step equals childIndent; the
	// entry's fields therefore live at 2*childIndent (matches 2- and 4-space files).
	fi := strings.Repeat(" ", childIndent*2)
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = fmt.Sprintf("%q", a)
	}
	var b strings.Builder
	b.WriteString(ci + name + ":\n")
	b.WriteString(fi + fmt.Sprintf("command: %q\n", command))
	b.WriteString(fi + "args: [" + strings.Join(quoted, ", ") + "]\n")
	return b.String()
}

// mcpServersLine reports whether a line is exactly the top-level `mcp_servers:`
// key (column 0, nothing after the colon but optional trailing space/comment).
func mcpServersLine(line string) bool {
	if leadingSpaces(line) != 0 {
		return false
	}
	t := strings.TrimSpace(line)
	return t == "mcp_servers:"
}

// writeYAMLServer ensures ~/.hermes/config.yaml has our console entry under
// mcp_servers. Idempotent: changed=false when the file already matches.
func writeYAMLServer(path, name, command string, args []string) (bool, error) {
	cur, err := readFileOrEmpty(path)
	if err != nil {
		return false, err
	}
	next := upsertHermesServer(cur, name, command, args)
	if next == cur {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, []byte(next), 0o644)
}

// removeYAMLServer strips our console entry from mcp_servers. changed=false when
// absent.
func removeYAMLServer(path, name string) (bool, error) {
	cur, err := readFileOrEmpty(path)
	if err != nil {
		return false, err
	}
	next := stripHermesServer(cur, name)
	if next == cur {
		return false, nil
	}
	return true, os.WriteFile(path, []byte(next), 0o644)
}

// findMcpServersBlock returns the index of the mcp_servers: line and the
// [start,end) line range of its body (indented children), or ok=false if the key
// is absent.
func findMcpServersBlock(lines []string) (keyIdx, start, end int, ok bool) {
	keyIdx = -1
	for i, ln := range lines {
		if mcpServersLine(ln) {
			keyIdx = i
			break
		}
	}
	if keyIdx == -1 {
		return 0, 0, 0, false
	}
	start = keyIdx + 1
	end = start
	for end < len(lines) {
		ln := lines[end]
		if strings.TrimSpace(ln) == "" { // blank lines belong to the block
			end++
			continue
		}
		if leadingSpaces(ln) == 0 { // next top-level key ends the block
			break
		}
		end++
	}
	return keyIdx, start, end, true
}

// childIndentOf returns the indent of the first non-blank line in the block
// range, or def when the block is empty.
func childIndentOf(lines []string, start, end, def int) int {
	for i := start; i < end; i++ {
		if strings.TrimSpace(lines[i]) != "" {
			return leadingSpaces(lines[i])
		}
	}
	return def
}

// removeServerEntry drops the `<indent><name>:` line and its deeper-indented
// children from the block range, returning the trimmed lines.
func removeServerEntry(lines []string, start, end, childIndent int, name string) []string {
	header := strings.Repeat(" ", childIndent) + name + ":"
	var out []string
	i := 0
	for i < len(lines) {
		inBlock := i >= start && i < end
		if inBlock && leadingSpaces(lines[i]) == childIndent && strings.TrimSpace(lines[i]) == name+":" && strings.HasPrefix(lines[i], header) {
			i++ // skip the header line
			for i < len(lines) && i < end {
				if strings.TrimSpace(lines[i]) == "" || leadingSpaces(lines[i]) > childIndent {
					i++ // skip children/blank
					continue
				}
				break
			}
			continue
		}
		out = append(out, lines[i])
		i++
	}
	return out
}

// upsertHermesServer inserts or replaces our console entry under mcp_servers,
// creating the mcp_servers block if needed.
func upsertHermesServer(content, name, command string, args []string) string {
	if strings.TrimSpace(content) == "" {
		return "mcp_servers:\n" + hermesBlock(2, name, command, args)
	}
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	keyIdx, start, end, ok := findMcpServersBlock(lines)
	if !ok {
		// No mcp_servers key — append a fresh block.
		block := "mcp_servers:\n" + hermesBlock(2, name, command, args)
		if len(lines) > 0 {
			return strings.Join(lines, "\n") + "\n" + block
		}
		return block
	}
	childIndent := childIndentOf(lines, start, end, 2)
	// Drop any existing console entry, then recompute the block bounds.
	lines = removeServerEntry(lines, start, end, childIndent, name)
	keyIdx, start, _, _ = findMcpServersBlock(lines)
	block := hermesBlock(childIndent, name, command, args)
	blockLines := strings.Split(strings.TrimRight(block, "\n"), "\n")
	// Insert right after the mcp_servers: line.
	out := make([]string, 0, len(lines)+len(blockLines))
	out = append(out, lines[:start]...)
	out = append(out, blockLines...)
	out = append(out, lines[start:]...)
	_ = keyIdx
	return strings.Join(out, "\n") + "\n"
}

// stripHermesServer removes our console entry from mcp_servers (leaving the
// mcp_servers key even if it becomes empty — harmless and simplest).
func stripHermesServer(content, name string) string {
	if strings.TrimSpace(content) == "" {
		return content
	}
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	_, start, end, ok := findMcpServersBlock(lines)
	if !ok {
		return content
	}
	childIndent := childIndentOf(lines, start, end, 2)
	trimmed := removeServerEntry(lines, start, end, childIndent, name)
	return strings.Join(trimmed, "\n") + "\n"
}
