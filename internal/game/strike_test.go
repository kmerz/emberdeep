package game

import "testing"

func TestWalkingIntoAnEnemyStrikesIt(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Sorcerer, 30, 25)
	m := g.spawnMon('z', 31, 25)
	m.StunUntil = 1 << 30
	hp := m.HP
	if err := g.Apply(1, Intent{Kind: IntMove, Dir: dirIndex(1, 0, 0)}); err != nil {
		t.Fatal(err)
	}
	if m.HP >= hp || h.X != 30 {
		t.Fatalf("bumping hits and doesn't move: hp %d/%d x %d", m.HP, hp, h.X)
	}
	hp = m.HP
	_ = g.Apply(1, Intent{Kind: IntMove, Dir: dirIndex(1, 0, 0)})
	if m.HP != hp {
		t.Fatal("strikes have a short cooldown")
	}
}

func TestOutOfResourceTheSkillKeyStrikes(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Sorcerer, 30, 25)
	m := g.spawnMon('I', 31, 25) // an incubus has drained us
	m.StunUntil = 1 << 30
	h.Res = 0
	hp := m.HP
	if err := cast(g, h, 0, 31, 25); err != nil {
		t.Fatalf("no mana: the key still strikes, got %v", err)
	}
	if m.HP >= hp {
		t.Fatal("the strike hurts")
	}
	g.place(h, 40, 25) // nothing next to us: the key reports why it did nothing
	g.Now += StrikeTicks
	if err := cast(g, h, 0, 45, 25); err != ErrResource {
		t.Fatalf("no enemy in reach: %v", err)
	}
}

func TestKegsAreNotStruckByWalking(t *testing.T) {
	g := arena(1)
	hero(g, 1, Warrior, 30, 25)
	k := g.spawnMon(KegKind, 31, 25)
	_ = g.Apply(1, Intent{Kind: IntMove, Dir: dirIndex(1, 0, 0)})
	if k.HP <= 0 || len(g.pend) != 0 {
		t.Fatal("walking into a keg must not light it")
	}
}
