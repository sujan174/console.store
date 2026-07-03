package cli

import (
	"bytes"
	"strings"
	"testing"

	"consolestore/internal/broker/api"
	"consolestore/internal/localstore"
)

// A multi-match food pick list marks the candidate whose menu item comes back
// out of stock, and leaves the available one unmarked.
func TestOrderMultiPresetListMarksFoodSoldOut(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedTwoBreakfasts(t) // "Blue Tokai" (r1) then "Truffles" (r1) — both preset ItemID "i1"
	var out bytes.Buffer
	be := &fakeBackend{
		cart: availCart(),
		menu: api.Menu{Items: []api.MenuItem{{ID: "i1", InStock: false}}},
	}
	code := Dispatch([]string{"order", "breakfast"}, Deps{
		SignedIn: true, Armed: true, Interactive: false, Out: &out, In: strings.NewReader(""), Backend: be,
	})
	if code != 0 {
		t.Fatalf("listing exit = %d:\n%s", code, out.String())
	}
	s := out.String()
	if !strings.Contains(s, "sold out: Cold Coffee") {
		t.Fatalf("both candidates share the sold-out item id; expected a sold-out tag:\n%s", s)
	}
	if be.menuCalls == 0 {
		t.Fatal("expected the pick list to probe the menu")
	}
}

// A multi-match instamart pick list marks the candidate whose spinId search
// result comes back out of stock.
func TestOrderMultiPresetListMarksInstamartSoldOut(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p1 := imBasePreset("milk")
	p1.RestaurantName = "Instamart"
	p2 := imBasePreset("milk")
	p2.RestaurantName = "Instamart"
	p2.Lines[0].ItemID = "spin-2" // distinct spinId so we can tell them apart
	seedPreset(t, p1)
	seedPreset(t, p2)
	var out bytes.Buffer
	be := &fakeBackend{
		imCart: imAvailCart(),
		imSearch: map[string][]api.IMProduct{
			"Amul Milk 500ml": {{
				ID: "p1", Name: "Amul Milk 500ml",
				Variants: []api.IMVariantSel{
					{SpinID: "spin-1", InStock: false},
					{SpinID: "spin-2", InStock: true},
				},
			}},
		},
	}
	code := Dispatch([]string{"order", "milk"}, Deps{
		SignedIn: true, Armed: true, Interactive: false, Out: &out, In: strings.NewReader(""), Backend: be,
	})
	if code != 0 {
		t.Fatalf("listing exit = %d:\n%s", code, out.String())
	}
	s := out.String()
	if !strings.Contains(s, "1) Instamart") || !strings.Contains(s, "2) Instamart") {
		t.Fatalf("should list both instamart candidates:\n%s", s)
	}
	// Row 1 (spin-1, out of stock) must be marked; row 2 (spin-2, in stock) must not.
	lines := strings.Split(s, "\n")
	var row1, row2 string
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "1)") {
			row1 = l
		}
		if strings.HasPrefix(strings.TrimSpace(l), "2)") {
			row2 = l
		}
	}
	if !strings.Contains(row1, "sold out") {
		t.Fatalf("row 1 (spin-1, out of stock) should be marked sold out:\n%s", row1)
	}
	if strings.Contains(row2, "sold out") {
		t.Fatalf("row 2 (spin-2, in stock) should NOT be marked sold out:\n%s", row2)
	}
}

// A probe failure (menu fetch error) must never block the flow — the pick
// list still renders, just without any sold-out marking.
func TestOrderMultiPresetListProbeErrorLeavesUnmarked(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedTwoBreakfasts(t)
	var out bytes.Buffer
	be := &fakeBackend{cart: availCart(), menuErr: errBoom}
	code := Dispatch([]string{"order", "breakfast"}, Deps{
		SignedIn: true, Armed: true, Interactive: false, Out: &out, In: strings.NewReader(""), Backend: be,
	})
	if code != 0 {
		t.Fatalf("listing exit = %d:\n%s", code, out.String())
	}
	s := out.String()
	if strings.Contains(s, "sold out") {
		t.Fatalf("a probe error must leave rows unmarked, not block or mark them:\n%s", s)
	}
	if !strings.Contains(s, "1) Blue Tokai") || !strings.Contains(s, "2) Truffles") {
		t.Fatalf("listing should still show both candidates despite the probe error:\n%s", s)
	}
}

