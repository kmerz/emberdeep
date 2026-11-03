package game

import (
	"fmt"
	"math"
)

// Gear: every hero has a weapon slot and a trinket slot. A piece of gear is rolled when it drops: a base
// (picked for the class of the hero it drops for), an item level (the floor), a rarity, and 0-2 bonuses.
// In the rucksack and on the floor it rides along on a Stack or an Item (field G).

const (
	Common = iota
	Magic
	Rare
)

// Gear is one rolled weapon or trinket. It never changes after it is rolled, so stacks and saves may share it.
type Gear struct {
	Slot   string // "weapon" or "trinket"
	Base   string // a GearBases id
	Name   string
	Rarity int
	Lvl    int
	Mods   []Mod  // the base stat first, then the bonuses
	Lore   string `json:",omitempty"` // uniques: a line of flavour
}

type Mod struct {
	K string // a GearStats key
	V int
}

// Stat totals what the gear gives for one stat.
func (g *Gear) Stat(k string) int {
	if g == nil {
		return 0
	}
	n := 0
	for _, m := range g.Mods {
		if m.K == k {
			n += m.V
		}
	}
	return n
}

// Value is what Grimble pays for it.
func (g *Gear) Value() int {
	return (GearValueBase + GearValueLvl*g.Lvl) * GearValueMult[g.Rarity]
}

// GearStat is one stat a piece of gear can raise. Its value at item level L is (Base + PerLvl·L), times a roll
// between GearRollMin and 1.
type GearStat struct {
	Prefix, Suffix string  // name parts when it is a bonus
	Base, PerLvl   float64 // roll size
}

var GearStats = map[string]GearStat{
	"dmg":   {"Vicious", "of Slaughter", 5, 1.5},  // % damage
	"crit":  {"Keen", "of Precision", 3, 0.7},     // % chance to hit twice as hard
	"leech": {"Leeching", "of the Leech", 2, 0.3}, // % of damage dealt healed
	"cdr":   {"Quick", "of Haste", 3, 0.7},        // % shorter skill cooldowns
	"res":   {"Fervent", "of Zeal", 8, 1.5},       // % more resource gained (and regenerated)
	"armor": {"Warded", "of Iron", 3, 0.8},        // % less damage taken
	"hp":    {"Sturdy", "of the Bear", 8, 4},      // max HP
	// only on uniques (never in a bonus pool); the sizes are for comparing gear
	"regen":   {"Mending", "of Mending", 1, 0.2}, // HP per second
	"thorns":  {"Thorned", "of Thorns", 10, 2},   // % of damage taken dealt back
	"slayer":  {"Slaying", "of the Hunt", 10, 2}, // % more damage to bosses and hunt targets
	"lantern": {"Bright", "of Light", 1, 0},      // a lantern that never runs out
}

// Caps on the hero's totals.
var GearCaps = map[string]int{"crit": 40, "leech": 15, "cdr": 35, "armor": 50}

// GearBase is a kind of weapon or trinket. Its own stat is always on it; a weapon's is damage.
type GearBase struct {
	ID, Name, Slot string
	Stat           string
	Base, PerLvl   float64
	Class          ClassID // weapons drop for this class ("" = trinkets: anyone)
	Bonuses        []string
}

var weaponBonuses = []string{"dmg", "crit", "leech", "cdr", "res"}
var trinketBonuses = []string{"hp", "armor", "leech", "cdr", "res", "crit"}

