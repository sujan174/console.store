package mcp

import (
	"context"
	"testing"
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
