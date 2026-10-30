package game

import "testing"

func downHeroNow(g *Game, h *Hero) {
	g.damageHero(h, h.HP+1000, monIDBase+1)
}

func TestHeroDownedNotDead(t *testing.T) {
	g := arena(2)
	a := hero(g, 1, Warrior, 40, 25)
	hero(g, 2, Rogue, 41, 25)
	downHeroNow(g, a)
	if !a.Downed || a.Dead || a.HP != 0 {
		t.Fatalf("0 HP means downed, not dead: %+v", a)
	}
	if a.DownedUntil-g.Now != DownedTicks {
		t.Fatalf("downed timer should be 20 s, got %d ticks", a.DownedUntil-g.Now)
	}
	if err := g.Apply(1, Intent{Kind: IntMove, Dir: 2}); err != ErrDowned {
		t.Fatalf("a downed hero cannot act, got %v", err)
	}
	// monsters ignore downed heroes
	m := g.spawnMon('z', 39, 25)
	for i := 0; i < 40; i++ {
		g.Tick()
	}
	if a.HP != 0 || !a.Downed {
		t.Fatal("downed hero must not be attacked")
	}
	if tg := g.pickTarget(m); tg == nil || tg.ID != 2 {
		t.Fatalf("monster should pick the standing hero, got %v", tg)
	}
}

