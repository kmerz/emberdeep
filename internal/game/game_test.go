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