// A single-match order must NOT probe — the cart push validates immediately,
// so a pre-probe would only add latency.
func TestOrderSingleMatchSkipsProbe(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedPreset(t, basePreset("breakfast"))
	var out bytes.Buffer
	be := &fakeBackend{cart: availCart(), placed: api.Order{ID: "999"}}
	code := Dispatch([]string{"order", "breakfast"}, Deps{
		SignedIn: true, Armed: true, Interactive: true, Out: &out, In: strings.NewReader("\n"), Backend: be,
	})
	if code != 0 {
		t.Fatalf("order exit = %d:\n%s", code, out.String())
	}
	if be.menuCalls != 0 {
		t.Fatalf("single-match order should skip the availability probe entirely, got %d menu calls", be.menuCalls)
	}
}

// `console alias list` WITHOUT --check never probes.
func TestAliasListWithoutCheckDoesNotProbe(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedPreset(t, basePreset("breakfast"))
	var out bytes.Buffer
	be := &fakeBackend{menu: api.Menu{Items: []api.MenuItem{{ID: "i1", InStock: false}}}}
	code := Dispatch([]string{"alias", "list"}, Deps{SignedIn: true, Out: &out, Backend: be})
	if code != 0 {
		t.Fatalf("alias list exit = %d", code)
	}
	if be.menuCalls != 0 {
		t.Fatal("plain alias list must not probe availability")
	}
	if strings.Contains(out.String(), "sold out") {
		t.Fatalf("plain alias list must not show availability marking:\n%s", out.String())
	}
}

// `console alias list --check` probes and marks sold-out presets.
func TestAliasListCheckMarksSoldOut(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedPreset(t, basePreset("breakfast"))
	var out bytes.Buffer
	be := &fakeBackend{menu: api.Menu{Items: []api.MenuItem{{ID: "i1", InStock: false}}}}
	code := Dispatch([]string{"alias", "list", "--check"}, Deps{SignedIn: true, Out: &out, Backend: be})
	if code != 0 {
		t.Fatalf("alias list --check exit = %d", code)
	}
	if be.menuCalls == 0 {
		t.Fatal("alias list --check should probe availability")
	}
	if !strings.Contains(out.String(), "sold out: Cold Coffee") {
		t.Fatalf("should mark the sold-out preset:\n%s", out.String())
	}
}

// `console alias list --check` with everything available prints a reassuring
// hint instead of silence.
func TestAliasListCheckAllAvailableHint(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedPreset(t, basePreset("breakfast"))
	var out bytes.Buffer
	be := &fakeBackend{menu: api.Menu{Items: []api.MenuItem{{ID: "i1", InStock: true}}}}
	code := Dispatch([]string{"alias", "list", "--check"}, Deps{SignedIn: true, Out: &out, Backend: be})
	if code != 0 {
		t.Fatalf("alias list --check exit = %d", code)
	}
	if strings.Contains(out.String(), "sold out") {
		t.Fatalf("nothing should be marked sold out:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "available") {
		t.Fatalf("should print an all-available hint:\n%s", out.String())
	}
}

// A preset line whose ItemID is missing from the live menu entirely (delisted,
// not just out of stock) is also treated as unavailable.
func TestProbeFoodAvailabilityMissingItemIsUnavailable(t *testing.T) {
	be := &fakeBackend{menu: api.Menu{Items: []api.MenuItem{{ID: "other", InStock: true}}}}
	a := probeAvailability(be, basePreset("x"))
	if !a.known || !a.unavailable {
		t.Fatalf("a line missing from the menu should be unavailable: %+v", a)
	}
}

// A menu fetch error yields an "unknown" result — known=false — never marked
// unavailable.
func TestProbeFoodAvailabilityErrorIsUnknown(t *testing.T) {
	be := &fakeBackend{menuErr: errBoom}
	a := probeAvailability(be, basePreset("x"))
	if a.known {
		t.Fatalf("a probe error should be unknown, not known: %+v", a)
	}
	if soldOutSuffix(a, newStyle(false)) != "" {
		t.Fatalf("unknown availability must not render a suffix")
	}
}

// probeAvailability caps at maxProbeLines even for a preset with more lines,
// so a large preset can't spray unbounded probe calls.
func TestProbeIMAvailabilityCapsLines(t *testing.T) {
	p := imBasePreset("big")
	p.Lines = nil
	for i := 0; i < maxProbeLines+3; i++ {
		p.Lines = append(p.Lines, localstore.PresetLine{ItemID: "spin-ok", Name: "Item", Qty: 1})
	}
	be := &fakeBackend{imSearch: map[string][]api.IMProduct{
		"Item": {{Variants: []api.IMVariantSel{{SpinID: "spin-ok", InStock: true}}}},
	}}
	probeAvailability(be, p)
	if be.imSearchCalls != maxProbeLines {
		t.Fatalf("expected exactly %d probe calls (capped), got %d", maxProbeLines, be.imSearchCalls)
	}
}

var errBoom = errBoomType{}

type errBoomType struct{}

func (errBoomType) Error() string { return "boom" }
