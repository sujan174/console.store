package localstore

import "testing"

func TestUpsertReplacesSameGroup(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var ts Taste
	ts.Upsert(TasteEntry{
		RestaurantID:   "r1",
		RestaurantName: "Starbucks",
		ItemName:       "Latte",
		Picks: []TastePick{
			{GroupName: "Milk", ChoiceName: "Whole milk"},
		},
	}, 100)

	// Second explicit write for the same group should replace, not append.
	ts.Upsert(TasteEntry{
		RestaurantID: "r1",
		ItemName:     "Latte",
		Picks: []TastePick{
			{GroupName: "milk", ChoiceName: "Oat milk"}, // case-insensitive match
		},
	}, 200)

	e, ok := ts.Find("r1", "latte")
	if !ok {
		t.Fatalf("expected entry to exist")
	}
	if len(e.Picks) != 1 {
		t.Fatalf("expected 1 pick after replace, got %d: %+v", len(e.Picks), e.Picks)
	}
	if e.Picks[0].ChoiceName != "Oat milk" || e.Picks[0].Source != "explicit" {
		t.Fatalf("pick = %+v", e.Picks[0])
	}
	if e.LastUsedUnix != 200 {
		t.Fatalf("LastUsedUnix = %d", e.LastUsedUnix)
	}
	if e.RestaurantName != "Starbucks" {
		t.Fatalf("RestaurantName should be preserved, got %q", e.RestaurantName)
	}
}

func TestUpsertMergesDontCareAndAvoid(t *testing.T) {
	var ts Taste
	ts.Upsert(TasteEntry{
		RestaurantID: "r1", ItemName: "Latte",
		DontCare: []string{"Size"},
		Avoid:    []string{"Whipped cream"},
	}, 100)
	ts.Upsert(TasteEntry{
		RestaurantID: "r1", ItemName: "Latte",
		DontCare: []string{"size", "Sugar"}, // dup + new
		Avoid:    []string{"Whipped Cream"}, // dup, different case
	}, 200)

	e, _ := ts.Find("r1", "Latte")
	if len(e.DontCare) != 2 {
		t.Fatalf("DontCare = %v", e.DontCare)
	}
	if len(e.Avoid) != 1 {
		t.Fatalf("Avoid = %v", e.Avoid)
	}
}

func TestObserveCountsToThreshold(t *testing.T) {
	var ts Taste
	pick := []TastePick{{GroupName: "Milk", ChoiceName: "Oat milk"}}
	ts.Observe("r1", "Starbucks", "Latte", "item1", pick, 100)
	ts.Observe("r1", "Starbucks", "Latte", "item1", pick, 200)
	ts.Observe("r1", "Starbucks", "Latte", "item1", pick, 300)

	e, ok := ts.Find("r1", "Latte")
	if !ok {
		t.Fatalf("expected entry")
	}
	if len(e.Picks) != 1 || e.Picks[0].Count != 3 || e.Picks[0].Source != "inferred" {
		t.Fatalf("pick = %+v", e.Picks)
	}

	sugg := ts.Suggestions()
	if len(sugg) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(sugg))
	}
	if len(sugg[0].Picks) != 1 || sugg[0].Picks[0].ChoiceName != "Oat milk" {
		t.Fatalf("suggestion picks = %+v", sugg[0].Picks)
	}
}

func TestObserveContradictionResetsCount(t *testing.T) {
	var ts Taste
	ts.Observe("r1", "Starbucks", "Latte", "item1", []TastePick{{GroupName: "Milk", ChoiceName: "Oat milk"}}, 100)
	ts.Observe("r1", "Starbucks", "Latte", "item1", []TastePick{{GroupName: "Milk", ChoiceName: "Oat milk"}}, 200)
	// Contradiction: different choice for the same group.
	ts.Observe("r1", "Starbucks", "Latte", "item1", []TastePick{{GroupName: "Milk", ChoiceName: "Almond milk"}}, 300)

	e, _ := ts.Find("r1", "Latte")
	if len(e.Picks) != 1 {
		t.Fatalf("expected 1 pick, got %+v", e.Picks)
	}
	if e.Picks[0].ChoiceName != "Almond milk" || e.Picks[0].Count != 1 {
		t.Fatalf("pick after contradiction = %+v", e.Picks[0])
	}
}

