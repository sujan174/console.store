package api

// Tracking is a live order's status + ETA parsed from track_food_order.
type Tracking struct {
	OrderID string
	Status  string
	ETA     string
	Active  bool
}
