package game

import "testing"

func spawnChest(g *Game, x, y int) *Chest {
	c := &Chest{ID: g.newID(), X: x, Y: y}
	g.Chests = append(g.Chests, c)
	g.chestByID[c.ID] = c
	g.occ[y*LW+x] = int32(c.ID)
	return c
}

func spawnMimic(g *Game, x, y int) *Monster {
	m := g.spawnMon('M', x, y)
	m.Disguised = true
	return m
}

func TestBumpOpensChestWithLootForEveryone(t *testing.T) {
	g := arena(2)
	g.chestByID = map[int]*Chest{}
	g.Chests = nil
	a := hero(g, 1, Warrior, 40, 25)
	hero(g, 2, Rogue, 30, 25)
	spawnChest(g, 41, 25)
	step(t, g, a, 2)
	if a.X != 40 || len(g.Chests) != 0 || !g.cellFree(41, 25) {
		t.Fatalf("bumping opens the chest without moving: hero at %d, chests %d", a.X, len(g.Chests))
	}
	gold, mine := 0, map[int]int{}
	for _, it := range g.Items {
		if it.Def == "" {
			gold += it.Amount
		} else {
			mine[it.Owner]++
		}
	}
	if gold < ChestGold || mine[1] != ChestRolls || mine[2] != ChestRolls {
		t.Fatalf("expected gold and %d items per hero, got gold %d items %v", ChestRolls, gold, mine)
	}
	step(t, g, a, 2) // now walk onto the loot
	if a.X != 41 || a.Gold == 0 {
		t.Fatalf("walking onto the open spot picks up the loot: x %d gold %d", a.X, a.Gold)
	}
}

func TestMimicBitesWhenBumped(t *testing.T) {
	g := arena(1)
	a := hero(g, 1, Warrior, 40, 25)
	m := spawnMimic(g, 41, 25)
	for i := 0; i < 40; i++ {
		g.Tick()
	}
	if a.HP != a.HPMax || m.X != 41 {
		t.Fatal("a disguised mimic waits and does nothing")
	}
	step(t, g, a, 2)
	if m.Disguised || a.HP >= a.HPMax || a.X != 40 {
		t.Fatalf("bumping reveals the mimic and it bites: disguised %v HP %d/%d", m.Disguised, a.HP, a.HPMax)
	}
}

func TestHittingAMimicRevealsItAndItDropsTreasure(t *testing.T) {
	g := arena(1)
	a := hero(g, 1, Sorcerer, 40, 25)
	m := spawnMimic(g, 44, 25)
	if err := cast(g, a, 0, 44, 25); err != nil { // spark
		t.Fatal(err)
	}
	if m.Disguised {
		t.Fatal("damage should reveal the mimic")
	}
	g.killMonster(m, 1)
	items := 0
	for _, it := range g.Items {
		if it.Def != "" && it.Owner == 1 {
			items++
		}
	}
	if items < ChestRolls {
		t.Fatalf("a dead mimic drops the chest's treasure, got %d items", items)
	}
}

func TestChestsOnFloors(t *testing.T) {
	total, mimics := 0, 0
	for _, seed := range []uint32{1, 7, 42, 99, 1234} {
		for d := 1; d <= MaxDepth; d++ {
			l := Generate(seed, d, 2)
			blocked := map[Pt]bool{}
			for _, c := range l.Chests {
				total++
				if c.Mimic {
					mimics++
					if d == 1 {
						t.Fatalf("no mimics on floor 1")
					}
				}
				if !l.Walkable(c.X, c.Y) || c == (ChestSpawn{X: l.Start.X, Y: l.Start.Y}) {
					t.Fatalf("seed %d depth %d: bad chest spot %v", seed, d, c)
				}
				blocked[Pt{c.X, c.Y}] = true
			}
			// the stairs must stay reachable with every chest treated as a wall
			if !reachableAround(l, blocked) {
				t.Fatalf("seed %d depth %d: chests block the way to the stairs", seed, d)
			}
		}
	}
	if total < 50 || mimics == 0 {
		t.Fatalf("expected chests and some mimics across 50 floors, got %d chests, %d mimics", total, mimics)
	}
}

func reachableAround(l *Level, blocked map[Pt]bool) bool {
	seen := map[Pt]bool{l.Start: true}
	q := []Pt{l.Start}
	for len(q) > 0 {
		p := q[0]
		q = q[1:]
		if p == l.Stairs {
			return true
		}
		for _, d := range dirs8 {
			n := Pt{p.X + d.X, p.Y + d.Y}
			if !seen[n] && l.Walkable(n.X, n.Y) && !blocked[n] {
				seen[n] = true
				q = append(q, n)
			}
		}
	}
	return false
}

func TestFloorLoadsChestsAndMimics(t *testing.T) {
	for d := 2; d <= MaxDepth; d++ {
		g := NewAt(42, Settings{}, 2, d)
		hidden := 0
		for _, c := range g.Chests {
			if c.Vault {
				hidden-- // the vault chest is the game's, not the level's
			}
		}
		for _, m := range g.Mons {
			if m.Disguised {
				hidden++
			}
		}
		if len(g.Chests)+hidden != len(g.Level.Chests) {
			t.Fatalf("depth %d: %d chests + %d mimics, level has %d", d, len(g.Chests), hidden, len(g.Level.Chests))
		}
	}
}
