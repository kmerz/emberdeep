package game

import "testing"

func villageWithSmith(t *testing.T) (*Game, *Hero) {
	t.Helper()
	g, h := villageWithTrader(t)
	var sm *Npc
	for _, n := range g.Npcs {
		if n.Smith {
			sm = n
		}
	}
	if sm == nil || sm.Name != "Bramwick" {
		t.Fatal("Bramwick should be at his forge")
	}
	p, _ := g.Level.nearestFree(Pt{sm.X, sm.Y + 1}, func(x, y int) bool { return g.occ[y*LW+x] == 0 })
	g.place(h, p.X, p.Y)
	if !g.nextToSmith(h) || g.nextToTrader(h) {
		t.Fatal("couldn't stand next to the smith (and only him)")
	}
	return g, h
}

func TestTemper(t *testing.T) {
	g, h := villageWithSmith(t)
	axe := &Gear{Slot: "trinket", Base: "amulet", Rarity: Common, Lvl: 6, Mods: []Mod{{"hp", 20}}}
	axe.Name = gearName(axe)
	h.wear("trinket", axe)
	hpMax := h.HPMax
	if err := g.Apply(1, Intent{Kind: IntBuy, Skill: "temper0"}); err != ErrNoItem {
		t.Fatalf("no weapon worn: %v", err)
	}
	h.Gold = TemperPrice(axe) - 1
	if err := g.Apply(1, Intent{Kind: IntBuy, Skill: "temper1"}); err != ErrNoGold {
		t.Fatalf("too poor: %v", err)
	}
	h.Gold = 1000
	if err := g.Apply(1, Intent{Kind: IntBuy, Skill: "temper1"}); err != nil {
		t.Fatal(err)
	}
	tr := h.Trinket
	if tr == axe || tr.Rarity != Magic || len(tr.Mods) != 2 || tr.Mods[0] != axe.Mods[0] || h.Gold != 1000-TemperPrice(axe) {
		t.Fatalf("tempered into a new magic piece with the same base stat: %+v gold %d", tr, h.Gold)
	}
	if len(axe.Mods) != 1 || axe.Rarity != Common {
		t.Fatal("the old piece must not change")
	}
	if tr.Name == axe.Name {
		t.Fatalf("a new bonus gives a new name: %s", tr.Name)
	}
	if want := hpMax + tr.Stat("hp") - axe.Stat("hp"); h.HPMax != want {
		t.Fatalf("max HP follows the gear: %d, want %d", h.HPMax, want)
	}
	_ = g.Apply(1, Intent{Kind: IntBuy, Skill: "temper1"})
	if h.Trinket.Rarity != Rare || len(h.Trinket.Mods) != 3 || h.Trinket.Mods[1] != tr.Mods[1] {
		t.Fatalf("magic to rare keeps the first bonus: %+v", h.Trinket)
	}
	if err := g.Apply(1, Intent{Kind: IntBuy, Skill: "temper1"}); err != ErrBadIntent {
		t.Fatalf("rare is as far as tempering goes: %v", err)
	}
	h.wear("trinket", uniqueFor('Q'))
	if err := g.Apply(1, Intent{Kind: IntBuy, Skill: "reforge1"}); err != ErrBadIntent {
		t.Fatalf("he won't touch a unique: %v", err)
	}
	g.place(h, g.Level.Start.X, g.Level.Start.Y)
	if err := g.Apply(1, Intent{Kind: IntBuy, Skill: "temper0"}); err != ErrNotHere {
		t.Fatalf("only at the forge: %v", err)
	}
}

func TestReforge(t *testing.T) {
	g, h := villageWithSmith(t)
	w := &Gear{Slot: "weapon", Base: "staff", Rarity: Common, Lvl: 6, Mods: []Mod{{"dmg", 20}}}
	h.wear("weapon", w)
	h.Gold = 5000
	if err := g.Apply(1, Intent{Kind: IntBuy, Skill: "reforge0"}); err != ErrBadIntent {
		t.Fatalf("a common piece has nothing to reforge: %v", err)
	}
	w = &Gear{Slot: "weapon", Base: "staff", Rarity: Rare, Lvl: 6, Mods: []Mod{{"dmg", 20}, {"crit", 1}, {"leech", 1}}}
	h.wear("weapon", w)
	changed := false
	for i := 0; i < 10; i++ {
		before := h.Weapon
		gold := h.Gold
		if err := g.Apply(1, Intent{Kind: IntBuy, Skill: "reforge0"}); err != nil {
			t.Fatal(err)
		}
		nw := h.Weapon
		if gold-h.Gold != ReforgePrice(before) || nw.Rarity != Rare || len(nw.Mods) != 3 || nw.Mods[0] != w.Mods[0] || nw.Mods[1].K == nw.Mods[2].K {
			t.Fatalf("a rare stays a rare with its base stat and two different bonuses: %+v", nw)
		}
		changed = changed || nw.Mods[1] != w.Mods[1] || nw.Mods[2] != w.Mods[2]
	}
	if !changed {
		t.Fatal("reforging should roll new bonuses")
	}
}
