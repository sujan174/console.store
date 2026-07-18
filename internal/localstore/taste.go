package localstore

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Anti-bloat knobs for the taste store. See docs/superpowers/specs/
// 2026-07-02-seamless-ordering-harness-design.md for the design rationale.
const (
	MaxTasteEntries  = 40  // hard cap on Taste.Entries; explicit entries are never evicted
	InferredTTLDays  = 120 // inferred picks unused this long are pruned on next write
	SuggestThreshold = 3   // inferred Count at which a pick becomes a suggestion candidate
)

const secondsPerDay = 24 * 60 * 60

// TastePick is one remembered customization choice for a taste entry's option
// group. Counting is per-pick (per option group), not per-entry — this is what
// lets "oat milk regardless of size" work: the Milk pick accumulates evidence
// while the Size pick stays weak or absent.
type TastePick struct {
	GroupName  string `json:"groupName"`          // durable key, e.g. "Milk"
	ChoiceName string `json:"choiceName"`         // durable key, e.g. "Oat milk"
	GroupID    string `json:"groupId,omitempty"`  // fast-path hint, may be stale
	ChoiceID   string `json:"choiceId,omitempty"` // fast-path hint, may be stale
	Variant    bool   `json:"variant"`            // routing, mirrors presets.PresetSel
	Absolute   bool   `json:"absolute"`
	Source     string `json:"source"`             // "explicit" | "inferred"
	Count      int    `json:"count"`              // inferred-evidence tally
	Declined   bool   `json:"declined,omitempty"` // user declined the suggestion; stop re-asking
}

// TasteEntry is the remembered taste profile for one (restaurant, item) pair.
type TasteEntry struct {
	RestaurantID   string      `json:"restaurantId"`
	RestaurantName string      `json:"restaurantName"`
	ItemName       string      `json:"itemName"`
	ItemID         string      `json:"itemId,omitempty"`
	Picks          []TastePick `json:"picks,omitempty"`
	DontCare       []string    `json:"dontCare,omitempty"` // group names user doesn't care about
	Avoid          []string    `json:"avoid,omitempty"`    // disliked choice names
	LastUsedUnix   int64       `json:"lastUsed"`
}

// Taste is the on-disk taste store, keyed by (RestaurantID, normalized ItemName).
type Taste struct {
	Version int          `json:"version"`
	Entries []TasteEntry `json:"entries"`
}

// normItemName normalizes an item name into the durable lookup key.
func normItemName(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func tastePath() (string, error) {
	p, err := configPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "taste.json"), nil
}

// LoadTaste reads the taste store, returning an empty (version 1) Taste when
// the file does not exist yet.
func LoadTaste() (Taste, error) {
	p, err := tastePath()
	if err != nil {
		return Taste{}, err
	}
	raw, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return Taste{Version: 1}, nil
	}
	if err != nil {
		return Taste{}, err
	}
	var t Taste
	if err := json.Unmarshal(raw, &t); err != nil {
		return Taste{}, err
	}
	return t, nil
}

// SaveTaste enforces the decay+cap rules, then writes the taste store (0600).
func SaveTaste(t Taste) error {
	if t.Version == 0 {
		t.Version = 1
	}
	t.enforceCaps(time.Now().Unix())
	p, err := tastePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(p, raw, 0o600)
}

// Find looks up the entry for (restID, itemName), if any.
func (t *Taste) Find(restID, itemName string) (TasteEntry, bool) {
	key := normItemName(itemName)
	for _, e := range t.Entries {
		if e.RestaurantID == restID && normItemName(e.ItemName) == key {
			return e, true
		}
	}
	return TasteEntry{}, false
}

func (t *Taste) findIndex(restID, itemName string) int {
	key := normItemName(itemName)
	for i := range t.Entries {
		if t.Entries[i].RestaurantID == restID && normItemName(t.Entries[i].ItemName) == key {
			return i
		}
	}
	return -1
}

