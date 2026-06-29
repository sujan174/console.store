package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"

	xterm "github.com/charmbracelet/x/term"

	"console.store/internal/localstore"
)

// pickPreset lets the user choose among same-named presets. On a real terminal
// it's an arrow-key picker (↑/↓ or j/k to move, ↵ to choose, a number to jump
// straight to it, q/Esc/Ctrl-C to cancel). Otherwise — tests, pipes, or a
// non-terminal stdin — it falls back to a numbered list + a typed number.
// Returns the chosen index; ok=false on cancel.
func pickPreset(d Deps, name string, matches []localstore.Preset, st style) (int, bool) {
	if f, isFile := d.In.(*os.File); isFile {
		if idx, picked, handled := rawPick(f, d.Out, name, matches, st); handled {
			return idx, picked
		}
	}
	// Fallback: list + typed number (no raw terminal available).
	listPresets(d.Out, name, matches, st)
	fmt.Fprintf(d.Out, "\n%s ", st.dim(fmt.Sprintf("pick 1-%d:", len(matches))))
	sel := prompt(d)
	if sel == "" {
		return 0, false
	}
	n, err := strconv.Atoi(sel)
	if err != nil || n < 1 || n > len(matches) {
		return 0, false
	}
	return n - 1, true
}

// pickRow renders one preset row for the arrow picker. The selected row gets a
// ❯ marker + highlighted name.
func pickRow(i int, p localstore.Preset, st style, selected bool) string {
	num := strconv.Itoa(i+1) + ")"
	tail := st.dim("· " + shortAddr(p.AddrLine) + " · " + summarize(p))
	if selected {
		return st.num("❯ "+num) + " " + st.ok(p.RestaurantName) + " " + tail
	}
	return "  " + st.dim(num) + " " + st.head(p.RestaurantName) + " " + tail
}

// rawPick runs the in-place arrow-key picker in raw terminal mode. handled is
// false when raw mode can't be entered (the caller then falls back).
func rawPick(in *os.File, out io.Writer, name string, matches []localstore.Preset, st style) (idx int, picked, handled bool) {
	old, err := xterm.MakeRaw(in.Fd())
	if err != nil {
		return 0, false, false
	}
	defer xterm.Restore(in.Fd(), old)

	cur := 0
	rows := len(matches) + 1 // header + N rows, redrawn in place
	draw := func(first bool) {
		if !first {
			fmt.Fprintf(out, "\x1b[%dA", rows) // cursor up to the header
		}
		fmt.Fprintf(out, "\r\x1b[K%s\r\n", st.dim(fmt.Sprintf("%d presets named %q  ·  ↑/↓ then ↵, or a number:", len(matches), name)))
		for i, p := range matches {
			fmt.Fprintf(out, "\r\x1b[K%s\r\n", pickRow(i, p, st, i == cur))
		}
	}
	draw(true)

	buf := make([]byte, 8)
	for {
		n, err := in.Read(buf)
		if err != nil || n == 0 {
			return 0, false, true
		}
		b := buf[:n]
		switch {
		case b[0] == 3, b[0] == 'q': // Ctrl-C / q → cancel
			return 0, false, true
		case b[0] == '\r' || b[0] == '\n': // Enter → choose highlighted
			return cur, true, true
		case b[0] >= '1' && b[0] <= '9': // number → jump straight to it
			if j := int(b[0] - '1'); j < len(matches) {
				return j, true, true
			}
		case n == 1 && b[0] == 0x1b: // bare Esc → cancel
			return 0, false, true
		case n >= 3 && b[0] == 0x1b && b[1] == '[': // arrow keys
			if b[2] == 'A' && cur > 0 {
				cur--
			} else if b[2] == 'B' && cur < len(matches)-1 {
				cur++
			}
			draw(false)
		case b[0] == 'k' && cur > 0:
			cur--
			draw(false)
		case b[0] == 'j' && cur < len(matches)-1:
			cur++
			draw(false)
		}
	}
}