func TestReviveTimingAndAdjacencyReset(t *testing.T) {
	g := arena(2)
	a := hero(g, 1, Warrior, 40, 25)
	b := hero(g, 2, Rogue, 41, 25)
	downHeroNow(g, a)
	if err := g.Apply(2, Intent{Kind: IntRevive, Target: 1, Hold: true}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < ReviveTicks-1; i++ {
		g.Tick()
	}
	if !a.Downed {
		t.Fatal("revive needs a full 3 s")
	}
	g.Tick()
	if a.Downed || a.HP != int(float64(a.HPMax)*0.3+0.5) {
		t.Fatalf("expected revive at 30%% HP, got downed=%v hp=%d/%d", a.Downed, a.HP, a.HPMax)
	}
	// moving away resets progress
	downHeroNow(g, a)
	_ = g.Apply(2, Intent{Kind: IntRevive, Target: 1, Hold: true})
	for i := 0; i < 40; i++ {
		g.Tick()
	}
	if b.ReviveProg == 0 {
		t.Fatal("progress should accumulate")
	}
	b.NextMove = 0
	if err := g.Apply(2, Intent{Kind: IntMove, Dir: 2}); err != nil { // step east, still adjacent? no: 42 vs 40
		t.Fatal(err)
	}
	if b.ReviveProg != 0 || b.ReviveTarget != 0 {
		t.Fatal("moving must reset revive progress")
	}
	for i := 0; i < 5; i++ {
		g.Tick()
	}
	if !a.Downed {
		t.Fatal("no progress while away")
	}
	// releasing resets too
	g.place(b, 41, 25)
	_ = g.Apply(2, Intent{Kind: IntRevive, Target: 1, Hold: true})
	for i := 0; i < 30; i++ {
		g.Tick()
	}
	_ = g.Apply(2, Intent{Kind: IntRevive, Target: 1, Hold: false})
	if b.ReviveProg != 0 {
		t.Fatal("releasing E resets progress")
	}
	// not adjacent -> rejected
	g.place(b, 45, 25)
	if err := g.Apply(2, Intent{Kind: IntRevive, Target: 1, Hold: true}); err != ErrRange {
		t.Fatalf("expected ErrRange, got %v", err)
	}
	// reviving a standing ally is rejected
	if err := g.Apply(1, Intent{Kind: IntRevive, Target: 2, Hold: true}); err == nil {
		t.Log("downed hero cannot revive")
	}
}

func TestDownedExpiryMeansDeadAndSpectates(t *testing.T) {
	g := arena(2)
	a := hero(g, 1, Warrior, 40, 25)
	hero(g, 2, Rogue, 50, 25)
	downHeroNow(g, a)
	for i := 0; i < DownedTicks+1; i++ {
		g.Tick()
	}
	if !a.Dead || a.Downed {
		t.Fatalf("expired downed hero is dead: %+v", a)
	}
	if g.occ[25*LW+40] != 0 {
		t.Fatal("a dead hero stops blocking its cell")
	}
	g.Tick()
	if g.Phase != PhasePlaying {
		t.Fatal("a standing hero keeps the run going")
	}
	// the dead hero's vision stops contributing (shared vision continues from the living)
	if g.IsVisible(40, 25) && dist(40, 25, 50, 25) > VisionRadius {
		t.Fatal("unexpected vision")
	}
}

func TestWipeWhenEveryoneDownedOrDead(t *testing.T) {
	g := arena(3)
	a := hero(g, 1, Warrior, 40, 25)
	b := hero(g, 2, Rogue, 42, 25)
	c := hero(g, 3, Monk, 44, 25)
	downHeroNow(g, a)
	g.Tick()
	downHeroNow(g, b)
	g.Tick()
	if g.Phase != PhasePlaying {
		t.Fatal("one hero still stands")
	}
	downHeroNow(g, c)
	g.Tick()
	if g.Phase != PhaseWiped {
		t.Fatal("expected a wipe when everyone is downed at once")
	}
	// inputs are refused after the wipe
	if err := g.Apply(1, Intent{Kind: IntPotion}); err != ErrPhase {
		t.Fatalf("expected ErrPhase, got %v", err)
	}
	// a mix of downed and dead also wipes
	g2 := arena(2)
	x := hero(g2, 1, Warrior, 40, 25)
	y := hero(g2, 2, Rogue, 42, 25)
	downHeroNow(g2, x)
	for i := 0; i < DownedTicks+2; i++ {
		g2.Tick()
	}
	if !x.Dead || g2.Phase != PhasePlaying {
		t.Fatal("setup")
	}
	downHeroNow(g2, y)
	g2.Tick()
	if g2.Phase != PhaseWiped {
		t.Fatal("dead + downed must wipe")
	}
}

func TestDisconnectedHeroIsInvulnerableAndIdle(t *testing.T) {
	g := arena(2)
	a := hero(g, 1, Warrior, 40, 25)
	hero(g, 2, Rogue, 60, 38)
	g.SetConnected(1, false)
	m := g.spawnMon('z', 41, 25)
	m.Awake = true
	for i := 0; i < 60; i++ {
		g.Tick()
	}
	if a.HP != a.HPMax {
		t.Fatal("a disconnected hero is invulnerable")
	}
	if err := g.Apply(1, Intent{Kind: IntMove, Dir: 2}); err == nil {
		t.Fatal("a disconnected hero is idle")
	}
	g.damageHero(a, 50, m.ID)
	if a.HP != a.HPMax {
		t.Fatal("damage must be ignored")
	}
}

func TestStairsWindow(t *testing.T) {
	g := arena(3)
	st := g.Level.Stairs
	a := hero(g, 1, Warrior, st.X-1, st.Y)
	b := hero(g, 2, Rogue, st.X, st.Y-1)
	c := hero(g, 3, Monk, 40, 25)
	g.Tick()
	if g.StairsAt != 2 || g.StairsNeed != 3 {
		t.Fatalf("expected 2/3 at the stairs, got %d/%d", g.StairsAt, g.StairsNeed)
	}
	for i := 0; i < 100; i++ {
		g.Tick()
	}
	if g.Phase != PhasePlaying {
		t.Fatal("must not descend without the whole party")
	}
	g.place(c, st.X-1, st.Y-1)
	for i := 0; i < StairsWindow-2; i++ {
		g.Tick()
	}
	if g.Phase != PhasePlaying || g.StairsLeft == 0 {
		t.Fatal("window running but not finished")
	}
	// someone leaves: the window cancels
	g.place(c, 40, 25)
	g.Tick()
	if g.StairsLeft != 0 {
		t.Fatal("leaving cancels the window")
	}
	g.place(c, st.X-1, st.Y-1)
	for i := 0; i < StairsWindow+2; i++ {
		g.Tick()
	}
	if g.Phase != PhaseCleared {
		t.Fatalf("expected the level to be cleared after 3 s together (a=%v b=%v)", a.X, b.X)
	}
	// dead and disconnected heroes do not have to be there
	g2 := arena(3)
	s2 := g2.Level.Stairs
	hero(g2, 1, Warrior, s2.X, s2.Y)
	d := hero(g2, 2, Rogue, 40, 25)
	e := hero(g2, 3, Monk, 41, 25)
	d.Dead = true
	g2.SetConnected(3, false)
	_ = e
	for i := 0; i < StairsWindow+3; i++ {
		g2.Tick()
	}
	if g2.Phase != PhaseCleared {
		t.Fatal("only living connected heroes count")
	}
}

func TestThreatWindow(t *testing.T) {
	g := arena(2)
	a := hero(g, 1, Warrior, 40, 25)
	b := hero(g, 2, Rogue, 44, 25)
	m := g.spawnMon('z', 41, 25) // adjacent to a, farther from b
	if tg := g.pickTarget(m); tg != a {
		t.Fatal("nearest hero when there is no threat")
	}
	g.addThreat(m, b.ID, 30)
	g.addThreat(m, a.ID, 10)
	if tg := g.pickTarget(m); tg != b {
		t.Fatal("highest damage in the last 5 s wins")
	}
	g.Now += ThreatWindow + 1
	if tg := g.pickTarget(m); tg != a {
		t.Fatal("threat expires after 5 s")
	}
	g.addThreat(m, b.ID, 30)
	g.damageHero(b, 1000, m.ID) // b goes down: never targeted
	if tg := g.pickTarget(m); tg != a {
		t.Fatal("downed heroes are never targeted")
	}
}

func TestSharedGoldPersonalPotions(t *testing.T) {
	g := arena(3)
	a := hero(g, 1, Warrior, 40, 25)
	b := hero(g, 2, Rogue, 50, 25)
	c := hero(g, 3, Monk, 55, 25)
	c.Connected = false
	g.Items = append(g.Items, &Item{ID: g.newID(), Kind: '$', X: 41, Y: 25, Amount: 11})
	if err := g.Apply(1, Intent{Kind: IntMove, Dir: 2}); err != nil {
		t.Fatal(err)
	}
	if a.Gold+b.Gold != 11 || b.Gold != 5 || a.Gold != 6 || c.Gold != 0 {
		t.Fatalf("gold split among connected heroes with the remainder to the picker: a=%d b=%d c=%d", a.Gold, b.Gold, c.Gold)
	}
	if g.PartyGold() != 11 {
		t.Fatal("party gold total")
	}
	// personal potion: only the owner can pick it up
	g.Items = append(g.Items, &Item{ID: g.newID(), Kind: '!', X: 42, Y: 25, Amount: 1, Owner: 2})
	a.NextMove = 0
	if err := g.Apply(1, Intent{Kind: IntMove, Dir: 2}); err != nil {
		t.Fatal(err)
	}
	if a.Potions != StartPotions || len(g.Items) != 1 {
		t.Fatalf("a must not pick up b's potion: potions=%d items=%d", a.Potions, len(g.Items))
	}
	g.place(b, 41, 25)
	b.NextMove = 0
	if err := g.Apply(2, Intent{Kind: IntMove, Dir: 2}); err == nil && b.X == 42 && b.Potions != StartPotions+1 {
		t.Fatal("owner should pick up the potion")
	}
}

func TestKillDropsPersonalPotionsPerPlayer(t *testing.T) {
	g := arena(2)
	hero(g, 1, Warrior, 40, 25)
	hero(g, 2, Rogue, 40, 27)
	owners := map[int]int{}
	for i := 0; i < 400; i++ {
		m := g.spawnMon('f', 45, 30)
		g.killMonster(m, 1)
	}
	for _, it := range g.Items {
		if it.Kind == '!' {
			if it.Owner != 1 && it.Owner != 2 {
				t.Fatalf("potion without an owner")
			}
			owners[it.Owner]++
		}
	}
	if owners[1] == 0 || owners[2] == 0 {
		t.Fatalf("both players should get personal drops: %v", owners)
	}
}
