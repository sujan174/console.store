package swiggy

import "testing"

func TestLiveOrdersDefaultArming(t *testing.T) {
	t.Setenv("CONSOLE_LIVE_ORDERS", "") // env not forcing
	defer func(v string) { liveOrdersDefault = v }(liveOrdersDefault)

	liveOrdersDefault = "0"
	if liveOrdersEnabled() {
		t.Fatal("default \"0\" + no env should be disarmed")
	}
	liveOrdersDefault = "1"
	if !liveOrdersEnabled() {
		t.Fatal("default \"1\" should be armed")
	}
}

func TestEnvOverridesDisarmedDefault(t *testing.T) {
	defer func(v string) { liveOrdersDefault = v }(liveOrdersDefault)
	liveOrdersDefault = "0"
	t.Setenv("CONSOLE_LIVE_ORDERS", "1")
	if !liveOrdersEnabled() {
		t.Fatal("env \"1\" should arm even when default is \"0\"")
	}
}
