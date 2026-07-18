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
	// vertical is "" (food) or "instamart". Routes place_order to the right
	// Backend methods and re-verification path.
	vertical string
}

// orderIdentity carries the real Swiggy identity a placement should record on the
// taste card. restaurantID is empty for ad-hoc orders (cart carries only a name),
// which makes bumpFavorite correctly skip — we never key a favorite by a name.
// vertical is "" (food) or "instamart"; propagated onto the pendingOrder so
// place_order knows how to re-verify and place.
type orderIdentity struct {
	restaurantID   string
	restaurantName string
	addrLabel      string
	vertical       string
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

// sweepLocked drops confirmations older than the TTL. The caller holds s.mu.
// Called on every put so abandoned confirmations (prepared, never placed) don't
// accumulate for the lifetime of a long-lived `console mcp` stdio server.
func (s *confirmStore) sweepLocked(nowUnix int64) {
	for id, p := range s.m {
		if nowUnix-p.createdAt > confirmTTLSeconds {
			delete(s.m, id)
		}
	}
}

func (s *confirmStore) put(addressID string, c api.Cart, ident orderIdentity, nowUnix int64) string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	id := hex.EncodeToString(b[:])
	s.mu.Lock()
	s.sweepLocked(nowUnix)
	s.m[id] = pendingOrder{
		addressID: addressID, total: c.Total,
		restaurantID: ident.restaurantID, restaurantName: ident.restaurantName, addrLabel: ident.addrLabel,
		hash: cartHash(addressID, c), createdAt: nowUnix, vertical: ident.vertical,
	}
	s.mu.Unlock()
	return id
}

// imCartHash binds a confirmation to the exact Instamart lines + address +
// total the user saw. Mirrors cartHash but keys lines by spinId.
func imCartHash(addressID string, c api.IMCart) string {
	type kv struct {
		id  string
		qty int
	}
	lines := make([]kv, 0, len(c.Lines))
	for _, l := range c.Lines {
		lines = append(lines, kv{l.SpinID, l.Quantity})
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].id < lines[j].id })
	h := sha256.New()
	fmt.Fprintf(h, "addr=%s;total=%d;", addressID, c.Total)
	for _, l := range lines {
		fmt.Fprintf(h, "%s:%d;", l.id, l.qty)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// putIM mints a confirmation for an Instamart cart. Same TTL/lookup discipline
// as put; vertical is always "instamart" so place_order routes correctly.
func (s *confirmStore) putIM(addressID string, c api.IMCart, ident orderIdentity, nowUnix int64) string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	id := hex.EncodeToString(b[:])
	s.mu.Lock()
	s.sweepLocked(nowUnix)
	s.m[id] = pendingOrder{
		addressID: addressID, total: c.Total,
		restaurantID: ident.restaurantID, restaurantName: ident.restaurantName, addrLabel: ident.addrLabel,
		hash: imCartHash(addressID, c), createdAt: nowUnix, vertical: "instamart",
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
