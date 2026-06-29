package mem

import (
	"fmt"

	"consolestore/internal/catalog"
)

// Repo is the in-memory curated catalogue. It implements catalog.Repository.
type Repo struct {
	addresses []catalog.Address
	places    []catalog.Place
	instamart []catalog.Item
}

// New returns a Repo seeded with the curated mock data.
func New() *Repo {
	return &Repo{addresses: addresses, places: places, instamart: instamartItems}
}

func serves(p catalog.Place, addrID string) bool {
	for _, id := range p.ServesAddressIDs {
		if id == addrID {
			return true
		}
	}
	return false
}

func (r *Repo) Addresses() []catalog.Address { return r.addresses }

func (r *Repo) Places(addr catalog.Address, section catalog.Section) []catalog.Place {
	var out []catalog.Place
	for _, p := range r.places {
		if p.Section == section && serves(p, addr.ID) {
			out = append(out, p)
		}
	}
	return out
}

func (r *Repo) Menu(placeID string) (catalog.Place, bool) {
	for _, p := range r.places {
		if p.ID == placeID {
			return p, true
		}
	}
	return catalog.Place{}, false
}

func (r *Repo) Usual(addr catalog.Address) (catalog.Usual, bool) {
	if p, ok := r.Menu(usualPin.PlaceID); ok && serves(p, addr.ID) {
		for _, it := range p.Items {
			if it.ID == usualPin.ItemID {
				return catalog.Usual{PlaceID: p.ID, Item: it,
					Label: fmt.Sprintf("%s · %s", it.Name, p.Name)}, true
			}
		}
	}
	for _, p := range r.places {
		if serves(p, addr.ID) && len(p.Items) > 0 {
			it := p.Items[0]
			return catalog.Usual{PlaceID: p.ID, Item: it,
				Label: fmt.Sprintf("%s · %s", it.Name, p.Name)}, true
		}
	}
	return catalog.Usual{}, false
}

func (r *Repo) Trending(addr catalog.Address) (catalog.Trending, bool) {
	// Prefer the editorial pin when it's serviceable here.
	if p, ok := r.Menu(trendingPin.PlaceID); ok && serves(p, addr.ID) {
		for _, it := range p.Items {
			if it.ID == trendingPin.ItemID {
				return catalog.Trending{PlaceID: p.ID, Item: it, Count: trendingPin.Count, ETA: p.ETA}, true
			}
		}
	}
	// Fallback: the top-rated item among serviceable places.
	var best catalog.Trending
	found := false
	for _, p := range r.places {
		if !serves(p, addr.ID) {
			continue
		}
		for _, it := range p.Items {
			if !found || it.Rating > best.Item.Rating {
				best = catalog.Trending{PlaceID: p.ID, Item: it, Count: 200, ETA: p.ETA}
				found = true
			}
		}
	}
	return best, found
}

func (r *Repo) InstamartItems(addr catalog.Address) []catalog.Item {
	return r.instamart
}
