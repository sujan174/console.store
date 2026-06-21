// internal/catalog/repository.go
package catalog

// Repository is the catalogue data seam. Mock fills it now; Postgres+Swiggy
// fill it later behind the SAME interface so screens never change.
type Repository interface {
	// Addresses returns the signed-in user's saved addresses.
	Addresses() []Address
	// Places returns curated places in a section that are serviceable at addr.
	Places(addr Address, section Section) []Place
	// Menu returns a place (with items) by id.
	Menu(placeID string) (Place, bool)
	// Usual returns the pinned reorder for addr, if one is serviceable.
	Usual(addr Address) (Usual, bool)
	// Trending returns the hero "trending now" pick for addr, if serviceable.
	Trending(addr Address) (Trending, bool)
	// InstamartItems returns the flat curated Instamart list for addr.
	InstamartItems(addr Address) []Item
}
