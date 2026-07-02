package mcp

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"consolestore/internal/localstore"
)

type CardFavoriteDTO struct {
	RestaurantID   string `json:"restaurant_id"`
	RestaurantName string `json:"name"`
	Count          int    `json:"count"`
}

type AddrRefDTO struct {
	ID    string `json:"id,omitempty"`
	Label string `json:"label,omitempty"`
}
type CardAddressDTO struct {
	Default AddrRefDTO `json:"default"`
	Last    AddrRefDTO `json:"last"`
}

type TastePickDTO struct {
	GroupName  string `json:"group_name"`
	ChoiceName string `json:"choice_name"`
	Source     string `json:"source"`
	Count      int    `json:"count,omitempty"`
}
type TasteEntryDTO struct {
	RestaurantID   string         `json:"restaurant_id"`
	RestaurantName string         `json:"restaurant_name"`
	ItemName       string         `json:"item_name"`
	Picks          []TastePickDTO `json:"picks"`
	DontCare       []string       `json:"dont_care,omitempty"`
	Avoid          []string       `json:"avoid,omitempty"`
}

type CardDTO struct {
	Address     CardAddressDTO    `json:"address"`
	Favorites   []CardFavoriteDTO `json:"favorites"`
	Policies    []string          `json:"policies"`
	Taste       []TasteEntryDTO   `json:"taste"`
	Suggestions []TasteEntryDTO   `json:"suggestions"`
}

func tastePicksToDTO(picks []localstore.TastePick) []TastePickDTO {
	out := make([]TastePickDTO, 0, len(picks))
	for _, p := range picks {
		out = append(out, TastePickDTO{GroupName: p.GroupName, ChoiceName: p.ChoiceName, Source: p.Source, Count: p.Count})
	}
	return out
}

func tasteEntryToDTO(e localstore.TasteEntry) TasteEntryDTO {
	d := TasteEntryDTO{
		RestaurantID:   e.RestaurantID,
		RestaurantName: e.RestaurantName,
		ItemName:       e.ItemName,
		Picks:          tastePicksToDTO(e.Picks),
		DontCare:       e.DontCare,
		Avoid:          e.Avoid,
	}
	if d.Picks == nil {
		d.Picks = []TastePickDTO{}
	}
	return d
}

// cardToDTO projects the local card + taste store into the agent-facing shape.
// All slices are initialized non-nil so an empty card marshals as [] (not null).
func cardToDTO(c localstore.Card, t localstore.Taste) CardDTO {
	d := CardDTO{
		Address: CardAddressDTO{
			Default: AddrRefDTO{ID: c.DefaultAddrID, Label: c.AddrLabel},
			Last:    AddrRefDTO{ID: c.LastAddrID, Label: c.LastAddrLabel},
		},
		Favorites:   []CardFavoriteDTO{},
		Policies:    []string{},
		Taste:       []TasteEntryDTO{},
		Suggestions: []TasteEntryDTO{},
	}
	if len(c.Prefs) > 0 {
		d.Policies = c.Prefs
	}
	for _, f := range c.Favorites {
		d.Favorites = append(d.Favorites, CardFavoriteDTO{RestaurantID: f.RestaurantID, RestaurantName: f.RestaurantName, Count: f.Count})
	}
	for _, e := range t.Entries {
		d.Taste = append(d.Taste, tasteEntryToDTO(e))
	}
	for _, e := range t.Suggestions() {
		d.Suggestions = append(d.Suggestions, tasteEntryToDTO(e))
	}
	return d
}

type GetCardIn struct{}
type GetCardOut struct {
	Card     CardDTO  `json:"card"`
	Warnings []string `json:"warnings,omitempty"`
}

// cardSnapshot loads the card + taste, reconciles against live addresses (caching
// them and persisting any healing), and returns the agent-facing DTO plus any
// staleness warnings. Best-effort: a load error degrades to a partial/empty card
// rather than failing the caller. Shared by get_card and auth_status (the latter
// embeds it so the agent gets auth + card in a single opening call).
func (s *Server) cardSnapshot() (CardDTO, []string) {
	c, err := localstore.LoadCard()
	if err != nil {
		return cardToDTO(localstore.Card{}, localstore.Taste{}), nil
	}
	t, _ := localstore.LoadTaste()
	var warns []string
	if addrs, aerr := s.be.Addresses(); aerr == nil {
		_ = localstore.CacheAddresses(addrs, nowUnix())
		healed, w := localstore.ReconcileCard(c, addrs)
		warns = w
		if healed.DefaultAddrID != c.DefaultAddrID || healed.AddrLabel != c.AddrLabel ||
			healed.LastAddrID != c.LastAddrID || healed.LastAddrLabel != c.LastAddrLabel {
			_ = localstore.SaveCard(healed)
		}
		c = healed
	}
	return cardToDTO(c, t), warns
}

func (s *Server) handleGetCard(ctx context.Context, _ *mcp.CallToolRequest, _ GetCardIn) (*mcp.CallToolResult, GetCardOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, GetCardOut{}, err
	}
	card, warns := s.cardSnapshot()
	return nil, GetCardOut{Card: card, Warnings: warns}, nil
}

// --- remember ---

type TastePickIn struct {
	GroupName  string `json:"group_name"`
	ChoiceName string `json:"choice_name"`
	GroupID    string `json:"group_id,omitempty"`
	ChoiceID   string `json:"choice_id,omitempty"`
	Variant    bool   `json:"variant,omitempty"`
	Absolute   bool   `json:"absolute,omitempty"`
}

