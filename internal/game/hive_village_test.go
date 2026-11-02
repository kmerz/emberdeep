package game

import (
	"bytes"
	"testing"
)

func worker(g *Game, x, y int) *Monster {
	m := g.addMon('a', x, y, 1)
	m.Neutral, m.Awake = true, false
	return m
}

func TestHiveFloor(t *testing.T) {
	if ThemeAt(4).ID != "hive" || Floor(4).Boss != 'Q' || ThemeAt(5).ID != "crypt" {
		t.Fatal("floor 4 is the hive with the queen; floor 5 is the crypt again")
	}
	g := NewAt(42, Settings{}, 2, 4)
	workers, queens := 0, 0
	for _, m := range g.Mons {
		if m.Kind == 'a' && m.Neutral {
			workers++
		}
		if m.Kind == 'Q' && m.Elite {
			queens++
		}
	}
	if workers == 0 || queens != 1 {
		t.Fatalf("the hive has neutral workers and one queen: %d workers, %d queens", workers, queens)
	}
}

func TestWorkersAreNeutral(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Warrior, 40, 25)
	worker(g, 41, 25)
	worker(g, 40, 26)
	for i := 0; i < 80; i++ {
		g.Tick()
	}
	if h.HP != h.HPMax {
		t.Fatal("neutral workers don't attack")
	}
}

func TestHittingAnAntAlarmsTheNeighbours(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Warrior, 30, 25)
	hit := worker(g, 31, 25)
	near := worker(g, 40, 25) // 9 cells from the one hit
	far := worker(g, 50, 25)  // 19 cells
	g.damageMonster(hit, 1, h.ID, false)
	if hit.Neutral || near.Neutral || !far.Neutral {
		t.Fatalf("ants within %v cells join in, farther ones don't: hit %v near %v far %v", AlarmRange, hit.Neutral, near.Neutral, far.Neutral)
	}
	if g.pickTarget(near) != h {
		t.Fatal("alarmed ants go for the attacker")
	}
}

func TestQueenLaysSoldiersAndStays(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Warrior, 30, 25)
	q := g.addMon('Q', 45, 25, 1)
	for i := 0; i < QueenEvery*(QueenMax+3); i++ {
		g.Tick()
		h.HP = h.HPMax
	}
	soldiers := 0
	for _, m := range g.Mons {
		if m.Parent == q.ID && m.Kind == 'A' {
			soldiers++
		}
	}
	if soldiers == 0 || soldiers > QueenMax || q.X != 45 || q.Y != 25 {
		t.Fatalf("1..%d soldiers and a queen that never moves: %d soldiers, queen at %d,%d", QueenMax, soldiers, q.X, q.Y)
	}
}

func TestVillageAfterFloorFive(t *testing.T) {
	g := NewAt(7, Settings{}, 1, VillageAfter)
	h := g.AddHero(1, "kmerz", Rogue)
	takeStairs(g)
	if !g.InVillage || g.Depth != VillageAfter || len(g.Mons) != 0 || len(g.Chests) != 0 {
		t.Fatalf("floor 5's stairs lead to a safe village: village %v depth %d mons %d", g.InVillage, g.Depth, len(g.Mons))
	}
	traders := 0
	for _, n := range g.Npcs {
		if n.Trader {
			traders++
		}
	}
	if len(g.Npcs) < 4 || traders != 1 || !g.Level.Reachable() {
		t.Fatalf("gnomes and one trader, stairs reachable: %d gnomes, %d traders", len(g.Npcs), traders)
	}
	takeStairs(g)
	if g.InVillage || g.Depth != VillageAfter+1 || h.Dead {
		t.Fatalf("the village stairs lead to floor %d, got %d", VillageAfter+1, g.Depth)
	}
}

func villageWithTrader(t *testing.T) (*Game, *Hero) {
	t.Helper()
	g := NewAt(7, Settings{}, 1, VillageAfter)
	h := g.AddHero(1, "kmerz", Monk)
	takeStairs(g)
	var tr *Npc
	for _, n := range g.Npcs {
		if n.Trader {
			tr = n
		}
	}
	p, _ := g.Level.nearestFree(Pt{tr.X, tr.Y + 1}, func(x, y int) bool { return g.occ[y*LW+x] == 0 })
	g.place(h, p.X, p.Y)
	if cheb(h.X, h.Y, tr.X, tr.Y) > 1 {
		t.Fatal("couldn't stand next to the trader")
	}
	return g, h
}

