package cli

import (
	"fmt"

	"console.store/internal/broker/api"
)

// runStatus prints the account's live orders with their richest available status
// (active list + per-order tracking line), or "no live orders".
func runStatus(d Deps) int {
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
		fmt.Fprintln(d.Out, "no live orders.")
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
		printOrderStatus(d, o, status, eta)
	}
	return 0
}

func printOrderStatus(d Deps, o api.Order, status, eta string) {
	fmt.Fprintf(d.Out, "order %s — %s\n", o.ID, o.Restaurant)
	fmt.Fprintf(d.Out, "  status: %s\n", status)
	if eta != "" {
		fmt.Fprintf(d.Out, "  eta:    %s\n", eta)
	}
	if o.Total > 0 {
		fmt.Fprintf(d.Out, "  total:  ₹%d\n", o.Total)
	}
}
