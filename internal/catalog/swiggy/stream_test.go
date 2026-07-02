package swiggy

import (
	"testing"

	"consolestore/internal/catalog"
)

func items(ids ...string) []catalog.Item {
	out := make([]catalog.Item, len(ids))
	for i, id := range ids {
		out[i] = catalog.Item{ID: id, Name: "item-" + id}
	}
	return out
}

func TestMergeMenuPageReplacesAndAppends(t *testing.T) {
	s := NewSnapshot()
	s.SetMenu(catalog.Place{ID: "r1", Items: items("stale")})

	if n := s.MergeMenuPage("r1", items("a", "b"), true, false); n != 2 {
		t.Fatalf("page1 replace: count = %d, want 2 (stale seed dropped)", n)
	}
	if n := s.MergeMenuPage("r1", items("b", "c"), false, false); n != 3 {
		t.Fatalf("page2 append: count = %d, want 3 (dup b skipped)", n)
	}
	p, ok := s.getMenu("r1")
	if !ok || len(p.Items) != 3 || p.Items[0].ID != "a" || p.Items[2].ID != "c" {
		t.Fatalf("merged menu = %+v", p.Items)
	}
}

func TestStagedMenuPromoteSwapsAtomically(t *testing.T) {
	s := NewSnapshot()
	// Visible menu = disk seed; refresh streams into staging.
	s.SetMenu(catalog.Place{ID: "r1", Items: items("cached1", "cached2", "cached3")})
	s.MergeMenuPage("r1", items("fresh1"), true, true)

	// Mid-stream the visible menu must be untouched.
	if p, _ := s.getMenu("r1"); len(p.Items) != 3 {
		t.Fatalf("visible menu changed mid-stage: %+v", p.Items)
	}

	s.MergeMenuPage("r1", items("fresh2"), false, true)
	s.PromoteStagedMenu("r1")
	p, _ := s.getMenu("r1")
	if len(p.Items) != 2 || p.Items[0].ID != "fresh1" || p.Items[1].ID != "fresh2" {
		t.Fatalf("promoted menu = %+v, want the staged refresh", p.Items)
	}
	// Staging slot cleared; a second promote is a no-op.
	s.PromoteStagedMenu("r1")
	if p, _ := s.getMenu("r1"); len(p.Items) != 2 {
		t.Fatalf("second promote mutated menu: %+v", p.Items)
	}
}

func TestDropStagedMenuDiscardsRefresh(t *testing.T) {
	s := NewSnapshot()
	s.SetMenu(catalog.Place{ID: "r1", Items: items("cached")})
	s.MergeMenuPage("r1", items("fresh"), true, true)
	s.DropStagedMenu("r1")
	s.PromoteStagedMenu("r1") // nothing staged — must not clobber
	if p, _ := s.getMenu("r1"); len(p.Items) != 1 || p.Items[0].ID != "cached" {
		t.Fatalf("menu after drop = %+v, want the cached seed intact", p.Items)
	}
}

func TestMergePlacesPageReplacesAndDedups(t *testing.T) {
	s := NewSnapshot()
	seed := []catalog.Place{{ID: "seeded"}}
	s.SetPlaces("a1", "pizza", seed)

	if n := s.MergePlacesPage("a1", "pizza", []catalog.Place{{ID: "p1"}, {ID: "p2"}}, true); n != 2 {
		t.Fatalf("page1 replace: count = %d, want 2", n)
	}
	if n := s.MergePlacesPage("a1", "pizza", []catalog.Place{{ID: "p2"}, {ID: "p3"}}, false); n != 3 {
		t.Fatalf("page2 append: count = %d, want 3 (dup p2 skipped)", n)
	}
	got := s.getPlaces("a1", "pizza")
	if len(got) != 3 || got[0].ID != "p1" || got[2].ID != "p3" {
		t.Fatalf("merged places = %+v", got)
	}
}
