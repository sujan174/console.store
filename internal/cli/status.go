package cli

import (
	"fmt"
	"strings"

	"consolestore/internal/broker/api"
)

// runStatus prints the account's live orders with their richest available status
// (active list + per-order tracking line), or "no live orders".
func runStatus(d Deps) int {
	st := newStyle(d.Color)
	addrID, err := firstAddressID(d)
	if err != nil {
		fmt.Fprintf(d.Out, "store: %v\n", err)
		return 1
	}
	orders, err := d.Backend.ActiveOrders(addrID)
	if err != nil {
		fmt.Fprintf(d.Out, "store: couldn't fetch orders: %v\n", err)
		return 1
	}
	if len(orders) == 0 {
		fmt.Fprintf(d.Out, "%s\n", st.dim("no live orders."))
		return 0
	}
	for _, o := range orders {
		status, eta := o.Status, ""
		// Enrich with the granular tracking line when available.
		if t, terr := d.Backend.TrackOrder(o.ID); terr == nil {
			if t.Status != "" {
				status = t.Status
			}
			eta = t.ETA
		}
		printOrderStatus(d, st, o, status, eta)
	}
	return 0
}

func printOrderStatus(d Deps, st style, o api.Order, status, eta string) {
	fmt.Fprintf(d.Out, "%s %s  %s\n", st.dim("order"), st.num(o.ID), st.head(o.Restaurant))
	fmt.Fprintf(d.Out, "  %s  %s\n", st.dim("status"), st.ok(status))
	if eta != "" && !strings.EqualFold(strings.TrimSpace(eta), "N/A") {
		fmt.Fprintf(d.Out, "  %s     %s\n", st.dim("eta"), st.link(eta))
	}
	if o.Total > 0 {
		fmt.Fprintf(d.Out, "  %s   %s\n", st.dim("total"), st.money(fmt.Sprintf("₹%d", o.Total)))
	}
}