func sameGroup(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// Upsert is the EXPLICIT write path (reconcile-on-write): find or create the
// entry, then for each incoming pick mark it explicit and replace any existing
// pick with the same GroupName (case-insensitive), else append. DontCare/Avoid
// are merged (case-insensitive dedupe, union). Explicit picks always win over
// inferred ones.
func (t *Taste) Upsert(e TasteEntry, nowUnix int64) {
	idx := t.findIndex(e.RestaurantID, e.ItemName)
	if idx == -1 {
		t.Entries = append(t.Entries, TasteEntry{
			RestaurantID:   e.RestaurantID,
			RestaurantName: e.RestaurantName,
			ItemName:       e.ItemName,
			ItemID:         e.ItemID,
		})
		idx = len(t.Entries) - 1
	}
	cur := &t.Entries[idx]
	if e.RestaurantName != "" {
		cur.RestaurantName = e.RestaurantName
	}
	if e.ItemID != "" {
		cur.ItemID = e.ItemID
	}

	for _, p := range e.Picks {
		p.Source = "explicit"
		replaced := false
		for i := range cur.Picks {
			if sameGroup(cur.Picks[i].GroupName, p.GroupName) {
				cur.Picks[i] = p
				replaced = true
				break
			}
		}
		if !replaced {
			cur.Picks = append(cur.Picks, p)
		}
	}

	cur.DontCare = mergeCaseInsensitive(cur.DontCare, e.DontCare)
	cur.Avoid = mergeCaseInsensitive(cur.Avoid, e.Avoid)
	cur.LastUsedUnix = nowUnix
}

// mergeCaseInsensitive unions two string slices, deduping case-insensitively
// while preserving the first-seen casing.
func mergeCaseInsensitive(existing, incoming []string) []string {
	if len(incoming) == 0 {
		return existing
	}
	seen := make(map[string]bool, len(existing))
	out := make([]string, 0, len(existing)+len(incoming))
	for _, s := range existing {
		key := strings.ToLower(strings.TrimSpace(s))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s)
	}
	for _, s := range incoming {
		key := strings.ToLower(strings.TrimSpace(s))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s)
	}
	return out
}

// Observe is the INFERRED counting path, driven from a real placement. It
// finds or creates the entry, then for each observed pick: an existing
// explicit pick for that group is left untouched (explicit wins); an existing
// inferred pick with the same ChoiceName gets Count++; an existing inferred
// pick with a different ChoiceName is a contradiction and is replaced with
// Count reset to 1; no existing pick for that group appends a new inferred
// pick with Count 1. Caps are enforced after mutation.
func (t *Taste) Observe(restID, restName, itemName, itemID string, picks []TastePick, nowUnix int64) {
	idx := t.findIndex(restID, itemName)
	if idx == -1 {
		t.Entries = append(t.Entries, TasteEntry{
			RestaurantID:   restID,
			RestaurantName: restName,
			ItemName:       itemName,
			ItemID:         itemID,
		})
		idx = len(t.Entries) - 1
	}
	cur := &t.Entries[idx]
	if restName != "" {
		cur.RestaurantName = restName
	}
	if itemID != "" {
		cur.ItemID = itemID
	}

	for _, obs := range picks {
		found := false
		for i := range cur.Picks {
			p := &cur.Picks[i]
			if !sameGroup(p.GroupName, obs.GroupName) {
				continue
			}
			found = true
			if p.Source == "explicit" {
				// Explicit wins; leave untouched.
				break
			}
			if strings.EqualFold(strings.TrimSpace(p.ChoiceName), strings.TrimSpace(obs.ChoiceName)) {
				p.Count++
				if obs.GroupID != "" {
					p.GroupID = obs.GroupID
				}
				if obs.ChoiceID != "" {
					p.ChoiceID = obs.ChoiceID
				}
				p.Variant = obs.Variant
				p.Absolute = obs.Absolute
			} else {
				// Contradiction: new choice for this group resets the tally.
				*p = obs
				p.Source = "inferred"
				p.Count = 1
				p.Declined = false
			}
			break
		}
		if !found {
			np := obs
			np.Source = "inferred"
			np.Count = 1
			np.Declined = false
			cur.Picks = append(cur.Picks, np)
		}
	}

	cur.LastUsedUnix = nowUnix
	t.enforceCaps(nowUnix)
}

// Suggestions returns entries that have at least one inferred, non-declined
// pick with Count >= SuggestThreshold. Each returned entry contains ONLY the
// qualifying picks, so the caller sees exactly what crossed the threshold. The
// result is a copy; the store is not mutated.
func (t *Taste) Suggestions() []TasteEntry {
	var out []TasteEntry
	for _, e := range t.Entries {
		var qualifying []TastePick
		for _, p := range e.Picks {
			if p.Source == "inferred" && p.Count >= SuggestThreshold && !p.Declined {
				qualifying = append(qualifying, p)
			}
		}
		if len(qualifying) == 0 {
			continue
		}
		cp := e
		cp.Picks = qualifying
		out = append(out, cp)
	}
	return out
}

