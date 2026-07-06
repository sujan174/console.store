// Render turns Game's headless state into a string for direct display: a
// w×h grid of colored cells, batched into ANSI runs per same-color span.
package dodge

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// cell is one glyph + foreground color in the render grid. A zero-value cell
// is a blank space with no color override.
type cell struct {
	r rune
	c string // "" = no color (plain space)
}

// grid is a w×h cell buffer, row-major, row 0 = top.
type grid struct {
	w, h  int
	cells [][]cell
}

func newGrid(w, h int) *grid {
	g := &grid{w: w, h: h, cells: make([][]cell, h)}
	for y := range g.cells {
		row := make([]cell, w)
		for x := range row {
			row[x] = cell{r: ' '}
		}
		g.cells[y] = row
	}
	return g
}

// set writes a single cell if (x,y) is in bounds.
func (g *grid) set(x, y int, r rune, color string) {
	if x < 0 || x >= g.w || y < 0 || y >= g.h {
		return
	}
	g.cells[y][x] = cell{r: r, c: color}
}

// blitSprite draws a sprite with its top-left at (x0,y0), skipping
// transparent (rune 0) cells and anything out of bounds.
func (g *grid) blitSprite(x0, y0 int, s sprite) {
	for dy, row := range s {
		for dx, c := range row {
			if c.r == 0 {
				continue
			}
			g.set(x0+dx, y0+dy, c.r, c.c)
		}
	}
}

// putText writes a plain string starting at (x0,y0) in the given color,
// clipped to the grid bounds.
func (g *grid) putText(x0, y0 int, s string, color string) {
	for i, r := range []rune(s) {
		g.set(x0+i, y0, r, color)
	}
}

// centerText writes s centered in row y across the full grid width.
func (g *grid) centerText(y int, s string, color string) {
	rs := []rune(s)
	x0 := (g.w - len(rs)) / 2
	if x0 < 0 {
		x0 = 0
	}
	g.putText(x0, y, s, color)
}

// render serializes the grid into exactly g.h lines, each exactly g.w
// visible runes wide, batching consecutive same-color cells into one ANSI
// span per ANSI-256-truecolor lipgloss style.
func (g *grid) render() string {
	var out strings.Builder
	for y, row := range g.cells {
		renderRow(&out, row)
		if y != len(g.cells)-1 {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

func renderRow(out *strings.Builder, row []cell) {
	i := 0
	for i < len(row) {
		j := i + 1
		for j < len(row) && row[j].c == row[i].c {
			j++
		}
		var sb strings.Builder
		for k := i; k < j; k++ {
			sb.WriteRune(row[k].r)
		}
		text := sb.String()
		if row[i].c == "" {
			out.WriteString(text)
		} else {
			out.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(row[i].c)).Render(text))
		}
		i = j
	}
}

// Render returns the current frame as a string: exactly g.h lines, each with
// an ANSI-stripped visible width of exactly g.w. The bottom row is always a
// slim status/hint strip; the rows above are the play field (ground line,
// scooter, cars).
func (g *Game) Render() string {
	w, h := g.w, g.h
	if w <= 0 {
		w = 1
	}
	if h <= 0 {
		h = 1
	}
	grd := newGrid(w, h)

	playH := g.playRows()
	ground := g.groundRow()

	switch g.state {
	case Attract:
		drawGround(grd, playH, ground, colFaint)
		drawScooter(grd, g.px, ground, 0)
		grd.centerText(h-1, "press ENTER — dodge the traffic while you wait", colBright)
	case Playing:
		drawGround(grd, playH, ground, colDim)
		drawObstacles(grd, g.obstacles, ground)
		drawScooter(grd, g.px, ground, g.py)
		strip := fmt.Sprintf("score %d   best %d", g.Score(), g.Best())
		grd.centerText(h-1, strip, colGreen)
	case Dead:
		drawGround(grd, playH, ground, colFaint)
		drawObstacles(grd, g.obstacles, ground)
		drawScooter(grd, g.px, ground, g.py)
		strip := fmt.Sprintf("crashed!  score %d  ·  ENTER to retry", g.Score())
		grd.centerText(h-1, strip, colRed)
	}

	return grd.render()
}

// drawGround paints the lowest play row (groundRow) as a solid line across
// the full width, for every row from 0..playH-1 only touching the ground row
// itself (rows above stay blank sky).
func drawGround(g *grid, playH, ground int, color string) {
	if ground < 0 || ground >= playH {
		return
	}
	for x := 0; x < g.w; x++ {
		g.set(x, ground, '_', color)
	}
}

// drawScooter blits the rider sprite so its bottom row sits on groundRow,
// raised by py rows (rounded to nearest cell) for the jump offset.
func drawScooter(g *grid, px float64, ground int, py float64) {
	s := scooterSprite()
	x0 := int(px)
	jump := int(py + 0.5)
	y0 := ground - (len(s) - 1) - jump
	g.blitSprite(x0, y0, s)
}

// drawObstacles blits each ground car so its bottom row sits on groundRow.
func drawObstacles(g *grid, obstacles []obstacle, ground int) {
	for _, o := range obstacles {
		s := carSprite(o.w, o.h)
		x0 := int(o.x)
		y0 := ground - (len(s) - 1)
		g.blitSprite(x0, y0, s)
	}
}
