package cli

import (
	"fmt"
	"strconv"
	"strings"

	"consolestore/internal/localstore"
)

// runAlias handles `console alias list` and `console alias rm <name> [n]`. (Preset
// CREATION happens inside the TUI via `:alias set`.)
func runAlias(d Deps, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(d.Out, "usage: console alias list | console alias rm <name> [n]")
		return 2
	}
	switch args[0] {
	case "list", "ls":
		return aliasList(d)
	case "rm", "remove", "delete":
		if len(args) < 2 {
			fmt.Fprintln(d.Out, "usage: console alias rm <name> [n]")
			return 2
		}
		idx := -1 // -1 means no explicit index given
		if len(args) >= 3 {
			n, err := strconv.Atoi(args[2])
			if err != nil || n < 1 {
				fmt.Fprintln(d.Out, "store: index must be a positive number")
				return 2
			}
			idx = n - 1
		}
		return aliasRemove(d, args[1], idx)
	case "set":
		fmt.Fprintln(d.Out, "create presets inside the app: open store, build a cart, then `:alias set <name>`")
		return 2
	default:
		fmt.Fprintf(d.Out, "store: unknown alias command %q\n", args[0])
		return 2
	}
}

func aliasList(d Deps) int {
	ps, err := localstore.LoadPresets()
	if err != nil {
		fmt.Fprintf(d.Out, "store: %v\n", err)
		return 1
	}
	if len(ps.Items) == 0 {
		fmt.Fprintln(d.Out, "no presets yet — create one in the app with `:alias set <name>`")
		return 0
	}
	// Group by name in first-seen order.
	seen := map[string]bool{}
	for _, p := range ps.Items {
		key := strings.ToLower(p.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		group := ps.ByName(p.Name)
		fmt.Fprintf(d.Out, "%s (%d)\n", p.Name, len(group))
		for i, g := range group {
			fmt.Fprintf(d.Out, "  %d) %s · %s · %s\n", i+1, g.RestaurantName, g.AddrLine, summarize(g))
		}
	}
	return 0
}

func aliasRemove(d Deps, name string, idx int) int {
	ps, err := localstore.LoadPresets()
	if err != nil {
		fmt.Fprintf(d.Out, "store: %v\n", err)
		return 1
	}
	matches := ps.ByName(name)
	if len(matches) == 0 {
		fmt.Fprintf(d.Out, "store: no preset named %q\n", name)
		return 1
	}
	if idx < 0 {
		// No explicit index given.
		if len(matches) == 1 {
			// Unambiguous: remove the only match.
			idx = 0
		} else {
			// Ambiguous: refuse and list them.
			fmt.Fprintf(d.Out, "%d presets named %q — specify which to remove:\n", len(matches), name)
			for i, p := range matches {
				fmt.Fprintf(d.Out, "  %d) %s · %s · %s\n", i+1, p.RestaurantName, p.AddrLine, summarize(p))
			}
			fmt.Fprintf(d.Out, "run: console alias rm %s <n>\n", name)
			return 1
		}
	}
	// idx >= 0: explicit (or resolved single-match) index.
	ok, err := ps.Remove(name, idx)
	if err != nil {
		fmt.Fprintf(d.Out, "store: %v\n", err)
		return 1
	}
	if !ok {
		fmt.Fprintf(d.Out, "store: no preset %q #%d\n", name, idx+1)
		return 1
	}
	if err := localstore.SavePresets(ps); err != nil {
		fmt.Fprintf(d.Out, "store: %v\n", err)
		return 1
	}
	fmt.Fprintf(d.Out, "removed preset %q.\n", name)
	return 0
}
