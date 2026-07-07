package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type tickMsg time.Time

type model struct {
	idx    int
	demo   Demo
	frame  int
	paused bool
	w, h   int
	tick   time.Duration
}

func (m model) tickCmd() tea.Cmd {
	return tea.Tick(m.tick, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) Init() tea.Cmd { return m.tickCmd() }

func (m model) load(i int) model {
	n := len(registry)
	m.idx = ((i % n) + n) % n
	m.demo = registry[m.idx]()
	m.frame = 0
	if m.w > 0 && m.h > 1 {
		m.demo.Init(m.w, m.h-1)
	}
	return m
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m.load(m.idx), nil
	case tickMsg:
		if !m.paused && m.demo != nil {
			m.frame++
			m.demo.Step(m.frame)
		}
		return m, m.tickCmd()
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "right", "l", "]", "tab":
			return m.load(m.idx + 1), nil
		case "left", "h", "[", "shift+tab":
			return m.load(m.idx - 1), nil
		case " ":
			m.paused = !m.paused
			return m, nil
		case "r":
			return m.load(m.idx), nil
		default:
			s := msg.String()
			if len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
				if i := int(s[0] - '1'); i < len(registry) {
					return m.load(i), nil
				}
			}
			if s == "0" && len(registry) >= 10 {
				return m.load(9), nil
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.demo == nil || m.w == 0 {
		return "…"
	}
	return m.demo.View() + "\n" + m.statusBar()
}

func (m model) statusBar() string {
	name := m.demo.Name()
	left := fmt.Sprintf(" %d/%d  %s ", m.idx+1, len(registry), name)
	mid := " " + m.demo.Tagline()
	right := " ←/→ switch · 1-9 jump · space pause · r reset · q quit "
	if m.paused {
		right = " ⏸ paused ·" + right
	}
	pad := m.w - len([]rune(left)) - len([]rune(mid)) - len([]rune(right))
	if pad < 0 {
		right = ""
		pad = m.w - len([]rune(left)) - len([]rune(mid))
		if pad < 0 {
			mid = ""
			pad = m.w - len([]rune(left))
			if pad < 0 {
				pad = 0
			}
		}
	}
	var b strings.Builder
	b.WriteString(bgSeq(BgHi))
	b.WriteString(fgSeq(Blue))
	b.WriteString(left)
	b.WriteString(fgSeq(Comment))
	b.WriteString(mid)
	b.WriteString(strings.Repeat(" ", pad))
	b.WriteString(right)
	b.WriteString(reset)
	return b.String()
}

func main() {
	list := flag.Bool("list", false, "list demos and exit")
	demo := flag.String("demo", "", "start on (or -snap render) this demo")
	snap := flag.Int("snap", -1, "headless: step N ticks, print one frame, exit (needs -demo)")
	width := flag.Int("w", 100, "width for -snap")
	height := flag.Int("h", 30, "height for -snap")
	tick := flag.Duration("tick", 60*time.Millisecond, "tick interval (app uses 60ms)")
	flag.Parse()

	if *list {
		for i, mk := range registry {
			d := mk()
			fmt.Printf("%2d  %-10s %s\n", i+1, d.Name(), d.Tagline())
		}
		return
	}

	startIdx := 0
	if *demo != "" {
		found := false
		for i, mk := range registry {
			if mk().Name() == *demo {
				startIdx, found = i, true
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "unknown demo %q — try -list\n", *demo)
			os.Exit(1)
		}
	}

	if *snap >= 0 {
		if *demo == "" {
			fmt.Fprintln(os.Stderr, "-snap needs -demo")
			os.Exit(1)
		}
		d := registry[startIdx]()
		d.Init(*width, *height)
		for n := 1; n <= *snap; n++ {
			d.Step(n)
		}
		fmt.Println(d.View() + reset)
		return
	}

	m := model{tick: *tick}.load(startIdx)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
