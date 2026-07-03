package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"

	xterm "github.com/charmbracelet/x/term"

	"consolestore/internal/localstore"
)

// pickPreset lets the user choose among same-named presets. On a real terminal
// it's an arrow-key picker (↑/↓ or j/k to move, ↵ to choose, a number to jump
// straight to it, q/Esc/Ctrl-C to cancel). Otherwise — tests, pipes, or a
// non-terminal stdin — it falls back to a numbered list + a typed number.
// avail marks unavailable candidates inline (nil/short = no marking).
// Returns the chosen index; ok=false on cancel.
func pickPreset(d Deps, name string, matches []localstore.Preset, avail []availability, st style) (int, bool) {
	if f, isFile := d.In.(*os.File); isFile {
		if idx, picked, handled := rawPick(f, d.Out, name, matches, avail, st); handled {
			return idx, picked
		}
	}
	// Fallback: list + typed number (no raw terminal available).
	listPresets(d.Out, name, matches, avail, st)
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

// rawPick runs the in-place arrow-key picker in raw terminal mode. handled is
// false when raw mode can't be entered (the caller then falls back). Rows are
// truncated to the terminal width so they never soft-wrap — a wrapped line would
// break the cursor-up redraw math and the list would stack/glitch.
func rawPick(in *os.File, out io.Writer, name string, matches []localstore.Preset, avail []availability, st style) (idx int, picked, handled bool) {
	old, err := xterm.MakeRaw(in.Fd())
	if err != nil {
		return 0, false, false
	}
	defer xterm.Restore(in.Fd(), old)

	fmt.Fprint(out, "\x1b[?25l")           // hide cursor (no flicker on redraw)
	defer fmt.Fprint(out, "\x1b[?25h\r\n") // show cursor + drop to a fresh line on exit

	width, _, werr := xterm.GetSize(in.Fd())
	if werr != nil || width < 20 {
		width = 80
	}
	maxRow := width - 1

	cur := 0
	rows := len(matches) + 1 // header + N rows, redrawn in place
	draw := func(first bool) {
		if !first {
			fmt.Fprintf(out, "\x1b[%dA", rows) // cursor up to the header
		}
		header := truncate(fmt.Sprintf("%d presets named %q  ·  ↑/↓ then ↵, or a number", len(matches), name), maxRow)
		fmt.Fprintf(out, "\r\x1b[K%s\r\n", st.dim(header))
		for i, p := range matches {
			// number + restaurant + items (no address — it's in the bill after).
			text := truncate(fmt.Sprintf("%d) %s  ·  %s", i+1, p.RestaurantName, summarize(p)), maxRow-2)
			suffix := ""
			if i < len(avail) {
				suffix = soldOutSuffix(avail[i], st)
			}
			if i == cur {
				fmt.Fprintf(out, "\r\x1b[K%s%s%s\r\n", st.num("❯ "), st.bright(text), suffix)
			} else {
				fmt.Fprintf(out, "\r\x1b[K  %s%s\r\n", st.text(text), suffix)
			}
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
