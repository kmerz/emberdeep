package game

import "testing"

// teach gives the hero a skill at a rank and puts it on key 1.
func teach(h *Hero, id string, rank int) {
	i := h.SkillIndex(id)
	h.Ranks[i] = rank
	h.Binds[0] = i
}

func learn(g *Game, h *Hero, id string) error {
	return g.Apply(h.ID, Intent{Kind: IntLearn, Skill: id})
}

func TestNewHeroTree(t *testing.T) {
	g := arena(1)
	h := g.AddHero(1, "kmerz", Warrior)
	if h.Ranks != [TreeSize]int{1} || h.Binds != [3]int{0, -1, -1} || h.Points != 1 {
		t.Fatalf("start: cleave rank 1 on key 1 and one point, got ranks %v binds %v points %d", h.Ranks, h.Binds, h.Points)
	}
	if err := g.Apply(1, Intent{Kind: IntCast, Slot: 1, X: h.X + 1, Y: h.Y}); err != ErrLocked {
		t.Fatalf("an empty key can't cast, got %v", err)
	}
}

func TestLearningRules(t *testing.T) {
	g := arena(1)
	h := g.AddHero(1, "kmerz", Warrior)
	g.place(h, 30, 25)
	if err := learn(g, h, "ironwall"); err != ErrLocked {
		t.Fatalf("tier 2 is locked at level 1, got %v", err)
	}
	if err := learn(g, h, "shieldbash"); err != nil {
		t.Fatal(err)
	}
	if h.Points != 0 || h.Binds[1] != h.SkillIndex("shieldbash") {
		t.Fatalf("learning spends the point and fills key 2: %v %d", h.Binds, h.Points)
	}
	if err := learn(g, h, "cleave"); err != ErrNoPoints {
		t.Fatalf("no points left, got %v", err)
	}
	h.Lvl, h.Points = SkillTiers[1], 10
	if err := learn(g, h, "warcry"); err != nil {
		t.Fatalf("tier 2 opens at level %d: %v", SkillTiers[1], err)
	}
	if err := learn(g, h, "whirlwind"); err != ErrLocked {
		t.Fatalf("tier 3 is still locked, got %v", err)
	}
	for i := 0; i < MaxRank-1; i++ {
		if err := learn(g, h, "cleave"); err != nil {
			t.Fatal(err)
		}
	}
	if err := learn(g, h, "cleave"); err != ErrMaxRank {
		t.Fatalf("rank %d is the cap, got %v", MaxRank, err)
	}

	// a tier-3 skill needs a tier-2 one: a fresh hero at level 8 with only tier 1 can't jump ahead
	o := g.AddHero(2, "nyx", Rogue)
	o.Lvl, o.Points = SkillTiers[2], 5
	if err := learn(g, o, "knives"); err != ErrLocked {
		t.Fatalf("tier 3 needs a tier-2 skill first, got %v", err)
	}
}

func TestBindSwapsAndCooldownStaysWithTheSkill(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Warrior, 30, 25) // cleave, ironwall, leap on 1-3
	if err := g.Apply(1, Intent{Kind: IntBind, Slot: 0, Skill: "ironwall"}); err != nil {
		t.Fatal(err)
	}
	if h.Binds != [3]int{1, 0, 2} {
		t.Fatalf("binding a skill that's on another key swaps them: %v", h.Binds)
	}
	if err := g.Apply(1, Intent{Kind: IntBind, Slot: 0, Skill: "warcry"}); err != ErrBadIntent {
		t.Fatalf("unlearned skills can't be bound, got %v", err)
	}
	h.Res = 100
	if err := g.Apply(1, Intent{Kind: IntCast, Slot: 0}); err != nil { // iron wall
		t.Fatal(err)
	}
	_ = g.Apply(1, Intent{Kind: IntBind, Slot: 1, Skill: "ironwall"})
	h.Res = 100
	if err := g.Apply(1, Intent{Kind: IntCast, Slot: 1}); err != ErrCooldown {
		t.Fatalf("moving a skill to another key keeps its cooldown, got %v", err)
	}
}