var GearBases = map[string]*GearBase{
	"axe":      {ID: "axe", Name: "Axe", Slot: "weapon", Stat: "dmg", Base: 8, PerLvl: 2.5, Class: Warrior, Bonuses: weaponBonuses},
	"sword":    {ID: "sword", Name: "Sword", Slot: "weapon", Stat: "dmg", Base: 8, PerLvl: 2.5, Class: Warrior, Bonuses: weaponBonuses},
	"dagger":   {ID: "dagger", Name: "Dagger", Slot: "weapon", Stat: "dmg", Base: 8, PerLvl: 2.5, Class: Rogue, Bonuses: weaponBonuses},
	"kris":     {ID: "kris", Name: "Kris", Slot: "weapon", Stat: "dmg", Base: 8, PerLvl: 2.5, Class: Rogue, Bonuses: weaponBonuses},
	"wand":     {ID: "wand", Name: "Wand", Slot: "weapon", Stat: "dmg", Base: 8, PerLvl: 2.5, Class: Sorcerer, Bonuses: weaponBonuses},
	"rod":      {ID: "rod", Name: "Rod", Slot: "weapon", Stat: "dmg", Base: 8, PerLvl: 2.5, Class: Sorcerer, Bonuses: weaponBonuses},
	"staff":    {ID: "staff", Name: "Staff", Slot: "weapon", Stat: "dmg", Base: 8, PerLvl: 2.5, Class: Monk, Bonuses: weaponBonuses},
	"knuckles": {ID: "knuckles", Name: "Knuckles", Slot: "weapon", Stat: "dmg", Base: 8, PerLvl: 2.5, Class: Monk, Bonuses: weaponBonuses},
	"amulet":   {ID: "amulet", Name: "Amulet", Slot: "trinket", Stat: "hp", Base: 6, PerLvl: 3, Bonuses: trinketBonuses},
	"charm":    {ID: "charm", Name: "Charm", Slot: "trinket", Stat: "armor", Base: 2, PerLvl: 0.6, Bonuses: trinketBonuses},
	"talisman": {ID: "talisman", Name: "Talisman", Slot: "trinket", Stat: "res", Base: 5, PerLvl: 1, Bonuses: trinketBonuses},
}

// GearBaseOrder fixes the roll order (map order is random, the simulation must not be).
var GearBaseOrder = []string{"axe", "sword", "dagger", "kris", "wand", "rod", "staff", "knuckles", "amulet", "charm", "talisman"}

// GearMaterial names a weapon's metal by item level.
var GearMaterial = []string{"Rusty", "Rusty", "Iron", "Iron", "Steel", "Steel", "Runed", "Runed", "Emberforged", "Emberforged"}

const (
	GearRollMin     = 0.6
	GearKillChance  = 0.025 // per hero, on every kill (not summoned minions)
	GearChestChance = 0.35  // per hero, when a chest opens
	GearTrinketOdds = 0.35  // share of drops that are trinkets
	GearRareBase    = 0.08  // chance of a rare, plus GearRareDepth per floor
	GearRareDepth   = 0.012
	GearMagicChance = 0.35 // of what is not rare
	GearValueBase   = 10
	GearValueLvl    = 4
)

var GearValueMult = [4]int{1, 3, 8, 15}

// RarityName is shown in descriptions.
var RarityName = [4]string{"common", "magic", "rare", "unique"}

// rollGear makes a new piece of gear for a hero of class cls, at item level lvl, of at least rarity minRar.
func (g *Game) rollGear(cls ClassID, lvl, minRar int) *Gear {
	lvl = min(max(lvl, 1), MaxDepth)
	var bases []*GearBase
	trinket := g.rng.Chance(GearTrinketOdds)
	for _, id := range GearBaseOrder {
		b := GearBases[id]
		if (trinket && b.Slot == "trinket") || (!trinket && b.Class == cls) {
			bases = append(bases, b)
		}
	}
	b := bases[g.rng.Intn(len(bases))]
	rar := Common
	if g.rng.Chance(GearRareBase + GearRareDepth*float64(lvl)) {
		rar = Rare
	} else if g.rng.Chance(GearMagicChance) {
		rar = Magic
	}
	rar = max(rar, minRar)
	roll := func(base, per float64) int {
		v := (base + per*float64(lvl)) * (GearRollMin + (1-GearRollMin)*g.rng.Float())
		return max(1, int(math.Round(v)))
	}
	gr := &Gear{Slot: b.Slot, Base: b.ID, Rarity: rar, Lvl: lvl, Mods: []Mod{{b.Stat, roll(b.Base, b.PerLvl)}}}
	pool := append([]string(nil), b.Bonuses...)
	for i := 0; i < rar; i++ {
		j := g.rng.Intn(len(pool))
		k := pool[j]
		pool = append(pool[:j], pool[j+1:]...)
		st := GearStats[k]
		gr.Mods = append(gr.Mods, Mod{k, roll(st.Base, st.PerLvl)})
	}
	gr.Name = gearName(gr)
	return gr
}