// Promote turns every qualifying inferred pick (Count >= SuggestThreshold, not
// declined) on the matching entry into an explicit pick (Count preserved,
// Declined reset). Returns true if anything was promoted.
func (t *Taste) Promote(restID, itemName string) bool {
	idx := t.findIndex(restID, itemName)
	if idx == -1 {
		return false
	}
	cur := &t.Entries[idx]
	promoted := false
	for i := range cur.Picks {
		p := &cur.Picks[i]
		if p.Source == "inferred" && p.Count >= SuggestThreshold && !p.Declined {
			p.Source = "explicit"
			p.Declined = false
			promoted = true
		}
	}
	return promoted
}

// DeclineSuggestion marks all qualifying inferred picks on the matching entry
// as declined, so Suggestions() stops surfacing them.
func (t *Taste) DeclineSuggestion(restID, itemName string) {
	idx := t.findIndex(restID, itemName)
	if idx == -1 {
		return
	}
	cur := &t.Entries[idx]
	for i := range cur.Picks {
		p := &cur.Picks[i]
		if p.Source == "inferred" && p.Count >= SuggestThreshold {
			p.Declined = true
		}
	}
}

// ForgetPick removes the pick with the given GroupName from the matching
// entry. If the entry ends up with no picks, no DontCare, and no Avoid, the
// entry itself is removed. Returns true if something was removed.
func (t *Taste) ForgetPick(restID, itemName, groupName string) bool {
	idx := t.findIndex(restID, itemName)
	if idx == -1 {
		return false
	}
	cur := &t.Entries[idx]
	removed := false
	for i := range cur.Picks {
		if sameGroup(cur.Picks[i].GroupName, groupName) {
			cur.Picks = append(cur.Picks[:i], cur.Picks[i+1:]...)
			removed = true
			break
		}
	}
	if !removed {
		return false
	}
	if len(cur.Picks) == 0 && len(cur.DontCare) == 0 && len(cur.Avoid) == 0 {
		t.Entries = append(t.Entries[:idx], t.Entries[idx+1:]...)
	}
	return true
}

// ForgetEntry removes the whole entry for (restID, itemName). Returns true if
// an entry was removed.
func (t *Taste) ForgetEntry(restID, itemName string) bool {
	idx := t.findIndex(restID, itemName)
	if idx == -1 {
		return false
	}
	t.Entries = append(t.Entries[:idx], t.Entries[idx+1:]...)
	return true
}

// enforceCaps applies the anti-bloat rules: (1) prune inferred picks whose
// owning entry has gone stale past InferredTTLDays (explicit picks are NEVER
// pruned); entries left with no picks/dontCare/avoid are dropped. (2) If over
// MaxTasteEntries, evict lowest-salience entries — evictable only if the entry
// has no explicit picks, oldest LastUsedUnix first, then lowest total Count —
// until at cap. Entries that are all-explicit are never evicted, even over cap.
func (t *Taste) enforceCaps(nowUnix int64) {
	ttlCutoff := nowUnix - int64(InferredTTLDays)*secondsPerDay

	kept := t.Entries[:0:0]
	for _, e := range t.Entries {
		stale := e.LastUsedUnix < ttlCutoff
		if stale {
			var picks []TastePick
			for _, p := range e.Picks {
				if p.Source == "explicit" {
					picks = append(picks, p)
				}
			}
			e.Picks = picks
		}
		if len(e.Picks) == 0 && len(e.DontCare) == 0 && len(e.Avoid) == 0 {
			continue
		}
		kept = append(kept, e)
	}
	t.Entries = kept

	if len(t.Entries) <= MaxTasteEntries {
		return
	}

	hasExplicit := func(e TasteEntry) bool {
		for _, p := range e.Picks {
			if p.Source == "explicit" {
				return true
			}
		}
		return false
	}
	totalCount := func(e TasteEntry) int {
		n := 0
		for _, p := range e.Picks {
			n += p.Count
		}
		return n
	}

	for len(t.Entries) > MaxTasteEntries {
		evictIdx := -1
		for i, e := range t.Entries {
			if hasExplicit(e) {
				continue
			}
			if evictIdx == -1 {
				evictIdx = i
				continue
			}
			cand := t.Entries[evictIdx]
			switch {
			case e.LastUsedUnix < cand.LastUsedUnix:
				evictIdx = i
			case e.LastUsedUnix == cand.LastUsedUnix && totalCount(e) < totalCount(cand):
				evictIdx = i
			}
		}
		if evictIdx == -1 {
			// Everything left is explicit-protected; never drop explicit data.
			break
		}
		t.Entries = append(t.Entries[:evictIdx], t.Entries[evictIdx+1:]...)
	}
}