func TestBuyingAndSelling(t *testing.T) {
	g, h := villageWithTrader(t)
	h.Gold = 20
	if err := g.Apply(1, Intent{Kind: IntBuy, Skill: "potion"}); err != ErrNoGold {
		t.Fatalf("a 30-gold potion with 20 gold: %v", err)
	}
	h.Gold = 200
	if err := g.Apply(1, Intent{Kind: IntBuy, Skill: "torch"}); err != nil {
		t.Fatal(err)
	}
	if h.Gold != 185 || h.Count("torch") != StartTorches+1 {
		t.Fatalf("a torch costs 15: gold %d torches %d", h.Gold, h.Count("torch"))
	}
	if err := g.Apply(1, Intent{Kind: IntBuy, Skill: "gem"}); err != ErrNoItem {
		t.Fatalf("garnets aren't for sale: %v", err)
	}
	h.AddItem("gem", 2)
	if err := g.Apply(1, Intent{Kind: IntSell, Slot: h.firstSlot("gem")}); err != nil {
		t.Fatal(err)
	}
	if h.Gold != 225 || h.Count("gem") != 1 {
		t.Fatalf("a garnet sells for 40: gold %d", h.Gold)
	}
	_ = g.Apply(1, Intent{Kind: IntSell, Slot: h.firstSlot("torch")})
	if h.Gold != 232 {
		t.Fatalf("a torch sells for half: gold %d", h.Gold)
	}
	for i := 0; i < PackSlots; i++ {
		h.Pack[i] = Stack{Item: "gem", N: Items["gem"].MaxStack}
	}
	if err := g.Apply(1, Intent{Kind: IntBuy, Skill: "potion"}); err != ErrPackFull {
		t.Fatalf("no room: %v", err)
	}
	g.place(h, g.Level.Start.X, g.Level.Start.Y)
	if err := g.Apply(1, Intent{Kind: IntSell, Slot: 0}); err != ErrNotHere {
		t.Fatalf("trading only next to Grimble: %v", err)
	}
}

func TestScrollOfReturn(t *testing.T) {
	g := NewAt(7, Settings{}, 1, VillageAfter-1)
	h := g.AddHero(1, "kmerz", Warrior)
	h.AddItem("returnscroll", 2)
	if err := g.Apply(1, Intent{Kind: IntUse, Slot: h.firstSlot("returnscroll")}); err != ErrNotHere {
		t.Fatalf("no portal above the village: %v", err)
	}
	takeStairs(g) // 5
	takeStairs(g) // village
	if err := g.Apply(1, Intent{Kind: IntUse, Slot: h.firstSlot("returnscroll")}); err != ErrNotHere {
		t.Fatalf("no portal in the village: %v", err)
	}
	takeStairs(g) // 6
	takeStairs(g) // 7
	layout := append([]byte(nil), g.Level.Tiles...)
	clearMons(g)
	if err := g.Apply(1, Intent{Kind: IntUse, Slot: h.firstSlot("returnscroll")}); err != nil {
		t.Fatal(err)
	}
	if g.Portal == nil || h.Count("returnscroll") != 1 {
		t.Fatal("the scroll opens a portal and is used up")
	}
	for i := 0; i < PortalWindow+2; i++ {
		g.Tick()
	}
	if !g.InVillage || g.ReturnDepth != 7 {
		t.Fatalf("gathering at the portal returns to the village: village %v return %d", g.InVillage, g.ReturnDepth)
	}
	takeStairs(g)
	if g.InVillage || g.Depth != 7 || !bytes.Equal(layout, g.Level.Tiles) || len(g.Mons) == 0 {
		t.Fatalf("the way back is a fresh floor 7: depth %d mons %d", g.Depth, len(g.Mons))
	}
}

func TestPortalCloses(t *testing.T) {
	g := NewAt(7, Settings{}, 1, 8)
	h := g.AddHero(1, "kmerz", Warrior)
	g.VillageSeen = true
	h.AddItem("returnscroll", 1)
	clearMons(g)
	_ = g.Apply(1, Intent{Kind: IntUse, Slot: h.firstSlot("returnscroll")})
	g.place(h, h.X+3, h.Y) // nobody stays in it
	for i := 0; i < PortalTicks+2; i++ {
		g.Tick()
	}
	if g.Portal != nil || g.InVillage {
		t.Fatal("an unused portal closes after 30 s")
	}
}

func TestVillageStairsNeverEndTheRun(t *testing.T) {
	g := NewAt(7, Settings{}, 1, MaxDepth)
	g.AddHero(1, "kmerz", Warrior)
	g.VillageSeen, g.ReturnDepth = true, MaxDepth
	g.loadVillage()
	takeStairs(g)
	if g.Phase != PhasePlaying || g.Depth != MaxDepth || g.InVillage {
		t.Fatal("from the village the stairs lead back to floor 10, not to the end")
	}
	takeStairs(g)
	if g.Phase != PhaseCleared {
		t.Fatal("floor 10's own stairs still end the run")
	}
}
