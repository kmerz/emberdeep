package game

import "testing"

func TestBlobSplitsOnce(t *testing.T) {
	g := arena(1)
	hero(g, 1, Warrior, 30, 25)
	b := g.addMon('o', 40, 25, 1)
	g.killMonster(b, 1)
	var kids []*Monster
	for _, m := range g.Mons {
		if m.Kind == 'o' {
			kids = append(kids, m)
		}
	}
	if len(kids) != SplitInto {
		t.Fatalf("a blob splits into %d, got %d", SplitInto, len(kids))
	}
	for _, k := range kids {
		if k.Gen != 1 || k.HPMax >= b.HPMax || cheb(k.X, k.Y, 40, 25) != 1 {
			t.Fatalf("bloblings are smaller and next to the blob: %+v", k)
		}
	}
	g.killMonster(kids[0], 1)
	n := 0
	for _, m := range g.Mons {
		if m.Kind == 'o' {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("bloblings don't split again, %d blobs left", n)
	}
}

func TestCallerSummonsUpToMaxAndKeepsAway(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Warrior, 30, 25)
	c := g.addMon('n', 40, 25, 1)
	for i := 0; i < CallerEvery*(CallerMax+2); i++ {
		g.Tick()
		h.HP = h.HPMax // keep the hero alive; we only watch the caller
	}
	rats := 0
	for _, m := range g.Mons {
		if m.Parent == c.ID && m.Summoned && m.Kind == 'r' {
			rats++
		}
	}
	if rats == 0 || rats > CallerMax {
		t.Fatalf("expected 1..%d bone rats, got %d", CallerMax, rats)
	}
	if c.HP <= 0 || dist(c.X, c.Y, h.X, h.Y) < CallerKeepOff {
		t.Fatalf("the caller should hang back, at %d,%d (hero %d,%d)", c.X, c.Y, h.X, h.Y)
	}
}

func TestCallerBacksAwayFromAdjacentHero(t *testing.T) {
	g := arena(1)
	hero(g, 1, Warrior, 39, 25)
	c := g.addMon('n', 40, 25, 1)
	c.NextSpawn = 1 << 30 // no summoning in this test
	for i := 0; i < 20; i++ {
		g.Tick()
	}
	if cheb(c.X, c.Y, 39, 25) <= 1 {
		t.Fatalf("the caller should have backed off, still at %d,%d", c.X, c.Y)
	}
}

func TestSummonsDropNothing(t *testing.T) {
	g := arena(1)
	hero(g, 1, Warrior, 30, 25)
	for i := 0; i < 200; i++ {
		r := g.addMon('r', 45, 30, 1)
		r.Summoned = true
		g.killMonster(r, 1)
	}
	if len(g.Items) != 0 {
		t.Fatalf("summoned minions drop no loot, got %d items", len(g.Items))
	}
}

func TestSpeedRoles(t *testing.T) {
	if MonDefs['r'].MoveTicks >= MoveTicks || MonDefs['h'].MoveTicks <= MonDefs['z'].MoveTicks {
		t.Fatal("rats are faster than heroes, hulks slower than zombies")
	}
	for id, th := range Themes {
		for i := 0; i < len(th.Kinds); i++ {
			if MonDefs[th.Kinds[i]].Role == "caller" && MonDefs[MonDefs[th.Kinds[i]].Summon].Name == "" {
				t.Errorf("theme %s: caller summons an unknown kind", id)
			}
		}
	}
}