// gearName: "Iron Axe", "Keen Iron Axe", "Keen Iron Axe of Haste"; trinkets have no metal.
func gearName(gr *Gear) string {
	b := GearBases[gr.Base]
	name := b.Name
	if b.Slot == "weapon" {
		name = GearMaterial[gr.Lvl-1] + " " + name
	}
	if len(gr.Mods) > 1 {
		name = GearStats[gr.Mods[1].K].Prefix + " " + name
	}
	if len(gr.Mods) > 2 {
		name += " " + GearStats[gr.Mods[2].K].Suffix
	}
	return name
}

// dropGear puts a new piece of gear for h on the floor at x, y.
func (g *Game) dropGear(h *Hero, x, y, minRar int) {
	gr := g.rollGear(h.Cls, g.Depth, minRar)
	g.Items = append(g.Items, &Item{ID: g.newID(), Kind: Items[gr.Slot].Glyph, Def: gr.Slot, X: x, Y: y, Amount: 1, Owner: h.ID, G: gr})
	if gr.Rarity >= Rare {
		g.emit(Event{K: "raredrop", Actor: h.ID, X: x, Y: y, Text: gr.Name, Amount: gr.Rarity})
	}
}

// GearStat is the hero's total for a stat from everything worn, capped.
func (h *Hero) GearStat(k string) int {
	n := h.Weapon.Stat(k) + h.Trinket.Stat(k)
	if c, ok := GearCaps[k]; ok {
		n = min(n, c)
	}
	return n
}

// addGear puts a piece of gear in the first empty rucksack slot. It reports whether there was room.
func (h *Hero) addGear(gr *Gear) bool {
	for i := range h.Pack[:h.PackCap] {
		if h.Pack[i].N == 0 {
			h.Pack[i] = Stack{Item: gr.Slot, N: 1, G: gr}
			return true
		}
	}
	return false
}

// wear puts gr on (nil takes the slot off) and returns what was worn before. Max HP follows the gear.
func (h *Hero) wear(slot string, gr *Gear) *Gear {
	p := &h.Weapon
	if slot == "trinket" {
		p = &h.Trinket
	}
	old := *p
	d := gr.Stat("hp") - old.Stat("hp")
	*p = gr
	h.HPMax += d
	h.HP = max(1, min(h.HPMax, h.HP+max(d, 0)))
	return old
}

// equip wears the gear in a rucksack slot; whatever was worn goes into that slot.
func (g *Game) equip(h *Hero, slot int) error {
	s := h.Pack[slot]
	if s.G == nil {
		return ErrBadIntent
	}
	old := h.wear(s.G.Slot, s.G)
	h.Pack[slot] = Stack{}
	if old != nil {
		h.Pack[slot] = Stack{Item: old.Slot, N: 1, G: old}
	}
	g.emit(Event{K: "equip", Actor: h.ID, X: h.X, Y: h.Y, Text: s.G.Name, FX: s.G.Slot, Amount: s.G.Rarity})
	return nil
}

// unequip takes off the weapon (slot 0) or trinket (slot 1) and puts it in the rucksack.
func (g *Game) unequip(h *Hero, slot int) error {
	name := "weapon"
	if slot == 1 {
		name = "trinket"
	} else if slot != 0 {
		return ErrBadIntent
	}
	gr := h.Weapon
	if slot == 1 {
		gr = h.Trinket
	}
	if gr == nil {
		return ErrNoItem
	}
	if !h.addGear(gr) {
		return ErrPackFull
	}
	h.wear(name, nil)
	g.emit(Event{K: "unequip", Actor: h.ID, X: h.X, Y: h.Y, Text: gr.Name, FX: name, Amount: gr.Rarity})
	return nil
}

