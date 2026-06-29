//go:build windows

package updater

import (
	"os"
	"os/exec"
)

// reexec on Windows: spawn the new binary inheriting stdio, wait, and exit with
// its code (no execve). CONSOLE_UPDATED guards against an update loop.
func reexec(path string) error {
	cmd := exec.Command(path, os.Args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), "CONSOLE_UPDATED=1")
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		}
		return err
	}
	os.Exit(0)
	return nil
}
