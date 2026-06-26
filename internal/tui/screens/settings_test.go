package screens

import (
	"strings"
	"testing"
)

func TestSettingsModalRendersDisconnect(t *testing.T) {
	v := NewSettings(true).ModalView()
	for _, want := range []string{"settings", "Disconnect Swiggy", "esc close"} {
		if !strings.Contains(v, want) {
			t.Fatalf("settings modal missing %q:\n%s", want, v)
		}
	}
}

func TestSettingsDisconnectGatedOnConnected(t *testing.T) {
	if a := NewSettings(true).SelectedAction(); a != "disconnect" {
		t.Fatalf("connected action = %q, want disconnect", a)
	}
	if a := NewSettings(false).SelectedAction(); a != "" {
		t.Fatalf("disconnected action = %q, want \"\"", a)
	}
	if v := NewSettings(false).ModalView(); !strings.Contains(v, "not connected") {
		t.Fatalf("disconnected modal should note (not connected):\n%s", v)
	}
}
