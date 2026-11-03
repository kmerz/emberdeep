package game

// Shrines: from floor 2 on, a floor may hold a shrine. Walking into it gives the whole party a short blessing,
// or a gamble; then it goes dark for good (the floor remembers).

const (
	ShrineChance   = 0.6           // chance a floor below the first has a shrine
	ShrineTicks    = 45 * TickHz   // how long a blessing lasts
	HasteMoveTicks = MoveTicks - 1 // a hasted hero's step
	FuryPct        = 30            // Shrine of Fury: % more damage
	BloodLeech     = 12            // Shrine of Blood: % of damage healed (on top of gear)
	FortuneMult    = 2             // Shrine of Fortune: gold picked up on this floor counts this many times
	ChanceRare     = 0.4           // Shrine of Chance: a rare for everyone...
	ChanceHeal     = 0.35          // ...or the party healed in full; otherwise an ambush
	AmbushSize     = 4             // monsters in a Shrine of Chance ambush (plus one per extra hero)
)

type ShrineDef struct {
	ID, Name, Text string
}

// ShrineOrder is also the roll order.
var ShrineOrder = []ShrineDef{
	{"haste", "Shrine of Haste", "Your feet feel light."},
	{"fury", "Shrine of Fury", "Red mist. Something wants you to hit things."},
	{"blood", "Shrine of Blood", "Your wounds drink from your enemies."},
	{"fortune", "Shrine of Fortune", "Gold on this floor is worth double."},
	{"chance", "Shrine of Chance", "The shrine rolls its dice..."},
}

func shrineDef(id string) *ShrineDef {
	for i := range ShrineOrder {
		if ShrineOrder[i].ID == id {
			return &ShrineOrder[i]
		}
	}
	return nil
}

// useShrine: a hero walks into a shrine.
func (g *Game) useShrine(h *Hero, n *Npc) {
	if n.Used {
		g.emit(Event{K: "shrine", Actor: h.ID, Target: n.ID, X: n.X, Y: n.Y, Text: "The shrine is cold and dark."})
		return
	}
	n.Used = true
	g.memo().Shrine = true
	def := shrineDef(n.Shrine)
	text := def.Text
	switch def.ID {
	case "fortune":
		g.memo().Fortune = true
	case "chance":
		switch r := g.rng.Float(); {
		case r < ChanceRare:
			text = "The dice land well: a gift for everyone."
			for _, o := range g.Heroes {
				if !o.Dead && o.Connected {
					g.dropGear(o, h.X, h.Y, Rare)
				}
			}
		case r < ChanceRare+ChanceHeal:
			text = "The dice land well: your wounds close."
			for _, o := range g.Heroes {
				if !o.Dead && !o.Downed {
					o.HP = o.HPMax
				}
			}
		default:
			text = "The dice land badly. Something answers."
			g.ambush(n)
		}
	default:
		for _, o := range g.Heroes {
			if !o.Dead && o.Connected {
				o.Shrine, o.ShrineUntil, o.shrineOn = def.ID, g.Now+ShrineTicks, def.ID
			}
		}
	}
	g.emit(Event{K: "shrine", Actor: h.ID, Target: n.ID, X: n.X, Y: n.Y, FX: def.ID, Text: def.Name + ": " + text})
}

// ambush: monsters of the floor's kinds climb out around the shrine.
func (g *Game) ambush(n *Npc) {
	kinds := Floor(g.Depth).Kinds
	if kinds == "" {
		kinds = "zs"
	}
	for i := 0; i < PackSize(AmbushSize, g.N); i++ {
		k := kinds[g.rng.Intn(len(kinds))]
		if MonDefs[k].Neutral { // ants that ignore you are no ambush
			k = 'A'
		}
		p, ok := g.freeNear(n.X, n.Y)
		if !ok {
			return
		}
		g.addMon(k, p.X, p.Y, 1)
	}
}
