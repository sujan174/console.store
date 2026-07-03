package screens

import (
	"strings"
	"testing"
)

func TestRailEntriesOrder(t *testing.T) {
	r := NewRail([]string{"Coffee", "Pizza"})
	// ⌕ Search, Home, Coffee, Pizza
	if r.Len() != 4 {
		t.Fatalf("expected 4 entries, got %d", r.Len())
	}
	if !strings.Contains(r.EntryLabel(RailSearch), "Search") {
		t.Errorf("index 0 must be Search: %q", r.EntryLabel(RailSearch))
	}
	if r.EntryLabel(RailHome) != "Home" {
		t.Errorf("index 1 must be Home: %q", r.EntryLabel(RailHome))
	}
	if r.EntryLabel(2) != "Coffee" {
		t.Errorf("categories start at index 2: %q", r.EntryLabel(2))
	}
}

func TestRailViewShowsEntriesAndSearchIcon(t *testing.T) {
	v := NewRail([]string{"Coffee"}).WithActive(RailHome).WithHeight(10).View()
	for _, want := range []string{"⌕", "Search", "Home", "Coffee"} {
		if !strings.Contains(v, want) {
			t.Errorf("rail view missing %q:\n%s", want, v)
		}
	}
}

func TestRailFixedWidth(t *testing.T) {
	r := NewRail([]string{"Coffee"})
	if r.Width() < 10 {
		t.Fatalf("rail width should be a sensible fixed column, got %d", r.Width())
	}
}

// A Home-less rail (Instamart): ⌕ Search then categories, no Home slot. The
// first category lands at RailHome and CatBase()/IsCategory account for the
// missing Home so category lookups stay correct.
func TestRailCategoriesHomeless(t *testing.T) {
	r := NewRailCategories([]string{"Energy Drinks", "Chips"})
	if r.Len() != 3 {
		t.Fatalf("expected 3 entries (Search + 2 cats), got %d", r.Len())
	}
	if r.HasHome() {
		t.Fatal("NewRailCategories must not carry a Home slot")
	}
	if r.CatBase() != RailHome {
		t.Fatalf("Home-less CatBase must be %d (categories start where Home would), got %d", RailHome, r.CatBase())
	}
	if r.EntryLabel(RailHome) != "Energy Drinks" {
		t.Errorf("first category must land at index %d: %q", RailHome, r.EntryLabel(RailHome))
	}
	if idx, ok := r.IsCategory(RailHome); !ok || idx != 0 {
		t.Errorf("IsCategory(%d) = (%d,%v), want (0,true)", RailHome, idx, ok)
	}
	if idx, ok := r.IsCategory(2); !ok || idx != 1 {
		t.Errorf("IsCategory(2) = (%d,%v), want (1,true)", idx, ok)
	}
	if v := r.WithActive(RailHome).WithHeight(6).View(); strings.Contains(v, "Home") || strings.Contains(v, "Usuals") {
		t.Errorf("Home-less rail view must not show Home/Usuals:\n%s", v)
	}
}
