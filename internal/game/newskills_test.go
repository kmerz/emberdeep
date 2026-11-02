package game

import "testing"

func tough(m *Monster) *Monster { m.HP, m.HPMax, m.StunUntil = 1000, 1000, 0; return m }

func TestShieldBashStuns(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Warrior, 40, 25)
	teach(h, "shieldbash", 1)
	m := tough(g.spawnMon('z', 41, 25))
	if err := cast(g, h, 0, 41, 25); err != nil {
		t.Fatal(err)
	}
	if m.HP >= 1000 || m.StunUntil <= g.Now {
		t.Fatal("shield bash hurts and stuns")
	}
}

func TestWarCryTauntsAndBuffs(t *testing.T) {
	g := arena(2)
	w := hero(g, 1, Warrior, 40, 25)
	r := hero(g, 2, Rogue, 42, 25)
	teach(w, "warcry", 1)
	m := tough(g.spawnMon('z', 45, 25))
	g.addThreat(m, r.ID, 50)
	if err := cast(g, w, 0, 40, 25); err != nil {
		t.Fatal(err)
	}
	if g.pickTarget(m) != w || r.CryUntil <= g.Now || w.CryUntil <= g.Now {
		t.Fatal("war cry pulls the monster onto the warrior and buffs heroes in range")
	}
	plain := r.dmgScale(100)
	r.buff = 1 + CryBuffPct/100.0
	if r.dmgScale(100) <= plain {
		t.Fatal("the buff raises damage")
	}
}

func TestWhirlwindHitsAllAround(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Warrior, 40, 25)
	teach(h, "whirlwind", 1)
	var ms []*Monster
	for _, v := range []Pt{{-1, 0}, {1, 0}, {0, 1}, {1, -1}} {
		ms = append(ms, tough(g.spawnMon('z', 40+v.X, 25+v.Y)))
	}
	if err := cast(g, h, 0, 41, 25); err != nil {
		t.Fatal(err)
	}
	for _, m := range ms {
		if m.HP >= 1000 {
			t.Fatal("whirlwind hits every adjacent enemy")
		}
	}
}

func TestPoisonBladeChargesStabs(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Rogue, 40, 25) // stab on key 1
	h.Binds[1] = h.SkillIndex("poisonblade")
	h.Ranks[h.Binds[1]] = 1
	m := tough(g.spawnMon('z', 41, 25))
	if err := cast(g, h, 1, 40, 25); err != nil {
		t.Fatal(err)
	}
	if h.PoisonCharges != 3 {
		t.Fatalf("poison blade gives 3 charges, got %d", h.PoisonCharges)
	}
	h.Res = 100
	if err := cast(g, h, 0, 41, 25); err != nil {
		t.Fatal(err)
	}
	if h.PoisonCharges != 2 || m.BleedUntil <= g.Now || m.BleedDmg <= 0 {
		t.Fatal("a stab spends a charge and poisons")
	}
}

func TestSmokeStepHides(t *testing.T) {
	g := arena(2)
	r := hero(g, 1, Rogue, 40, 25)
	o := hero(g, 2, Warrior, 50, 25)
	teach(r, "smokestep", 1)
	m := tough(g.spawnMon('z', 41, 25))
	g.addThreat(m, r.ID, 100)
	if err := cast(g, r, 0, 40, 25); err != nil {
		t.Fatal(err)
	}
	if tg := g.pickTarget(m); tg == r {
		t.Fatal("a hidden rogue can't be targeted")
	} else if tg != o {
		t.Fatalf("the monster goes for the other hero, got %v", tg)
	}
	g.Now = r.HiddenUntil
	if !g.eligibleHero(r) {
		t.Fatal("hiding wears off")
	}
}

