package auth

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type AccountStore interface {
	FindOrCreateAccount(ctx context.Context, phone string) (string, error)
	LinkPubkey(ctx context.Context, accountID, pubkey string) error
	PutToken(ctx context.Context, accountID, accessToken, refreshToken string, expiresAt time.Time) error
}

type Config struct {
	HTTPClient  *http.Client
	Metadata    Metadata
	ClientID    string
	RedirectURI string
	Scope       string
	Store       AccountStore
	Now         func() time.Time
}

type pending struct {
	flowID   string
	verifier string
	pubkey   string
}

type Manager struct {
	cfg Config
	mu  sync.Mutex
	// pendingByState is keyed by CSRF state; done is keyed by flowID.
	pendingByState map[string]*pending
	done           map[string]bool
}

func NewManager(cfg Config) *Manager {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Manager{cfg: cfg, pendingByState: map[string]*pending{}, done: map[string]bool{}}
}

type Pending struct {
	FlowID       string
	AuthorizeURL string
}

func (m *Manager) Start(pubkey string) (Pending, error) {
	verifier := GenerateVerifier()
	state := RandState()
	flowID := RandState()

	authURL := m.cfg.Metadata.AuthorizationEndpoint + "?" + url.Values{
		"response_type":         {"code"},
		"client_id":             {m.cfg.ClientID},
		"redirect_uri":          {m.cfg.RedirectURI},
		"scope":                 {m.cfg.Scope},
		"state":                 {state},
		"code_challenge":        {Challenge(verifier)},
		"code_challenge_method": {"S256"},
	}.Encode()

	m.mu.Lock()
	m.pendingByState[state] = &pending{flowID: flowID, verifier: verifier, pubkey: pubkey}
	m.mu.Unlock()
	return Pending{FlowID: flowID, AuthorizeURL: authURL}, nil
}

func (m *Manager) HandleCallback(ctx context.Context, state, code string) error {
	// Atomically claim the pending entry: read AND delete in one critical
	// section so (a) a failed exchange leaves the state consumed (not
	// replayable) and (b) two concurrent callbacks with the same state can
	// only one succeed — the other sees !ok immediately.
	m.mu.Lock()
	p, ok := m.pendingByState[state]
	if ok {
		delete(m.pendingByState, state)
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("auth: unknown, expired, or already-used state (CSRF check failed)")
	}

	tok, err := Exchange(ctx, m.cfg.HTTPClient, m.cfg.Metadata.TokenEndpoint,
		m.cfg.ClientID, code, p.verifier, m.cfg.RedirectURI)
	if err != nil {
		return err
	}
	id, err := IdentityFromAccessToken(tok.AccessToken)
	if err != nil || id.Phone == "" {
		// Fallback: key the account on the subject when no phone claim exists.
		// (Open item: a production fallback would trigger a consolestore OTP.)
		if id.Subject == "" {
			return fmt.Errorf("auth: token has neither phone nor sub claim: %w", err)
		}
		id.Phone = "sub:" + id.Subject
	}

	accountID, err := m.cfg.Store.FindOrCreateAccount(ctx, id.Phone)
	if err != nil {
		return err
	}
	if err := m.cfg.Store.LinkPubkey(ctx, accountID, p.pubkey); err != nil {
		return err
	}
	expiresAt := m.cfg.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	if err := m.cfg.Store.PutToken(ctx, accountID, tok.AccessToken, tok.RefreshToken, expiresAt); err != nil {
		return err
	}

	m.mu.Lock()
	m.done[p.flowID] = true
	m.mu.Unlock()
	return nil
}

func (m *Manager) CallbackHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			http.Error(w, "authorize error: "+e, http.StatusBadRequest)
			return
		}
		if err := m.HandleCallback(r.Context(), q.Get("state"), q.Get("code")); err != nil {
			// Log the real reason (swallowing it makes "authorization failed"
			// impossible to diagnose). A CSRF/state failure almost always means the
			// callback hit a DIFFERENT consolestore process than the one that
			// opened the link — i.e. a second instance is holding this port.
			log.Printf("auth: callback rejected: %v", err)
			http.Error(w, "authorization failed: "+err.Error()+
				"\n\nIf another consolestore is running, close it (it's holding this port) and try again.",
				http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("consolestore authorized. Return to your terminal."))
	}
}

func (m *Manager) Authorized(flowID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.done[flowID]
}
