package tui

import (
	"os"
	"testing"
)

// TestMain isolates EVERY test in this package from the real user config dir.
// The place-order flow (OrderPlacedMsg) persists an active order via
// localstore, which is keyed off XDG_CONFIG_HOME — without this, a test that
// drives a placement writes to ~/.config/console-store/active-order.json and
// leaves a phantom "order-99" tracked order on the real machine.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "console-tui-test")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_CONFIG_HOME", dir)
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
