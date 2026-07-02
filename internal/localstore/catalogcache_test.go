package localstore

import (
	"encoding/json"
	"os"
	"path/filepath"
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
