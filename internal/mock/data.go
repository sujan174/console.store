package mock

// Item is a single orderable menu item. Price is in whole rupees.
type Item struct {
	ID    string
	Name  string
	Price int
	Tag   string // "", "new"
}

// Restaurant carries a delivery-time *window* (Food is standard ~30-60 min).
type Restaurant struct {
	ID    string
	Name  string
	City  string
	ETA   string // e.g. "35-45 min"
	Fav   bool
	Items []Item
}

type Address struct {
	ID    string
	Label string // home / work
	City  string
	Line  string // "HSR Layout"
}

var Addresses = []Address{
	{"a1", "home", "Bangalore", "HSR Layout"},
	{"a2", "work", "Bangalore", "Koramangala"},
	{"a3", "mom", "Bangalore", "Indiranagar"},
}

var Restaurants = []Restaurant{
	{"r1", "Blue Tokai", "Bangalore", "35-45 min", true, []Item{
		{"i1", "Cold Coffee", 149, ""},
		{"i2", "Hazelnut Cold Brew", 169, ""},
		{"i3", "Vietnamese Cold Brew", 159, "new"},
		{"i4", "Almond Croissant", 129, ""},
		{"i5", "Banana Bread Slice", 99, ""},
	}},
	{"r2", "Third Wave", "Bangalore", "30-40 min", false, []Item{
		{"i6", "Flat White", 159, ""},
		{"i7", "Filter Coffee", 99, ""},
	}},
	{"r3", "Sleepy Owl", "Bangalore", "40-50 min", false, []Item{
		{"i8", "Cold Brew Original", 129, "new"},
		{"i9", "Mocha Cold Brew", 149, ""},
	}},
	{"r4", "Subko", "Bangalore", "45-55 min", false, []Item{
		{"i10", "Single-Origin Pour", 179, ""},
		{"i11", "Cardamom Bun", 139, ""},
	}},
}

// Usual returns the pinned "the usual" item. In Plan 1 it is hardcoded;
// later plans derive it from real order history.
func Usual() (Item, bool) {
	return Restaurants[0].Items[0], true // Cold Coffee · Blue Tokai
}
