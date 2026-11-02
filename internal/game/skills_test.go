package game

import "testing"

// arena returns a game on an empty walled arena with no monsters; the interior spans x 20..60, y 10..40.
func arena(n int) *Game {
	g := New(1, Settings{}, n)
	for i := range g.Level.Tiles {
		g.Level.Tiles[i] = ' '
	}
	for y := 9; y <= 41; y++ {
		for x := 19; x <= 61; x++ {
			t := byte(TFloor)
			if x == 19 || x == 61 || y == 9 || y == 41 {
				t = TWall
			}
			g.Level.Tiles[y*LW+x] = t
		}
	}
	g.Level.Start = Pt{30, 25}
	g.Level.Stairs = Pt{58, 38}
	g.Level.Tiles[38*LW+58] = TStairs
	g.Mons, g.Items, g.Patches = nil, nil, nil
	g.monByID = map[int]*Monster{}
	for i := range g.occ {
		g.occ[i] = 0
	}
	return g
}

func (g *Game) place(h *Hero, x, y int) {
	g.moveEntity(int32(h.ID), &h.X, &h.Y, x, y)
}

func (g *Game) spawnMon(kind byte, x, y int) *Monster {
	def := MonDefs[kind]
	m := &Monster{ID: g.newID(), Kind: kind, X: x, Y: y, HP: def.HP, HPMax: def.HP, Facing: 6}
	g.Mons = append(g.Mons, m)
	g.monByID[m.ID] = m
	g.occ[y*LW+x] = int32(m.ID)
	return m
}

func hero(g *Game, id int, cls ClassID, x, y int) *Hero {
	h := g.AddHero(id, "h"+string(rune('a'+id)), cls)
	g.place(h, x, y)
	h.Res = h.ResMax
	// the original three skills at rank 1 on keys 1-3 (tests of the tree itself start from scratch)
	h.Ranks = [TreeSize]int{1, 1, 1}
	h.Binds = [3]int{0, 1, 2}
	return h
}

func cast(g *Game, h *Hero, slot, x, y int) error {
	g.Now += 200 // clear cooldowns between casts in tests
	h.NextMove = 0
	return g.Apply(h.ID, Intent{Kind: IntCast, Slot: slot, X: x, Y: y})
}

func TestCleaveHitsArcOnly(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Warrior, 40, 25)
	front := g.spawnMon('z', 41, 25)
	diag1 := g.spawnMon('z', 41, 24)
	diag2 := g.spawnMon('z', 41, 26)
	side := g.spawnMon('z', 40, 24)
	back := g.spawnMon('z', 39, 25)
	if err := cast(g, h, 0, 45, 25); err != nil {
		t.Fatal(err)
	}
	for name, m := range map[string]*Monster{"front": front, "diag1": diag1, "diag2": diag2} {
		if m.HP >= m.HPMax {
			t.Errorf("%s should be hit", name)
		}
	}
	if side.HP != side.HPMax || back.HP != back.HPMax {
		t.Error("side/back must not be hit by the arc")
	}
	if h.Res == 0 {
		t.Error("warrior should gain rage from hits")
	}
}

func TestIronWallAbsorbsAndGeneratesThreat(t *testing.T) {
	g := arena(2)
	w := hero(g, 1, Warrior, 40, 25)
	r := hero(g, 2, Rogue, 40, 27)
	m := g.spawnMon('z', 41, 25)
	w.Res = 100
	if err := cast(g, w, 1, 40, 25); err != nil {
		t.Fatal(err)
	}
	if w.Res != 75 {
		t.Fatalf("iron wall should cost 25 rage, res=%d", w.Res)
	}
	// threat: the monster prefers the warrior even though the rogue is also close
	if tg := g.pickTarget(m); tg != w {
		t.Fatalf("iron wall must draw threat, target=%v", tg)
	}
	hp := w.HP
	g.damageHero(w, 20, m.ID)
	if w.HP != hp || w.WallAbsorb != 40 {
		t.Fatalf("damage should be absorbed: hp %d->%d absorb %d", hp, w.HP, w.WallAbsorb)
	}
	g.Now += Sec(6) // wall expires
	g.damageHero(w, 20, m.ID)
	if w.HP != hp-20 {
		t.Fatalf("absorb must expire after 5 s, hp=%d", w.HP)
	}
	_ = r
}

