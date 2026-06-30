package mcp

import (
	"context"
	"errors"
	"testing"

	"consolestore/internal/broker/api"
)

func TestPrepareThenPlaceSucceeds(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{
		cart:  api.Cart{Restaurant: "McDonald's", Total: 250, Lines: []api.CartLine{{ItemID: "i1", Quantity: 1, Price: 250, Available: true}}},
		order: api.Order{ID: "OID1", Status: "placed", Restaurant: "McDonald's", Total: 250, ETA: "30 mins"},
	}
	s := NewServer(be, &fakeAuth{token: true})

	_, prep, err := s.handlePrepareOrder(context.Background(), nil, PrepareOrderIn{AddressID: "a1"})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if prep.ConfirmationID == "" || prep.Bill.Total != 250 {
		t.Fatalf("prep = %+v", prep)
	}
	_, plc, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: prep.ConfirmationID})
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	if plc.Order.ID != "OID1" {
		t.Fatalf("order = %+v", plc.Order)
	}
	if be.placed != 1 {
		t.Fatalf("placed = %d, want 1", be.placed)
	}
}

func TestPlaceRejectsUnknownConfirmation(t *testing.T) {
	be := &fakeBackend{cart: api.Cart{Total: 250}}
	s := NewServer(be, &fakeAuth{token: true})
	_, _, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: "nope"})
	if err == nil {
		t.Fatalf("expected rejection for unknown confirmation id")
	}
	if be.placed != 0 {
		t.Fatalf("placed = %d, want 0", be.placed)
	}
}

func TestPlaceRejectsChangedCart(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{cart: api.Cart{Restaurant: "X", Total: 250, Lines: []api.CartLine{{ItemID: "i1", Quantity: 1, Available: true}}}}
	s := NewServer(be, &fakeAuth{token: true})
	_, prep, _ := s.handlePrepareOrder(context.Background(), nil, PrepareOrderIn{AddressID: "a1"})
	be.cart.Total = 999 // cart drifted after prepare
	_, _, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: prep.ConfirmationID})
	if err == nil {
		t.Fatalf("expected rejection for changed cart")
	}
	if be.placed != 0 {
		t.Fatalf("placed = %d, want 0", be.placed)
	}
}

func TestPlaceOrderNeverRetries(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{
		cart:     api.Cart{Restaurant: "X", Total: 250, Lines: []api.CartLine{{ItemID: "i1", Quantity: 1, Available: true}}},
		placeErr: errors.New("502 bad gateway"),
	}
	s := NewServer(be, &fakeAuth{token: true})
	_, prep, _ := s.handlePrepareOrder(context.Background(), nil, PrepareOrderIn{AddressID: "a1"})
	_, _, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: prep.ConfirmationID})
	if err == nil {
		t.Fatalf("expected place error")
	}
	if be.placed != 1 {
		t.Fatalf("PlaceOrder called %d times, want exactly 1 (no retry)", be.placed)
	}
}
