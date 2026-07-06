package dodge

import "testing"

func newGame(t *testing.T, w, h int) *Game {
	t.Helper()
	g := New()
	g.SetSize(w, h)
	return g
}

// A grounded overlap collides; lifting the player above the car clears it.
func TestJumpClearsObstacle(t *testing.T) {
	g := newGame(t, 60, 8)
	g.Key("enter") // -> Playing
	g.forceObstacleUnderPlayer()
	if !g.hitTest() {
		t.Fatal("grounded overlap should collide")
	}
	g.py = 3 // above a 1-2 row car
	if g.hitTest() {
		t.Fatal("airborne player should clear a ground car")
	}
}

// Running into a car while grounded kills.
func TestCollisionKills(t *testing.T) {
	g := newGame(t, 60, 8)
	g.Key("enter")
	g.forceObstacleUnderPlayer()
	for i := 1; i <= 3; i++ {
		g.Tick(i)
	}
	if g.State() != Dead {
		t.Fatalf("expected Dead, got %v", g.State())
	}
}

// Speed ramps upward but never exceeds the cap.
func TestSpeedBounded(t *testing.T) {
	g := newGame(t, 60, 8)
	g.Key("enter")
	for i := 1; i <= 20000; i++ {
		g.Tick(i)
		if g.State() == Dead {
			g.Key("enter")
		}
		if g.speed > maxSpeed+1e-9 {
			t.Fatalf("speed %.3f exceeded cap", g.speed)
		}
	}
	if g.speed <= startSpeed {
		t.Fatal("speed never ramped")
	}
}

// Fairness: the reference bot survives long runs with only rare high-speed
// deaths — an unjumpable obstacle would spike this into the hundreds.
func TestObstaclesAreClearable(t *testing.T) {
	total := 0
	for seed := 1; seed <= 5; seed++ {
		g := New()
		g.SetSize(60, 8)
		g.seed(uint64(seed))
		g.Key("enter")
		for i := 1; i <= 6000; i++ {
			g.botStep()
			prev := g.State()
			g.Tick(i)
			if prev == Playing && g.State() == Dead {
				total++
				g.Key("enter")
			}
		}
	}
	if total > 25 {
		t.Fatalf("bot died %d times — an obstacle is likely unjumpable/unfair", total)
	}
}
