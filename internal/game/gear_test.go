package game

import (
	"encoding/json"
	"strings"
	"testing"
)

func gearItem(g *Game, gr *Gear, x, y, owner int) *Item {
	it := &Item{ID: g.newID(), Kind: Items[gr.Slot].Glyph, Def: gr.Slot, X: x, Y: y, Amount: 1, Owner: owner, G: gr}
	g.Items = append(g.Items, it)
	return it
}

func TestRolledGearFitsTheClassAndRarity(t *testing.T) {
	g := arena(1)
	seen := map[int]bool{}
	for i := 0; i < 400; i++ {
		gr := g.rollGear(Rogue, 1+i%MaxDepth, Common)
		b := GearBases[gr.Base]
		if b.Slot == "weapon" && b.Class != Rogue {
			t.Fatalf("a rogue got a %s", gr.Name)
		}
		if len(gr.Mods) != 1+gr.Rarity || gr.Mods[0].K != b.Stat {
			t.Fatalf("%s (rarity %d) has mods %+v", gr.Name, gr.Rarity, gr.Mods)
		}
		for _, m := range gr.Mods {
			if m.V < 1 {
				t.Fatalf("%s has a useless mod %+v", gr.Name, m)
			}
		}
		if gr.Rarity == Rare && !strings.Contains(gr.Name, " of ") {
			t.Fatalf("a rare should have a suffix: %q", gr.Name)
		}
		seen[gr.Rarity] = true
	}
	if !seen[Common] || !seen[Magic] || !seen[Rare] {
		t.Fatalf("expected every rarity in 400 rolls: %v", seen)
	}
	if gr := g.rollGear(Monk, 3, Rare); gr.Rarity != Rare {
		t.Fatalf("a minimum rarity must hold: %+v", gr)
	}
}

func TestPickingUpGearWearsItWhenTheSlotIsEmpty(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Warrior, 40, 25)
	hp := h.HPMax
	a := &Gear{Slot: "trinket", Base: "amulet", Name: "Amulet", Lvl: 1, Mods: []Mod{{"hp", 20}}}
	gearItem(g, a, 41, 25, 1)
	step(t, g, h, 2)
	if h.Trinket != a || h.HPMax != hp+20 || len(g.Items) != 0 {
		t.Fatalf("the amulet should be worn at once: trinket %v, hp max %d (was %d)", h.Trinket, h.HPMax, hp)
	}
	b := &Gear{Slot: "trinket", Base: "amulet", Name: "Better Amulet", Lvl: 2, Mods: []Mod{{"hp", 30}}}
	gearItem(g, b, 42, 25, 1)
	step(t, g, h, 2)
	slot := h.firstSlot("trinket")
	if h.Trinket != a || slot < 0 || h.Pack[slot].G != b {
		t.Fatal("with the slot taken, new gear goes into the rucksack")
	}
	if err := g.Apply(1, Intent{Kind: IntUse, Slot: slot}); err != nil {
		t.Fatal(err)
	}
	if h.Trinket != b || h.Pack[slot].G != a || h.HPMax != hp+30 {
		t.Fatalf("equipping should swap: worn %v, slot %v, hp max %d", h.Trinket.Name, h.Pack[slot].G, h.HPMax)
	}
	if err := g.Apply(1, Intent{Kind: IntUnequip, Slot: 1}); err != nil {
		t.Fatal(err)
	}
	if h.Trinket != nil || h.HPMax != hp || h.HP > h.HPMax {
		t.Fatalf("taking it off should drop max HP back: %d/%d (base %d)", h.HP, h.HPMax, hp)
	}
}