func TestObserveLeavesExplicitPickUntouched(t *testing.T) {
	var ts Taste
	ts.Upsert(TasteEntry{
		RestaurantID: "r1", ItemName: "Latte",
		Picks: []TastePick{{GroupName: "Milk", ChoiceName: "Whole milk"}},
	}, 50)
	// An observation contradicting the explicit pick should not change it.
	ts.Observe("r1", "", "Latte", "", []TastePick{{GroupName: "Milk", ChoiceName: "Oat milk"}}, 100)

	e, _ := ts.Find("r1", "Latte")
	if len(e.Picks) != 1 || e.Picks[0].ChoiceName != "Whole milk" || e.Picks[0].Source != "explicit" {
		t.Fatalf("explicit pick should be untouched, got %+v", e.Picks)
	}
}

func TestPromotePicksThreshold(t *testing.T) {
	var ts Taste
	pick := []TastePick{{GroupName: "Milk", ChoiceName: "Oat milk"}}
	for _, when := range []int64{100, 200, 300} {
		ts.Observe("r1", "Starbucks", "Latte", "item1", pick, when)
	}
	if !ts.Promote("r1", "Latte") {
		t.Fatalf("expected Promote to succeed")
	}
	e, _ := ts.Find("r1", "Latte")
	if e.Picks[0].Source != "explicit" || e.Picks[0].Count != 3 {
		t.Fatalf("pick after promote = %+v", e.Picks[0])
	}

	// Promote again should be a no-op (nothing left qualifying as inferred).
	if ts.Promote("r1", "Latte") {
		t.Fatalf("expected second Promote to be a no-op")
	}
}

func TestDeclineSuggestionStopsResurfacing(t *testing.T) {
	var ts Taste
	pick := []TastePick{{GroupName: "Milk", ChoiceName: "Oat milk"}}
	ts.Observe("r1", "Starbucks", "Latte", "item1", pick, 100)
	ts.Observe("r1", "Starbucks", "Latte", "item1", pick, 200)
	ts.Observe("r1", "Starbucks", "Latte", "item1", pick, 300)

	if len(ts.Suggestions()) != 1 {
		t.Fatalf("expected a suggestion before decline")
	}
	ts.DeclineSuggestion("r1", "Latte")
	if len(ts.Suggestions()) != 0 {
		t.Fatalf("expected no suggestions after decline")
	}

	e, _ := ts.Find("r1", "Latte")
	if !e.Picks[0].Declined {
		t.Fatalf("pick should be marked declined: %+v", e.Picks[0])
	}
}

func TestForgetPickRemovesEntryWhenEmpty(t *testing.T) {
	var ts Taste
	ts.Upsert(TasteEntry{
		RestaurantID: "r1", ItemName: "Latte",
		Picks: []TastePick{{GroupName: "Milk", ChoiceName: "Oat milk"}},
	}, 100)

	if !ts.ForgetPick("r1", "Latte", "milk") {
		t.Fatalf("expected ForgetPick to succeed")
	}
	if _, ok := ts.Find("r1", "Latte"); ok {
		t.Fatalf("expected entry to be removed once empty")
	}
	if ts.ForgetPick("r1", "Latte", "milk") {
		t.Fatalf("expected second ForgetPick to report false")
	}
}

func TestForgetPickKeepsEntryWithOtherData(t *testing.T) {
	var ts Taste
	ts.Upsert(TasteEntry{
		RestaurantID: "r1", ItemName: "Latte",
		Picks:    []TastePick{{GroupName: "Milk", ChoiceName: "Oat milk"}},
		DontCare: []string{"Size"},
	}, 100)

	if !ts.ForgetPick("r1", "Latte", "Milk") {
		t.Fatalf("expected ForgetPick to succeed")
	}
	e, ok := ts.Find("r1", "Latte")
	if !ok {
		t.Fatalf("expected entry to remain (has DontCare)")
	}
	if len(e.Picks) != 0 {
		t.Fatalf("expected picks cleared, got %+v", e.Picks)
	}
}

func TestForgetEntry(t *testing.T) {
	var ts Taste
	ts.Upsert(TasteEntry{RestaurantID: "r1", ItemName: "Latte"}, 100)
	ts.Upsert(TasteEntry{RestaurantID: "r2", ItemName: "Mocha"}, 100)

	if !ts.ForgetEntry("r1", "Latte") {
		t.Fatalf("expected ForgetEntry to succeed")
	}
	if _, ok := ts.Find("r1", "Latte"); ok {
		t.Fatalf("expected entry removed")
	}
	if _, ok := ts.Find("r2", "Mocha"); !ok {
		t.Fatalf("expected other entry untouched")
	}
	if ts.ForgetEntry("r1", "Latte") {
		t.Fatalf("expected second ForgetEntry to report false")
	}
}

