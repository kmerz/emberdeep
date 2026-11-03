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
	g.damageHero(h, 40, m.ID, "")
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

func TestGrimbleSellsGearForYourClass(t *testing.T) {
	g, h := villageWithTrader(t)
	w := g.Wares[h.ID]
	if len(w) != WaresPerHero {
		t.Fatalf("expected %d wares, got %d", WaresPerHero, len(w))
	}
	for _, gr := range w {
		if b := GearBases[gr.Base]; gr.Rarity < Magic || gr.Lvl != VillageAfter+1 || (b.Slot == "weapon" && b.Class != Monk) {
			t.Fatalf("wares should be magic or better, level %d, for a monk: %+v", VillageAfter+1, gr)
		}
	}
	gr := w[0]
	h.Gold = WarePrice(gr) - 1
	if err := g.Apply(1, Intent{Kind: IntBuy, Skill: "gear0"}); err != ErrNoGold {
		t.Fatalf("too poor: %v", err)
	}
	h.Gold++
	if err := g.Apply(1, Intent{Kind: IntBuy, Skill: "gear0"}); err != nil {
		t.Fatal(err)
	}
	worn := h.Weapon
	if gr.Slot == "trinket" {
		worn = h.Trinket
	}
	if h.Gold != 0 || worn != gr || w[0] != nil {
		t.Fatalf("the piece should be worn and paid for: gold %d", h.Gold)
	}
	if err := g.Apply(1, Intent{Kind: IntBuy, Skill: "gear0"}); err != ErrNoItem {
		t.Fatalf("a sold piece is gone: %v", err)
	}
	if err := g.Apply(1, Intent{Kind: IntBuy, Skill: "gear9"}); err != ErrNoItem {
		t.Fatalf("no such ware: %v", err)
	}
}

func TestBossUniques(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Sorcerer, 40, 25)
	m := g.addMon('K', 42, 25, 1)
	m.Elite = true
	g.killMonster(m, h.ID)
	var wick *Gear
	for _, it := range g.Items {
		if it.G != nil && it.G.Rarity == Unique {
			wick = it.G
		}
	}
	if wick == nil || wick.Name != "The Tallow King's Wick" {
		t.Fatal("the Tallow King should drop his wick")
	}
	h.wear("trinket", wick)
	// thorns land after the monsters act
	z := g.addMon('h', 41, 25, 1)
	hp := z.HP
	g.damageHero(h, 20, z.ID, "")
	if z.HP != hp {
		t.Fatal("thorns must wait for the end of the tick")
	}
	g.payThorns()
	if z.HP != hp-7 { // 20 less the wick's 12% armor is 18; 40% of that is 7
		t.Fatalf("40%% of the 18 damage taken should be dealt back: %d -> %d", hp, z.HP)
	}
	// a lantern that never runs out
	g.Level.Theme = Themes["dark"]
	h.Oil = 0
	if !g.LanternLit(h) {
		t.Fatal("the wick lights the dark without oil")
	}
	// the dragon's heart: slayer
	h.wear("trinket", uniqueFor(FinalBoss))
	boss := g.addMon('K', 44, 26, 1)
	boss.Elite = true
	hp = boss.HP
	g.damageMonster(boss, 100, h.ID, true)
	if hp-boss.HP != 130 {
		t.Fatalf("+30%% against a boss: dealt %d", hp-boss.HP)
	}
	// the locket: HP regeneration
	h.wear("trinket", uniqueFor('Q'))
	h.HP = 10
	for i := 0; i < TickHz; i++ {
		h.regenHP()
	}
	if h.HP != 12 {
		t.Fatalf("2 HP a second: hp %d", h.HP)
	}
}

func TestWorldUniques(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Rogue, 40, 25)
	for i := 0; i < 500; i++ {
		g.dropGear(h, 41, 25, Common)
	}
	for _, it := range g.Items {
		if it.G != nil && it.G.Rarity == Unique {
			t.Fatal("no world uniques on the upper floors")
		}
	}
	g.Items = nil
	g.Depth = UniqueWorldDepth
	got := map[string]int{}
	for i := 0; i < 2000; i++ {
		g.dropGear(h, 41, 25, Common)
	}
	for _, it := range g.Items {
		if it.G != nil && it.G.Rarity == Unique {
			got[it.G.Name]++
			if it.G.Slot == "weapon" && it.G.Base != "kris" {
				t.Fatalf("a rogue only finds the rogue's weapon: %s", it.G.Name)
			}
		}
	}
	if len(got) != 3 {
		t.Fatalf("the kris and both trinkets should drop: %v", got)
	}
	for n, c := range got {
		if c != 1 {
			t.Fatalf("a hero never gets a second %s while the first is still around (%d)", n, c)
		}
	}
}

func TestExecuteAndGold(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Rogue, 40, 25)
	for _, u := range Uniques {
		if u.ID == "whisper" {
			h.wear("weapon", u.make())
		}
		if u.ID == "coin" {
			h.wear("trinket", u.make())
		}
	}
	m := g.addMon('h', 42, 25, 1)
	m.HP, m.HPMax = 1000, 1000
	g.damageMonster(m, 100, h.ID, true)
	if m.HP != 900 {
		t.Fatalf("a healthy monster takes plain damage: %d", m.HP)
	}
	m.HP = 300
	g.damageMonster(m, 100, h.ID, true)
	if m.HP != 150 {
		t.Fatalf("+50%% below %d%% HP: %d", ExecuteBelow, m.HP)
	}
	g.Items = append(g.Items, &Item{ID: g.newID(), Kind: '$', X: h.X, Y: h.Y, Amount: 20})
	gold := h.Gold
	g.pickup(h)
	if h.Gold-gold != 30 {
		t.Fatalf("+50%% gold: got %d", h.Gold-gold)
	}
}
