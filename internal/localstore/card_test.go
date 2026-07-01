package localstore

import (
	"testing"

	"consolestore/internal/broker/api"
)

func TestRecordOrderBumpsFavoriteAndDefault(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := RecordOrder("addr1", "Home", "r1", "McDonald's", 1000); err != nil {
		t.Fatalf("RecordOrder: %v", err)
	}
	if err := RecordOrder("addr1", "Home", "r1", "McDonald's", 2000); err != nil {
		t.Fatalf("RecordOrder 2: %v", err)
	}
	c, err := LoadCard()
	if err != nil {
		t.Fatalf("LoadCard: %v", err)
	}
	if c.DefaultAddrID != "addr1" || c.AddrLabel != "Home" {
		t.Fatalf("default = %q/%q", c.DefaultAddrID, c.AddrLabel)
	}
	if len(c.Favorites) != 1 || c.Favorites[0].Count != 2 || c.Favorites[0].RestaurantID != "r1" {
		t.Fatalf("favorites = %+v", c.Favorites)
	}
}

func TestReconcileCardWarnsOnMissingAddress(t *testing.T) {
	c := Card{Version: 1, DefaultAddrID: "gone", AddrLabel: "Home"}
	got, warns := ReconcileCard(c, []api.Address{{ID: "other", Label: "Office"}})
	if len(warns) != 1 {
		t.Fatalf("warns = %v", warns)
	}
	if got.DefaultAddrID != "" {
		t.Fatalf("expected default cleared, got %q", got.DefaultAddrID)
	}
}

func TestRecordOrderStickyDefaultButUpdatesLast(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// First order seeds the default.
	if err := RecordOrder("addr1", "Home", "r1", "McDonald's", 1000); err != nil {
		t.Fatalf("RecordOrder 1: %v", err)
	}
	// Second order, different address, should NOT change the default but
	// should update Last*.
	if err := RecordOrder("addr2", "Office", "r1", "McDonald's", 2000); err != nil {
		t.Fatalf("RecordOrder 2: %v", err)
	}

	c, err := LoadCard()
	if err != nil {
		t.Fatalf("LoadCard: %v", err)
	}
	if c.DefaultAddrID != "addr1" || c.AddrLabel != "Home" {
		t.Fatalf("default should remain sticky, got %q/%q", c.DefaultAddrID, c.AddrLabel)
	}
	if c.LastAddrID != "addr2" || c.LastAddrLabel != "Office" || c.LastUsedUnix != 2000 {
		t.Fatalf("last address = %q/%q/%d", c.LastAddrID, c.LastAddrLabel, c.LastUsedUnix)
	}
}

func TestSetDefaultAddress(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := RecordOrder("addr1", "Home", "r1", "McDonald's", 1000); err != nil {
		t.Fatalf("RecordOrder: %v", err)
	}
	if err := SetDefaultAddress("addr2", "Office", 5000); err != nil {
		t.Fatalf("SetDefaultAddress: %v", err)
	}
	c, err := LoadCard()
	if err != nil {
		t.Fatalf("LoadCard: %v", err)
	}
	if c.DefaultAddrID != "addr2" || c.AddrLabel != "Office" {
		t.Fatalf("default = %q/%q", c.DefaultAddrID, c.AddrLabel)
	}
	if c.UpdatedAtUnix != 5000 {
		t.Fatalf("UpdatedAtUnix = %d", c.UpdatedAtUnix)
	}
}

func TestCacheAddressesRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	addrs := []api.Address{
		{ID: "addr1", Label: "Home", City: "Bengaluru"},
		{ID: "addr2", Label: "Office", City: "Bengaluru"},
	}
	if err := CacheAddresses(addrs, 42); err != nil {
		t.Fatalf("CacheAddresses: %v", err)
	}
	c, err := LoadCard()
	if err != nil {
		t.Fatalf("LoadCard: %v", err)
	}
	if len(c.AddrCache) != 2 || c.AddrCache[0].ID != "addr1" || c.AddrCacheUnix != 42 {
		t.Fatalf("AddrCache = %+v, unix = %d", c.AddrCache, c.AddrCacheUnix)
	}
}

func TestReconcileCardHealsLastAddress(t *testing.T) {
	c := Card{
		Version:    1,
		LastAddrID: "gone", LastAddrLabel: "Old Place",
	}
	got, warns := ReconcileCard(c, []api.Address{{ID: "other", Label: "Office"}})
	if len(warns) != 1 {
		t.Fatalf("warns = %v", warns)
	}
	if got.LastAddrID != "" || got.LastAddrLabel != "" {
		t.Fatalf("expected last address cleared, got %q/%q", got.LastAddrID, got.LastAddrLabel)
	}
}

func TestReconcileCardRefreshesLastAddressLabel(t *testing.T) {
	c := Card{Version: 1, LastAddrID: "addr1", LastAddrLabel: "Stale Label"}
	got, warns := ReconcileCard(c, []api.Address{{ID: "addr1", Label: "Fresh Label"}})
	if len(warns) != 0 {
		t.Fatalf("warns = %v", warns)
	}
	if got.LastAddrLabel != "Fresh Label" {
		t.Fatalf("LastAddrLabel = %q", got.LastAddrLabel)
	}
}
