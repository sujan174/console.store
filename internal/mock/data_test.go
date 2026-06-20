package mock

import "testing"

func TestSampleDataShape(t *testing.T) {
	if len(Addresses) == 0 {
		t.Fatal("want at least one address")
	}
	if len(Restaurants) < 3 {
		t.Fatalf("want >=3 restaurants, got %d", len(Restaurants))
	}
	for _, r := range Restaurants {
		if r.ETA == "" {
			t.Fatalf("restaurant %s missing ETA window", r.Name)
		}
		if len(r.Items) == 0 {
			t.Fatalf("restaurant %s has no items", r.Name)
		}
	}
	if u, ok := Usual(); !ok || u.Name == "" {
		t.Fatal("want a non-empty usual")
	}
}
