// Package dodge is a headless endless-runner minigame core, embedded in the
// order-tracking TUI. Coordinates are cell resolution (terminal columns/rows),
// not pixels. This file has no rendering — see the render package that wraps
// Game for the visual layer.
//
// Ported and shrunk from experiments/scooterjump/world.go: same reset/step/
// spawn/hitTest/die structure and coyote+jump-buffer feel, but jump-only (no
// duck/drone) and tuned to cell units instead of pixels.
package dodge

import (
	"math"
	"math/rand/v2"
	"sync/atomic"
)

// State is the game's coarse phase.
type State int

const (
	Attract State = iota
	Playing
	Dead
)

// Tuning constants (cell resolution, one Tick == one 60ms frame).
//
// airtime ≈ 2*jumpV/gravity ≈ 36 frames, peak height ≈ jumpV²/(2*gravity) ≈ 3.6 rows.
// (Started from gravity=0.030/jumpV=0.42 per the design doc; raised airtime/
// peak here so a jump reliably clears a 2-row car — see botLead below.)
const (
	gravity = 0.022
	jumpV   = 0.40

	startSpeed = 0.32
	maxSpeed   = 0.85
	speedRamp  = 0.00035 // per frame, ×current speed

	coyoteFrames = 3
	bufferFrames = 5
)

// airtimeFrames is the nominal hang time of a jump; spawn gaps never go below
// this + a small margin so the previous jump can always land before the next
// obstacle needs one.
var airtimeFrames = int(math.Floor(2 * jumpV / gravity)) // ≈ 36

const minGapMargin = 8

// obstacle is a ground car: a rectangle in cell space moving left at speed
// cells/frame. All are jump-over (no ducking, no aerial obstacles).
type obstacle struct {
	x      float64 // left cell, world moves it leftward
	w, h   int     // width/height in cells
	passed bool
}

// Game is the headless runner core: physics, spawns, collision, scoring and
// difficulty ramp. No rendering.
type Game struct {
	w, h int // panel size in cells
	rng  *rand.Rand

	state State
	frame int // frames since last reset/death settle

	// player
	px       float64 // fixed column
	py       float64 // rows above ground (0 = grounded)
	vy       float64
	grounded bool
	airT     int // frames since last grounded (for coyote)
	bufT     int // jump buffer countdown

	speed    float64
	distance float64
	score    int
	best     int

	obstacles []obstacle
	spawnIn   int
}

var seedNonce atomic.Uint64

// New creates a fresh Game in the Attract state, seeded from a package-level
// nonce so successive games in the same process don't repeat.
func New() *Game {
	n := seedNonce.Add(1)
	g := &Game{rng: rand.New(rand.NewPCG(n, 0xC0FFEE))}
	g.reset()
	g.state = Attract
	return g
}

// SetSize sets the panel size in terminal cells (w cols, h rows). The
// playfield is the panel minus a 1-row bottom status strip; the ground is the
// lowest play row.
func (g *Game) SetSize(w, h int) {
	g.w, g.h = w, h
	g.px = math.Max(2, float64(w)*0.14)
}

// seed re-seeds the RNG deterministically. Test-only helper.
func (g *Game) seed(s uint64) {
	g.rng = rand.New(rand.NewPCG(s, 0xC0FFEE))
}

// playRows is the number of rows available for gameplay above the bottom
// status strip.
func (g *Game) playRows() int {
	if g.h <= 1 {
		return 1
	}
	return g.h - 1
}

// groundRow is the lowest play row (0 = grounded baseline).
func (g *Game) groundRow() int { return g.playRows() - 1 }

func (g *Game) reset() {
	g.state = Playing
	g.frame = 0
	g.py, g.vy = 0, 0
	g.grounded = true
	g.airT = 0
	g.bufT = 0
	g.speed = startSpeed
	g.distance = 0
	g.score = 0
	g.obstacles = g.obstacles[:0]
	g.spawnIn = 40
}

// State returns the current coarse game phase.
func (g *Game) State() State { return g.state }

// Score returns the current distance-based score.
func (g *Game) Score() int { return g.score }

// Best returns the in-session best score (updated on death).
func (g *Game) Best() int { return g.best }

// Key handles only "enter" and "space"; all other input is ignored.
func (g *Game) Key(k string) {
	switch k {
	case "enter":
		switch g.state {
		case Attract, Dead:
			g.reset()
		}
	case "space":
		if g.state == Playing {
			g.bufT = bufferFrames
		}
	}
}

func (g *Game) requestJump() {
	g.bufT = bufferFrames
}

