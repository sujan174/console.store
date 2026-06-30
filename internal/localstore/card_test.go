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
