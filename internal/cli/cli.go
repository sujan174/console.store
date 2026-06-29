// Package cli implements consolestore's headless shell commands (status, order,
// alias, help). It drives the account-pinned broker backend and prints plain
// text — it never opens the TUI and never imports internal/tui.
package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"consolestore/internal/broker/api"
)

type Backend interface {
	Addresses() ([]api.Address, error)
	UpdateCart(addressID, restaurantID, restaurantName string, items []api.CartItem) (api.Cart, error)
	GetCart(addressID, restaurantName string) (api.Cart, error)
	PlaceOrder(addressID string) (api.Order, error)
	ActiveOrders(addressID string) ([]api.Order, error)
	TrackOrder(orderID string) (api.Tracking, error)
	Logout() error
}

// Deps carries everything a command needs. In/Out default to os.Stdin/os.Stdout
// in main; tests inject buffers. Armed mirrors swiggy.LiveOrdersEnabled().
type Deps struct {
	Backend     Backend
	Armed       bool
	SignedIn    bool
	Color       bool // emit ANSI colour (set when Out is a terminal; off in tests/pipes)
	Interactive bool // stdin is a terminal — required to confirm a real order placement
	In          io.Reader
	Out         io.Writer
}

// Dispatch routes a headless command and returns a process exit code.
func Dispatch(args []string, d Deps) int {
	if d.Out == nil {
		return 2
	}
	if len(args) == 0 {
		printUsage(d.Out)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		printUsage(d.Out)
		return 0
	case "status":
		return requireAuth(d, func() int { return runStatus(d) })
	case "order":
		// `store order status` is a synonym for `store status`.
		if len(args) >= 2 && args[1] == "status" {
			return requireAuth(d, func() int { return runStatus(d) })
		}
		if len(args) < 2 {
			fmt.Fprintln(d.Out, "usage: store order <name> [number]")
			return 2
		}
		idx := 0 // optional preset number: `store order coffee 2`
		if len(args) >= 3 {
			n, err := strconv.Atoi(args[2])
			if err != nil || n < 1 {
				fmt.Fprintln(d.Out, "usage: store order <name> [number]   (number picks among same-named presets)")
				return 2
			}
			idx = n
		}
		return requireAuth(d, func() int { return runOrder(d, args[1], idx) })
	case "logout", "disconnect", "signout":
		return runLogout(d)
	case "whoami", "account":
		return runWhoami(d)
	case "alias":
		return runAlias(d, args[1:]) // alias list/rm need no backend
	case "version", "--version", "-v":
		return runVersion(d)
	case "update", "upgrade", "self-update":
		return runUpdate(d, args[1:])
	default:
		fmt.Fprintf(d.Out, "store: unknown command %q\n\n", args[0])
		printUsage(d.Out)
		return 2
	}
}

func requireAuth(d Deps, fn func() int) int {
	if !d.SignedIn {
		fmt.Fprintln(d.Out, "not signed in — run `store` once to authorize, then try again.")
		return 1
	}
	return fn()
}

// firstAddressID returns the account's primary saved address id (presets carry
// their own; status uses the primary).
func firstAddressID(d Deps) (string, error) {
	addrs, err := d.Backend.Addresses()
	if err != nil {
		return "", err
	}
	if len(addrs) == 0 {
		return "", fmt.Errorf("no saved addresses on the account")
	}
	return addrs[0].ID, nil
}

// prompt reads one trimmed line from d.In (empty Reader → "" = treated as Enter).
func prompt(d Deps) string {
	if d.In == nil {
		return ""
	}
	var b strings.Builder
	buf := make([]byte, 1)
	for {
		n, err := d.In.Read(buf)
		if n > 0 {
			if buf[0] == '\n' {
				break
			}
			b.WriteByte(buf[0])
		}
		if err != nil {
			break
		}
	}
	return strings.TrimSpace(b.String())
}
