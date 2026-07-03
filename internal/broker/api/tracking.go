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
}
