package mcp

import (
	"context"
	"testing"

	"consolestore/internal/localstore"
)

func TestGetPreviousOrders(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_ = localstore.AppendOrder("a1", localstore.PlacedOrder{RestaurantName: "BK", Total: 199})
	s := NewServer(&fakeBackend{}, &fakeAuth{token: true})
	_, out, err := s.handleGetPreviousOrders(context.Background(), nil, GetPreviousOrdersIn{AddressID: "a1"})
	if err != nil || len(out.Orders) != 1 || out.Orders[0].RestaurantName != "BK" {
		t.Fatalf("orders=%+v err=%v", out, err)
	}
}

func TestSetAddressDefaultLocks(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s := NewServer(&fakeBackend{}, &fakeAuth{token: true})
	_, out, err := s.handleSetAddress(context.Background(), nil, SetAddressIn{AddressID: "a2", Label: "Work", AsDefault: true})
	if err != nil || out.Active.ID != "a2" {
		t.Fatalf("set out=%+v err=%v", out, err)
	}
	ap, _ := localstore.LoadAddrPref()
	if !ap.Locked || ap.DefaultAddrID != "a2" {
		t.Fatalf("expected locked default a2, got %+v", ap)
	}
}
