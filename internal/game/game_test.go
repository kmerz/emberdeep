package game

import "testing"

func TestDungeonConnectivity(t *testing.T) {
	for seed := uint32(1); seed <= 60; seed++ {
		for _, n := range []int{1, 4} {
			l := Generate(seed*7919, n)
			if len(l.Rooms) < 6 {
				t.Fatalf("seed %d: only %d rooms", seed, len(l.Rooms))
			}
			if !l.Reachable() {
				t.Fatalf("seed %d: level not connected", seed)
			}
			for _, m := range l.Mons {
				if !l.Walkable(m.X, m.Y) {
					t.Fatalf("seed %d: monster in wall at %d,%d", seed, m.X, m.Y)
				}
			}
		}
	}
}

func TestDeterministicGeneration(t *testing.T) {
	a, b := Generate(1234, 2), Generate(1234, 2)
	if string(a.Tiles) != string(b.Tiles) || len(a.Mons) != len(b.Mons) {
		t.Fatal("generation is not deterministic")
	}
}

func fovSet(l *Level, x, y int, r float64) map[Pt]bool {
	s := map[Pt]bool{}
	ComputeFOV(l, x, y, r, func(px, py int) { s[Pt{px, py}] = true })
	return s
}

func TestFOVSymmetry(t *testing.T) {
	l := Generate(99, 1)
	rng := NewRNG(5)
	checked := 0
	for i := 0; i < 40; i++ {
		r := l.Rooms[rng.Intn(len(l.Rooms))]
		ax, ay := r.X0+rng.Intn(r.W()), r.Y0+rng.Intn(r.H())
		if !l.Walkable(ax, ay) {
			continue
		}
		vis := fovSet(l, ax, ay, VisionRadius)
		for p := range vis {
			if !l.Walkable(p.X, p.Y) {
				continue
			}
			back := fovSet(l, p.X, p.Y, VisionRadius)
			if !back[Pt{ax, ay}] {
				t.Fatalf("asymmetric: %d,%d sees %d,%d but not back", ax, ay, p.X, p.Y)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("nothing checked")
	}
}

func TestFOVWallBlocks(t *testing.T) {
	l := &Level{W: LW, H: LH, Tiles: make([]byte, LW*LH)}
	for i := range l.Tiles {
		l.Tiles[i] = TFloor
	}
	for y := 0; y < LH; y++ {
		l.Tiles[y*LW+20] = TWall
	}
	vis := fovSet(l, 10, 30, VisionRadius)
	if !vis[Pt{20, 30}] {
		t.Fatal("should see the wall face")
	}
	if vis[Pt{22, 30}] {
		t.Fatal("should not see behind the wall")
	}
	if !vis[Pt{15, 30}] {
		t.Fatal("should see open floor")
	}
}

func newTestGame(n int) (*Game, []*Hero) {
	g := New(42, Settings{}, n)
	var hs []*Hero
	for i := 1; i <= n; i++ {
		hs = append(hs, g.AddHero(i, "p"+string(rune('a'+i)), Sorcerer))
	}
	g.Tick()
	return g, hs
}

func TestMoveSpeedAndTolerance(t *testing.T) {
	g, hs := newTestGame(1)
	h := hs[0]
	for i := 0; i < 5; i++ {
		g.Tick()
	}
	moved := 0
	for d := 0; d < 8; d++ {
		nx, ny := h.X+dirs8[d].X, h.Y+dirs8[d].Y
		if g.cellFree(nx, ny) {
			if err := g.Apply(h.ID, Intent{Kind: IntMove, Dir: d, Seq: 1}); err != nil {
				t.Fatal(err)
			}
			moved++
			// second step in the same tick must be rejected
			if err := g.Apply(h.ID, Intent{Kind: IntMove, Dir: d, Seq: 2}); err != ErrTooFast {
				t.Fatalf("expected ErrTooFast, got %v", err)
			}
			break
		}
	}
	if moved == 0 {
		t.Fatal("no free neighbour")
	}
}

func TestSharedVisionUnion(t *testing.T) {
	g := New(42, Settings{}, 2)
	a := g.AddHero(1, "aa", Sorcerer)
	// place the second hero in the farthest room
	far := g.Level.Rooms[g.Level.BossRoom].Center()
	b := g.AddHero(2, "bb", Sorcerer)
	g.moveEntity(2, &b.X, &b.Y, far.X, far.Y)
	g.Tick()
	if !g.IsVisible(a.X, a.Y) || !g.IsVisible(b.X, b.Y) {
		t.Fatal("both heroes' cells must be visible")
	}
	if !g.Explored[far.Y*LW+far.X] || !g.Explored[a.Y*LW+a.X] {
		t.Fatal("exploration must be shared")
	}
	// deltas are reported once
	first := g.TakeExplored()
	if len(first) == 0 {
		t.Fatal("expected explore deltas")
	}
	g.Tick()
	if again := g.TakeExplored(); len(again) != 0 {
		t.Fatalf("no new tiles expected, got %d", len(again))
	}
	if len(g.ExploredTiles()) != len(first) {
		t.Fatal("full sync must equal the sum of deltas")
	}
	// removing a hero removes its vision but not exploration
	g.RemoveHero(2)
	g.Tick()
	if g.IsVisible(far.X, far.Y) && dist(a.X, a.Y, far.X, far.Y) > VisionRadius {
		t.Fatal("vision of a removed hero must go away")
	}
	if !g.Explored[far.Y*LW+far.X] {
		t.Fatal("explored tiles persist")
	}
}

func TestJoinInProgressSpawnsNextToParty(t *testing.T) {
	g := New(42, Settings{}, 1)
	a := g.AddHero(1, "aa", Warrior)
	g.moveEntity(1, &a.X, &a.Y, g.Level.Rooms[g.Level.BossRoom].Center().X-2, g.Level.Rooms[g.Level.BossRoom].Center().Y+1)
	b := g.AddHero(2, "bb", Rogue)
	if cheb(a.X, a.Y, b.X, b.Y) > 3 {
		t.Fatalf("new hero spawned %d cells away", cheb(a.X, a.Y, b.X, b.Y))
	}
	if g.occ[b.Y*LW+b.X] != 2 {
		t.Fatal("occupancy not set")
	}
	// nobody alive: spawn in the start room
	a.Dead, b.Dead = true, true
	c := g.AddHero(3, "cc", Monk)
	if cheb(c.X, c.Y, g.Level.Start.X, g.Level.Start.Y) > 3 {
		t.Fatal("expected spawn at the start room when nobody is alive")
	}
}
