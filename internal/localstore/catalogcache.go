package localstore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Catalog cache: last-known menus and restaurant lists, persisted so a repeat
// visit paints instantly from disk while the live fetch streams over it
// (stale-while-revalidate). Prices and availability here are only a first
// paint — the live cart sync at checkout is always the authority on money, so
// a stale cache can never mis-bill. Everything is best-effort: a cache miss
// or write failure silently falls back to the live-only path.

// catalogCacheTTL is how long a cached entry is trusted for first paint.
// Beyond it the entry is ignored (menus drift; a week-old one confuses more
// than it helps).
const catalogCacheTTL = 7 * 24 * time.Hour

// menuCacheCap bounds the number of cached menus (LRU by write time) so the
// cache dir can't grow without bound for a heavy browser.
const menuCacheCap = 24

// CachedMenuItem is the persisted slice of a catalog.Item — only what the
// menu list renders. localstore must not import catalog (it sits below it),
// so the TUI maps in and out.
type CachedMenuItem struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Price        int     `json:"price"`
	Veg          bool    `json:"veg,omitempty"`
	Desc         string  `json:"desc,omitempty"`
	Rating       float64 `json:"rating,omitempty"`
	Customizable bool    `json:"customizable,omitempty"`
	Category     string  `json:"category,omitempty"`
	OutOfStock   bool    `json:"out_of_stock,omitempty"`
}

// CachedPlace is the persisted slice of a restaurant-list entry.
type CachedPlace struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	City        string  `json:"city,omitempty"`
	ETA         string  `json:"eta,omitempty"`
	Rating      float64 `json:"rating,omitempty"`
	Description string  `json:"description,omitempty"`
	Offer       string  `json:"offer,omitempty"`
}

type cachedMenu struct {
	SavedAt int64            `json:"saved_at"`
	Items   []CachedMenuItem `json:"items"`
}

type cachedPlaces struct {
	SavedAt int64         `json:"saved_at"`
	Places  []CachedPlace `json:"places"`
}

func catalogCacheDir() (string, error) {
	base, err := baseConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "catalog-cache"), nil
}

// cacheKey hashes an identifier into a fixed-width filename component, so
// address ids / queries with odd characters can't escape the cache dir.
func cacheKey(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func readCacheFile(path string, out any) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return json.Unmarshal(raw, out) == nil
}

func writeCacheFile(path string, v any) {
	raw, err := json.Marshal(v)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	// Write-then-rename so a crash mid-write can't leave a torn JSON file
	// that poisons every later launch.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

func fresh(savedAt int64) bool {
	return savedAt > 0 && time.Since(time.Unix(savedAt, 0)) < catalogCacheTTL
}

// LoadCachedMenu returns the last-saved menu for a restaurant, or ok=false on
// miss/stale/corrupt.
func LoadCachedMenu(restaurantID string) ([]CachedMenuItem, bool) {
	dir, err := catalogCacheDir()
	if err != nil || restaurantID == "" {
		return nil, false
	}
	var m cachedMenu
	if !readCacheFile(filepath.Join(dir, "menu-"+cacheKey(restaurantID)+".json"), &m) {
		return nil, false
	}
	if !fresh(m.SavedAt) || len(m.Items) == 0 {
		return nil, false
	}
	return m.Items, true
}

// SaveCachedMenu persists a complete menu (called when a live stream finishes,
// never for partial loads) and prunes the menu cache past its cap.
func SaveCachedMenu(restaurantID string, items []CachedMenuItem) {
	dir, err := catalogCacheDir()
	if err != nil || restaurantID == "" || len(items) == 0 {
		return
	}
	writeCacheFile(filepath.Join(dir, "menu-"+cacheKey(restaurantID)+".json"), cachedMenu{
		SavedAt: time.Now().Unix(),
		Items:   items,
	})
	pruneMenuCache(dir)
}

// LoadCachedPlaces returns the last-saved restaurant list for an
// (addressID, query) pair, or ok=false on miss/stale/corrupt.
func LoadCachedPlaces(addressID, query string) ([]CachedPlace, bool) {
	dir, err := catalogCacheDir()
	if err != nil || addressID == "" {
		return nil, false
	}
	var p cachedPlaces
	if !readCacheFile(filepath.Join(dir, "places-"+cacheKey(addressID, query)+".json"), &p) {
		return nil, false
	}
	if !fresh(p.SavedAt) || len(p.Places) == 0 {
		return nil, false
	}
	return p.Places, true
}

// SaveCachedPlaces persists a restaurant list for an (addressID, query) pair.
func SaveCachedPlaces(addressID, query string, places []CachedPlace) {
	dir, err := catalogCacheDir()
	if err != nil || addressID == "" || len(places) == 0 {
		return
	}
	writeCacheFile(filepath.Join(dir, "places-"+cacheKey(addressID, query)+".json"), cachedPlaces{
		SavedAt: time.Now().Unix(),
		Places:  places,
	})
}

// pruneMenuCache deletes the oldest menu files past menuCacheCap.
func pruneMenuCache(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type stamped struct {
		path string
		mod  time.Time
	}
	var menus []stamped
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" || len(e.Name()) < 5 || e.Name()[:5] != "menu-" {
			continue
		}
		info, ierr := e.Info()
		if ierr != nil {
			continue
		}
		menus = append(menus, stamped{filepath.Join(dir, e.Name()), info.ModTime()})
	}
	if len(menus) <= menuCacheCap {
		return
	}
	sort.Slice(menus, func(a, b int) bool { return menus[a].mod.Before(menus[b].mod) })
	for _, m := range menus[:len(menus)-menuCacheCap] {
		if err := os.Remove(m.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return
		}
	}
}
