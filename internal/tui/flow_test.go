package tui

import (
	"bytes"
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
		return bytes.Contains(b, []byte("to pay (COD)")) && bytes.Contains(b, []byte("Cold Coffee"))
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}
