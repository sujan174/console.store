package swiggy

import (
	"context"
	"fmt"
	"testing"
)

// dish builds a sparse dish stub as search_restaurants returns them: only
// {id, name, cuisines:[]}, with NO avgRating/areaName/deliveryTime/availability,
// so onlyRestaurants drops it (a dish has no restaurant link — a dead end).
func dish(id, name string) map[string]any {
	return map[string]any{"id": id, "name": name, "cuisines": []string{}}
}

func hasName(rs []Restaurant, name string) bool {
	for _, r := range rs {
		if r.Name == name {
			return true
		}
	}
	return false
}

// dishWallServer reproduces the live "blue tokai" behavior: Swiggy ranks 50
// dish stubs ("Blue Tokai Coffee Pancakes", …) ahead of the actual restaurant,
// which only surfaces at raw offset 50. Raw pages are 10 entries each.
func dishWallServer(t *testing.T, calls *int) toolFn {
	return func(args map[string]any) (any, error) {
		*calls++
		off := int(args["offset"].(float64))
		switch {
		case off < 50:
			var ds []map[string]any
			for i := 0; i < 10; i++ {
				ds = append(ds, dish(fmt.Sprintf("d%d", off+i), "Blue Tokai Coffee Pancakes"))
			}
			return map[string]any{"restaurants": ds}, nil
		case off == 50:
			return map[string]any{"restaurants": []map[string]any{
				rest("703703", "Blue Tokai Coffee Roasters"),
				rest("999", "Suchali's Artisan Bakehouse"),
			}}, nil
		default:
			return map[string]any{"restaurants": []map[string]any{}}, nil
		}
	}
}

// TestSearchOrganicPageCrossesDishWall — the widget/store-home paginated search
// must walk past a wall of dish stubs (while it has found ZERO restaurants) to
// surface a brand-name restaurant Swiggy buried at offset 50. Before the fix
// the 3-page cap stopped at offset 30 and returned nothing.
func TestSearchOrganicPageCrossesDishWall(t *testing.T) {
	var calls int
	srv := newFakeMCP(t, map[string]toolFn{"search_restaurants": dishWallServer(t, &calls)})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))

	got, _, _, err := c.SearchOrganicPage(context.Background(), "a1", "blue tokai", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !hasName(got, "Blue Tokai Coffee Roasters") {
		t.Fatalf("dish wall not crossed: got %d restaurants %v, want Blue Tokai Coffee Roasters", len(got), names(got))
	}
}

// TestSearchOrganicCrossesDishWall — the TUI global search box (SearchOrganic →
// searchFill) has the same duty. Before the fix its 2-page cap returned nothing
// for "blue tokai".
func TestSearchOrganicCrossesDishWall(t *testing.T) {
	var calls int
	srv := newFakeMCP(t, map[string]toolFn{"search_restaurants": dishWallServer(t, &calls)})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))

	got, err := c.SearchOrganic(context.Background(), "a1", "blue tokai")
	if err != nil {
		t.Fatal(err)
	}
	if !hasName(got, "Blue Tokai Coffee Roasters") {
		t.Fatalf("dish wall not crossed: got %d restaurants %v, want Blue Tokai Coffee Roasters", len(got), names(got))
	}
}

// TestSearchOrganicPageNoExtraWalkWhenFound — the deep walk must be adaptive:
// a normal query that yields restaurants immediately must NOT keep paging. Here
// page 0 already has real restaurants, so exactly ONE raw page is fetched.
func TestSearchOrganicPageNoExtraWalkWhenFound(t *testing.T) {
	var full []map[string]any
	for i := 0; i < 10; i++ {
		full = append(full, rest(fmt.Sprintf("r%d", i), fmt.Sprintf("Place %d", i)))
	}
	var calls int
	srv := newFakeMCP(t, map[string]toolFn{
		"search_restaurants": func(args map[string]any) (any, error) {
			calls++
			if int(args["offset"].(float64)) == 0 {
				return map[string]any{"restaurants": full}, nil
			}
			return map[string]any{"restaurants": []map[string]any{}}, nil
		},
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))

	if _, _, _, err := c.SearchOrganicPage(context.Background(), "a1", "pizza", 0); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("adaptive walk fetched %d pages for a query with results on page 1, want 1", calls)
	}
}

// names is a small helper for failure messages.
func names(rs []Restaurant) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Name
	}
	return out
}