// pickGear picks up a piece of gear: straight onto the hero if that slot is empty, else into the rucksack.
func (g *Game) pickGear(h *Hero, it *Item) bool {
	worn := h.Weapon
	if it.G.Slot == "trinket" {
		worn = h.Trinket
	}
	if worn == nil {
		h.wear(it.G.Slot, it.G)
		g.emit(Event{K: "equip", Actor: h.ID, X: h.X, Y: h.Y, Text: it.G.Name, FX: it.G.Slot, Amount: it.G.Rarity})
		return true
	}
	if !h.addGear(it.G) {
		return false
	}
	g.emit(Event{K: "gearpick", Actor: h.ID, X: h.X, Y: h.Y, Amount: it.G.Rarity, FX: it.Def, Text: it.G.Name})
	return true
}

// gearScore is a rough worth for comparing gear (bots use it to pick the better piece).
func gearScore(gr *Gear) float64 {
	if gr == nil {
		return 0
	}
	s := 0.0
	for _, m := range gr.Mods {
		st := GearStats[m.K]
		s += float64(m.V) / (st.Base + st.PerLvl*5)
	}
	return s
}

// EquipBetter wears any rucksack gear that beats what is worn (for bots). It reports whether it changed anything.
func (g *Game) EquipBetter(h *Hero) bool {
	for i, s := range h.Pack {
		if s.G == nil {
			continue
		}
		worn := h.Weapon
		if s.G.Slot == "trinket" {
			worn = h.Trinket
		}
		if gearScore(s.G) > gearScore(worn)*1.05 {
			return g.equip(h, i) == nil
		}
	}
	return false
}

// critRoll: does a hit from h land as a crit? Gear crit chance only; skills with their own crits pass crit in.
func (g *Game) critRoll(h *Hero) bool {
	c := h.GearStat("crit")
	return c > 0 && g.rng.Intn(100) < c
}

// leech heals the hero by its life steal share of damage dealt.
func (h *Hero) leech(dmg int) {
	l := h.GearStat("leech")
	if h.shrineOn == "blood" {
		l += BloodLeech
	}
	if l <= 0 || h.Downed || h.Dead {
		return
	}
	h.leechAcc += float64(dmg*l) / 100
	if n := int(h.leechAcc); n > 0 {
		h.leechAcc -= float64(n)
		h.HP = min(h.HPMax, h.HP+n)
	}
}

// resGain scales a resource gain by the hero's gear.
func (h *Hero) resGain(n float64) float64 {
	return n * (1 + float64(h.GearStat("res"))/100)
}

// Grimble's gear: each visit to the village he has a few rolled pieces for every hero, made for their class
// and the floor below. They are bought with IntBuy, Skill "gear0", "gear1", ...
const (
	WaresPerHero = 3
	WaresBuyMult = 2 // he sells for this many times what he pays
)

// WarePrice is what Grimble asks for a piece.
func WarePrice(gr *Gear) int { return gr.Value() * WaresBuyMult }

// stockWares rolls a hero's wares if they have none for this visit.
func (g *Game) stockWares(h *Hero) {
	if !g.InVillage || g.Wares[h.ID] != nil {
		return
	}
	lvl := g.ReturnDepth
	if lvl == 0 {
		lvl = g.Depth + 1 // the floor after the village
	}
	w := make([]*Gear, WaresPerHero)
	for i := range w {
		w[i] = g.rollGear(h.Cls, lvl, Magic)
	}
	g.Wares[h.ID] = w
}

// buyWare buys ware i ("gear0"...) of the hero's own wares. It goes on if the slot is empty, else into the rucksack.
func (g *Game) buyWare(h *Hero, id string) error {
	var i int
	if _, err := fmt.Sscanf(id, "gear%d", &i); err != nil {
		return ErrNoItem
	}
	w := g.Wares[h.ID]
	if i < 0 || i >= len(w) || w[i] == nil {
		return ErrNoItem
	}
	gr := w[i]
	price := WarePrice(gr)
	if h.Gold < price {
		return ErrNoGold
	}
	worn := h.Weapon
	if gr.Slot == "trinket" {
		worn = h.Trinket
	}
	if worn == nil {
		h.wear(gr.Slot, gr)
	} else if !h.addGear(gr) {
		return ErrPackFull
	}
	w[i] = nil
	h.Gold -= price
	g.emit(Event{K: "trade", Actor: h.ID, X: h.X, Y: h.Y, FX: gr.Slot, Amount: -price, Text: gr.Name})
	return nil
}