func TestRankRaisesDamage(t *testing.T) {
	base := Classes[Warrior].Skills[0]
	r5 := base.Ranked(5)
	if r5.Dmg <= base.Dmg || r5.CD >= base.CD || base.Ranked(1) != base {
		t.Fatalf("ranks scale damage up and cooldown down: %+v vs %+v", r5, base)
	}
	g := arena(1)
	h := hero(g, 1, Warrior, 40, 25)
	m := g.spawnMon('z', 41, 25)
	m.HP, m.HPMax = 1000, 1000
	_ = cast(g, h, 0, 41, 25)
	d1 := 1000 - m.HP
	teach(h, "cleave", 5)
	m.HP = 1000
	_ = cast(g, h, 0, 41, 25)
	if d5 := 1000 - m.HP; d5 <= d1 {
		t.Fatalf("rank 5 cleave should hit harder: %d vs %d", d5, d1)
	}
}

func TestRespecOnlyAtTheStairs(t *testing.T) {
	g := arena(1)
	h := g.AddHero(1, "kmerz", Rogue)
	g.place(h, 30, 25)
	h.Lvl, h.Points = 9, 0
	h.Ranks = [TreeSize]int{3, 2, 1, 1, 0, 0}
	h.Binds = [3]int{2, 0, 1}
	if err := g.Apply(1, Intent{Kind: IntRespec}); err != ErrNotHere {
		t.Fatalf("respec away from the stairs: %v", err)
	}
	st := g.Level.Stairs
	g.place(h, st.X-1, st.Y)
	if err := g.Apply(1, Intent{Kind: IntRespec}); err != nil {
		t.Fatal(err)
	}
	if h.Ranks != [TreeSize]int{1} || h.Binds != [3]int{0, -1, -1} || h.Points != 6 {
		t.Fatalf("respec keeps rank 1 of the first skill and refunds the rest: %v %v %d", h.Ranks, h.Binds, h.Points)
	}
}

func TestTreeCarriesOverAndRetryRestores(t *testing.T) {
	g := New(13, Settings{}, 1)
	h := g.AddHero(1, "kmerz", Monk)
	if err := learn(g, h, "staffsweep"); err != nil {
		t.Fatal(err)
	}
	takeStairs(g)
	if h.Ranks[h.SkillIndex("staffsweep")] != 1 || g.Depth != 2 {
		t.Fatal("the tree carries over to the next floor")
	}
	h.Points = 3
	g.arrival[1] = h.save()
	h.Points = 0
	h.Ranks[0] = 4
	downHeroNow(g, h)
	for i := 0; i < 3; i++ {
		g.Tick()
	}
	g.RetryFloor()
	if h.Points != 3 || h.Ranks[0] != 1 {
		t.Fatalf("a retry restores the tree as it was on arrival: points %d ranks %v", h.Points, h.Ranks)
	}
}

func TestXPCurveAndPoints(t *testing.T) {
	g := arena(1)
	h := g.AddHero(1, "kmerz", Warrior)
	g.gainXP(700) // about one floor of kills
	if h.Lvl != 3 || h.Points != 1+2 {
		t.Fatalf("700 XP should reach level 3 with 2 new points, got level %d, %d points", h.Lvl, h.Points)
	}
}

func TestNextSkillForBots(t *testing.T) {
	// at level 1 only tier 1 is open: the lowest-ranked learnable skill is the other tier-1 skill
	if got := NextSkill([TreeSize]int{1}, 1); got != 3 {
		t.Fatalf("at level 1 a bot learns the other tier-1 skill, got %d", got)
	}
	if got := NextSkill([TreeSize]int{1}, 3); got != 1 {
		t.Fatalf("at level 3 a bot learns skill 1 first, got %d", got)
	}
	if got := NextSkill([TreeSize]int{5, 5, 5, 5, 5, 5}, 20); got != -1 {
		t.Fatalf("a maxed tree has nothing to learn, got %d", got)
	}
}
