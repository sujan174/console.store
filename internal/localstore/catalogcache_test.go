package localstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMenuCacheRoundtrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if _, ok := LoadCachedMenu("r1"); ok {
		t.Fatal("empty cache must miss")
	}
	SaveCachedMenu("r1", []CachedMenuItem{{ID: "i1", Name: "Latte", Price: 250, Rating: 4.2}})
	got, ok := LoadCachedMenu("r1")
	if !ok || len(got) != 1 || got[0].Name != "Latte" || got[0].Rating != 4.2 {
		t.Fatalf("roundtrip = %+v ok=%v", got, ok)
	}
	if _, ok := LoadCachedMenu("other"); ok {
		t.Fatal("different restaurant must miss")
	}
}

func TestPlacesCacheRoundtripKeyedByAddrAndQuery(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	SaveCachedPlaces("a1", "pizza", []CachedPlace{{ID: "p1", Name: "Slice House"}})
	if got, ok := LoadCachedPlaces("a1", "pizza"); !ok || got[0].Name != "Slice House" {
		t.Fatalf("roundtrip = %+v ok=%v", got, ok)
	}
	if _, ok := LoadCachedPlaces("a2", "pizza"); ok {
		t.Fatal("different address must miss")
	}
	if _, ok := LoadCachedPlaces("a1", "coffee"); ok {
		t.Fatal("different query must miss")
	}
}

func TestStaleCacheEntryIgnored(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	SaveCachedMenu("r1", []CachedMenuItem{{ID: "i1", Name: "Latte"}})
	// Rewrite the file with a savedAt past the TTL.
	dir, _ := catalogCacheDir()
	path := filepath.Join(dir, "menu-"+cacheKey("r1")+".json")
	raw, _ := json.Marshal(cachedMenu{
		SavedAt: time.Now().Add(-catalogCacheTTL - time.Hour).Unix(),
		Items:   []CachedMenuItem{{ID: "i1", Name: "Latte"}},
	})
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := LoadCachedMenu("r1"); ok {
		t.Fatal("stale entry must be ignored")
	}
}

func TestCorruptCacheFileIsMiss(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	SaveCachedMenu("r1", []CachedMenuItem{{ID: "i1", Name: "Latte"}})
	dir, _ := catalogCacheDir()
	path := filepath.Join(dir, "menu-"+cacheKey("r1")+".json")
	if err := os.WriteFile(path, []byte("{torn"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := LoadCachedMenu("r1"); ok {
		t.Fatal("corrupt entry must be a miss, not a crash")
	}
}

func TestMenuCachePrunesPastCap(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	for i := 0; i < menuCacheCap+5; i++ {
		SaveCachedMenu(string(rune('a'+i%26))+string(rune('0'+i/26)), []CachedMenuItem{{ID: "i", Name: "x"}})
	}
	dir, _ := catalogCacheDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	menus := 0
	for _, e := range entries {
		if len(e.Name()) >= 5 && e.Name()[:5] == "menu-" {
			menus++
		}
	}
	if menus > menuCacheCap {
		t.Fatalf("menu cache holds %d files, cap is %d", menus, menuCacheCap)
	}
}

func TestInstamartCacheRoundtripWithVariants(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	SaveCachedInstamart("a1", "red bull", []CachedIMProduct{{
		ID: "RB1", Name: "Red Bull", Brand: "Red Bull", InStock: true,
		Variants: []CachedIMVariant{
			{SpinID: "sp250", Label: "250 ml", Price: 112, MRP: 125, InStock: true},
			{SpinID: "sp4", Label: "250 ml x 4", Price: 433, InStock: false},
		},
	}})
	got, ok := LoadCachedInstamart("a1", "red bull")
	if !ok || len(got) != 1 || got[0].Name != "Red Bull" || len(got[0].Variants) != 2 {
		t.Fatalf("roundtrip = %+v ok=%v", got, ok)
	}
	if got[0].Variants[0].SpinID != "sp250" || got[0].Variants[0].Price != 112 || !got[0].Variants[0].InStock {
		t.Fatalf("variant not preserved: %+v", got[0].Variants[0])
	}
	// Keyed by address AND query, like places.
	if _, ok := LoadCachedInstamart("a2", "red bull"); ok {
		t.Fatal("different address must miss")
	}
	if _, ok := LoadCachedInstamart("a1", ""); ok {
		t.Fatal("different query (go-to list) must miss")
	}
}

func TestInstamartCachePrunesPastCap(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	for i := 0; i < imCacheCap+8; i++ {
		q := "q" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		SaveCachedInstamart("a1", q, []CachedIMProduct{{ID: "p", Name: "x", InStock: true}})
	}
	dir, _ := catalogCacheDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	ims := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "im-") {
			ims++
		}
	}
	if ims > imCacheCap {
		t.Fatalf("instamart cache holds %d files, cap is %d", ims, imCacheCap)
	}
}