// Uniques: hand-made pieces, one per boss. Every hero gets one when the boss dies. They carry effects random
// gear never rolls: "regen" (HP per second), "thorns" (% of melee damage taken dealt back), "slayer" (% more
// damage to bosses and hunt targets) and "lantern" (a light that never runs out).
const Unique = Rare + 1

type UniqueDef struct {
	ID, Name, Base, Lore string
	Boss                 byte
	Lvl                  int
	Mods                 []Mod
}

var Uniques = []UniqueDef{
	{ID: "jelly", Name: "Royal Jelly Locket", Base: "amulet", Boss: 'Q', Lvl: 4, Lore: "Still warm. The hive would very much like it back.",
		Mods: []Mod{{"hp", 40}, {"regen", 2}, {"res", 15}}},
	{ID: "wick", Name: "The Tallow King's Wick", Base: "charm", Boss: 'K', Lvl: 5, Lore: "It burns without burning down. He never shared it.",
		Mods: []Mod{{"armor", 12}, {"thorns", 40}, {"lantern", 1}}},
	{ID: "heart", Name: "Heart of the Ember Dragon", Base: "talisman", Boss: FinalBoss, Lvl: 10, Lore: "It still beats, slowly, when you are angry.",
		Mods: []Mod{{"dmg", 25}, {"crit", 12}, {"slayer", 30}}},
}

// uniqueFor makes the boss's unique, or nil if it has none.
func uniqueFor(boss byte) *Gear {
	for _, u := range Uniques {
		if u.Boss == boss {
			gr := &Gear{Slot: GearBases[u.Base].Slot, Base: u.Base, Name: u.Name, Rarity: Unique, Lvl: u.Lvl, Lore: u.Lore, Mods: append([]Mod(nil), u.Mods...)}
			return gr
		}
	}
	return nil
}

// dropUnique drops the boss's unique for h, if it has one. It reports whether it did.
func (g *Game) dropUnique(h *Hero, m *Monster) bool {
	gr := uniqueFor(m.Kind)
	if gr == nil {
		return false
	}
	g.Items = append(g.Items, &Item{ID: g.newID(), Kind: Items[gr.Slot].Glyph, Def: gr.Slot, X: m.X, Y: m.Y, Amount: 1, Owner: h.ID, G: gr})
	g.emit(Event{K: "raredrop", Actor: h.ID, X: m.X, Y: m.Y, Text: gr.Name, Amount: gr.Rarity})
	return true
}

// thorn is thorns damage owed to a monster; it lands after the monsters have acted (tick.go), so a monster
// never dies in the middle of its own attack.
type thorn struct{ hero, mon, dmg int }

func (g *Game) payThorns() {
	q := g.thorns
	g.thorns = nil
	for _, t := range q {
		if m := g.monByID[t.mon]; m != nil && m.HP > 0 {
			g.damageMonster(m, t.dmg, t.hero, false)
		}
	}
}

// regenHP heals a hero with "regen" gear a little every tick.
func (h *Hero) regenHP() {
	r := h.GearStat("regen")
	if r <= 0 || h.Downed || h.Dead || h.HP >= h.HPMax {
		return
	}
	h.regenAcc += r
	if n := h.regenAcc / TickHz; n > 0 {
		h.regenAcc -= n * TickHz
		h.HP = min(h.HPMax, h.HP+n)
	}
}

// slayer is the hero's bonus against bosses and hunt targets (0 for no hero).
func (h *Hero) slayer() int {
	if h == nil {
		return 0
	}
	return h.GearStat("slayer")
}
