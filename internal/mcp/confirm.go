package mcp

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"

	"consolestore/internal/broker/api"
)

const confirmTTLSeconds = 600 // 10 minutes

type pendingOrder struct {
	addressID      string
	restaurantID   string // "" when no Swiggy id is known (ad-hoc prepare_order)
	restaurantName string
	addrLabel      string
	total          int
	hash           string
	createdAt      int64
}

// orderIdentity carries the real Swiggy identity a placement should record on the
// taste card. restaurantID is empty for ad-hoc orders (cart carries only a name),
// which makes bumpFavorite correctly skip — we never key a favorite by a name.
type orderIdentity struct {
	restaurantID   string
	restaurantName string
	addrLabel      string
}

type confirmStore struct {
	mu sync.Mutex
	m  map[string]pendingOrder
}

func newConfirmStore() *confirmStore { return &confirmStore{m: map[string]pendingOrder{}} }

// cartHash binds a confirmation to the exact lines + address + total the user saw.
func cartHash(addressID string, c api.Cart) string {
	type kv struct {
		id  string
		qty int
	}
	lines := make([]kv, 0, len(c.Lines))
	for _, l := range c.Lines {
		lines = append(lines, kv{l.ItemID, l.Quantity})
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].id < lines[j].id })
	h := sha256.New()
	fmt.Fprintf(h, "addr=%s;total=%d;", addressID, c.Total)
	for _, l := range lines {
		fmt.Fprintf(h, "%s:%d;", l.id, l.qty)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (s *confirmStore) put(addressID string, c api.Cart, ident orderIdentity, nowUnix int64) string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	id := hex.EncodeToString(b[:])
	s.mu.Lock()
	s.m[id] = pendingOrder{
		addressID: addressID, total: c.Total,
		restaurantID: ident.restaurantID, restaurantName: ident.restaurantName, addrLabel: ident.addrLabel,
		hash: cartHash(addressID, c), createdAt: nowUnix,
	}
	s.mu.Unlock()
	return id
}

// take removes and returns the pending order if present and not expired.
func (s *confirmStore) take(id string, nowUnix int64) (pendingOrder, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.m[id]
	if !ok {
		return pendingOrder{}, false
	}
	delete(s.m, id)
	if nowUnix-p.createdAt > confirmTTLSeconds {
		return pendingOrder{}, false
	}
	return p, true
}
