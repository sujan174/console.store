package cli

import (
	"fmt"
	"strings"

	"consolestore/internal/broker/api"
	"consolestore/internal/localstore"
)

// runStatus prints the account's live orders with their richest available status
// (active list + per-order tracking line), or "no live orders". Food and
// Instamart are independent verticals — both are checked and printed together.
func runStatus(d Deps) int {
	st := newStyle(d.Color)
	// Neither vertical's failure may hide the other's live orders (an in-flight
	// COD order is real money with no cancellation — the user MUST see it). A
	// food-side failure degrades to a warning line; Instamart is still checked.
	foodErr := ""
	printed := false
	if addrID, err := firstAddressID(d); err != nil {
		foodErr = err.Error()
	} else if orders, err := d.Backend.ActiveOrders(addrID); err != nil {
		foodErr = fmt.Sprintf("couldn't fetch orders: %v", err)
	} else {
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
			printed = true
		}
	}
	if foodErr != "" {
		fmt.Fprintf(d.Out, "%s\n", st.dim(fmt.Sprintf("(food status unavailable: %s)", foodErr)))
	}

	// Instamart errors must never break Food's status output — surface a dim
	// warning line instead of failing the whole command.
	imOrders, imErr := d.Backend.IMOrders(true)
	if imErr != nil {
		fmt.Fprintf(d.Out, "%s\n", st.dim(fmt.Sprintf("(instamart status unavailable: %v)", imErr)))
	}
	// get_orders carries NO coordinates (harvested live) — the persisted
	// ActiveOrder holds the ones captured from the cart at placement time.
	saved, savedOK, _ := localstore.LoadActiveOrder()
	for _, o := range imOrders {
		status, eta := o.Status, o.ETA
		// track_order requires coordinates; use the order's own when present
		// (drift tolerance), else the persisted ones for the order WE placed.
		// Without any, fall back to the Status/ETA already in IMOrders.
		detail := o.Detail
		if o.Lat == 0 && o.Lng == 0 && savedOK && saved.Vertical == "instamart" && saved.OrderID == o.ID {
			o.Lat, o.Lng = saved.Lat, saved.Lng
		}
		if o.Lat != 0 || o.Lng != 0 {
			if t, terr := d.Backend.IMTrack(o.ID, o.Lat, o.Lng); terr == nil {
				if t.Status != "" {
					status = t.Status
				}
				if t.ETA != "" {
					eta = t.ETA
				}
				if t.Detail != "" {
					detail = t.Detail
				}
			}
		}
		printOrderStatus(d, st, api.Order{ID: o.ID, Status: status, Restaurant: "Instamart", Total: o.Total}, status, eta)
		// Rider update line ("SANJAY J is on the way…") — Instamart-only detail.
		if detail != "" && !strings.EqualFold(detail, status) {
			fmt.Fprintf(d.Out, "  %s\n", st.dim(detail))
		}
		printed = true
	}

	if !printed {
		fmt.Fprintf(d.Out, "%s\n", st.dim("no live orders."))
	}
	if foodErr != "" && !printed {
		return 1
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
