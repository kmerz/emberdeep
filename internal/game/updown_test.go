package game

import "testing"

// climb puts the party at the stairs up and waits out the countdown.
func climb(g *Game) {
	up := g.Level.Up
	for _, h := range g.Heroes {
		p, _ := g.Level.nearestFree(up, func(x, y int) bool { return g.occ[y*LW+x] == 0 && cheb(x, y, up.X, up.Y) <= StairsReach })
		g.place(h, p.X, p.Y)
	}
	for i := 0; i < StairsWindow+2; i++ {
		g.Tick()
	}
}

func TestGoUpAndFindTheFloorAsYouLeftIt(t *testing.T) {
	g := New(77, Settings{}, 1)
	h := g.AddHero(1, "kmerz", Warrior)
	if g.Level.HasUp {
		t.Fatal("floor 1 has no stairs up")
	}
	// kill one monster and open one chest on floor 1, explore a little
	victim := g.Mons[0]
	spawn := victim.Spawn
	g.killMonster(victim, h.ID)
	var opened int
	if len(g.Chests) > 0 {
		opened = g.Chests[0].Spot
		g.openChest(h, g.Chests[0])
	}
	g.Tick()
	exploredBefore := 0
	for _, e := range g.Explored {
		if e {
			exploredBefore++
		}
	}

	takeStairs(g)
	if g.Depth != 2 || !g.Level.HasUp || g.Level.At(g.Level.Up.X, g.Level.Up.Y) != TUp {
		t.Fatalf("floor 2 should have stairs up: depth %d up %v", g.Depth, g.Level.HasUp)
	}
	if cheb(g.Level.Up.X, g.Level.Up.Y, g.Level.Start.X, g.Level.Start.Y) < UpStairsGap {
		t.Fatal("the stairs up keep their distance from the arrival point")
	}
	for i := 0; i < StairsWindow+2; i++ {
		g.Tick()
	}
	if g.Depth != 2 {
		t.Fatal("arriving must not start the way back up")
	}

	clearMons(g)
	climb(g)
	if g.Depth != 1 {
		t.Fatalf("the stairs up lead to floor 1, got %d", g.Depth)
	}
	if cheb(h.X, h.Y, g.Level.Stairs.X, g.Level.Stairs.Y) > 2 {
		t.Fatal("coming up, the party arrives at the stairs down")
	}
	for _, m := range g.Mons {
		if m.Spawn == spawn {
			t.Fatal("a monster killed before stays dead")
		}
	}
	for _, c := range g.Chests {
		if opened > 0 && c.Spot == opened {
			t.Fatal("an opened chest stays open")
		}
	}
	explored := 0
	for _, e := range g.Explored {
		if e {
			explored++
		}
	}
	if explored < exploredBefore {
		t.Fatalf("the map stays explored: %d of %d cells", explored, exploredBefore)
	}
	for i := 0; i < StairsWindow+2; i++ {
		g.Tick()
	}
	if g.Depth != 1 {
		t.Fatal("standing on the stairs down after coming up must not send the party straight back")
	}
	g.place(h, g.Level.Start.X, g.Level.Start.Y) // walk away, then come back
	g.Tick()
	takeStairs(g)
	if g.Depth != 2 {
		t.Fatalf("after stepping away the stairs down work again, depth %d", g.Depth)
	}
}

func TestUpThroughTheVillage(t *testing.T) {
	g := NewAt(31, Settings{}, 1, VillageAfter+1)
	g.AddHero(1, "kmerz", Monk)
	g.VillageSeen = true
	clearMons(g)
	climb(g)
	if !g.InVillage {
		t.Fatalf("up from floor 6 is the gnome village: depth %d hasUp %v up %v phase %v at %d/%d", g.Depth, g.Level.HasUp, g.Level.Up, g.Phase, g.StairsAt, g.StairsNeed)
	}
	if !g.Level.HasUp {
		t.Fatal("the village has stairs up too")
	}
	climb(g)
	if g.InVillage || g.Depth != VillageAfter {
		t.Fatalf("up from the village is floor 5, got village %v depth %d", g.InVillage, g.Depth)
	}
	clearMons(g)
	for _, h := range g.Heroes {
		g.place(h, g.Level.Start.X, g.Level.Start.Y)
	}
	g.Tick()
	takeStairs(g)
	if !g.InVillage {
		t.Fatal("down from floor 5 is the village again")
	}
}

func TestSaveKeepsTheFloorsMemory(t *testing.T) {
	g := New(78, Settings{}, 1)
	h := g.AddHero(1, "kmerz", Rogue)
	victim := g.Mons[0]
	g.killMonster(victim, h.ID)
	s := g.Save(nil)
	g2 := Load(s, 1)
	g2.AddSavedHero(1, "kmerz", s.Heroes["kmerz"])
	for _, m := range g2.Mons {
		if m.Spawn == victim.Spawn {
			t.Fatal("continuing a save must not bring back monsters that died")
		}
	}
}

func TestEveryFloorBelowTheFirstHasStairsUp(t *testing.T) {
	for _, seed := range []uint32{1, 2, 3, 42, 777} {
		for d := 2; d <= MaxDepth; d++ {
			g := NewAt(seed, Settings{}, 2, d)
			l := g.Level
			if !l.HasUp {
				t.Fatalf("seed %d depth %d has no stairs up", seed, d)
			}
			d := l.bfsDist(l.Start)
			if d[l.Up.Y*l.W+l.Up.X] < 0 || d[l.Stairs.Y*l.W+l.Stairs.X] < 0 {
				t.Fatalf("seed %d depth %d: the stairs up can't be reached", seed, d)
			}
		}
	}
}
