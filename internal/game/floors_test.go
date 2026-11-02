package game

import (
	"bytes"
	"testing"
)

// clearMons removes every monster so tests can wait at the stairs undisturbed.
func clearMons(g *Game) {
	for _, m := range g.Mons {
		g.occ[m.Y*LW+m.X] = 0
	}
	g.Mons, g.monByID = nil, map[int]*Monster{}
}

// takeStairs puts every hero next to the stairs and ticks until the window completes.
func takeStairs(g *Game) {
	clearMons(g)
	st := g.Level.Stairs
	for i, h := range g.Heroes {
		if h.Dead {
			continue
		}
		p, _ := g.Level.nearestFree(Pt{st.X, st.Y}, func(x, y int) bool { return g.occ[y*LW+x] == 0 })
		_ = i
		g.place(h, p.X, p.Y)
	}
	for i := 0; i < StairsWindow+2; i++ {
		g.Tick()
	}
}

func TestDescendKeepsProgressAndLoadsNewFloor(t *testing.T) {
	g := New(42, Settings{}, 2)
	a := g.AddHero(1, "kmerz", Warrior)
	b := g.AddHero(2, "nyx", Rogue)
	a.Lvl, a.XP, a.Gold, a.Potions = 3, 17, 55, 4
	b.HP = 0
	b.Downed = true
	old := append([]byte(nil), g.Level.Tiles...)
	g.TakeLevelChanged()

	takeStairs(g)
	if g.Phase != PhasePlaying || g.Depth != 2 {
		t.Fatalf("expected to be playing on depth 2, got phase %v depth %d", g.Phase, g.Depth)
	}
	if !g.TakeLevelChanged() || g.TakeLevelChanged() {
		t.Fatal("level change should be reported exactly once")
	}
	if bytes.Equal(old, g.Level.Tiles) {
		t.Fatal("depth 2 should have a different layout")
	}
	if a.Lvl != 3 || a.XP != 17 || a.Gold != 55 || a.Potions != 4 {
		t.Fatalf("progress must carry over: %+v", a.save())
	}
	if b.Downed || b.Dead || b.HP != b.HPMax/2 {
		t.Fatalf("the fallen stand up at half HP, got HP %d/%d downed=%v", b.HP, b.HPMax, b.Downed)
	}
	for _, h := range g.Heroes {
		if cheb(h.X, h.Y, g.Level.Start.X, g.Level.Start.Y) > 3 {
			t.Errorf("%s should start near the start room, at %d,%d", h.Name, h.X, h.Y)
		}
	}
}

func TestFloorsAreDeterministicAndBossesEveryFifth(t *testing.T) {
	for d := 1; d <= MaxDepth; d++ {
		a, b := Generate(7, d, 2), Generate(7, d, 2)
		if !bytes.Equal(a.Tiles, b.Tiles) {
			t.Fatalf("depth %d is not deterministic", d)
		}
		if !a.Reachable() {
			t.Fatalf("depth %d: stairs unreachable", d)
		}
		boss := false
		for _, m := range a.Mons {
			boss = boss || m.Elite
		}
		if want := d%BossEvery == 0; boss != want {
			t.Errorf("depth %d: boss=%v, want %v", d, boss, want)
		}
	}
}

func TestDepthScalesMonsters(t *testing.T) {
	g := New(9, Settings{}, 1)
	hp1 := map[byte]int{}
	for _, m := range g.Mons {
		hp1[m.Kind] = m.HPMax
	}
	g.loadFloor(4)
	for _, m := range g.Mons {
		if base, ok := hp1[m.Kind]; ok && m.HPMax <= base {
			t.Fatalf("%c on depth 4 should have more HP than on depth 1 (%d vs %d)", m.Kind, m.HPMax, base)
		}
	}
}

func TestLastFloorStairsClearTheRun(t *testing.T) {
	g := New(3, Settings{}, 1)
	g.AddHero(1, "kmerz", Monk)
	g.loadFloor(MaxDepth)
	takeStairs(g)
	if g.Phase != PhaseCleared || g.Depth != MaxDepth {
		t.Fatalf("expected the run to be cleared on depth %d, got phase %v depth %d", MaxDepth, g.Phase, g.Depth)
	}
}

func TestRetryFloorRestoresArrival(t *testing.T) {
	g := New(11, Settings{}, 1)
	a := g.AddHero(1, "kmerz", Sorcerer)
	takeStairs(g) // now on depth 2
	a.Gold, a.Potions = 30, 3
	g.arrival[1] = a.save() // as if they arrived with this
	arrived := a.save()
	layout := append([]byte(nil), g.Level.Tiles...)

	a.Gold, a.Potions, a.XP = 99, 0, 40
	downHeroNow(g, a)
	for i := 0; i < 3; i++ {
		g.Tick()
	}
	if g.Phase != PhaseWiped {
		t.Fatal("expected a wipe")
	}
	late := g.AddHero(2, "late", Rogue)
	late.Lvl = 5

	g.RetryFloor()
	if g.Phase != PhasePlaying || g.Depth != 2 {
		t.Fatalf("retry should replay depth 2, got phase %v depth %d", g.Phase, g.Depth)
	}
	if !bytes.Equal(layout, g.Level.Tiles) {
		t.Fatal("retry should load the same layout")
	}
	if a.Downed || a.save() != arrived {
		t.Fatalf("hero should be back as they arrived: %+v downed=%v", a.save(), a.Downed)
	}
	if late.Lvl != 1 {
		t.Fatalf("a hero who joined during the floor starts fresh, got level %d", late.Lvl)
	}
}
