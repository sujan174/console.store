//go:build !windows

package updater

import (
	"os"
	"syscall"
)

// reexec replaces this process image with the freshly-swapped binary, so the
// current invocation continues on the new version. CONSOLE_UPDATED guards
// against an update loop.
func reexec(path string) error {
	env := append(os.Environ(), "CONSOLE_UPDATED=1")
	return syscall.Exec(path, os.Args, env)
}
