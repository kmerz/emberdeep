package game

import (
	"encoding/json"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	g := New(4242, Settings{Difficulty: 2}, 1)
	h := g.AddHero(1, "kmerz", Warrior)
	h.Lvl, h.Gold, h.Points = 4, 123, 2
	h.AddItem("firebomb", 3)
	g.Quests[0].State, g.Quests[0].Progress = QuestActive, 1
	g.VillageSeen, g.Kills = true, 77
	g.descend()

	b, err := json.Marshal(g.Save(map[string]SavedHero{"away": {Cls: Monk, Hero: heroSave{Lvl: 9, HPMax: 200, HP: 200}}}))
	if err != nil {
		t.Fatal(err)
	}
	var s RunSave
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	if s.Depth != 2 || s.Seed != 4242 || s.Settings.Difficulty != 2 || len(s.Heroes) != 2 || s.Heroes["away"].Lvl() != 9 {
		t.Fatalf("save lost something: %+v", s)
	}

	g2 := Load(s, 2)
	if g2.Depth != 2 || !g2.VillageSeen || g2.Kills != 77 || g2.Quests[0].State != QuestActive || g2.Quests[0].Progress != 1 {
		t.Fatalf("run not restored: depth %d village %v kills %d quest %+v", g2.Depth, g2.VillageSeen, g2.Kills, g2.Quests[0])
	}
	if g2.Level.Start != Generate(4242, 2, 2).Start {
		t.Fatal("the floor should be the seed's floor 2")
	}
	h2 := g2.AddSavedHero(5, "kmerz", s.Heroes["kmerz"])
	if h2.Lvl != 4 || h2.Gold != 123 || h2.Points != 2 || h2.Count("firebomb") != 3 || h2.Cls != Warrior {
		t.Fatalf("hero not restored: %+v", h2)
	}
}

func TestSaveKeepsDownedHeroAsTheyArrived(t *testing.T) {
	g := New(1, Settings{}, 1)
	h := g.AddHero(1, "kmerz", Rogue)
	h.Gold = 10
	g.arrival[1] = h.save()
	h.Gold, h.HP, h.Downed = 500, 0, true
	s := g.SaveHero(h)
	if s.Hero.Gold != 10 || s.Hero.HP <= 0 {
		t.Fatalf("a downed hero is saved as they arrived, standing: %+v", s.Hero)
	}
}

func TestCatchUpToPartyLevel(t *testing.T) {
	g := New(1, Settings{}, 3)
	a := g.AddHero(1, "a", Warrior)
	b := g.AddHero(2, "b", Monk)
	a.Lvl, b.Lvl = 6, 4
	c := g.AddHero(3, "c", Sorcerer)
	pts := c.Points
	g.CatchUp(c)
	if c.Lvl != 5 || c.Points != pts+4 || c.HP != c.HPMax || c.HPMax <= Classes[Sorcerer].HP {
		t.Fatalf("new hero should reach level 5 with 4 more points and full HP: lvl %d pts %d hp %d/%d", c.Lvl, c.Points, c.HP, c.HPMax)
	}
}
