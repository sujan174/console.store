package api

// Tracking is a live order's status + ETA parsed from track_food_order.
type Tracking struct {
	OrderID string
	Status  string
	// Detail is a secondary line under the status — Instamart's rider updates
	// ("SANJAY J is on the way to deliver your order"); "" for Food.
	Detail string
	ETA    string
	Active bool
	// Known is false when the tracking text couldn't be parsed at all — an
	// unknown reply must never be treated as a delivery signal that clears a
	// still-live order.
	Known bool
}