type RememberIn struct {
	DefaultAddressID  string        `json:"default_address_id,omitempty"`
	Policy            string        `json:"policy,omitempty"`
	RestaurantID      string        `json:"restaurant_id,omitempty"`
	RestaurantName    string        `json:"restaurant_name,omitempty"`
	ItemName          string        `json:"item_name,omitempty"`
	ItemID            string        `json:"item_id,omitempty"`
	Picks             []TastePickIn `json:"picks,omitempty"`
	DontCare          []string      `json:"dont_care,omitempty"`
	Avoid             []string      `json:"avoid,omitempty"`
	ConfirmSuggestion bool          `json:"confirm_suggestion,omitempty" jsonschema:"promote the existing inferred suggestion for restaurant_id+item_name to an explicit preference"`
}
type RememberOut struct {
	Card CardDTO `json:"card"`
}

func hasFold(list []string, v string) bool {
	for _, s := range list {
		if strings.EqualFold(strings.TrimSpace(s), strings.TrimSpace(v)) {
			return true
		}
	}
	return false
}

func (s *Server) handleRemember(ctx context.Context, _ *mcp.CallToolRequest, in RememberIn) (*mcp.CallToolResult, RememberOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, RememberOut{}, err
	}
	now := nowUnix()

	if in.DefaultAddressID != "" {
		label := ""
		if addrs, aerr := s.be.Addresses(); aerr == nil {
			for _, a := range addrs {
				if a.ID == in.DefaultAddressID {
					label = a.Label
					break
				}
			}
		}
		if err := localstore.SetDefaultAddress(in.DefaultAddressID, label, now); err != nil {
			return nil, RememberOut{}, err
		}
	}

	if in.Policy != "" {
		c, err := localstore.LoadCard()
		if err != nil {
			return nil, RememberOut{}, err
		}
		if !hasFold(c.Prefs, in.Policy) {
			c.Prefs = append(c.Prefs, in.Policy)
			if err := localstore.SaveCard(c); err != nil {
				return nil, RememberOut{}, err
			}
		}
	}

	if in.ItemName != "" {
		t, err := localstore.LoadTaste()
		if err != nil {
			return nil, RememberOut{}, err
		}
		if in.ConfirmSuggestion {
			t.Promote(in.RestaurantID, in.ItemName)
		} else {
			picks := make([]localstore.TastePick, 0, len(in.Picks))
			for _, p := range in.Picks {
				picks = append(picks, localstore.TastePick{
					GroupName:  p.GroupName,
					ChoiceName: p.ChoiceName,
					GroupID:    p.GroupID,
					ChoiceID:   p.ChoiceID,
					Variant:    p.Variant,
					Absolute:   p.Absolute,
				})
			}
			t.Upsert(localstore.TasteEntry{
				RestaurantID:   in.RestaurantID,
				RestaurantName: in.RestaurantName,
				ItemName:       in.ItemName,
				ItemID:         in.ItemID,
				Picks:          picks,
				DontCare:       in.DontCare,
				Avoid:          in.Avoid,
			}, now)
		}
		if err := localstore.SaveTaste(t); err != nil {
			return nil, RememberOut{}, err
		}
	}

	c, err := localstore.LoadCard()
	if err != nil {
		return nil, RememberOut{}, err
	}
	t, err := localstore.LoadTaste()
	if err != nil {
		return nil, RememberOut{}, err
	}
	return nil, RememberOut{Card: cardToDTO(c, t)}, nil
}

// --- forget ---

type ForgetIn struct {
	Policy            string `json:"policy,omitempty"`
	RestaurantID      string `json:"restaurant_id,omitempty"`
	ItemName          string `json:"item_name,omitempty"`
	GroupName         string `json:"group_name,omitempty" jsonschema:"empty removes the whole taste entry; set removes just that option group"`
	DeclineSuggestion bool   `json:"decline_suggestion,omitempty" jsonschema:"when true, keep the inferred taste for restaurant_id+item_name but stop offering it as a suggestion (the user declined); does not delete anything"`
}
type ForgetOut struct {
	Card CardDTO `json:"card"`
}

func (s *Server) handleForget(ctx context.Context, _ *mcp.CallToolRequest, in ForgetIn) (*mcp.CallToolResult, ForgetOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, ForgetOut{}, err
	}

	if in.Policy != "" {
		c, err := localstore.LoadCard()
		if err != nil {
			return nil, ForgetOut{}, err
		}
		out := c.Prefs[:0:0]
		for _, p := range c.Prefs {
			if !strings.EqualFold(strings.TrimSpace(p), strings.TrimSpace(in.Policy)) {
				out = append(out, p)
			}
		}
		c.Prefs = out
		if err := localstore.SaveCard(c); err != nil {
			return nil, ForgetOut{}, err
		}
	}

	if in.ItemName != "" {
		t, err := localstore.LoadTaste()
		if err != nil {
			return nil, ForgetOut{}, err
		}
		switch {
		case in.DeclineSuggestion:
			// User said no to a suggestion: silence it, keep the data.
			t.DeclineSuggestion(in.RestaurantID, in.ItemName)
		case in.GroupName == "":
			t.ForgetEntry(in.RestaurantID, in.ItemName)
		default:
			t.ForgetPick(in.RestaurantID, in.ItemName, in.GroupName)
		}
		if err := localstore.SaveTaste(t); err != nil {
			return nil, ForgetOut{}, err
		}
	}

	c, err := localstore.LoadCard()
	if err != nil {
		return nil, ForgetOut{}, err
	}
	t, err := localstore.LoadTaste()
	if err != nil {
		return nil, ForgetOut{}, err
	}
	return nil, ForgetOut{Card: cardToDTO(c, t)}, nil
}
