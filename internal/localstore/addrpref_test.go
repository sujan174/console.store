package localstore

import "testing"

func TestAddrPrefActiveAndLock(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p := AddrPref{}
	p = p.SetActive("a1", "Home")
	if id, _ := p.Active(); id != "a1" {
		t.Fatalf("active=%q want a1", id)
	}
	// lock to a2: Active must return the locked default regardless of last
	p = p.SetDefault("a2", "Work")
	p = p.SetActive("a3", "Gym") // last changes, but locked default wins
	if id, _ := p.Active(); id != "a2" {
		t.Fatalf("locked active=%q want a2", id)
	}
	// placement while locked updates Last but not the active default
	p = p.RecordPlacement("a4", "Cafe", 123)
	if id, _ := p.Active(); id != "a2" {
		t.Fatalf("post-placement locked active=%q want a2", id)
	}
	if p.LastAddrID != "a4" {
		t.Fatalf("last=%q want a4", p.LastAddrID)
	}
}

func TestAddrPrefUnlockedUsesLast(t *testing.T) {
	p := AddrPref{}.SetActive("a1", "Home").RecordPlacement("a5", "New", 9)
	if id, _ := p.Active(); id != "a5" {
		t.Fatalf("unlocked active=%q want a5 (last)", id)
	}
}

func TestAddrPrefRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	want := AddrPref{}.SetDefault("a2", "Work")
	if err := SaveAddrPref(want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadAddrPref()
	if err != nil || got.DefaultAddrID != "a2" || !got.Locked {
		t.Fatalf("roundtrip got=%+v err=%v", got, err)
	}
}

// ForgetActive clears a stale Last address, and — when that same id is the
// locked Default — also clears the Default+lock so a deleted default doesn't
// keep resolving to a dead address.
func TestForgetActiveClearsLastAndStaleDefault(t *testing.T) {
	p := AddrPref{}.SetDefault("a2", "Work")
	p = p.RecordPlacement("a2", "Work", 1) // Last == Default == a2
	p = p.ForgetActive("a2")
	if p.LastAddrID != "" || p.LastLabel != "" {
		t.Fatalf("Last not cleared: %+v", p)
	}
	if p.DefaultAddrID != "" || p.DefaultLabel != "" || p.Locked {
		t.Fatalf("stale default not cleared: %+v", p)
	}
}

// ForgetActive must not touch a Default that isn't the stale id.
func TestForgetActiveKeepsUnrelatedDefault(t *testing.T) {
	p := AddrPref{}.SetDefault("a2", "Work")
	p = p.RecordPlacement("a5", "Other", 1) // Last=a5, Default stays a2 (locked)
	p = p.ForgetActive("a5")
	if p.LastAddrID != "" {
		t.Fatalf("Last not cleared: %+v", p)
	}
	if p.DefaultAddrID != "a2" || !p.Locked {
		t.Fatalf("unrelated default wrongly cleared: %+v", p)
	}
}

func TestRecordPlacementKeepsLabelWhenEmpty(t *testing.T) {
	p := AddrPref{}.SetActive("a1", "Home")
	p = p.RecordPlacement("a1", "", 5) // app path: empty label
	if p.LastLabel != "Home" {
		t.Fatalf("label wiped: %q want Home", p.LastLabel)
	}
	p = p.RecordPlacement("a2", "Work", 6) // non-empty updates
	if p.LastAddrID != "a2" || p.LastLabel != "Work" {
		t.Fatalf("got %s/%s want a2/Work", p.LastAddrID, p.LastLabel)
	}
}
