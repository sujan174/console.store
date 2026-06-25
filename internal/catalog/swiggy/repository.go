package swiggy

import "console.store/internal/catalog"

// Repository implements catalog.Repository over a Snapshot. Reads are sync and
// never do I/O; a cache miss returns the zero value (empty list / ok=false),
// which the screens already render as an empty/loading state.
type Repository struct{ snap *Snapshot }

func NewRepository(snap *Snapshot) *Repository { return &Repository{snap: snap} }

func (r *Repository) Addresses() []catalog.Address { return r.snap.getAddresses() }

func (r *Repository) Places(addr catalog.Address, section catalog.Section) []catalog.Place {
	return r.snap.getPlaces(addr.ID, section)
}

func (r *Repository) Menu(placeID string) (catalog.Place, bool) {
	return r.snap.getMenu(placeID)
}

// Usual/Trending are not yet sourced live; they are absent on the live backend,
// which the menu screen renders without a "usual".
func (r *Repository) Usual(addr catalog.Address) (catalog.Usual, bool) {
	return catalog.Usual{}, false
}

func (r *Repository) Trending(addr catalog.Address) (catalog.Trending, bool) {
	return catalog.Trending{}, false
}

func (r *Repository) InstamartItems(addr catalog.Address) []catalog.Item {
	return r.snap.getInstamart(addr.ID)
}
