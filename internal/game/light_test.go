package game

import "testing"

// darkArena is the test arena with the pitch-black theme.
func darkArena(n int) *Game {
	g := arena(n)
	g.Level.Theme = Themes["dark"]
	return g
}

func TestDarkFloorOnlyShowsLitCells(t *testing.T) {
	g := darkArena(1)
	h := hero(g, 1, Warrior, 30, 25)
	m := g.spawnMon('z', 40, 25) // 10 cells away: in line of sight, but unlit
	m.StunUntil = 1 << 30
	g.Tick()
	if !g.IsVisible(31, 25) || g.IsVisible(36, 25) || g.IsVisible(m.X, m.Y) {
		t.Fatal("without a torch a hero sees only the cells next to them")
	}
	if err := g.Apply(1, Intent{Kind: IntUse, Slot: h.firstSlot("torch")}); err != nil {
		t.Fatal(err)
	}
	g.Tick()
	if !g.IsVisible(40, 25) || g.IsVisible(50, 25) {
		t.Fatal("a lit torch shows about 12 cells")
	}
	if h.Count("torch") != StartTorches-1 {
		t.Fatal("lighting a torch uses one")
	}
	g.Now += TorchTicks
	g.Tick()
	if g.IsVisible(40, 25) {
		t.Fatal("the torch burns out")
	}
}

func TestLightOrbLightsAnAreaForEveryone(t *testing.T) {
	g := darkArena(1)
	h := hero(g, 1, Sorcerer, 30, 25)
	h.AddItem("lightorb", 1)
	slot := h.firstSlot("lightorb")
	if err := g.Apply(1, Intent{Kind: IntUse, Slot: slot, X: 58, Y: 25}); err != ErrRange {
		t.Fatalf("orbs have a range, got %v", err)
	}
	if err := g.Apply(1, Intent{Kind: IntUse, Slot: slot, X: 42, Y: 25}); err != nil {
		t.Fatal(err)
	}
	g.Tick()
	if !g.IsVisible(42, 25) || !g.IsVisible(46, 25) || g.IsVisible(54, 25) {
		t.Fatal("the orb lights the area around it (and only that)")
	}
	g.Now += OrbTicks
	g.Tick()
	if g.IsVisible(42, 25) || len(g.Orbs) != 0 {
		t.Fatal("the orb goes out after a minute")
	}
}

func TestLitFloorsIgnoreLight(t *testing.T) {
	g := arena(1)
	hero(g, 1, Warrior, 30, 25)
	g.Tick()
	if !g.IsVisible(45, 25) {
		t.Fatal("normal floors see as far as before")
	}
}

func TestFloorSevenIsDark(t *testing.T) {
	for d := 1; d <= MaxDepth; d++ {
		if got := Floor(d).Theme.Dark; got != (d == 7) {
			t.Errorf("depth %d dark=%v", d, got)
		}
	}
	l := Generate(42, 7, 2)
	if len(l.Torches) != 0 || len(l.Glows) != 0 {
		t.Fatal("the dark floor has no wall torches or glowing decor")
	}
}

func TestTorchGoesOutOnTheStairs(t *testing.T) {
	g := New(5, Settings{}, 1)
	h := g.AddHero(1, "kmerz", Monk)
	_ = g.Apply(1, Intent{Kind: IntUse, Slot: h.firstSlot("torch")})
	takeStairs(g)
	if h.TorchUntil != 0 {
		t.Fatal("a lit torch doesn't carry over to the next floor")
	}
}
