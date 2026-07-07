package main

// Demo is one animation experiment. Init is called on entry and on resize
// (fresh state); Step advances one tick; View renders at the Init'd size.
type Demo interface {
	Name() string
	Tagline() string
	Init(w, h int)
	Step(n int)
	View() string
}

var registry = []func() Demo{
	func() Demo { return &Spinners{} },
	func() Demo { return &Boot{} },
	func() Demo { return &Logo{} },
	func() Demo { return &Steam{} },
	func() Demo { return &Rider{} },
	func() Demo { return &Confetti{} },
	func() Demo { return &Rain{} },
	func() Demo { return &Fire{} },
	func() Demo { return &Plasma{} },
	func() Demo { return &Donut{} },
}