func TestFanOfKnives(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Rogue, 30, 25)
	teach(h, "knives", 1)
	a := tough(g.spawnMon('z', 34, 25))
	b := tough(g.spawnMon('z', 34, 23))
	far := tough(g.spawnMon('z', 45, 25))
	if err := cast(g, h, 0, 40, 25); err != nil {
		t.Fatal(err)
	}
	if a.HP >= 1000 || b.HP >= 1000 || far.HP < 1000 {
		t.Fatalf("knives hit the cone, not beyond their range: a %d b %d far %d", a.HP, b.HP, far.HP)
	}
}

func TestFrostRingSlows(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Sorcerer, 40, 25)
	teach(h, "frostring", 1)
	near := tough(g.spawnMon('z', 44, 25))
	far := tough(g.spawnMon('z', 55, 25))
	if err := cast(g, h, 0, 40, 25); err != nil {
		t.Fatal(err)
	}
	if near.SlowUntil <= g.Now || far.SlowUntil > g.Now {
		t.Fatal("frost ring slows enemies in its radius only")
	}
}

func TestLightSkillMakesAnOrb(t *testing.T) {
	g := darkArena(1)
	h := hero(g, 1, Sorcerer, 30, 25)
	teach(h, "light", 1)
	if err := cast(g, h, 0, 42, 25); err != nil {
		t.Fatal(err)
	}
	g.Tick()
	if len(g.Orbs) != 1 || !g.IsVisible(42, 25) {
		t.Fatal("light puts an orb at the cursor")
	}
}

func TestChainSparkJumps(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Sorcerer, 30, 25)
	teach(h, "chainspark", 1)
	a := tough(g.spawnMon('z', 36, 25))
	b := tough(g.spawnMon('z', 39, 25))
	c := tough(g.spawnMon('z', 42, 26))
	far := tough(g.spawnMon('z', 58, 40))
	if err := cast(g, h, 0, 36, 25); err != nil {
		t.Fatal(err)
	}
	if a.HP >= 1000 || b.HP >= 1000 || c.HP >= 1000 || far.HP < 1000 {
		t.Fatalf("the spark jumps to nearby enemies only: %d %d %d far %d", a.HP, b.HP, c.HP, far.HP)
	}
	if 1000-c.HP >= 1000-a.HP {
		t.Fatal("each jump is weaker")
	}
}

func TestStaffSweepReachesTwo(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Monk, 40, 25)
	teach(h, "staffsweep", 1)
	near := tough(g.spawnMon('z', 41, 25))
	two := tough(g.spawnMon('z', 42, 25))
	three := tough(g.spawnMon('z', 43, 25))
	if err := cast(g, h, 0, 41, 25); err != nil {
		t.Fatal(err)
	}
	if near.HP >= 1000 || two.HP >= 1000 || three.HP < 1000 {
		t.Fatalf("the sweep reaches 2 cells: %d %d %d", near.HP, two.HP, three.HP)
	}
}

func TestHealingBreath(t *testing.T) {
	g := arena(2)
	m := hero(g, 1, Monk, 40, 25)
	a := hero(g, 2, Warrior, 43, 25)
	teach(m, "healbreath", 1)
	m.HP, a.HP = 10, 10
	if err := cast(g, m, 0, 40, 25); err != nil {
		t.Fatal(err)
	}
	if m.HP <= 10 || a.HP <= 10 {
		t.Fatal("healing breath heals the monk and nearby allies")
	}
}

func TestHundredFists(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Monk, 40, 25)
	teach(h, "fists", 1)
	if err := cast(g, h, 0, 41, 25); err != ErrBadTarget {
		t.Fatalf("no target, no flurry: %v", err)
	}
	m := tough(g.spawnMon('z', 41, 25))
	m.StunUntil = 1 << 30
	if err := cast(g, h, 0, 41, 25); err != nil {
		t.Fatal(err)
	}
	hits := 0
	for i := 0; i < 12; i++ {
		for _, e := range g.Tick() {
			if e.K == "hit" && e.Target == m.ID {
				hits++
			}
		}
	}
	if hits != 5 {
		t.Fatalf("expected 5 blows, got %d", hits)
	}
}