func TestGearStatsWork(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Warrior, 40, 25)
	base := h.dmgScale(100)
	h.Weapon = &Gear{Slot: "weapon", Base: "axe", Lvl: 1, Mods: []Mod{{"dmg", 20}, {"leech", 10}}}
	if got := h.dmgScale(100); got != base*120/100 {
		t.Fatalf("+20%% damage: %d -> %d", base, got)
	}
	// life steal
	m := g.addMon('h', 41, 25, 1)
	h.HP = 50
	g.damageMonster(m, 50, h.ID, true)
	if h.HP != 55 {
		t.Fatalf("10%% of 50 damage should heal 5: hp %d", h.HP)
	}
	// armor
	h.Trinket = &Gear{Slot: "trinket", Base: "charm", Lvl: 1, Mods: []Mod{{"armor", 25}}}
	h.HP = 100
	g.damageHero(h, 40, m.ID)
	if h.HP != 70 {
		t.Fatalf("25%% armor should turn 40 damage into 30: hp %d", h.HP)
	}
	// caps
	h.Trinket = &Gear{Slot: "trinket", Base: "charm", Lvl: 1, Mods: []Mod{{"armor", 90}}}
	if h.GearStat("armor") != GearCaps["armor"] {
		t.Fatalf("armor should be capped at %d, got %d", GearCaps["armor"], h.GearStat("armor"))
	}
	// cooldowns
	h.Trinket = &Gear{Slot: "trinket", Base: "charm", Lvl: 1, Mods: []Mod{{"cdr", 20}}}
	g.Now += 1000
	h.Res = h.ResMax
	if err := g.Apply(1, Intent{Kind: IntCast, Slot: 1, X: h.X, Y: h.Y}); err != nil { // Iron Wall
		t.Fatal(err)
	}
	if want := g.Now + Classes[Warrior].Skills[1].CD*80/100; h.CDReady[1] != want {
		t.Fatalf("20%% shorter cooldown: ready at %d, want %d", h.CDReady[1], want)
	}
}

func TestBossesDropARareForEveryone(t *testing.T) {
	g := arena(2)
	a := hero(g, 1, Warrior, 40, 25)
	b := hero(g, 2, Sorcerer, 44, 25)
	m := g.addMon('K', 42, 25, 1)
	m.Elite = true
	g.killMonster(m, a.ID)
	got := map[int]int{}
	for _, it := range g.Items {
		if it.G != nil && it.G.Rarity == Rare {
			got[it.Owner]++
		}
	}
	if got[a.ID] != 1 || got[b.ID] != 1 {
		t.Fatalf("each hero should get one rare: %v", got)
	}
}

func TestGearSurvivesSaveAndSells(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Monk, 40, 25)
	h.wear("weapon", g.rollGear(Monk, 4, Rare))
	h.addGear(g.rollGear(Monk, 4, Magic))
	b, err := json.Marshal(g.Save(nil))
	if err != nil {
		t.Fatal(err)
	}
	var s RunSave
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	sh := s.Heroes[h.Name].Hero
	if sh.Weapon == nil || sh.Weapon.Name != h.Weapon.Name || sh.HPMax != h.HPMax {
		t.Fatalf("worn gear should be saved: %+v", sh.Weapon)
	}
	slot := h.firstSlot("weapon")
	if slot < 0 || sh.Pack[slot].G == nil || sh.Pack[slot].G.Name != h.Pack[slot].G.Name {
		t.Fatal("rucksack gear should be saved")
	}
	if h.Pack[slot].G.Value() <= h.Pack[slot].G.Lvl {
		t.Fatal("gear should be worth something to a trader")
	}
}

func TestDroppedGearKeepsItsRoll(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Rogue, 40, 25)
	gr := g.rollGear(Rogue, 2, Magic)
	h.addGear(gr)
	slot := h.firstSlot(gr.Slot)
	if err := g.Apply(1, Intent{Kind: IntDrop, Slot: slot}); err != nil {
		t.Fatal(err)
	}
	if len(g.Items) != 1 || g.Items[0].G != gr || h.Pack[slot].N != 0 {
		t.Fatal("the dropped piece should lie on the floor as it was")
	}
}

func TestBotsWearBetterGear(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Warrior, 40, 25)
	h.wear("weapon", &Gear{Slot: "weapon", Base: "axe", Lvl: 1, Mods: []Mod{{"dmg", 5}}})
	better := &Gear{Slot: "weapon", Base: "axe", Lvl: 3, Mods: []Mod{{"dmg", 14}, {"crit", 5}}}
	h.addGear(better)
	if !g.EquipBetter(h) || h.Weapon != better {
		t.Fatal("a bot should wear the better axe")
	}
	if g.EquipBetter(h) {
		t.Fatal("and not swap back")
	}
}
