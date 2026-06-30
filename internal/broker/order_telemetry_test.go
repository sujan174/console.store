package broker

import (
	"errors"
	"testing"

	"consolestore/internal/broker/api"
)

func TestShouldPingOrder(t *testing.T) {
	if !shouldPingOrder(api.Order{ID: "123"}, nil) {
		t.Fatal("want ping on success with id")
	}
	if shouldPingOrder(api.Order{}, errors.New("disabled")) {
		t.Fatal("no ping on error")
	}
	if shouldPingOrder(api.Order{ID: ""}, nil) {
		t.Fatal("no ping on empty id")
	}
}
