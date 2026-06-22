package screens

import (
	"strings"
	"testing"
)

func TestCartConflictShowsRestaurantsItemAndActions(t *testing.T) {
	v := NewCartConflict("Blue Tokai", "Third Wave", "Flat White").View()
	for _, want := range []string{
		"Blue Tokai", // current cart restaurant
		"Third Wave", // incoming restaurant
		"Flat White", // the item being added
		"new cart",   // the title copy
		"y",          // confirm affordance
		"n",          // cancel affordance
	} {
		if !strings.Contains(v, want) {
			t.Errorf("conflict view missing %q:\n%s", want, v)
		}
	}
}
