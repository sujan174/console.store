package broker

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"console.store/internal/broker/api"
)

func TestRPCRoundTripAddresses(t *testing.T) {
	mcp := fakeMCP(t, map[string]func(map[string]any) any{
		"get_addresses": func(map[string]any) any {
			return []map[string]any{{"id": "a1", "annotation": "home"}}
		},
	})
	store := &fakeStore{tokens: map[string]string{"acct-X": "tok"}}
	svc := NewService(Config{Store: store, Auth: &fakeAuthz{}, FoodBaseURL: mcp.URL, HTTPClient: mcp.Client()})

	sock := filepath.Join(t.TempDir(), "b.sock")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go Serve(ctx, svc, sock)
	waitForSocket(t, sock)

	cli, err := api.Dial(sock)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	got, err := cli.Addresses("acct-X")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "a1" {
		t.Fatalf("round-trip addresses = %+v", got)
	}

	// auth passthrough
	st, err := cli.StartAuth("ssh-key")
	if err != nil || st.FlowID != "flow-1" {
		t.Fatalf("startauth = %+v err=%v", st, err)
	}
}

func waitForSocket(t *testing.T, path string) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if c, err := api.Dial(path); err == nil {
			c.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("broker socket never came up")
}
