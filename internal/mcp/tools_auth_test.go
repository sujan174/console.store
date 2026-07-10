package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"consolestore/internal/swiggy"
)

func TestAuthStatusReportsSignedIn(t *testing.T) {
	s := NewServer(&fakeBackend{}, &fakeAuth{token: true})
	_, out, err := s.handleAuthStatus(context.Background(), nil, AuthStatusIn{})
	if err != nil {
		t.Fatalf("auth_status: %v", err)
	}
	if !out.SignedIn {
		t.Fatalf("expected signed in")
	}
}

func TestSignInReturnsAuthorizeURL(t *testing.T) {
	s := NewServer(&fakeBackend{}, &fakeAuth{token: false, url: "https://auth.example/x", flow: "F1"})
	_, out, err := s.handleSignIn(context.Background(), nil, SignInIn{})
	if err != nil {
		t.Fatalf("sign_in: %v", err)
	}
	if out.AuthorizeURL != "https://auth.example/x" {
		t.Fatalf("url = %q", out.AuthorizeURL)
	}
}

func TestSignInWhenAlreadySignedIn(t *testing.T) {
	s := NewServer(&fakeBackend{}, &fakeAuth{token: true})
	_, out, err := s.handleSignIn(context.Background(), nil, SignInIn{})
	if err != nil {
		t.Fatalf("sign_in: %v", err)
	}
	if !out.AlreadySignedIn {
		t.Fatalf("expected AlreadySignedIn")
	}
}

// A revoked-but-present token leaves TokenPresent true, so a plain sign_in
// short-circuits with AlreadySignedIn and the widget has no way to recover.
// force:true must skip that short-circuit and start a fresh authorize flow.
func TestSignInForceStartsFreshFlowEvenWhenTokenPresent(t *testing.T) {
	s := NewServer(&fakeBackend{}, &fakeAuth{token: true, url: "https://auth.example/reconnect", flow: "F2"})
	_, out, err := s.handleSignIn(context.Background(), nil, SignInIn{Force: true})
	if err != nil {
		t.Fatalf("sign_in force: %v", err)
	}
	if out.AlreadySignedIn {
		t.Fatalf("expected a fresh flow, got AlreadySignedIn")
	}
	if out.AuthorizeURL != "https://auth.example/reconnect" {
		t.Fatalf("url = %q", out.AuthorizeURL)
	}
	if out.FlowID != "F2" {
		t.Fatalf("flow = %q", out.FlowID)
	}
}

func TestMapAuthErrTranslatesSentinels(t *testing.T) {
	cases := []error{swiggy.ErrTokenExpired, swiggy.ErrSessionRevoked, swiggy.ErrInsufficientScope}
	for _, sentinel := range cases {
		wrapped := errors.New("wrapped context") // ensure errors.Is unwrapping still matches when joined
		joined := errors.Join(wrapped, sentinel)
		got := mapAuthErr(joined)
		if got == nil {
			t.Fatalf("mapAuthErr(%v): expected a coded error, got nil", sentinel)
		}
		if !strings.HasPrefix(got.Error(), "unauthenticated: ") {
			t.Fatalf("mapAuthErr(%v) = %q, want unauthenticated: prefix", sentinel, got.Error())
		}
	}
}

func TestMapAuthErrPassesThroughOtherErrors(t *testing.T) {
	other := errors.New("some other failure")
	if got := mapAuthErr(other); got != other {
		t.Fatalf("mapAuthErr(other) = %v, want unchanged %v", got, other)
	}
}

func TestMapAuthErrPassesThroughNil(t *testing.T) {
	if got := mapAuthErr(nil); got != nil {
		t.Fatalf("mapAuthErr(nil) = %v, want nil", got)
	}
}
