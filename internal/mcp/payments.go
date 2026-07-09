package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"sync"

	"consolestore/internal/broker/api"
)

// paymentEntry pairs a live Swiggy pending payment with the bookkeeping context
// (address/taste identity/cart hash) minted at prepare time. Both are needed at
// confirm: `pay` to poll/confirm the money, `ord` to record the placement onto the
// active-order/taste/preset caches once the money clears.
type paymentEntry struct {
	pay api.PendingPayment
	ord pendingOrder
}

// paymentStore holds pending UPI payments between place_order (which starts the
// payment) and check_payment/confirm_order (which poll then finalize it). Unlike
// confirmStore, `get` PEEKS — the widget polls check_payment repeatedly, so the
// entry must survive until confirm_order succeeds or the caller drops it. Expiry is
// enforced by the caller against pay.ExpiresAt, not by a store TTL.
type paymentStore struct {
	mu sync.Mutex
	m  map[string]paymentEntry
}

func newPaymentStore() *paymentStore { return &paymentStore{m: map[string]paymentEntry{}} }

func (s *paymentStore) put(pay api.PendingPayment, ord pendingOrder) string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	id := hex.EncodeToString(b[:])
	s.mu.Lock()
	s.m[id] = paymentEntry{pay: pay, ord: ord}
	s.mu.Unlock()
	return id
}

// get peeks the entry without removing it (poll-safe).
func (s *paymentStore) get(id string) (paymentEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[id]
	return e, ok
}

// remove drops the entry — called after a successful confirm_order or when the
// window has closed, so a settled/dead payment can't be polled or confirmed again.
func (s *paymentStore) remove(id string) {
	s.mu.Lock()
	delete(s.m, id)
	s.mu.Unlock()
}
