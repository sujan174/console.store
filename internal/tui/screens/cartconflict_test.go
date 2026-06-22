package screens

import (
	"strings"
	"testing"
)

func TestCartConflictShowsRestaurantsItemAndActions(t *testing.T) {
	v := NewCartConflict("Blue Tokai", "Third Wave", "Flat White").View()
	for _, want := range []string{
		"Blue Tokai",   // current cart restaurant
		"Third Wave",   // incoming restaurant
		"Flat White",   // the item being added
		"new cart",     // title copy
		"start new",    // confirm button label
		"keep current", // cancel button label
		"move",         // hint line: ← → move
		"select",       // hint line: ↵ select
		"cancel",       // hint line: esc cancel
	} {
		if !strings.Contains(v, want) {
			t.Errorf("conflict view missing %q:\n%s", want, v)
		}
	}
}

func TestCartConflictFocusMovesHighlight(t *testing.T) {
	base := NewCartConflict("Blue Tokai", "Third Wave", "Flat White")

	// focus 0: the ▌ highlight bar precedes "start new".
	v0 := base.WithFocus(0).View()
	if !strings.Contains(v0, "▌") {
		t.Fatalf("focus 0 should render the ▌ highlight bar:\n%s", v0)
	}
	if !(strings.Index(v0, "▌") < strings.Index(v0, "start new")) {
		t.Errorf("focus 0: ▌ should sit on 'start new':\n%s", v0)
	}

	// focus 1: the ▌ bar moves to "keep current".
	v1 := base.WithFocus(1).View()
	bar := strings.Index(v1, "▌")
	if !(bar > strings.Index(v1, "start new") && bar < strings.Index(v1, "keep current")) {
		t.Errorf("focus 1: ▌ should sit on 'keep current':\n%s", v1)
	}
}
