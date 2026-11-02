package game

import (
	"bytes"
	"hash/fnv"
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
	a.Lvl, a.XP, a.Gold = 3, 17, 55
	a.AddItem("firebomb", 2)
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
	if a.Lvl != 3 || a.XP != 17 || a.Gold != 55 || a.Count("potion") != StartPotions || a.Count("firebomb") != 2 {
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
	for _, seed := range []uint32{1, 7, 42, 99, 1234} {
		for d := 1; d <= MaxDepth; d++ {
			checkFloor(t, seed, d)
		}
	}
}

func checkFloor(t *testing.T, seed uint32, d int) {
	t.Helper()
	{
		a, b := Generate(seed, d, 2), Generate(seed, d, 2)
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
		if want := d%BossEvery == 0 || ThemeAt(d).Boss != 0; boss != want {
			t.Errorf("depth %d: boss=%v, want %v", d, boss, want)
		}
	}
}

func TestThemeBands(t *testing.T) {
	want := map[int]string{1: "cathedral", 2: "cathedral", 3: "crypt", 5: "crypt", 6: "caverns", 8: "abyss", 9: "ember", 10: "ember"}
	for d, id := range want {
		if got := Floor(d).Theme.ID; got != id {
			t.Errorf("depth %d: theme %s, want %s", d, got, id)
		}
	}
	for id, th := range Themes {
		if th.ID != id || th.Name == "" || (th.Kinds == "" && id != "village") || th.Decor == "" {
			t.Errorf("theme %s is incomplete: %+v", id, th)
		}
		for i := 0; i < len(th.Kinds); i++ {
			if _, ok := MonDefs[th.Kinds[i]]; !ok {
				t.Errorf("theme %s: unknown monster kind %q", id, th.Kinds[i])
			}
		}
	}
}

// Floors 1-2 (Cathedral) must keep the exact layouts they had before themes existed.
func TestCathedralLayoutUnchanged(t *testing.T) {
	for _, c := range []struct {
		seed  uint32
		depth int
		hash  uint64
		mons  int
	}{
		{1, 1, 0x6a6434a99685e218, 90}, {1, 2, 0xb1351cb354c1de60, 98},
		{7, 1, 0xb32a2bd04d332d20, 93}, {7, 2, 0x66e8e29525baaf37, 96},
		{42, 1, 0xdd1294b8a0559549, 91}, {42, 2, 0x476e4d91ecd37642, 95},
		{99, 1, 0x79a52f7311be919, 95}, {99, 2, 0x924ea70243cdaa7f, 97},
		{1234, 1, 0xc90aafc1b84204a3, 83}, {1234, 2, 0xc071255b0f7e4f10, 86},
	} {
		l := Generate(c.seed, c.depth, 2)
		h := fnv.New64a()
		h.Write(l.Tiles)
		if h.Sum64() != c.hash || len(l.Mons) != c.mons {
			t.Errorf("seed %d depth %d changed: hash %#x mons %d", c.seed, c.depth, h.Sum64(), len(l.Mons))
		}
	}
}

func TestCaveFloors(t *testing.T) {
	for _, seed := range []uint32{1, 7, 42, 99, 1234} {
		l := Generate(seed, 6, 2)
		if !l.Theme.Caves {
			t.Fatal("depth 6 should be caverns")
		}
		for _, tl := range l.Tiles {
			if tl == TDoor || tl == TPillar {
				t.Fatalf("seed %d: caves have no doors or pillars", seed)
			}
		}
		br := l.Rooms[l.BossRoom]
		st := l.Stairs
		if st.X < br.X0 || st.X > br.X1 || st.Y < br.Y0 || st.Y > br.Y1 {
			t.Errorf("seed %d: stairs %v outside the boss room %v", seed, st, br)
		}
		if !l.Walkable(l.Start.X, l.Start.Y) {
			t.Errorf("seed %d: start not walkable", seed)
		}
		// a cave room is not a full rectangle
		r := l.Rooms[0]
		open := 0
		for y := r.Y0; y <= r.Y1; y++ {
			for x := r.X0; x <= r.X1; x++ {
				if l.Walkable(x, y) {
					open++
				}
			}
		}
		if open == r.W()*r.H() {
			t.Errorf("seed %d: cave room is a plain rectangle", seed)
		}
	}
}

func TestGlowsOnlyWhereThemed(t *testing.T) {
	for d := 1; d <= MaxDepth; d++ {
		l := Generate(42, d, 2)
		if len(l.Glows) > MaxGlows {
			t.Errorf("depth %d: %d glows over the cap", d, len(l.Glows))
		}
		if (l.Theme.Glow == 0) != (len(l.Glows) == 0) {
			t.Errorf("depth %d (%s): %d glows", d, l.Theme.ID, len(l.Glows))
		}
		for _, g := range l.Glows {
			if l.At(g.X, g.Y) != l.Theme.Glow {
				t.Errorf("depth %d: glow at %v is %q", d, g, l.At(g.X, g.Y))
			}
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
	a.Gold = 30
	a.AddItem("gem", 3)
	g.arrival[1] = a.save() // as if they arrived with this
	arrived := a.save()
	layout := append([]byte(nil), g.Level.Tiles...)

	a.Gold, a.XP = 99, 40
	a.Pack = [PackSlots]Stack{}
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
