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
	"strings"
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

// imCacheCap bounds the number of cached Instamart product lists. Higher than
// menus: it covers 13 fixed categories + the go-to list + arbitrary searches,
// all keyed per address.
const imCacheCap = 48

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

// CachedIMVariant is the persisted slice of one Instamart pack-size choice —
// enough to re-add it to the cart from a cached list without a live fetch (the
// spinId is the cart key). Prices/stock are a first paint only; the live cart
// sync at prepare/checkout is always the money authority.
type CachedIMVariant struct {
	SpinID  string `json:"spinId"`
	SkuID   string `json:"skuId,omitempty"`
	Label   string `json:"label,omitempty"`
	Price   int    `json:"price"`
	MRP     int    `json:"mrp,omitempty"`
	InStock bool   `json:"inStock"`
}

// CachedIMProduct is the persisted slice of an Instamart product — it mirrors
// the live api.IMProduct shape (name, brand, stock, variants) so the datasource
// can reconstruct catalog items through the SAME toIMItems synthesis the live
// path uses, keeping cached and live rows byte-identical.
type CachedIMProduct struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Brand    string            `json:"brand,omitempty"`
	InStock  bool              `json:"inStock"`
	Variants []CachedIMVariant `json:"variants,omitempty"`
}

type cachedMenu struct {
	SavedAt int64            `json:"saved_at"`
	Items   []CachedMenuItem `json:"items"`
}

type cachedInstamart struct {
	SavedAt  int64             `json:"saved_at"`
	Products []CachedIMProduct `json:"products"`
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
	// Write-then-rename (unique temp name) so a crash mid-write can't leave a
	// torn JSON file that poisons every later launch, and concurrent writers
	// don't clobber a shared ".tmp".
	_ = writeFileAtomic(path, raw, 0o600)
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

// LoadCachedInstamart returns the last-saved Instamart product list for an
// (addressID, query) pair ("" query = the go-to/Usuals list), or ok=false on
// miss/stale/corrupt — the mirror of LoadCachedPlaces for the grocery vertical,
// so a relaunched Instamart browse paints instantly instead of showing
// "loading…". Stale-while-revalidate: the live fetch always streams over it.
func LoadCachedInstamart(addressID, query string) ([]CachedIMProduct, bool) {
	dir, err := catalogCacheDir()
	if err != nil || addressID == "" {
		return nil, false
	}
	var im cachedInstamart
	if !readCacheFile(filepath.Join(dir, "im-"+cacheKey(addressID, query)+".json"), &im) {
		return nil, false
	}
	if !fresh(im.SavedAt) || len(im.Products) == 0 {
		return nil, false
	}
	return im.Products, true
}

// SaveCachedInstamart persists an Instamart product list for an (addressID,
// query) pair.
func SaveCachedInstamart(addressID, query string, products []CachedIMProduct) {
	dir, err := catalogCacheDir()
	if err != nil || addressID == "" || len(products) == 0 {
		return
	}
	writeCacheFile(filepath.Join(dir, "im-"+cacheKey(addressID, query)+".json"), cachedInstamart{
		SavedAt:  time.Now().Unix(),
		Products: products,
	})
	pruneCache(dir, "im-", imCacheCap)
}

// pruneMenuCache deletes the oldest menu files past menuCacheCap.
func pruneMenuCache(dir string) { pruneCache(dir, "menu-", menuCacheCap) }

// pruneCache deletes the oldest files (by mtime) with the given filename prefix
// past cap, bounding the cache dir for a heavy browser. Shared by the menu and
// Instamart caches (search-keyed lists accumulate one file per distinct query).
func pruneCache(dir, prefix string, cap int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type stamped struct {
		path string
		mod  time.Time
	}
	var files []stamped
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" || !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		info, ierr := e.Info()
		if ierr != nil {
			continue
		}
		files = append(files, stamped{filepath.Join(dir, e.Name()), info.ModTime()})
	}
	if len(files) <= cap {
		return
	}
	sort.Slice(files, func(a, b int) bool { return files[a].mod.Before(files[b].mod) })
	for _, f := range files[:len(files)-cap] {
		if err := os.Remove(f.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return
		}
	}
}
