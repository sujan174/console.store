package cli

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"consolestore/internal/agents"
	"consolestore/internal/localstore"
)

// Testability seams: tests override these so a run never deletes the test
// binary, touches real agent configs, or resolves the real executable path.
var (
	agentsRemove = agents.Remove
	removeSelf   = defaultRemoveSelf
	execPath     = os.Executable
)

// defaultRemoveSelf deletes the running binary. On Windows a running .exe cannot
// delete itself, so we rename it to path+".old" (the caller prints a note); on
// unix we unlink it outright.
func defaultRemoveSelf(path string) error {
	if runtime.GOOS == "windows" {
		return os.Rename(path, path+".old")
	}
	return os.Remove(path)
}

// runUninstall fully removes consolestore from the machine: it reverses the
// agent MCP/skill registrations, purges the Swiggy token from the keyring,
// deletes the local data dir, and deletes the binary itself. It is destructive
// and irreversible, so it confirms strictly (empty Enter does NOT confirm).
func runUninstall(d Deps, args []string) int {
	st := newStyle(d.Color)

	yes := false
	keepBinary := false
	for _, a := range args {
		switch a {
		case "--yes", "-y":
			yes = true
		case "--keep-binary":
			keepBinary = true
		default:
			fmt.Fprintln(d.Out, "usage: console uninstall [--yes] [--keep-binary]")
			return 2
		}
	}

	// Resolve paths up front for the preview.
	cfgDir, cfgErr := localstore.ConfigDir()
	if cfgErr != nil {
		cfgDir = ""
	}
	binPath, binErr := execPath()
	binKnown := binErr == nil && binPath != ""

	// Preview.
	fmt.Fprintf(d.Out, "%s\n", st.warn("this will completely remove consolestore from this machine:"))
	if !keepBinary {
		if binKnown {
			fmt.Fprintf(d.Out, "  %s the console binary at %s\n", st.warn("·"), st.bright(binPath))
		} else {
			fmt.Fprintf(d.Out, "  %s the console binary %s\n", st.warn("·"), st.dim("(path unknown — will skip)"))
		}
	}
	if cfgDir != "" {
		fmt.Fprintf(d.Out, "  %s all local data at %s\n", st.warn("·"), st.bright(cfgDir))
	}
	fmt.Fprintf(d.Out, "  %s the Swiggy token from your keyring\n", st.warn("·"))
	fmt.Fprintf(d.Out, "  %s agent MCP/skill registrations\n", st.warn("·"))
	fmt.Fprintf(d.Out, "%s\n", st.dim("this wipes your saved presets and logs you out — it cannot be undone."))

	// Confirmation — stricter than the order confirm: empty Enter must NOT proceed.
	if !yes {
		if !d.Interactive {
			fmt.Fprintln(d.Out, "refusing to uninstall without confirmation — re-run with --yes")
			return 1
		}
		fmt.Fprint(d.Out, "type 'yes' to uninstall (anything else aborts): ")
		if !confirmYes(d) {
			fmt.Fprintf(d.Out, "%s\n", st.dim("aborted — nothing was removed."))
			return 0
		}
	}

	// Best-effort cleanup: one failure must not abort the rest.
	failed := false

	// 1. Agents.
	fmt.Fprintf(d.Out, "%s\n", st.dim("removing agent registrations…"))
	if err := agentsRemove(d.Out); err != nil {
		fmt.Fprintf(d.Out, "  %s agents: %v\n", st.warn("✗"), err)
		failed = true
	}

	// 2. Token.
	if d.SignedIn && d.Backend != nil {
		if err := d.Backend.Logout(); err != nil {
			fmt.Fprintf(d.Out, "  %s keyring: logout failed: %v\n", st.warn("✗"), err)
			failed = true
		} else {
			fmt.Fprintf(d.Out, "  %s purged the Swiggy token from your keyring.\n", st.ok("✓"))
		}
	} else {
		fmt.Fprintf(d.Out, "  %s not signed in — no token to purge.\n", st.dim("·"))
	}

	// 3. Data dir.
	if err := localstore.RemoveAllData(); err != nil {
		fmt.Fprintf(d.Out, "  %s data: %v\n", st.warn("✗"), err)
		failed = true
	} else {
		fmt.Fprintf(d.Out, "  %s deleted local data.\n", st.ok("✓"))
	}

	// 4. Binary.
	if !keepBinary {
		if !binKnown {
			fmt.Fprintf(d.Out, "  %s binary: path unknown — remove it manually.\n", st.warn("✗"))
			failed = true
		} else if err := removeSelf(binPath); err != nil {
			fmt.Fprintf(d.Out, "  %s binary: %v\n", st.warn("✗"), err)
			failed = true
		} else if runtime.GOOS == "windows" {
			fmt.Fprintf(d.Out, "  %s renamed the binary to %s (delete it after this process exits).\n", st.ok("✓"), st.bright(binPath+".old"))
		} else {
			fmt.Fprintf(d.Out, "  %s deleted the binary.\n", st.ok("✓"))
		}
	}

	// Summary.
	if failed {
		fmt.Fprintf(d.Out, "%s\n", st.warn("some steps failed — see above; you may need to finish cleanup by hand."))
		return 1
	}
	fmt.Fprintf(d.Out, "%s\n%s\n", st.ok("✓ consolestore uninstalled."), st.dim("reinstall anytime: curl -fsSL consolestore.in/install | sh"))
	return 0
}

// confirmYes is the strict variant of confirm: it returns true ONLY on an
// explicit y/yes (case-insensitive, trimmed). Empty Enter, any other answer,
// EOF/read error, a raw ETX byte, or ctx cancellation (Ctrl-C/SIGTERM) all
// return false — DO NOT destroy. It mirrors confirm's goroutine+select pattern
// because the process traps SIGINT (Ctrl-C only cancels d.Ctx).
func confirmYes(d Deps) bool {
	if d.In == nil {
		return false
	}
	ctx := d.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	type result struct {
		line    string
		newline bool
	}
	ch := make(chan result, 1)
	go func() {
		var b strings.Builder
		buf := make([]byte, 1)
		for {
			n, err := d.In.Read(buf)
			if n > 0 {
				switch buf[0] {
				case '\n':
					ch <- result{strings.TrimSpace(b.String()), true}
					return
				case 3: // ETX (raw-mode Ctrl-C) → abort
					ch <- result{}
					return
				default:
					b.WriteByte(buf[0])
				}
			}
			if err != nil { // EOF / read error before newline → abort
				ch <- result{}
				return
			}
		}
	}()

	select {
	case <-ctx.Done(): // Ctrl-C / SIGTERM → abort
		return false
	case r := <-ch:
		if !r.newline {
			return false
		}
		switch strings.ToLower(r.line) {
		case "y", "yes":
			return true
		default: // empty Enter or anything else → abort
			return false
		}
	}
}
