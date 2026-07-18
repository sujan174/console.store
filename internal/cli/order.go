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

// instamartMin mirrors tui.InstamartMin: Instamart's minimum order value.
// Duplicated because cli must not import tui.
const instamartMin = 99

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
			listPresets(d.Out, name, matches, nil, st)
			return 1
		}
		return orderPreset(d, matches[idx-1], st)
	}
	// Single-match orders skip the availability probe entirely: the cart push
	// a few lines into orderPreset validates the SAME thing immediately, so a
	// pre-probe here would just double the latency for zero benefit — there's
	// no pick list to annotate.
	if len(matches) == 1 {
		return orderPreset(d, matches[0], st)
	}

	// Several share the name — the pick list is about to show one option per
	// preset, and picking a sold-out one would otherwise only surface after
	// the cart push. Probe each candidate (sequential; downstream calls are
	// already rate-limited) so the list can mark them upfront.
	avail := probeAll(d.Backend, matches)

	// On a non-interactive stdin (pipe/redirect), just list — the user re-runs
	// with a number. Interactively, offer the picker.
	if !d.Interactive {
		listPresets(d.Out, name, matches, avail, st)
		fmt.Fprintf(d.Out, "\n%s\n", st.dim(fmt.Sprintf("run  console order %s <n>  to order one.", name)))
		return 0
	}
	i, ok := pickPreset(d, name, matches, avail, st)
	if !ok {
		fmt.Fprintf(d.Out, "%s\n", st.dim("cancelled."))
		return 0
	}
	return orderPreset(d, matches[i], st)
}

// probeAll runs probeAvailability for each preset in order. Sequential on
// purpose (see probeAvailability) — a probe failure never blocks, it just
// leaves that entry unmarked.
func probeAll(be Backend, ps []localstore.Preset) []availability {
	out := make([]availability, len(ps))
	for i, p := range ps {
		out[i] = probeAvailability(be, p)
	}
	return out
}

// soldOutSuffix renders the dim/warn " · sold out: <item>" tag for an
// unavailable candidate, or "" when available/unknown — same visual language
// as the rest of the CLI (a dim separator + a warn-coloured word, matching how
// renderCart marks a sold-out cart line).
func soldOutSuffix(a availability, st style) string {
	if !a.known || !a.unavailable {
		return ""
	}
	return "  " + st.dim("·") + " " + st.warn("sold out: "+a.itemName)
}

// orderPreset routes to the matching vertical's placement flow.
func orderPreset(d Deps, p localstore.Preset, st style) int {
	if p.IsInstamart() {
		return placeIMPreset(d, p, st)
	}
	return placePreset(d, p, st)
}

// listPresets prints the numbered presets sharing a name (short address).
// avail is optional (nil = no marking, used by the index-out-of-range path
// which isn't worth a probe) — when given, an unavailable candidate gets a
// trailing " · sold out: <item>" tag.
func listPresets(out io.Writer, name string, matches []localstore.Preset, avail []availability, st style) {
	fmt.Fprintf(out, "%s\n", st.dim(fmt.Sprintf("%d presets named %q:", len(matches), name)))
	for i, p := range matches {
		suffix := ""
		if i < len(avail) {
			suffix = soldOutSuffix(avail[i], st)
		}
		fmt.Fprintf(out, "  %s %s  %s %s %s %s%s\n",
			st.num(fmt.Sprintf("%d)", i+1)), st.head(p.RestaurantName),
			st.dim("·"), st.dim(shortAddr(p.AddrLine)),
			st.dim("·"), st.dim(summarize(p)), suffix)
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
	// UPI-first, COD fallback — the same payment resolution the TUI and MCP
	// use (Swiggy disabled the legacy Cash token for food 2026-07-09; most
	// accounts pay by scan-to-pay QR). None of these calls is ever retried.
	order, ok := placeFood(d, p.AddrID, st)
	if !ok {
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

// placeIMPreset mirrors placePreset step by step, but through the Instamart
// cart/checkout instead of Food's. Instamart's cart binds to the ADDRESS (not
// a restaurant) and adds a ₹99 minimum on top of the shared ₹1000 beta cap.
func placeIMPreset(d Deps, p localstore.Preset, st style) int {
	adjust := st.dim("open `console` to adjust.")
	items := localstore.PresetIMCartItems(p)
	// Push (replaces the whole Instamart cart), then pull the authoritative bill.
	if _, err := d.Backend.IMUpdateCart(p.AddrID, items); err != nil {
		fmt.Fprintf(d.Out, "%s\n%s\n", st.warn(fmt.Sprintf("%q isn't available right now (%v).", p.Name, err)), adjust)
		return 1
	}
	cart, err := d.Backend.IMGetCart()
	if err != nil {
		fmt.Fprintf(d.Out, "%s\n%s\n", st.warn(fmt.Sprintf("couldn't read the cart (%v).", err)), adjust)
		return 1
	}
	if unavailable := imUnavailableNames(cart); len(unavailable) > 0 {
		fmt.Fprintf(d.Out, "%s\n%s\n", st.warn(fmt.Sprintf("%q can't be ordered now — unavailable: %s.", p.Name, joinNames(unavailable))), adjust)
		return 1
	}
	if len(cart.Lines) == 0 {
		fmt.Fprintf(d.Out, "%s\n%s\n", st.warn(fmt.Sprintf("%q produced an empty cart (items may no longer exist).", p.Name)), adjust)
		return 1
	}

	renderIMCart(d.Out, p.AddrLine, cart, st)

	// Swiggy's MCP beta rejects checkout for carts ≥ ₹1000, same as Food's cap.
	if cart.Total >= swiggyBetaOrderCap {
		fmt.Fprintf(d.Out, "\n%s\n%s\n",
			st.warn(fmt.Sprintf("order is ₹%d — swiggy beta blocks instamart orders of ₹1000+.", cart.Total)),
			st.dim("place this one in the Swiggy app instead."))
		return 1
	}
	// Instamart also enforces a ₹99 minimum (MIN_ORDER_NOT_MET) — Food has none.
	if cart.Total < instamartMin {
		fmt.Fprintf(d.Out, "\n%s\n",
			st.warn(fmt.Sprintf("₹99 minimum on instamart — add ₹%d more.", instamartMin-cart.Total)))
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
	order, err := d.Backend.IMPlaceOrder(p.AddrID) // never retried
	if err != nil {
		fmt.Fprintf(d.Out, "%s\n%s\n", st.warn(fmt.Sprintf("order failed: %v", err)),
			st.dim("if you may have been charged, run `console status` before retrying."))
		return 1
	}
	// Force-clear the server cart after placement: checkout normally consumes
	// it, but leftovers have been seen live lingering in the Swiggy app cart.
	// Best-effort — clear_cart maps "Cart not found" (already empty) to success.
	_ = d.Backend.IMClearCart()
	etaLo, etaHi := localstore.ParseETAMinutes(order.ETA)
	active := localstore.ActiveOrder{
		OrderID: order.ID, Restaurant: "Instamart", AddrLine: p.AddrLine,
		ETALoMin: etaLo, ETAHiMin: etaHi, Total: order.Total, PlacedAt: time.Now().Unix(),
		Vertical: "instamart",
		// The cart's selectedAddressDetails is the ONLY source of the delivery
		// coordinates track_order requires — get_addresses and get_orders both
		// omit them (harvested 2026-07-03).
		Lat: cart.AddrLat, Lng: cart.AddrLng,
	}
	_ = localstore.SaveActiveOrder(active)
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

func imUnavailableNames(c api.IMCart) []string {
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
