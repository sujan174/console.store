package datasource

import (
	"errors"
	"testing"

	"consolestore/internal/broker/api"
	swiggysnap "consolestore/internal/catalog/swiggy"
)

// pagedBackend serves a scripted sequence of menu/search pages.
type pagedBackend struct {
	fakeBackend
	menuPages  []api.Menu // indexed by page-1
	placePages [][]api.Restaurant
	pageErr    error // returned for pages past the scripted ones
}

func (p *pagedBackend) MenuPage(_, _ string, page int) (api.Menu, bool, error) {
	if page-1 < len(p.menuPages) {
		return p.menuPages[page-1], page < len(p.menuPages), nil
	}
	return api.Menu{}, false, p.pageErr
}

func (p *pagedBackend) PlacesQueryPage(_, _ string, offset int) ([]api.Restaurant, int, bool, error) {
	i := 0
	if offset > 0 {
		i = 1
	}
	if i < len(p.placePages) {
		return p.placePages[i], offset + len(p.placePages[i]), i+1 < len(p.placePages), nil
	}
	return nil, offset, false, p.pageErr
}

func TestLoadMenuPageStreamsIntoSnapshot(t *testing.T) {
	b := &pagedBackend{menuPages: []api.Menu{
		{RestaurantID: "r1", Items: []api.MenuItem{{ID: "i1", Name: "Dosa"}}},
		{RestaurantID: "r1", Items: []api.MenuItem{{ID: "i2", Name: "Idli"}}},
	}}
	snap := swiggysnap.NewSnapshot()

	msg := LoadMenuPage(b, snap, "a1", "r1", 1, 7, false)()
	m, ok := msg.(MenuPageLoadedMsg)
	if !ok || m.Err != nil || m.Done || m.Page != 1 || m.Gen != 7 {
		t.Fatalf("page1 msg = %+v", msg)
	}
	if p, ok := swiggysnap.NewRepository(snap).Menu("r1"); !ok || len(p.Items) != 1 {
		t.Fatalf("after page1 snapshot menu = %+v ok=%v", p, ok)
	}

	msg = LoadMenuPage(b, snap, "a1", "r1", 2, 7, false)()
	m = msg.(MenuPageLoadedMsg)
	if m.Err != nil || !m.Done {
		t.Fatalf("page2 msg = %+v, want Done", m)
	}
	if p, _ := swiggysnap.NewRepository(snap).Menu("r1"); len(p.Items) != 2 {
		t.Fatalf("after page2 snapshot menu items = %d, want 2", len(p.Items))
	}
}

func TestLoadMenuPagePropagatesError(t *testing.T) {
	b := &pagedBackend{pageErr: errors.New("boom")}
	snap := swiggysnap.NewSnapshot()
	msg := LoadMenuPage(b, snap, "a1", "r1", 1, 1, false)()
	if m, ok := msg.(MenuPageLoadedMsg); !ok || m.Err == nil {
		t.Fatalf("msg = %+v, want error", msg)
	}
}

func TestLoadPlacesPageStreamsIntoSnapshot(t *testing.T) {
	b := &pagedBackend{placePages: [][]api.Restaurant{
		{{ID: "p1", Name: "One"}},
		{{ID: "p2", Name: "Two"}},
	}}
	snap := swiggysnap.NewSnapshot()

	msg := LoadPlacesPage(b, snap, "a1", "pizza", 0, 1, 3)()
	m, ok := msg.(PlacesPageLoadedMsg)
	if !ok || m.Err != nil || m.Done || m.Gen != 3 {
		t.Fatalf("page1 msg = %+v", msg)
	}
	msg = LoadPlacesPage(b, snap, "a1", "pizza", m.NextOffset, 2, 3)()
	m = msg.(PlacesPageLoadedMsg)
	if m.Err != nil || !m.Done {
		t.Fatalf("page2 msg = %+v, want Done", m)
	}
}
