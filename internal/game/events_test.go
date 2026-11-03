package game

import "testing"

// eventGame starts a run on the first floor that has event ev, with one hero on it.
func eventGame(t *testing.T, ev string) (*Game, *Hero) {
	t.Helper()
	for s := uint32(1); s < 500; s++ {
		for d := 2; d < MaxDepth; d++ {
			if FloorEvent(s, d) == ev {
				g := NewAt(s, Settings{}, 1, d)
				return g, g.AddHero(1, "kmerz", Warrior)
			}
		}
	}
	t.Fatalf("no floor with %q", ev)
	return nil, nil
}

func goblin(g *Game) *Monster {
	for _, m := range g.Mons {
		if m.Kind == GoblinKind {
			return m
		}
	}
	return nil
}

func TestFloorEventsAreFixedAndSkipSpecialFloors(t *testing.T) {
	seen := map[string]bool{}
	for s := uint32(1); s < 300; s++ {
		for d := 1; d <= MaxDepth; d++ {
			ev := FloorEvent(s, d)
			if ev != FloorEvent(s, d) {
				t.Fatal("the same seed and floor always get the same event")
			}
			if ev != "" && (d == 1 || d == MaxDepth || Floor(d).Boss != 0) {
				t.Fatalf("no event on floor %d (seed %d): %s", d, s, ev)
			}
			if ev != "" && FloorEvents[ev] == "" {
				t.Fatalf("event %q has no text", ev)
			}
			seen[ev] = true
		}
	}
	if !seen["goblin"] || !seen["stir"] || !seen["cursed"] || !seen[""] {
		t.Fatalf("every event comes up: %v", seen)
	}
}

func TestTreasureGoblinRunsAndEscapes(t *testing.T) {
	g, h := eventGame(t, "goblin")
	m := goblin(g)
	if m == nil || m.Awake || dist(m.X, m.Y, g.Level.Start.X, g.Level.Start.Y) < GoblinMinDist {
		t.Fatalf("a sleeping goblin far from the start: %+v", m)
	}
	clearOthers(g, m)
	p, _ := g.freeNear(m.X, m.Y)
	g.place(h, p.X, p.Y)
	d0 := dist(m.X, m.Y, h.X, h.Y)
	for i := 0; i < 2*TickHz; i++ {
		g.Tick()
	}
	if m.EscapeAt == 0 || dist(m.X, m.Y, h.X, h.Y) <= d0 {
		t.Fatalf("seen, it runs: escapeAt %d, %.1f -> %.1f", m.EscapeAt, d0, dist(m.X, m.Y, h.X, h.Y))
	}
	if h.HP != h.HPMax {
		t.Fatal("a goblin never fights")
	}
	for i := 0; i < GoblinEscape; i++ {
		g.Tick()
	}
	if goblin(g) != nil || !g.memo().GoblinGone {
		t.Fatal("it escapes when its time is up")
	}
	g.setupEvent(g.Depth, g.memo())
	if goblin(g) != nil {
		t.Fatal("a goblin that got away does not come back")
	}
}

func TestTreasureGoblinLoot(t *testing.T) {
	g, h := eventGame(t, "goblin")
	m := goblin(g)
	clearOthers(g, m)
	items := len(g.Items)
	g.damageMonster(m, m.HP, h.ID, true)
	gold, gear := 0, 0
	for _, it := range g.Items[items:] {
		if it.Kind == '$' {
			gold += it.Amount
		} else if it.G != nil && it.G.Rarity >= Rare {
			gear++
		}
	}
	if gold < GoblinGold*g.Depth || gear < 1 || !g.memo().GoblinGone || g.stats(h).Kills != 1 {
		t.Fatalf("a killed goblin spills gold %d and a rare (%d)", gold, gear)
	}
}

func TestTheDeadStir(t *testing.T) {
	g, h := eventGame(t, "stir")
	clearOthers(g, nil)
	g.Now = g.nextStir - 1
	g.Tick()
	n := 0
	for _, m := range g.Mons {
		if m.Risen && m.Summoned && m.Awake {
			n++
			if cheb(m.X, m.Y, h.X, h.Y) < 3 {
				t.Fatal("the dead rise a few steps away, not on top of you")
			}
		}
	}
	if n == 0 || g.nextStir != g.Now+StirEvery {
		t.Fatalf("the dead rise near the party: %d, next %d", n, g.nextStir)
	}
	for i := 0; i < 10; i++ {
		g.Now = g.nextStir
		g.tickStir()
	}
	alive := 0
	for _, m := range g.Mons {
		if m.Risen {
			alive++
		}
	}
	if alive > StirMax {
		t.Fatalf("at most %d risen at once, got %d", StirMax, alive)
	}
}

func TestCursedFloor(t *testing.T) {
	g, _ := eventGame(t, "cursed")
	m := g.addMon('z', 1, 1, 1)
	cursed := g.monsterDmg(m, 10)
	g.Event = ""
	if plain := g.monsterDmg(m, 10); cursed <= plain {
		t.Fatalf("monsters hit harder: %d vs %d", cursed, plain)
	}
	g.Event = "cursed"
	if g.eventGoldMult() != CursedGoldMult {
		t.Fatal("gold counts double")
	}
}

// clearOthers empties the floor of every monster but keep.
func clearOthers(g *Game, keep *Monster) {
	for _, m := range append([]*Monster(nil), g.Mons...) {
		if m != keep {
			g.removeMon(m)
		}
	}
}