func TestLeapMovesAndImpacts(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Warrior, 30, 25)
	m := g.spawnMon('z', 38, 25)
	if err := cast(g, h, 2, 36, 25); err != nil {
		t.Fatal(err)
	}
	if h.X != 36 || m.HP >= m.HPMax {
		t.Fatalf("leap should land at 36 and hit the zombie two cells away (x=%d hp=%d)", h.X, m.HP)
	}
	if err := cast(g, h, 2, 36+30, 25); err != ErrRange && err != ErrBadTarget {
		t.Fatalf("expected out of range, got %v", err)
	}
	// blocked line of sight
	for y := 20; y <= 30; y++ {
		g.Level.Tiles[y*LW+40] = TWall
	}
	if err := cast(g, h, 2, 44, 25); err != ErrLOS {
		t.Fatalf("expected no line of sight, got %v", err)
	}
}

func TestStabCritsFromBehind(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Rogue, 41, 25)
	m := g.spawnMon('z', 40, 25) // faces west (6): attacker at east is behind it
	m.HP, m.HPMax = 1000, 1000
	if err := cast(g, h, 0, 40, 25); err != nil {
		t.Fatal(err)
	}
	back := 1000 - m.HP
	g.place(h, 39, 25) // now in front of the monster
	if err := cast(g, h, 0, 40, 25); err != nil {
		t.Fatal(err)
	}
	front := 1000 - m.HP - back
	if back != 2*front || front == 0 {
		t.Fatalf("backstab should deal double damage: back=%d front=%d", back, front)
	}
}

func TestCaltropsSlowAndBleed(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Rogue, 30, 25)
	m := g.spawnMon('z', 36, 25)
	m.Awake = false
	if err := cast(g, h, 1, 36, 25); err != nil {
		t.Fatal(err)
	}
	if len(g.Patches) != 1 {
		t.Fatal("expected a patch")
	}
	hp := m.HP
	for i := 0; i < Sec(4); i++ {
		g.Tick()
	}
	if m.SlowUntil <= g.Now-5 && m.BleedUntil == 0 {
		t.Fatal("monster should have been slowed and bleeding")
	}
	if m.HP >= hp {
		t.Fatalf("bleed should hurt: %d -> %d", hp, m.HP)
	}
	for i := 0; i < Sec(4); i++ {
		g.Tick()
	}
	if len(g.Patches) != 0 {
		t.Fatal("patch should expire after 6 s")
	}
}

func TestShadowBombIsDelayedArea(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Rogue, 30, 25)
	near := g.spawnMon('z', 40, 25)
	far := g.spawnMon('z', 40, 35)
	if err := cast(g, h, 2, 40, 25); err != nil {
		t.Fatal(err)
	}
	if near.HP != near.HPMax {
		t.Fatal("impact must be delayed")
	}
	var impact bool
	for i := 0; i < 12; i++ {
		for _, e := range g.Tick() {
			if e.K == "impact" && e.FX == "shadowbomb" {
				impact = true
			}
		}
	}
	if !impact || near.HP >= near.HPMax {
		t.Fatal("bomb should have exploded")
	}
	if far.HP != far.HPMax {
		t.Fatal("bomb is an area effect, the far zombie must be untouched")
	}
}

func TestPalmKnockbackStopsAtWall(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Monk, 58, 25)
	m := g.spawnMon('z', 59, 25) // one cell from the east wall at x=61 (interior ends at 60)
	m.HP, m.HPMax = 500, 500
	if err := cast(g, h, 0, 59, 25); err != nil {
		t.Fatal(err)
	}
	if m.X != 60 {
		t.Fatalf("knockback must stop at the wall, x=%d", m.X)
	}
	g.place(h, 40, 25)
	m2 := g.spawnMon('z', 41, 25)
	m2.HP, m2.HPMax = 500, 500
	if err := cast(g, h, 0, 41, 25); err != nil {
		t.Fatal(err)
	}
	if m2.X != 43 {
		t.Fatalf("knockback is 2 cells, x=%d", m2.X)
	}
}

