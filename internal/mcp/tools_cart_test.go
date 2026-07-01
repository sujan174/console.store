package mcp

import (
	"context"
	"testing"

	"consolestore/internal/broker/api"
)

func TestGetCartReturnsBill(t *testing.T) {
	be := &fakeBackend{cart: api.Cart{Total: 250, ItemTotal: 200, Delivery: 30, Taxes: 20,
		Lines: []api.CartLine{{ItemID: "i1", Name: "Burger", Quantity: 1, Price: 200, Available: true}}}}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleGetCart(context.Background(), nil, GetCartIn{AddressID: "a1"})
	if err != nil {
		t.Fatalf("handleGetCart: %v", err)
	}
	if out.Cart.Total != 250 || len(out.Cart.Lines) != 1 || out.Cart.Lines[0].Name != "Burger" {
		t.Fatalf("cart = %+v", out.Cart)
	}
}

func TestUpdateCartReturnsCart(t *testing.T) {
	be := &fakeBackend{cart: api.Cart{Total: 200}}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleUpdateCart(context.Background(), nil, UpdateCartIn{
		AddressID: "a1", RestaurantID: "r1",
		Items: []CartItemIn{{ItemID: "i1", Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("handleUpdateCart: %v", err)
	}
	if out.Cart.Total != 200 {
		t.Fatalf("total = %d", out.Cart.Total)
	}
}
