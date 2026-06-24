package api

import (
	"bytes"
	"encoding/gob"
	"testing"
)

// DTOs must round-trip through gob (the RPC codec).
func TestDTOsGobRoundTrip(t *testing.T) {
	in := AddressesReply{Addresses: []Address{{ID: "a1", Label: "home", Lat: 12.9}}}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(in); err != nil {
		t.Fatal(err)
	}
	var out AddressesReply
	if err := gob.NewDecoder(&buf).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Addresses) != 1 || out.Addresses[0].ID != "a1" || out.Addresses[0].Lat != 12.9 {
		t.Fatalf("round-trip = %+v", out)
	}
}