func TestDashPassesThroughEnemiesNotWalls(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Monk, 40, 25)
	m := g.spawnMon('z', 42, 25)
	if err := cast(g, h, 1, 50, 25); err != nil {
		t.Fatal(err)
	}
	if h.X != 44 || m.HP >= m.HPMax {
		t.Fatalf("dash should go 4 cells through the zombie (x=%d hp=%d)", h.X, m.HP)
	}
	g.place(h, 59, 25)
	if err := cast(g, h, 1, 70, 25); err != nil {
		t.Fatal(err)
	}
	if h.X != 60 {
		t.Fatalf("dash must stop at the wall, x=%d", h.X)
	}
}

func TestBellReducesPartyDamage(t *testing.T) {
	g := arena(3)
	mk := hero(g, 1, Monk, 40, 25)
	ally := hero(g, 2, Warrior, 43, 25)
	far := hero(g, 3, Rogue, 40, 38)
	if err := cast(g, mk, 2, 40, 25); err != nil {
		t.Fatal(err)
	}
	dmg := func(h *Hero) int { before := h.HP; g.damageHero(h, 100, monIDBase+1); return before - h.HP }
	if d := dmg(ally); d != 70 {
		t.Fatalf("ally in range should take 70, took %d", d)
	}
	if d := dmg(far); d != 100 {
		t.Fatalf("ally out of range should take 100, took %d", d)
	}
	g.Now += Sec(13)
	ally.HP = ally.HPMax
	if d := dmg(ally); d != 100 {
		t.Fatalf("bell lasts 12 s, took %d", d)
	}
}

func TestSorcererSkills(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Sorcerer, 30, 25)
	m := g.spawnMon('z', 36, 25)
	if err := cast(g, h, 0, 40, 25); err != nil || m.HP >= m.HPMax {
		t.Fatalf("spark should hit the first monster on the line: %v", err)
	}
	if err := cast(g, h, 2, 36, 28); err != nil || h.X != 36 || h.Y != 28 {
		t.Fatalf("blink: %v pos=%d,%d", err, h.X, h.Y)
	}
	g.spawnMon('z', 36, 36)
	hp := m.HP
	if err := cast(g, h, 1, 36, 25); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		g.Tick()
	}
	if m.HP >= hp {
		t.Fatal("ember nova should hurt monsters in the radius")
	}
}

func TestFriendlyFire(t *testing.T) {
	for _, ff := range []bool{false, true} {
		g := arena(2)
		g.Settings.FriendlyFire = ff
		a := hero(g, 1, Sorcerer, 30, 25)
		b := hero(g, 2, Warrior, 40, 25)
		if err := cast(g, a, 1, 40, 25); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 12; i++ {
			g.Tick()
		}
		hurt := b.HP < b.HPMax
		if hurt != ff {
			t.Errorf("friendly fire=%v but ally hurt=%v", ff, hurt)
		}
	}
}

func TestMeleeAutoAimsAdjacent(t *testing.T) {
	for _, c := range []struct {
		cls  ClassID
		slot int
	}{{Warrior, 0}, {Rogue, 0}, {Monk, 0}} {
		g := arena(1)
		h := hero(g, 1, c.cls, 40, 25)
		m := g.spawnMon('z', 40, 24)
		m.HP, m.HPMax = 1000, 1000
		if err := cast(g, h, c.slot, 45, 25); err != nil {
			t.Fatal(err)
		}
		if m.HP >= 1000 {
			t.Fatalf("class %v: adjacent monster should be auto-targeted", c.cls)
		}
	}
}

func TestMeleePrefersAimedTarget(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Rogue, 40, 25)
	e := g.spawnMon('z', 41, 25)
	w := g.spawnMon('z', 39, 25)
	e.HP, e.HPMax, w.HP, w.HPMax = 1000, 1000, 1000, 1000
	if err := cast(g, h, 0, 45, 25); err != nil {
		t.Fatal(err)
	}
	if e.HP >= 1000 || w.HP != 1000 {
		t.Fatalf("only the aimed (east) monster should be hit: e=%d w=%d", e.HP, w.HP)
	}
}
