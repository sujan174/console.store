package tui

import (
	"errors"
	"testing"

	swiggysnap "console.store/internal/catalog/swiggy"

	"console.store/internal/broker/api"
	"console.store/internal/catalog"
	"console.store/internal/tui/datasource"
	"console.store/internal/tui/render"
	"console.store/internal/tui/screens"
)

// liveModel builds a live Model with a seeded (confirmed) one-item cart.
func liveModel(t *testing.T) Model {
	t.Helper()
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "acct-1", ""))
	// Seed a confirmed baseline of one item from "Diner".
	out, _ := m.Update(datasource.CartPulledMsg{Cart: api.Cart{
		Restaurant: "Diner",
		Lines:      []api.CartLine{{ItemID: "1", Name: "Soup", Quantity: 1, Price: 100}},
	}})
	return out.(Model)
}

// A failed cart sync rolls the optimistic change back to the last confirmed
// state and surfaces an error — the local cart never keeps an item that didn't
// reach Swiggy.
func TestCartSyncFailureRollsBack(t *testing.T) {
	m := liveModel(t)
	if !m.cartConfirmed || len(m.confirmedLines) != 1 {
		t.Fatalf("baseline not confirmed: confirmed=%v lines=%d", m.cartConfirmed, len(m.confirmedLines))
	}
	// Optimistically add a second item locally (simulating an add that fired a sync).
	m.lines = append(m.lines, screens.CartLine{Item: catalog.Item{ID: "2", Name: "Bread"}, Qty: 1, Price: 50})
	if len(m.lines) != 2 {
		t.Fatal("setup: expected 2 optimistic lines")
	}
	// The sync comes back as an error.
	out, _ := m.Update(datasource.CartSyncedMsg{Err: errors.New("INVALID_ITEM")})
	m = out.(Model)

	if len(m.lines) != 1 || m.lines[0].Item.ID != "1" {
		t.Fatalf("failed sync must revert to the confirmed 1-item cart, got %+v", m.lines)
	}
	if m.cartSyncErr == "" {
		t.Fatal("a failed sync must surface an error to the user")
	}
	if m.cartMutating {
		t.Fatal("a failed sync must unfreeze input")
	}
}

// A successful cart sync commits the optimistic change as the new confirmed
// baseline, so a LATER failure rolls back to it (not to the older state).
func TestCartSyncSuccessAdvancesBaseline(t *testing.T) {
	m := liveModel(t)
	// Add a second item, then a successful sync confirms the 2-item cart.
	m.lines = append(m.lines, screens.CartLine{Item: catalog.Item{ID: "2", Name: "Bread"}, Qty: 1, Price: 50})
	out, _ := m.Update(datasource.CartSyncedMsg{Cart: api.Cart{Total: 150}})
	m = out.(Model)
	if len(m.confirmedLines) != 2 {
		t.Fatalf("success must advance the baseline to 2 items, got %d", len(m.confirmedLines))
	}
	if m.cartSyncErr != "" {
		t.Fatalf("success must clear any error, got %q", m.cartSyncErr)
	}
	// A subsequent optimistic add that fails rolls back to the 2-item baseline.
	m.lines = append(m.lines, screens.CartLine{Item: catalog.Item{ID: "3", Name: "Pie"}, Qty: 1})
	out, _ = m.Update(datasource.CartSyncedMsg{Err: errors.New("boom")})
	m = out.(Model)
	if len(m.lines) != 2 {
		t.Fatalf("rollback should restore the advanced 2-item baseline, got %d", len(m.lines))
	}
}

// The confirmed snapshot is independent of later in-place qty edits (clone, not
// alias) — a rollback restores the original quantity.
func TestConfirmedSnapshotNotAliased(t *testing.T) {
	m := liveModel(t)
	// Mutate qty in place the way decLastByItem/appendOrInc do.
	m.lines[0].Qty = 9
	out, _ := m.Update(datasource.CartSyncedMsg{Err: errors.New("boom")})
	m = out.(Model)
	if m.lines[0].Qty != 1 {
		t.Fatalf("rollback should restore qty 1 from the snapshot, got %d", m.lines[0].Qty)
	}
}
