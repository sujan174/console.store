package broker

import (
	"testing"

	"consolestore/internal/broker/api"
)

func TestShouldPingOrder(t *testing.T) {
	if !shouldPingOrder(api.Order{ID: "123"}) {
		t.Fatal("want ping on success with id")
	}
	if shouldPingOrder(api.Order{ID: ""}) {
		t.Fatal("no ping on empty id (failed placement or disarmed no-op returns no id)")
	}
}
