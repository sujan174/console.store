package cli

import (
	"fmt"
	"io"
	"time"

	"consolestore/internal/broker/api"
	"consolestore/internal/localstore"
)

// swiggyBetaOrderCap mirrors screens.SwiggyBetaOrderCap: Swiggy's MCP beta blocks
// place_food_order for carts ≥ ₹1000. Duplicated because cli must not import tui.
const swiggyBetaOrderCap = 1000

// runOrder resolves preset(s) named `name` and orders one. idx (1-based, 0 =
// none given) selects directly: `console order coffee 2`. With no index and
// several matches it lists them and, when stdin is interactive, lets the user
// press a number; a single match runs straight through to the bill + confirm.
func runOrder(d Deps, name string, idx int) int {
	st := newStyle(d.Color)
	ps, err := localstore.LoadPresets()
	if err != nil {
		fmt.Fprintf(d.Out, "store: %v\n", err)
		return 1
	}
	matches := ps.ByName(name)
	if len(matches) == 0 {
		fmt.Fprintf(d.Out, "%s %s\n%s\n", st.warn("no preset"), st.head(name),
			st.dim(fmt.Sprintf("create one in the app: open store, build a cart, then `:alias set %s`", name)))
		return 1
	}
	if idx > 0 {
		if idx > len(matches) {
			fmt.Fprintf(d.Out, "%s\n", st.warn(fmt.Sprintf("no preset %q #%d.", name, idx)))
			listPresets(d.Out, name, matches, st)
			return 1
		}
		return placePreset(d, matches[idx-1], st)
	}
	if len(matches) == 1 {
		return placePreset(d, matches[0], st)
	}

	// Several share the name. On a non-interactive stdin (pipe/redirect), just
	// list — the user re-runs with a number. Interactively, offer the picker.
	if !d.Interactive {
		listPresets(d.Out, name, matches, st)
		fmt.Fprintf(d.Out, "\n%s\n", st.dim(fmt.Sprintf("run  console order %s <n>  to order one.", name)))
		return 0
	}
	i, ok := pickPreset(d, name, matches, st)
	if !ok {
		fmt.Fprintf(d.Out, "%s\n", st.dim("cancelled."))
		return 0
	}
	return placePreset(d, matches[i], st)
}

// listPresets prints the numbered presets sharing a name (short address).
func listPresets(out io.Writer, name string, matches []localstore.Preset, st style) {
	fmt.Fprintf(out, "%s\n", st.dim(fmt.Sprintf("%d presets named %q:", len(matches), name)))
	for i, p := range matches {
		fmt.Fprintf(out, "  %s %s  %s %s %s %s\n",
			st.num(fmt.Sprintf("%d)", i+1)), st.head(p.RestaurantName),
			st.dim("·"), st.dim(shortAddr(p.AddrLine)),
			st.dim("·"), st.dim(summarize(p)))
	}
}

