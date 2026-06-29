package version

import (
	"strings"
	"testing"
)

func TestDefaultsAreDev(t *testing.T) {
	if Version != "dev" {
		t.Fatalf("default Version = %q, want dev", Version)
	}
	if Channel != "stable" {
		t.Fatalf("default Channel = %q, want stable", Channel)
	}
	if !IsDev() {
		t.Fatal("IsDev() = false on default build, want true")
	}
}

func TestStringIncludesVersionAndChannel(t *testing.T) {
	s := String()
	if !strings.Contains(s, "dev") || !strings.Contains(s, "stable") {
		t.Fatalf("String() = %q, want it to mention version and channel", s)
	}
}
