package game

import "fmt"

// Bramwick's forge in the village works on the gear a hero wears (bought with IntBuy):
//   - "temper0" / "temper1": the weapon / trinket goes up one rarity and gains a bonus (common → magic → rare)
//   - "reforge0" / "reforge1": the bonuses of a magic or rare piece are rolled again
// Uniques are finished work: he won't touch them.

// ReforgeDiv: a reforge costs the piece's value divided by this.
const ReforgeDiv = 2

// TemperPrice is what tempering gr costs: what the piece is worth once it is tempered.
func TemperPrice(gr *Gear) int {
	return (GearValueBase + GearValueLvl*gr.Lvl) * GearValueMult[gr.Rarity+1]
}

// ReforgePrice is what reforging gr costs.
func ReforgePrice(gr *Gear) int { return max(1, gr.Value()/ReforgeDiv) }

func (g *Game) forge(h *Hero, id string) error {
	if !g.nextToSmith(h) {
		return ErrNotHere
	}
	var kind string
	var slot int
	if _, err := fmt.Sscanf(id, "temper%d", &slot); err == nil {
		kind = "temper"
	} else if _, err := fmt.Sscanf(id, "reforge%d", &slot); err == nil {
		kind = "reforge"
	} else {
		return ErrNoItem
	}
	name := "weapon"
	gr := h.Weapon
	if slot == 1 {
		name, gr = "trinket", h.Trinket
	} else if slot != 0 {
		return ErrNoItem
	}
	if gr == nil {
		return ErrNoItem
	}
	if gr.Rarity >= Unique || (kind == "temper" && gr.Rarity >= Rare) || (kind == "reforge" && gr.Rarity == Common) {
		return ErrBadIntent
	}
	price := TemperPrice(gr)
	if kind == "reforge" {
		price = ReforgePrice(gr)
	}
	if h.Gold < price {
		return ErrNoGold
	}
	// gear never changes once rolled (stacks and saves may share it): make a new piece
	ng := &Gear{Slot: gr.Slot, Base: gr.Base, Rarity: gr.Rarity, Lvl: gr.Lvl, Mods: []Mod{gr.Mods[0]}}
	if kind == "temper" {
		ng.Mods = append(ng.Mods, gr.Mods[1:]...)
		ng.Rarity++
		g.addBonuses(ng, 1)
	} else {
		g.addBonuses(ng, ng.Rarity)
	}
	ng.Name = gearName(ng)
	h.wear(name, ng)
	h.Gold -= price
	g.emit(Event{K: "forge", Actor: h.ID, X: h.X, Y: h.Y, FX: kind, Amount: -price, Text: ng.Name, TX: ng.Rarity})
	return nil
}
