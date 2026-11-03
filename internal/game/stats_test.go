package game

import "testing"

func TestRecapNamesWhatDownedTheHero(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Warrior, 40, 25)
	imp := g.spawnMon('i', 45, 25)
	g.events = nil
	g.damageHero(h, h.HP+5, imp.ID, shotName(imp))
	var down *Event
	for i := range g.events {
		if g.events[i].K == "down" {
			down = &g.events[i]
		}
	}
	if down == nil || down.Text != "an Imp's firebolt" || down.Amount != h.HPMax+5 {
		t.Fatalf("the down event names the killer and the hit: %+v", down)
	}
	s := g.stats(h)
	if s.Downs != 1 || s.LastDowned != "an Imp's firebolt" {
		t.Fatalf("stats keep the down: %+v", s)
	}
	cases := map[string]string{
		g.cause(g.spawnMon('z', 30, 25).ID, ""):     "a Zombie",
		g.cause(g.spawnMon('K', 30, 27).ID, "bite"): "the Tallow King's bite",
		g.cause(0, "a spike trap"):                  "a spike trap",
		g.cause(g.spawnMon('o', 30, 29).ID, ""):     "a Blob",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("cause: got %q, want %q", got, want)
		}
	}
}

func TestStatsCountTheRunAndSurviveARetry(t *testing.T) {
	g := arena(2)
	a := hero(g, 1, Warrior, 40, 25)
	b := hero(g, 2, Rogue, 41, 25)
	z := g.spawnMon('z', 40, 26)
	hp := z.HP
	g.damageMonster(z, hp+50, a.ID, true) // overkill does not count
	sa := g.stats(a)
	if sa.Kills != 1 || sa.Dmg != hp || sa.BestHit != hp+50 {
		t.Fatalf("a kill: %+v", sa)
	}
	downHeroNow(g, b)
	if err := g.Apply(1, Intent{Kind: IntRevive, Target: 2, Hold: true}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < ReviveTicks+1; i++ {
		g.Tick()
	}
	if b.Downed || g.stats(a).Revives != 1 {
		t.Fatalf("a revive counts: downed=%v %+v", b.Downed, g.stats(a))
	}
	// a wipe and a retry put the heroes back as they arrived, but the run stats stay
	downHeroNow(g, a)
	downHeroNow(g, b)
	g.checkWipe()
	g.RetryFloor()
	if g.stats(a).Kills != 1 || g.stats(b).Downs != 2 {
		t.Fatalf("stats survive a retry: a %+v b %+v", g.stats(a), g.stats(b))
	}
	// and a save
	l := Load(g.Save(nil), 2)
	if l.Stats[a.Name] == nil || l.Stats[a.Name].Kills != 1 {
		t.Fatalf("stats are saved: %+v", l.Stats)
	}
}

func TestSummaryTitles(t *testing.T) {
	g := arena(3)
	a := hero(g, 1, Warrior, 40, 25)
	b := hero(g, 2, Rogue, 42, 25)
	c := hero(g, 3, Monk, 44, 25)
	*g.stats(a) = RunStats{Kills: 9, Dmg: 900, Taken: 500}
	*g.stats(b) = RunStats{Kills: 3, Dmg: 950, Revives: 2}
	*g.stats(c) = RunStats{}
	s := g.Summary()
	if s[0].Title != "the Butcher" || s[1].Title != "the Furnace" || s[2].Title != "the Tourist" {
		t.Fatalf("titles: %q %q %q", s[0].Title, s[1].Title, s[2].Title)
	}
	if s[0].Kills != 9 || s[1].Name != b.Name || s[2].Cls != string(Monk) {
		t.Fatalf("summary rows: %+v", s)
	}
}
