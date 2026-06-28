package cli

import (
	"fmt"
	"strconv"
	"time"

	"console.store/internal/broker/api"
	"console.store/internal/localstore"
)

// runOrder resolves preset(s) named `name`, pushes the chosen one into the cart
// (overriding any existing cart), shows the live bill, confirms, and places
// (armed) or no-ops (disarmed/safestore).
func runOrder(d Deps, name string) int {
	ps, err := localstore.LoadPresets()
	if err != nil {
		fmt.Fprintf(d.Out, "store: %v\n", err)
		return 1
	}
	matches := ps.ByName(name)
	switch len(matches) {
	case 0:
		fmt.Fprintf(d.Out, "no preset %q.\ncreate one in the app: open store, build a cart, then `:alias set %s`\n", name, name)
		return 1
	case 1:
		return placePreset(d, matches[0])
	default:
		fmt.Fprintf(d.Out, "%d presets named %q:\n", len(matches), name)
		for i, p := range matches {
			fmt.Fprintf(d.Out, "  %d) %s · %s · %s\n", i+1, p.RestaurantName, p.AddrLine, summarize(p))
		}
		fmt.Fprintf(d.Out, "pick 1-%d: ", len(matches))
		sel := prompt(d)
		n, perr := strconv.Atoi(sel)
		if perr != nil || n < 1 || n > len(matches) {
			fmt.Fprintln(d.Out, "store: invalid choice — aborted.")
			return 1
		}
		return placePreset(d, matches[n-1])
	}
}

func placePreset(d Deps, p localstore.Preset) int {
	items := presetToCartItems(p)
	// Push (override any existing cart), then pull the authoritative cart/bill.
	if _, err := d.Backend.UpdateCart(p.AddrID, p.RestaurantID, p.RestaurantName, items); err != nil {
		fmt.Fprintf(d.Out, "store: %q isn't available right now (%v).\nopen `store` to adjust.\n", p.Name, err)
		return 1
	}
	cart, err := d.Backend.GetCart(p.AddrID, p.RestaurantName)
	if err != nil {
		fmt.Fprintf(d.Out, "store: couldn't read the cart (%v).\nopen `store` to adjust.\n", err)
		return 1
	}
	if unavailable := unavailableNames(cart); len(unavailable) > 0 {
		fmt.Fprintf(d.Out, "store: %q can't be ordered now — unavailable: %s.\nopen `store` to adjust.\n", p.Name, joinNames(unavailable))
		return 1
	}
	if len(cart.Lines) == 0 {
		fmt.Fprintf(d.Out, "store: %q produced an empty cart (items may no longer exist).\nopen `store` to adjust.\n", p.Name)
		return 1
	}

	renderCart(d.Out, p.AddrLine, p.RestaurantName, cart)

	if !d.Armed {
		fmt.Fprintln(d.Out, "\nbrowse-only build — order NOT placed.\nrun the armed `store` to place, or open `store` to adjust the cart.")
		return 0
	}

	fmt.Fprint(d.Out, "\npress Enter to place this order · Ctrl-C to cancel ")
	_ = prompt(d)                                // any line (incl. empty Enter) confirms; Ctrl-C kills the process
	order, err := d.Backend.PlaceOrder(p.AddrID) // never retried
	if err != nil {
		fmt.Fprintf(d.Out, "store: order failed: %v\n", err)
		fmt.Fprintln(d.Out, "if you may have been charged, run `store status` before retrying.")
		return 1
	}
	etaLo, etaHi := localstore.ParseETAMinutes(order.ETA)
	_ = localstore.SaveActiveOrder(localstore.ActiveOrder{
		OrderID: order.ID, Restaurant: p.RestaurantName, AddrLine: p.AddrLine,
		ETALoMin: etaLo, ETAHiMin: etaHi, Total: order.Total, PlacedAt: time.Now().Unix(),
	})
	fmt.Fprintf(d.Out, "\n✓ order placed — %s", order.ID)
	if order.ETA != "" {
		fmt.Fprintf(d.Out, " · eta %s", order.ETA)
	}
	fmt.Fprintln(d.Out)
	return 0
}

// presetToCartItems maps a preset's lines to api.CartItem, replaying the exact
// channel routing the TUI uses (variantsV2 / variantsLegacy / addons).
func presetToCartItems(p localstore.Preset) []api.CartItem {
	out := make([]api.CartItem, 0, len(p.Lines))
	for _, l := range p.Lines {
		ci := api.CartItem{ItemID: l.ItemID, Quantity: l.Qty}
		for _, s := range l.Sels {
			switch {
			case s.Variant && s.Absolute:
				ci.VariantsV2 = append(ci.VariantsV2, api.CartVariantSel{GroupID: s.GroupID, VariationID: s.ChoiceID})
			case s.Variant:
				ci.VariantsLegacy = append(ci.VariantsLegacy, api.CartVariantSel{GroupID: s.GroupID, VariationID: s.ChoiceID})
			default:
				ci.Addons = append(ci.Addons, api.CartAddonSel{GroupID: s.GroupID, ChoiceID: s.ChoiceID})
			}
		}
		out = append(out, ci)
	}
	return out
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
