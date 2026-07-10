package tui

import (
	"bytes"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"consolestore/internal/tui/render"
)

func TestFlowMenuToCart(t *testing.T) {
	tm := teatest.NewTestModel(t, New(render.Caps{}), teatest.WithInitialTermSize(80, 24))

	// skip the decode -> home landing, then activate go-to-shop -> menu
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	// open first restaurant
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	// add first dish
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	// go to cart
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("to pay (UPI)")) && bytes.Contains(b, []byte("Cold Coffee"))
	}, teatest.WithDuration(3*time.Second))

	// `q` must NOT quit from the checkout (quit is confined to resting
	// screens); ctrl+c is the universal quit.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}