func (g *Game) doJump() {
	g.vy = jumpV
	g.grounded = false
}

// Tick advances the game by one 60ms frame.
func (g *Game) Tick(frame int) {
	g.frame = frame
	if g.state != Playing {
		return
	}

	// difficulty ramp
	g.speed = math.Min(maxSpeed, g.speed+speedRamp*g.speed)
	g.distance += g.speed
	g.score = int(g.distance / 4)

	// jump buffer / coyote time
	if g.bufT > 0 {
		g.bufT--
	}
	canJump := g.grounded || g.airT < coyoteFrames
	if g.bufT > 0 && canJump && g.vy <= 0 {
		g.doJump()
		g.bufT = 0
	}
	if !g.grounded {
		g.airT++
		g.vy -= gravity
		g.py += g.vy
		if g.py <= 0 {
			g.py = 0
			g.vy = 0
			g.grounded = true
			g.airT = 0
		}
	}

	// spawns
	g.spawnIn--
	if g.spawnIn <= 0 {
		g.spawn()
	}

	// move obstacles
	kept := g.obstacles[:0]
	for _, o := range g.obstacles {
		o.x -= g.speed
		if !o.passed && o.x+float64(o.w) < g.px {
			o.passed = true
		}
		if o.x+float64(o.w) > -2 {
			kept = append(kept, o)
		}
	}
	g.obstacles = kept

	// collision
	if g.hitTest() {
		g.die()
	}
}

func (g *Game) die() {
	g.state = Dead
	g.frame = 0
	if g.score > g.best {
		g.best = g.score
	}
}

// spawn appends a new ground car (2-4 cells wide, 1-2 rows tall) offscreen to
// the right, and schedules the next spawn. The gap floor guarantees the
// previous jump can always land before the next obstacle needs one.
func (g *Game) spawn() {
	diff := (g.speed - startSpeed) / (maxSpeed - startSpeed)

	ow := 2 + g.rng.IntN(3) // 2..4
	oh := 1 + g.rng.IntN(2) // 1..2
	o := obstacle{x: float64(g.w) + 2, w: ow, h: oh}
	g.obstacles = append(g.obstacles, o)

	base := 52.0 - 16*diff
	jitter := g.rng.Float64() * (26 - 10*diff)
	gap := int(base + jitter)
	minGap := airtimeFrames + minGapMargin
	if gap < minGap {
		gap = minGap
	}
	g.spawnIn = gap
}

// hitTest does shrunk-AABB collision in cell space. The player occupies a
// 1-cell-wide column at px; grounded means the player's box sits on the
// ground row, airborne raises its top/bottom by py rows.
func (g *Game) hitTest() bool {
	// player box: 1 cell wide, 1 cell tall, riding at row (groundRow - py).
	pl := g.px
	pr := g.px + 1
	pbot := float64(g.groundRow()) - g.py + 1 // bottom edge (rows are top-down; +1 = just below deck)
	ptop := pbot - 1

	for _, o := range g.obstacles {
		ol := o.x
		or := o.x + float64(o.w)
		obot := float64(g.groundRow()) + 1
		otop := obot - float64(o.h)
		if pr > ol && pl < or && pbot > otop && ptop < obot {
			return true
		}
	}
	return false
}

// forceObstacleUnderPlayer appends a 1-cell-ahead ground car overlapping the
// player. Test-only helper.
func (g *Game) forceObstacleUnderPlayer() {
	g.obstacles = append(g.obstacles, obstacle{x: g.px, w: 3, h: 2})
}

// botLead is how many frames of warning the bot gives itself before an
// obstacle's leading edge would reach the player's box — enough for the jump
// to clear peak-obstacle height (2 rows) before the cars arrives, at any
// speed. Cell-resolution analogue of scooterjump's "8*speed" pixel lead: the
// trigger distance is speed-relative frames-to-arrival, just larger because a
// row of clearance is a bigger fraction of the jump arc than a pixel of it.
const botLead = 10

// botStep is the reference autopilot used by the clearability test: jump when
// the nearest ground car's leading edge is within ≈botLead*speed cells of the
// player's front and grounded.
func (g *Game) botStep() {
	if g.state == Attract || g.state == Dead {
		g.Key("enter")
		return
	}
	if !g.grounded {
		return
	}
	front := g.px + 1
	for _, o := range g.obstacles {
		if o.x < front {
			continue // already passed the player's leading edge
		}
		if o.x-front <= botLead*g.speed {
			g.requestJump()
			return
		}
	}
}
