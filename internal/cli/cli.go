// Package cli implements consolestore's headless shell commands (status, order,
// alias, help). It drives the account-pinned broker backend and prints plain
// text — it never opens the TUI and never imports internal/tui.
package cli

import (
	"context"
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

	// Menu is a READ-ONLY probe used to check preset-line availability before
	// pushing a cart (see availability.go). Signature mirrors
	// datasource.Backend.Menu exactly so datasource.BrokerBackend structurally
	// satisfies this interface too.
	Menu(addressID, restaurantID string) (api.Menu, error)

	// Instamart (grocery) vertical — a separate cart keyed by address, not
	// restaurant. Signatures mirror datasource.Backend's IM* methods exactly so
	// datasource.BrokerBackend structurally satisfies this interface too.
	IMUpdateCart(addressID string, items []api.IMCartItem) (api.IMCart, error)
	IMGetCart() (api.IMCart, error)
	IMPlaceOrder(addressID string) (api.Order, error)
	IMOrders(activeOnly bool) ([]api.IMOrder, error)
	IMTrack(orderID string, lat, lng float64) (api.Tracking, error)
	// IMSearch is a READ-ONLY probe (see availability.go) — same signature as
	// datasource.Backend.IMSearch.
	IMSearch(addressID, query string) ([]api.IMProduct, error)
}

// Deps carries everything a command needs. In/Out default to os.Stdin/os.Stdout
// in main; tests inject buffers. Armed mirrors swiggy.LiveOrdersEnabled().
type Deps struct {
	Backend     Backend
	Armed       bool
	SignedIn    bool
	Color       bool            // emit ANSI colour (set when Out is a terminal; off in tests/pipes)
	Interactive bool            // stdin is a terminal — required to confirm a real order placement
	Ctx         context.Context // canceled on Ctrl-C / SIGTERM — a canceled ctx means "do NOT place"
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
		// `console order status` is a synonym for `console status`.
		if len(args) >= 2 && args[1] == "status" {
			return requireAuth(d, func() int { return runStatus(d) })
		}
		if len(args) < 2 {
			fmt.Fprintln(d.Out, "usage: console order <name> [number]")
			return 2
		}
		idx := 0 // optional preset number: `console order coffee 2`
		if len(args) >= 3 {
			n, err := strconv.Atoi(args[2])
			if err != nil || n < 1 {
				fmt.Fprintln(d.Out, "usage: console order <name> [number]   (number picks among same-named presets)")
				return 2
			}
			idx = n
		}
		return requireAuth(d, func() int { return runOrder(d, args[1], idx) })
	case "logout", "disconnect", "signout":
		return runLogout(d)
	case "uninstall":
		return runUninstall(d, args[1:])
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
		fmt.Fprintln(d.Out, "not signed in — run `console` once to authorize, then try again.")
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

// confirm asks the user to approve a real, paid, non-cancellable order. It
// returns true ONLY on an affirmative line (empty Enter, or y/yes). It returns
// false — DO NOT place — on Ctrl-C/SIGTERM (d.Ctx canceled), on EOF or a read
// error before any newline, on a raw ETX byte (0x03), or on any other answer.
//
// This is the safety gate. The process traps SIGINT (signal.NotifyContext in
// main), so Ctrl-C does NOT kill us — it only cancels d.Ctx. We therefore must
// check the context ourselves; never assume "Ctrl-C killed the process".
func confirm(d Deps) bool {
	if d.In == nil {
		return false // no input source → cannot have confirmed
	}
	ctx := d.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	type result struct {
		line    string
		newline bool // a full line was read (vs EOF/error/ETX)
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
				case 3: // ETX (Ctrl-C delivered as a byte, e.g. raw mode) → cancel
					ch <- result{}
					return
				default:
					b.WriteByte(buf[0])
				}
			}
			if err != nil { // EOF / read error before a newline → cancel
				ch <- result{}
				return
			}
		}
	}()

	select {
	case <-ctx.Done(): // Ctrl-C / SIGTERM → cancel
		return false
	case r := <-ch:
		if !r.newline {
			return false
		}
		switch strings.ToLower(r.line) {
		case "", "y", "yes":
			return true
		default:
			return false
		}
	}
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
