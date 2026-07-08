package swiggy

import (
	"context"
	"fmt"
	"testing"
)

// rest builds a real-restaurant page entry (a non-zero rating makes
// onlyRestaurants keep it). Ads carry a trailing " (Ad)" in the name.
func rest(id, name string) map[string]any {
	return map[string]any{"id": id, "name": name, "avgRating": 4.2, "cuisines": []string{"Pizza"}}
}

// TestSearchOrganicPageFillsToPageSize verifies the store-home search
// primitive: it walks as many raw Swiggy pages as needed to reach
// ~searchPageSize ad-free restaurants (so the first page is a stable size),
// drops "(Ad)" listings, and reports a resume offset + has_more.
func TestSearchOrganicPageFillsToPageSize(t *testing.T) {
	// Page 0 (raw offset 0) carries only 5 real restaurants + 1 ad — short of
	// searchPageSize (8) after filtering, so the primitive must walk to the
	// next raw page. Page 1 (raw offset 6) adds 5 more, pushing the total past 8.
	page0 := []map[string]any{
		rest("r1", "Oven Story"), rest("r2", "MOJO"), rest("r3", "La Pino'z"),
		rest("r4", "Pizza Hut"), rest("r5", "GOPIZZA"), rest("ad1", "Sponsored Slice (Ad)"),
	}
	page1 := []map[string]any{
		rest("r6", "Crusto's"), rest("r7", "99 Slice"), rest("r8", "PizzaExpress"),
		rest("r9", "Caro Napoli"), rest("r10", "Baking Bad"),
	}
	var calls int
	srv := newFakeMCP(t, map[string]toolFn{
		"search_restaurants": func(args map[string]any) (any, error) {
			calls++
			off := int(args["offset"].(float64))
			switch off {
			case 0:
				return map[string]any{"restaurants": page0}, nil
			case 6:
				return map[string]any{"restaurants": page1}, nil
			default:
				return map[string]any{"restaurants": []map[string]any{}}, nil
			}
		},
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))

	got, next, more, err := c.SearchOrganicPage(context.Background(), "a1", "pizza", 0)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("expected to walk 2 raw pages to fill, walked %d", calls)
	}
	if len(got) < searchPageSize {
		t.Fatalf("filled %d restaurants, want >= %d", len(got), searchPageSize)
	}
	for _, r := range got {
		if isAd(r.Name) {
			t.Fatalf("ad leaked into organic results: %q", r.Name)
		}
	}
	// Resume cursor is past both raw pages (6 + 5), more is still true (last
	// raw page was non-empty).
	if next != 11 {
		t.Fatalf("next offset = %d, want 11", next)
	}
	if !more {
		t.Fatalf("has_more = false, want true (last page was full)")
	}
}

// TestSearchOrganicPageSinglePageWhenFull verifies the common/fast case: when
// one raw page already carries >= searchPageSize real restaurants, exactly one
// round trip happens (no wasteful second page).
func TestSearchOrganicPageSinglePageWhenFull(t *testing.T) {
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

	got, next, more, err := c.SearchOrganicPage(context.Background(), "a1", "pizza", 0)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("expected a single raw page, made %d calls", calls)
	}
	if len(got) != 10 || next != 10 || !more {
		t.Fatalf("got %d results, next %d, more %v", len(got), next, more)
	}
}

// TestSearchOrganicPageEmptyStopsPagination verifies has_more goes false when
// a page comes back empty — the signal the app uses to hide "load more".
func TestSearchOrganicPageEmptyStopsPagination(t *testing.T) {
	srv := newFakeMCP(t, map[string]toolFn{
		"search_restaurants": func(map[string]any) (any, error) {
			return map[string]any{"restaurants": []map[string]any{}}, nil
		},
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))

	got, next, more, err := c.SearchOrganicPage(context.Background(), "a1", "pizza", 40)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 || more {
		t.Fatalf("empty page should yield 0 results and more=false, got %d results more=%v", len(got), more)
	}
	if next != 40 {
		t.Fatalf("empty page must not advance the cursor: next=%d want 40", next)
	}
}
