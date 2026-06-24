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

// AddOn is an optional customization for an item (a Swiggy "addon"). Price is
// the extra rupees it adds; 0 = free (e.g. "no sugar"). Each add-on is an
// independent toggle.
type AddOn struct {
	ID    string
	Name  string
	Price int // extra rupees; 0 = free
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
	AddOns   []AddOn // optional customizations; empty = not customizable
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
	Description      string // one-line "quick look" blurb; empty in older data
	Items            []Item
	ServesAddressIDs []string
}

// Usual is the pinned one-tap reorder for an address.
type Usual struct {
	PlaceID string
	Item    Item
	Label   string // "Cold Coffee · Blue Tokai"
}

// Trending is the hero "trending now" pick for an address.
type Trending struct {
	PlaceID string
	Item    Item
	Count   int    // orders today
	ETA     string // delivery window of its place
}