func TestEnforceCapsProtectsExplicitOverCap(t *testing.T) {
	var ts Taste
	// Fill past the cap with inferred-only entries plus one explicit.
	for i := 0; i < MaxTasteEntries+5; i++ {
		name := string(rune('a' + i%26))
		ts.Entries = append(ts.Entries, TasteEntry{
			RestaurantID: "r", ItemName: name + string(rune(i)),
			Picks:        []TastePick{{GroupName: "G", ChoiceName: "C", Source: "inferred", Count: 1}},
			LastUsedUnix: int64(i), // increasing recency
		})
	}
	// One explicit entry with the OLDEST LastUsedUnix — must survive eviction.
	ts.Entries = append(ts.Entries, TasteEntry{
		RestaurantID: "r", ItemName: "explicit-item",
		Picks:        []TastePick{{GroupName: "G", ChoiceName: "C", Source: "explicit"}},
		LastUsedUnix: -1,
	})

	ts.enforceCaps(1_000_000) // far in the future so TTL-prune doesn't interfere here beyond explicit protection check
	// NB: TTL prune would strip inferred picks from stale entries first; that's fine,
	// this test only asserts the explicit entry is never evicted by the cap step.

	if _, ok := ts.Find("r", "explicit-item"); !ok {
		t.Fatalf("explicit entry must never be evicted")
	}
	if len(ts.Entries) > MaxTasteEntries {
		// Acceptable only if every remaining entry is now explicit (can't go lower).
		for _, e := range ts.Entries {
			hasExplicit := false
			for _, p := range e.Picks {
				if p.Source == "explicit" {
					hasExplicit = true
				}
			}
			if !hasExplicit {
				t.Fatalf("over cap (%d) but found non-explicit survivor: %+v", len(ts.Entries), e)
			}
		}
	}
}

func TestEnforceCapsPrunesStaleInferred(t *testing.T) {
	var ts Taste
	staleTime := int64(0)
	ts.Entries = []TasteEntry{
		{
			RestaurantID: "r1", ItemName: "Latte",
			Picks:        []TastePick{{GroupName: "Milk", ChoiceName: "Oat milk", Source: "inferred", Count: 2}},
			LastUsedUnix: staleTime,
		},
		{
			RestaurantID: "r2", ItemName: "Mocha",
			Picks: []TastePick{
				{GroupName: "Milk", ChoiceName: "Oat milk", Source: "inferred", Count: 2},
				{GroupName: "Size", ChoiceName: "Large", Source: "explicit"},
			},
			LastUsedUnix: staleTime,
		},
	}

	now := staleTime + int64(InferredTTLDays+1)*secondsPerDay
	ts.enforceCaps(now)

	// r1 had only an inferred pick and is stale -> entry fully pruned.
	if _, ok := ts.Find("r1", "Latte"); ok {
		t.Fatalf("expected stale all-inferred entry to be pruned")
	}
	// r2 had an explicit pick too -> entry survives, but its inferred pick is pruned.
	e, ok := ts.Find("r2", "Mocha")
	if !ok {
		t.Fatalf("expected entry with explicit pick to survive TTL prune")
	}
	if len(e.Picks) != 1 || e.Picks[0].Source != "explicit" {
		t.Fatalf("expected only explicit pick to remain, got %+v", e.Picks)
	}
}

func TestLoadTasteMissingFileReturnsEmptyVersion1(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	ts, err := LoadTaste()
	if err != nil {
		t.Fatalf("LoadTaste: %v", err)
	}
	if ts.Version != 1 || len(ts.Entries) != 0 {
		t.Fatalf("ts = %+v", ts)
	}
}

func TestSaveAndLoadTasteRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var ts Taste
	ts.Upsert(TasteEntry{
		RestaurantID: "r1", RestaurantName: "Starbucks", ItemName: "Latte",
		Picks: []TastePick{{GroupName: "Milk", ChoiceName: "Oat milk"}},
	}, 100)
	if err := SaveTaste(ts); err != nil {
		t.Fatalf("SaveTaste: %v", err)
	}
	got, err := LoadTaste()
	if err != nil {
		t.Fatalf("LoadTaste: %v", err)
	}
	e, ok := got.Find("r1", "latte")
	if !ok || len(e.Picks) != 1 || e.Picks[0].ChoiceName != "Oat milk" {
		t.Fatalf("round-tripped entry = %+v ok=%v", e, ok)
	}
}
