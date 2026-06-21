// internal/catalog/schema.go
package catalog

// Section is a top-level catalogue lane.
type Section string

const (
	SectionCoffee    Section = "coffee"
	SectionFood      Section = "food"
	SectionSnacks    Section = "snacks"
	SectionInstamart Section = "instamart"
)

// MenuSections is the ordered set shown in the menu tab strip
// (Instamart is a separate lane, not a tab).
var MenuSections = []Section{SectionCoffee, SectionFood, SectionSnacks}

// Address is a delivery address. Lat/Lng are required by Swiggy
// search_restaurants later; empty in mock.
type Address struct {
	ID    string
	Label string // "home", "work", "mom"
	City  string
	Line  string // short locality, e.g. "HSR Layout"
	Full  string // full formatted address (Swiggy needs this)
	Lat   float64
	Lng   float64
}

// Item is one orderable item. SwiggyID maps to a live menu item later.
type Item struct {
	ID       string
	SwiggyID string // live Swiggy menu-item id; empty in mock
	Name     string
	Price    int    // whole rupees
	Tag      string // "", "new"
	Veg      bool
	Desc     string  // one-line flavour/ingredient note (shown on selection)
	Kcal     int     // calories; 0 = unknown
	Rating   float64 // out of 5; 0 = unknown
	Section  Section
}

// Place is a restaurant/store. SwiggyID maps to a live restaurant id.
// ServesAddressIDs models per-address serviceability in mock; later this
// comes from a live search_restaurants call.
type Place struct {
	ID               string
	SwiggyID         string
	Name             string
	City             string
	Section          Section
	ETA              string // "35-45 min"
	Fav              bool
	Rating           float64
	Items            []Item
	ServesAddressIDs []string
}

// Usual is the pinned one-tap reorder for an address.
type Usual struct {
	PlaceID string
	Item    Item
	Label   string // "Cold Coffee · Blue Tokai"
}
