// Package swiggy provides a live, broker-backed catalog.Repository. A per-session
// Snapshot caches catalog data; the TUI's datasource fills it via async Cmds and
// the Repository reads it synchronously. It imports no TUI code (the catalog
// layer must not depend on tui).
package swiggy

import (
	"sync"

	"console.store/internal/catalog"
)

type placeKey struct {
	addrID string
	key    string // chip query (or legacy section string)
}

// Snapshot is the per-session cache the live Repository reads. All access is
// mutex-guarded so the async fill Cmds and the synchronous Repository reads
// (which run on the bubbletea update goroutine) never race.
type Snapshot struct {
	mu        sync.RWMutex
	addresses []catalog.Address
	places    map[placeKey][]catalog.Place
	menus     map[string]catalog.Place // by place id
	instamart map[string][]catalog.Item
}

func NewSnapshot() *Snapshot {
	return &Snapshot{
		places:    map[placeKey][]catalog.Place{},
		menus:     map[string]catalog.Place{},
		instamart: map[string][]catalog.Item{},
	}
}

func (s *Snapshot) SetAddresses(a []catalog.Address) {
	s.mu.Lock()
	s.addresses = a
	s.mu.Unlock()
}

func (s *Snapshot) SetPlaces(addrID, key string, places []catalog.Place) {
	s.mu.Lock()
	s.places[placeKey{addrID, key}] = places
	s.mu.Unlock()
}

func (s *Snapshot) SetMenu(p catalog.Place) {
	s.mu.Lock()
	s.menus[p.ID] = p
	s.mu.Unlock()
}

func (s *Snapshot) SetInstamart(addrID string, items []catalog.Item) {
	s.mu.Lock()
	s.instamart[addrID] = items
	s.mu.Unlock()
}

func (s *Snapshot) getAddresses() []catalog.Address {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.addresses
}

func (s *Snapshot) getPlaces(addrID, key string) []catalog.Place {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.places[placeKey{addrID, key}]
}

func (s *Snapshot) getMenu(placeID string) (catalog.Place, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.menus[placeID]
	return p, ok
}

func (s *Snapshot) getInstamart(addrID string) []catalog.Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.instamart[addrID]
}
