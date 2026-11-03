package game

import "testing"

func floorGold(g *Game, x, y, n int) {
	g.Items = append(g.Items, &Item{ID: g.newID(), Kind: '$', X: x, Y: y, Amount: n})
}

func shrineAt(g *Game, kind string, x, y int) *Npc {
	n := &Npc{Name: shrineDef(kind).Name, Shrine: kind}
	g.placeNpcAt(n, Pt{x, y})
	return n
}

func TestShrinesAppearBelowTheFirstFloor(t *testing.T) {
	found := 0
	for seed := uint32(1); seed <= 20; seed++ {
		for _, n := range NewAt(seed, Settings{}, 1, 3).Npcs {
			if n.Shrine != "" {
				found++
			}
		}
		for _, n := range NewAt(seed, Settings{}, 1, 1).Npcs {
			if n.Shrine != "" {
				t.Fatal("no shrines on floor 1")
			}
		}
	}
	if found < 6 || found > 18 {
		t.Fatalf("about %.0f%% of floors should have a shrine: %d of 20", ShrineChance*100, found)
	}
}

func TestShrineBlessesTheParty(t *testing.T) {
	g := arena(2)
	a := hero(g, 1, Warrior, 40, 25)
	b := hero(g, 2, Rogue, 30, 30)
	base := b.dmgScale(100)
	n := shrineAt(g, "fury", 41, 25)
	step(t, g, a, 2) // bump it
	if !n.Used || b.dmgScale(100) != base*130/100 {
		t.Fatalf("Fury should bless everyone: %d -> %d", base, b.dmgScale(100))
	}
	until := a.ShrineUntil
	g.Now += 10
	step(t, g, a, 2)
	if a.ShrineUntil != until {
		t.Fatal("a dark shrine gives nothing more")
	}
	g.Now = a.ShrineUntil
	g.Tick()
	if b.dmgScale(100) != base {
		t.Fatal("the blessing wears off")
	}
}

func TestShrineOfHasteAndFortune(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Warrior, 40, 25)
	shrineAt(g, "haste", 41, 25)
	step(t, g, h, 2)
	h.NextMove = 0
	before := g.Now
	step(t, g, h, 4)
	if h.NextMove != before+HasteMoveTicks {
		t.Fatalf("a hasted step takes %d ticks, got %d", HasteMoveTicks, h.NextMove-before)
	}
	shrineAt(g, "fortune", 40, 27)
	step(t, g, h, 4)
	floorGold(g, 40, 25, 10)
	step(t, g, h, 0)
	if h.Gold != 10*FortuneMult {
		t.Fatalf("gold counts double: %d", h.Gold)
	}
}

func TestShrineOfChanceAmbush(t *testing.T) {
	for seed := uint32(1); seed < 60; seed++ {
		g := arena(1)
		g.rng = NewRNG(uint64(seed))
		h := hero(g, 1, Warrior, 40, 25)
		n := shrineAt(g, "chance", 41, 25)
		mons := len(g.Mons)
		g.useShrine(h, n)
		if len(g.Mons) > mons {
			if len(g.Mons)-mons != AmbushSize {
				t.Fatalf("an ambush of %d, got %d", AmbushSize, len(g.Mons)-mons)
			}
			return
		}
	}
	t.Fatal("no ambush in 60 tries")
}
