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

// Choice is one selectable option within an OptionGroup (a Swiggy variation or
// addon choice). For a variant, Price is the FULL item price for that choice;
// for an addon, Price is the extra it adds.
type Choice struct {
	ID      string
	Name    string
	Price   int
	InStock bool
}

// OptionGroup is a customization group for a live item — a Swiggy variant group
// ("Choose Your Size", single-choice, price-setting) or addon group ("Choice of
// Milk", with min/max constraints). Min>0 means a selection is required.
type OptionGroup struct {
	ID      string
	Name    string
	Min     int  // minimum selections required (1 = required)
	Max     int  // maximum selectable (1 = single-choice; 0/<0 = unlimited)
	Variant bool // true = variant group (sets price); false = addon group (additive)
	Choices []Choice
}

// Selection is a chosen Choice within a group, carried on a cart line for both
// price display and the cart-send payload (group/choice ids).
type Selection struct {
	GroupID  string
	ChoiceID string
	Name     string
	Price    int
	Variant  bool
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
	AddOns   []AddOn // mock customizations (flat toggles); empty = not customizable

	// Live customization. Customizable is set from the menu's hasAddons/hasVariants
	// flags so the TUI knows to fetch Options (via search_menu) before adding.
	// Options is filled on demand by that fetch; empty until then.
	Customizable bool
	Options      []OptionGroup
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
