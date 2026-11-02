package game

import (
	"math"
	"testing"
)

func TestScalingMath(t *testing.T) {
	cases := []struct {
		n     int
		enemy float64
		elite float64
		pack  int
	}{{1, 1.0, 1.0, 3}, {2, 1.6, 1.8, 4}, {3, 2.2, 2.6, 5}, {4, 2.8, 3.4, 6}}
	for _, c := range cases {
		if got := EnemyHPMult(c.n); math.Abs(got-c.enemy) > 1e-9 {
			t.Errorf("EnemyHPMult(%d)=%v want %v", c.n, got, c.enemy)
		}
		if got := EliteHPMult(c.n); math.Abs(got-c.elite) > 1e-9 {
			t.Errorf("EliteHPMult(%d)=%v want %v", c.n, got, c.elite)
		}
		if got := PackSize(3, c.n); got != c.pack {
			t.Errorf("PackSize(3,%d)=%d want %d", c.n, got, c.pack)
		}
	}
}

func TestScalingAppliedToLevelAndMonsters(t *testing.T) {
	for _, n := range []int{1, 2, 4} {
		g := New(5, Settings{}, n)
		for _, m := range g.Mons {
			if m.Kind == KegKind { // a keg always breaks in one hit
				continue
			}
			base := float64(MonDefs[m.Kind].HP)
			mult := EnemyHPMult(n)
			if m.Elite {
				mult = EliteHPMult(n)
			}
			if want := int(math.Round(base * mult)); m.HP != want || m.HPMax != want {
				t.Fatalf("n=%d kind %c: hp %d want %d", n, m.Kind, m.HP, want)
			}
		}
	}
	// pack size grows by n-1 per room (same seed => same rooms)
	l1, l4 := Generate(5, 1, 1), Generate(5, 1, 4)
	if want := len(l1.Mons) + 3*(len(l1.Rooms)-1); len(l4.Mons) != want {
		t.Fatalf("expected %d monsters with 4 players, got %d", want, len(l4.Mons))
	}
}

func TestEveryClassHasThreeSkillsAndColors(t *testing.T) {
	for _, c := range AllClasses {
		d := Classes[c]
		if d == nil || d.HP <= 0 || d.ResMax <= 0 {
			t.Fatalf("class %s incomplete", c)
		}
		for i, s := range d.Skills {
			if s.ID == "" || s.Name == "" || s.CD <= 0 {
				t.Errorf("%s skill %d incomplete: %+v", c, i, s)
			}
		}
	}
}

func TestNoviceMonstersAreWeaker(t *testing.T) {
	novice := -1
	for i, d := range Difficulties {
		if d.Name == "NOVICE" {
			novice = i
		}
	}
	if novice <= 0 {
		t.Fatal("NOVICE must exist and NORMAL must stay the default (index 0)")
	}
	normal, easy := New(42, Settings{}, 1), New(42, Settings{Difficulty: novice}, 1)
	if len(normal.Mons) != len(easy.Mons) || len(normal.Mons) == 0 {
		t.Fatalf("same seed should give the same monsters: %d vs %d", len(normal.Mons), len(easy.Mons))
	}
	for i, m := range normal.Mons {
		e := easy.Mons[i]
		want := float64(m.HP) * Difficulties[novice].HPMult
		if float64(e.HP) < want-1 || float64(e.HP) > want+1 {
			t.Fatalf("%c: novice hp %d, want about %.0f (normal %d)", m.Kind, e.HP, want, m.HP)
		}
	}
	m := normal.Mons[0]
	if a, b := normal.monsterDmg(m, 20), easy.monsterDmg(easy.Mons[0], 20); b >= a {
		t.Fatalf("novice monsters should hit softer: %d vs normal %d", b, a)
	}
}
