// Package swiggy provides a live, broker-backed catalog.Repository. A per-session
// Snapshot caches catalog data; the TUI's datasource fills it via async Cmds and
// the Repository reads it synchronously. It imports no TUI code (the catalog
// layer must not depend on tui).
package swiggy

import (
	"sync"

	"consolestore/internal/catalog"
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
	// stagedMenus holds an in-flight refresh streaming BEHIND a disk-cache
	// seed shown in menus: pages accumulate here so the visible (seeded)
	// menu never shrinks mid-stream, then PromoteStagedMenu swaps it in
	// atomically when the stream completes.
	stagedMenus map[string]catalog.Place
	instamart   map[string][]catalog.Item
}

func NewSnapshot() *Snapshot {
	return &Snapshot{
		places:      map[placeKey][]catalog.Place{},
		menus:       map[string]catalog.Place{},
		stagedMenus: map[string]catalog.Place{},
		instamart:   map[string][]catalog.Item{},
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

// MergeMenuPage folds one streamed menu page into the cached menu. replace
// (page 1 of a fresh load) drops whatever was there — including a stale disk
// seed — while later pages append, deduplicated by item id so a retried page
// can't double an item. staged writes to the staging area instead of the
// visible menu (see stagedMenus). Returns the merged item count.
func (s *Snapshot) MergeMenuPage(placeID string, items []catalog.Item, replace, staged bool) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	tgt := s.menus
	if staged {
		tgt = s.stagedMenus
	}
	cur := tgt[placeID]
	cur.ID = placeID
	if replace {
		cur.Items = nil
	}
	seen := make(map[string]bool, len(cur.Items))
	for _, it := range cur.Items {
		seen[it.ID] = true
	}
	for _, it := range items {
		if it.ID == "" || seen[it.ID] {
			continue
		}
		seen[it.ID] = true
		cur.Items = append(cur.Items, it)
	}
	tgt[placeID] = cur
	return len(cur.Items)
}

// PromoteStagedMenu atomically swaps a completed staged refresh into the
// visible menu (and clears the staging slot). No-op when nothing is staged.
func (s *Snapshot) PromoteStagedMenu(placeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.stagedMenus[placeID]; ok {
		s.menus[placeID] = p
		delete(s.stagedMenus, placeID)
	}
}

// DropStagedMenu discards an abandoned staged refresh (stream died mid-way).
func (s *Snapshot) DropStagedMenu(placeID string) {
	s.mu.Lock()
	delete(s.stagedMenus, placeID)
	s.mu.Unlock()
}

// MergePlacesPage folds one streamed restaurant-search page into the cached
// list for (addrID, key), deduplicated by place id. replace starts the list
// fresh (page 1). Returns the merged count.
func (s *Snapshot) MergePlacesPage(addrID, key string, places []catalog.Place, replace bool) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := placeKey{addrID, key}
	cur := s.places[k]
	if replace {
		cur = nil
	}
	seen := make(map[string]bool, len(cur))
	for _, p := range cur {
		seen[p.ID] = true
	}
	for _, p := range places {
		if p.ID == "" || seen[p.ID] {
			continue
		}
		seen[p.ID] = true
		cur = append(cur, p)
	}
	s.places[k] = cur
	return len(cur)
}

// imKey scopes Instamart lists by BOTH address and query ("" = the go-to
// list). Keying by address alone let a slow search response overwrite the
// go-to list after the user had already escaped back to it — the stale msg was
// dropped but the snapshot was poisoned, so the next rebuild rendered search
// results under the "your usuals" view.
func imKey(addrID, query string) string { return addrID + "\x00" + query }

func (s *Snapshot) SetInstamart(addrID, query string, items []catalog.Item) {
	s.mu.Lock()
	s.instamart[imKey(addrID, query)] = items
	s.mu.Unlock()
}

// InstamartFor returns the Instamart list stored for an address + query
// ("" = the go-to list). The live TUI reads through this so a rebuild always
// renders the list matching its current query, never a raced write.
func (s *Snapshot) InstamartFor(addrID, query string) []catalog.Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.instamart[imKey(addrID, query)]
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

// getInstamart backs Repository.InstamartItems (which has no query param):
// it returns the go-to list — the query-scoped lists are read via InstamartFor.
func (s *Snapshot) getInstamart(addrID string) []catalog.Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.instamart[imKey(addrID, "")]
}
