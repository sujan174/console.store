package swiggy

import (
	"testing"

	"console.store/internal/catalog"
)

func TestSnapshotPlacesByQueryKey(t *testing.T) {
	s := NewSnapshot()
	s.SetPlaces("addr-1", "pizza", []catalog.Place{{ID: "r1", Name: "Pizza Hut"}})
	r := NewRepository(s)
	got := r.PlacesByQuery(catalog.Address{ID: "addr-1"}, "pizza")
	if len(got) != 1 || got[0].ID != "r1" {
		t.Fatalf("PlacesByQuery = %+v", got)
	}
	if len(r.PlacesByQuery(catalog.Address{ID: "addr-1"}, "biryani")) != 0 {
		t.Fatal("unexpected places for a different query key")
	}
}
