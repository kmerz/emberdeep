package game

import "testing"

func TestDeepFloors(t *testing.T) {
	if ThemeAt(8).ID != "abyss" || Floor(MaxDepth).Boss != FinalBoss || Floor(5).Boss != 'K' {
		t.Fatal("floor 8 is the Abyss Gate, floor 10 has the dragon, floor 5 still the king")
	}
	for _, id := range []string{"abyss", "ember", "caverns", "dark"} {
		th := Themes[id]
		for i := 0; i < len(th.Kinds); i++ {
			if _, ok := MonDefs[th.Kinds[i]]; !ok {
				t.Fatalf("theme %s: unknown kind %q", id, th.Kinds[i])
			}
		}
	}
}

func TestImpShootsAndKeepsAway(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Warrior, 30, 25)
	imp := g.addMon('i', 37, 25, 1)
	for i := 0; i < 30; i++ {
		g.Tick()
	}
	if h.HP >= h.HPMax {
		t.Fatal("the imp shoots from range")
	}
	if cheb(imp.X, imp.Y, 30, 25) <= 1 {
		t.Fatal("the imp stays out of melee")
	}
	// a hero that closes in makes it back off
	g.place(h, imp.X-2, imp.Y)
	x0 := imp.X
	for i := 0; i < 10; i++ {
		g.Tick()
		h.HP = h.HPMax
	}
	if dist(imp.X, imp.Y, h.X, h.Y) <= dist(x0, imp.Y, h.X, h.Y)-1 {
		t.Fatalf("the imp should back away: %d -> %d", x0, imp.X)
	}
}

func TestGolemLeavesBurningGround(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Warrior, 30, 25)
	golem := g.addMon('G', 40, 25, 1) // within the pathfinder's reach (it searches ~700 cells)
	start := Pt{golem.X, golem.Y}
	for i := 0; i < 40; i++ {
		g.Tick()
	}
	if golem.X == start.X {
		t.Fatal("the golem should have moved")
	}
	burning := false
	for _, f := range g.Fires {
		burning = burning || (f.X == start.X && f.Y == start.Y)
	}
	if !burning {
		t.Fatal("the golem leaves fire where it walked")
	}
	// a hero standing in fire takes damage once a second
	g.Fires = []Fire{{X: h.X, Y: h.Y, Until: g.Now + 100, Dmg: FireDmg}}
	golem.StunUntil = 1 << 30
	hp := h.HP
	for i := 0; i < TickHz+1; i++ {
		g.Tick()
	}
	if h.HP >= hp {
		t.Fatal("standing in fire hurts")
	}
}

func TestShadeIsHiddenUntilClose(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Warrior, 30, 25)
	s := g.addMon('S', 40, 25, 1)
	if g.Revealed(s) {
		t.Fatal("a shade 10 cells away is unseen")
	}
	g.place(h, 37, 25)
	if !g.Revealed(s) {
		t.Fatal("a shade within 3 cells shows itself")
	}
	if g.Revealed(g.addMon('z', 55, 25, 1)) == false {
		t.Fatal("ordinary monsters are always revealed")
	}
}

func TestSuccubusCharms(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Warrior, 30, 25)
	u := g.addMon('U', 36, 25, 1)
	g.Tick()
	if g.Now >= h.CharmUntil || h.CharmBy != u.ID {
		t.Fatal("the succubus charms a hero in range")
	}
	if err := g.Apply(1, Intent{Kind: IntMove, Dir: 6}); err != ErrCharmed {
		t.Fatalf("a charmed hero can't act: %v", err)
	}
	x0 := h.X
	for i := 0; i < CharmTicks; i++ {
		g.Tick()
	}
	if h.X <= x0 {
		t.Fatal("a charmed hero drifts toward the succubus")
	}
	if g.Now < h.CharmUntil {
		t.Fatal("the charm wears off")
	}
}

func TestIncubusDrainsAndBlinks(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Sorcerer, 40, 25)
	inc := g.addMon('I', 41, 25, 1)
	inc.HP = inc.HPMax - 10
	h.Res = 100
	for i := 0; i < 30 && h.Res == 100; i++ {
		g.Tick()
	}
	if h.Res >= 100 || inc.HP <= inc.HPMax-10 {
		t.Fatalf("the incubus drains resource and heals: res %d hp %d/%d", h.Res, inc.HP, inc.HPMax)
	}
	blinked := false
	for i := 0; i < 40 && !blinked; i++ {
		x, y := inc.X, inc.Y
		inc.NextAbility = 0
		g.damageMonster(inc, 1, h.ID, false)
		blinked = cheb(x, y, inc.X, inc.Y) >= 3
		inc.HP = inc.HPMax
	}
	if !blinked {
		t.Fatal("a hit incubus blinks away now and then")
	}
}

func TestDragonBreathIsTelegraphed(t *testing.T) {
	g := arena(2)
	front := hero(g, 1, Warrior, 36, 25)
	side := hero(g, 2, Rogue, 30, 33) // behind and below: outside the cone toward front
	d := g.addMon('D', 30, 25, 1)
	g.Tick()
	if d.BreathAt == 0 {
		t.Fatal("the dragon winds up a breath at a hero in reach")
	}
	hp := front.HP
	if front.HP != front.HPMax {
		t.Fatal("nothing lands during the wind-up")
	}
	for i := 0; i < BreathWind+1; i++ {
		g.Tick()
	}
	if front.HP >= hp || len(g.Fires) == 0 {
		t.Fatal("the breath burns the cone and leaves fire")
	}
	if side.HP != side.HPMax {
		t.Fatal("a hero outside the cone is safe")
	}
}
