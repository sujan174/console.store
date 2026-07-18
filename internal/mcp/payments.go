package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

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
	mu         sync.Mutex
	m          map[string]paymentEntry
	confirming map[string]bool // payment_ids with a confirm_order in flight
}

func newPaymentStore() *paymentStore {
	return &paymentStore{m: map[string]paymentEntry{}, confirming: map[string]bool{}}
}

func (s *paymentStore) put(pay api.PendingPayment, ord pendingOrder) string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	id := hex.EncodeToString(b[:])
	s.mu.Lock()
	// Sweep payments whose window has fully closed, so abandoned pending
	// payments (placed, never paid/confirmed) don't accumulate for the lifetime
	// of a long-lived `console mcp` server.
	now := time.Now().UnixMilli()
	for k, e := range s.m {
		if e.pay.ExpiresAt > 0 && now >= e.pay.ExpiresAt {
			delete(s.m, k)
			delete(s.confirming, k)
		}
	}
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

// claimConfirm atomically reserves an entry for a confirm_order attempt. It
// returns (entry, true) only when the entry exists AND no other confirm_order
// for the same payment_id is already in flight — so two concurrent
// confirm_order calls (the go-sdk dispatches tool calls asynchronously) can
// never both fire the non-idempotent ConfirmOrder for one payment. The caller
// MUST releaseConfirm on a failed attempt (to allow a later manual retry) or
// remove on success (so the settled payment can't be confirmed again).
func (s *paymentStore) claimConfirm(id string) (paymentEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[id]
	if !ok || s.confirming[id] {
		return paymentEntry{}, false
	}
	s.confirming[id] = true
	return e, true
}

// releaseConfirm clears the in-flight flag after a FAILED confirm so the caller
// can retry. A successful confirm calls remove instead.
func (s *paymentStore) releaseConfirm(id string) {
	s.mu.Lock()
	delete(s.confirming, id)
	s.mu.Unlock()
}

// remove drops the entry — called after a successful confirm_order or when the
// window has closed, so a settled/dead payment can't be polled or confirmed again.
func (s *paymentStore) remove(id string) {
	s.mu.Lock()
	delete(s.m, id)
	delete(s.confirming, id)
	s.mu.Unlock()
}
