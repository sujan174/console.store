package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRefreshExchangesRefreshToken(t *testing.T) {
	var gotGrant, gotRefresh, gotClient string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotGrant = r.Form.Get("grant_type")
		gotRefresh = r.Form.Get("refresh_token")
		gotClient = r.Form.Get("client_id")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`))
	}))
	defer srv.Close()

	tok, err := Refresh(context.Background(), srv.Client(), srv.URL, "client-1", "old-refresh")
	if err != nil {
		t.Fatal(err)
	}
	if gotGrant != "refresh_token" || gotRefresh != "old-refresh" || gotClient != "client-1" {
		t.Fatalf("bad refresh form: grant=%q refresh=%q client=%q", gotGrant, gotRefresh, gotClient)
	}
	if tok.AccessToken != "new-access" || tok.RefreshToken != "new-refresh" || tok.ExpiresIn != 3600 {
		t.Fatalf("parsed token = %+v", tok)
	}
}

func TestRefreshNon200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte("invalid_grant"))
	}))
	defer srv.Close()
	if _, err := Refresh(context.Background(), srv.Client(), srv.URL, "c", "r"); err == nil {
		t.Fatal("a non-200 refresh must return an error")
	}
}

func TestRefreshErrorClassifiesStatus(t *testing.T) {
	cases := []struct {
		status   int
		rejected bool
	}{
		{400, true},  // invalid_grant: refresh token dead → definitive rejection
		{401, true},  // unauthorized
		{403, true},  // forbidden
		{500, false}, // server fault → transient, not a rejection
		{503, false}, // unavailable → transient
	}
	for _, tc := range cases {
		status := tc.status
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			w.Write([]byte(`{"error":"invalid_grant"}`))
		}))
		_, err := Refresh(context.Background(), srv.Client(), srv.URL, "c", "r")
		srv.Close()
		var re *RefreshError
		if !errors.As(err, &re) {
			t.Fatalf("status %d: err = %v; want *RefreshError", status, err)
		}
		if re.Rejected() != tc.rejected {
			t.Fatalf("status %d: Rejected() = %v; want %v", status, re.Rejected(), tc.rejected)
		}
	}
}
