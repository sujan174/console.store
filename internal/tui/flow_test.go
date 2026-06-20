package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

func TestFlowMenuToCart(t *testing.T) {
	tm := teatest.NewTestModel(t, New(), teatest.WithInitialTermSize(80, 24))

	// open first restaurant
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	// add first item
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	// go to cart
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return contains(b, "to pay (COD)") && contains(b, "Cold Coffee")
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func contains(b []byte, s string) bool {
	return len(b) >= len(s) && (string(b) != "" && indexOf(string(b), s) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
