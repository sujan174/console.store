package dodge

// Tokyo Night palette, duplicated locally on purpose — dodge is a headless,
// decoupled package and must not import internal/tui/theme (see CLAUDE.md's
// note on deliberate palette duplication across the screens/tui boundary).
const (
	colBg     = "#15161f" // panel background (unused directly; cells default to fg only)
	colText   = "#a9b1d6" // default text
	colBright = "#c0caf5" // headings / hint text
	colDim    = "#565f89" // secondary / wheels / dimmed-on-death
	colFaint  = "#3b3b5a" // deep hints / ground line
	colCyan   = "#7dcfff" // scooter body
	colGreen  = "#9ece6a" // score/best strip accent
	colRed    = "#f7768e" // crashed strip accent
	colGold   = "#e0af68" // car body
)

// sprite is a small fixed glyph: rows of (rune, color) cells, top row first.
// A zero-value cell (rune 0) means "transparent" — the background/ground
// shows through instead of overwriting it.
type spriteCell struct {
	r rune
	c string
}

type sprite [][]spriteCell

// scooterSprite is the grounded rider: 3 cells wide, 2 rows tall.
//
//	,_o
//	O─O
func scooterSprite() sprite {
	return sprite{
		{{',', colCyan}, {'_', colCyan}, {'o', colCyan}},
		{{'O', colDim}, {'─', colDim}, {'O', colDim}},
	}
}

// carSmall is a 1-row car, width cells wide (2..4), all cells the same glyph.
func carSmall(width int) sprite {
	row := make([]spriteCell, width)
	for i := range row {
		switch i {
		case 0:
			row[i] = spriteCell{'[', colGold}
		case width - 1:
			row[i] = spriteCell{']', colGold}
		default:
			row[i] = spriteCell{'▪', colGold}
		}
	}
	return sprite{row}
}

// carTall is a 2-row car, width cells wide (2..4): a solid block top row and
// a wheel-hinted bottom row.
func carTall(width int) sprite {
	top := make([]spriteCell, width)
	bot := make([]spriteCell, width)
	for i := 0; i < width; i++ {
		switch i {
		case 0:
			top[i] = spriteCell{'▟', colGold}
		case width - 1:
			top[i] = spriteCell{'▙', colGold}
		default:
			top[i] = spriteCell{'█', colGold}
		}
		bot[i] = spriteCell{'▀', colGold}
	}
	return sprite{top, bot}
}

// carSprite picks the small or tall car glyph set for an obstacle's height.
func carSprite(width, height int) sprite {
	if height >= 2 {
		return carTall(width)
	}
	return carSmall(width)
}