func placePreset(d Deps, p localstore.Preset, st style) int {
	adjust := st.dim("open `console` to adjust.")
	items := localstore.PresetCartItems(p)
	// Push (override any existing cart), then pull the authoritative cart/bill.
	if _, err := d.Backend.UpdateCart(p.AddrID, p.RestaurantID, p.RestaurantName, items); err != nil {
		fmt.Fprintf(d.Out, "%s\n%s\n", st.warn(fmt.Sprintf("%q isn't available right now (%v).", p.Name, err)), adjust)
		return 1
	}
	cart, err := d.Backend.GetCart(p.AddrID, p.RestaurantName)
	if err != nil {
		fmt.Fprintf(d.Out, "%s\n%s\n", st.warn(fmt.Sprintf("couldn't read the cart (%v).", err)), adjust)
		return 1
	}
	if unavailable := unavailableNames(cart); len(unavailable) > 0 {
		fmt.Fprintf(d.Out, "%s\n%s\n", st.warn(fmt.Sprintf("%q can't be ordered now — unavailable: %s.", p.Name, joinNames(unavailable))), adjust)
		return 1
	}
	if len(cart.Lines) == 0 {
		fmt.Fprintf(d.Out, "%s\n%s\n", st.warn(fmt.Sprintf("%q produced an empty cart (items may no longer exist).", p.Name)), adjust)
		return 1
	}

	renderCart(d.Out, p.AddrLine, p.RestaurantName, cart, st)

	// Swiggy's MCP beta rejects place_food_order for carts ≥ ₹1000. Refuse here
	// with a clear message instead of letting it fail server-side. (Constant is
	// duplicated from screens.SwiggyBetaOrderCap — cli must not import tui.)
	if cart.Total >= swiggyBetaOrderCap {
		fmt.Fprintf(d.Out, "\n%s\n%s\n",
			st.warn(fmt.Sprintf("order is ₹%d — ₹1000 or more can't be placed here (Swiggy MCP beta).", cart.Total)),
			st.dim("place this one in the Swiggy app instead."))
		return 1
	}

	if !d.Armed {
		fmt.Fprintf(d.Out, "\n%s\n%s\n", st.warn("browse-only build — order NOT placed."),
			st.dim("run the armed `console` to place, or open `console` to adjust the cart."))
		return 0
	}

	if !d.Interactive {
		// stdin isn't a terminal (piped/redirected/EOF). prompt() would return ""
		// immediately and look like a confirming Enter, so we'd place a REAL order
		// with no human in the loop. Refuse instead.
		fmt.Fprintf(d.Out, "\n%s\n%s\n", st.warn("not placed — placing an order needs an interactive terminal."),
			st.dim(fmt.Sprintf("run  console order %s  directly in your shell to confirm and place.", p.Name)))
		return 1
	}

	fmt.Fprintf(d.Out, "\n%s %s\n", st.ok("press Enter to place this order"), st.dim("· Ctrl-C / n to cancel"))
	if !confirm(d) {
		// Ctrl-C (ctx canceled), EOF, or any non-affirmative answer. We trap SIGINT
		// in main, so Ctrl-C can't kill us — confirm() is the ONLY gate, and it
		// failing means the user did not approve. Place NOTHING.
		fmt.Fprintf(d.Out, "\n%s\n", st.dim("cancelled — no order placed."))
		return 0
	}
	order, err := d.Backend.PlaceOrder(p.AddrID) // never retried
	if err != nil {
		fmt.Fprintf(d.Out, "%s\n%s\n", st.warn(fmt.Sprintf("order failed: %v", err)),
			st.dim("if you may have been charged, run `console status` before retrying."))
		return 1
	}
	etaLo, etaHi := localstore.ParseETAMinutes(order.ETA)
	_ = localstore.SaveActiveOrder(localstore.ActiveOrder{
		OrderID: order.ID, Restaurant: p.RestaurantName, AddrLine: p.AddrLine,
		ETALoMin: etaLo, ETAHiMin: etaHi, Total: order.Total, PlacedAt: time.Now().Unix(),
	})
	// Accrete the taste card: a preset carries the real restaurant id + saved address.
	_ = localstore.RecordOrder(p.AddrID, p.AddrLine, p.RestaurantID, p.RestaurantName, time.Now().Unix())
	line := "✓ order placed — " + order.ID
	if order.ETA != "" {
		line += " · eta " + order.ETA
	}
	fmt.Fprintf(d.Out, "\n%s\n", st.ok(line))
	return 0
}

func unavailableNames(c api.Cart) []string {
	var out []string
	for _, l := range c.Lines {
		if !l.Available {
			out = append(out, l.Name)
		}
	}
	return out
}

func joinNames(ns []string) string {
	s := ""
	for i, n := range ns {
		if i > 0 {
			s += ", "
		}
		s += n
	}
	return s
}

func summarize(p localstore.Preset) string {
	s := ""
	for i, l := range p.Lines {
		if i > 0 {
			s += ", "
		}
		s += fmt.Sprintf("%d×%s", l.Qty, l.Name)
	}
	return s
}
