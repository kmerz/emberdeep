package game

import "fmt"

var sprintf = fmt.Sprintf

// Story, gods and vaults.
//
// Inscriptions: every floor has one; walking into it shows a page of Emberdeep's story.
// Gods: each class has a patron among the Four. At TrialLevels a hero's god sets a trial before they can level
// up: slay TrialKills foes, then pray at an altar (one on every floor below the first, and in the village). XP
// earned meanwhile is kept, so passing the trial grants every level it held back.
// Vaults: from floor 2, a dead-end room is locked. An elite carries the key (or it lies somewhere on the floor);
// walking into the door with the key opens it, and a vault chest waits inside.

const (
	TLocked     = 'L' // a locked vault door (solid until opened)
	VaultGold   = 120 // gold in a vault chest, on top of a normal chest's
	VaultRolls  = 3   // item rolls per hero in a vault chest (a chest has ChestRolls)
	AltarName   = "Altar of the Four"
	LoreName    = "Inscription"
	KeyItem     = "key"
	lockedEvery = 3 * TickHz // "the door is locked" at most this often per hero
)

// TrialLevels: reaching the next level from one of these needs the god's blessing.
var TrialLevels = map[int]int{4: 10, 7: 18, 10: 26} // level: foes to slay

type God struct {
	Name                 string
	Demand, Ready, Bless string // Demand has a %d for the number of foes
}

var Gods = map[ClassID]God{
	Warrior: {Name: "Korrath the Anvil", Demand: "Strength is a debt. Pay it: slay %d foes.",
		Ready: "Enough. Come, kneel at my altar.", Bless: "Rise, harder than before."},
	Rogue: {Name: "Vess Who Walks Unseen", Demand: "Every shadow has a price. Bring me %d.",
		Ready: "Quietly now. Find an altar.", Bless: "You were never here. Neither was I."},
	Sorcerer: {Name: "Ilun of the Burning Eye", Demand: "Power must be fed. Burn %d of them.",
		Ready: "The flame is fed. Kneel, and see.", Bless: "Open your eyes. Wider."},
	Monk: {Name: "Sabe the Still Water", Demand: "Stillness is earned in motion. Lay %d to rest.",
		Ready: "Now be still. An altar waits.", Bless: "The water is calm. Go deeper."},
}

// Lore: one page per floor (0 = the gnome village).
var Lore = map[int]string{
	1: "Carved into a pew: \"Ten thousand candles, and Brother Aldo lights every one. He says the deep is only " +
		"sleeping. He says the Four will wake too, if it does.\"",
	2: "A page in a fine, greasy hand: \"The town's candles burn too fast. The dead do not need their fat. Who is " +
		"to say what a candle is made of? Mine will burn forever.\" It is signed with a crown drawn in wax.",
	3: "Scratched into a coffin lid: \"The water came the night the candles went black. We prayed to the Four: " +
		"Korrath, Vess, Ilun and Sabe. Only the water answered.\"",
	4: "Written on a broken shield: \"The ants don't hate us. They are hungry, and the Queen is listening for " +
		"something far below. Don't hit the workers. I mean it. Don't.\"",
	5: "A torn diary: \"Day nine. The King's hall is behind us. Merrin swears the altars hum when we fight. If " +
		"the gods are watching, why don't they help? Maybe they want to see if we're worth it.\"",
	0: "A sampler stitched above a door: \"Spoonhollow, dug by hand (and spoon). When the people above forgot " +
		"the Four, the Four asked us to keep their altars lit. We did. Mostly. Wipe your feet.\"",
	6: "Glowing letters grown into the fungus: \"The gods did not leave. They are tired. Every trial you pass " +
		"gives back a little of what Ashfall took from them.\"",
	7: "Felt, not seen, cut deep into the rock: \"Here the light gets lost. So did we. If you can read this, " +
		"your lantern is still burning. Do not stop.\"",
	8: "On the gate itself: \"What smiles at you here was once a priest. The dragon pays well for those who " +
		"forget their gods.\"",
	9: "Burned into the stone: \"The dragon is the fire under Ashfall, and the fire is the dragon. Put it out " +
		"and the town will be cold before it thanks you. Do it anyway.\"",
	10: "The last page, in the same greasy hand as before: \"I only wanted the candles to last. Tell them I am " +
		"sorry. Tell them the fire was always going to wake.\"",
}

// ---- trials ----

// gated: the hero stands at a trial level their god has not yet blessed.
func (h *Hero) gated() bool {
	_, ok := TrialLevels[h.Lvl]
	return ok && h.Blessed < h.Lvl
}

// TrialNeed is how many foes the hero's current trial asks for (0 = no trial).
func (h *Hero) TrialNeed() int {
	if !h.TrialOn {
		return 0
	}
	return TrialLevels[h.Lvl]
}

// levelUps grants every level the hero's XP pays for, stopping at an unblessed trial level (and starting the
// trial there).
func (g *Game) levelUps(h *Hero) {
	for h.XP >= XPNeed(h.Lvl) {
		if h.gated() {
			if !h.TrialOn {
				h.TrialOn, h.TrialKills = true, 0
				god := Gods[h.Cls]
				g.emit(Event{K: "trial", Actor: h.ID, X: h.X, Y: h.Y, FX: god.Name, Amount: TrialLevels[h.Lvl],
					Text: sprintf(god.Demand, TrialLevels[h.Lvl])})
			}
			return
		}
		h.XP -= XPNeed(h.Lvl)
		h.Lvl++
		h.Points++
		inc := HPPerLevel(h.Def().HP)
		h.HPMax += inc
		if !h.Downed {
			h.HP = min(h.HPMax, h.HP+inc+h.HPMax/4)
		}
		g.emit(Event{K: "levelup", Actor: h.ID, X: h.X, Y: h.Y, Amount: h.Lvl})
	}
}

// trialKill counts a kill toward every running trial.
func (g *Game) trialKill() {
	for _, h := range g.Heroes {
		if !h.TrialOn || h.TrialKills >= h.TrialNeed() {
			continue
		}
		h.TrialKills++
		if h.TrialKills == h.TrialNeed() {
			god := Gods[h.Cls]
			g.emit(Event{K: "trialready", Actor: h.ID, X: h.X, Y: h.Y, FX: god.Name, Text: god.Ready})
		}
	}
}

// TrialDone: the foes are slain; only the prayer is missing.
func (h *Hero) TrialDone() bool { return h.TrialOn && h.TrialKills >= h.TrialNeed() }

// Bless ends a finished trial: the god grants the held-back levels and heals the hero.
func (g *Game) Bless(h *Hero) {
	if !h.TrialDone() {
		return
	}
	god := Gods[h.Cls]
	h.Blessed, h.TrialOn, h.TrialKills = h.Lvl, false, 0
	if !h.Downed && !h.Dead {
		h.HP = h.HPMax
	}
	g.emit(Event{K: "blessed", Actor: h.ID, X: h.X, Y: h.Y, FX: god.Name, Text: god.Bless})
	g.levelUps(h)
}

// pray: a hero walks into an altar.
func (g *Game) pray(h *Hero, n *Npc) {
	god := Gods[h.Cls]
	switch {
	case h.TrialDone():
		g.Bless(h)
	case h.TrialOn:
		g.emit(Event{K: "altar", Actor: h.ID, Target: n.ID, X: n.X, Y: n.Y, FX: god.Name,
			Text: sprintf("%s is waiting: %d more foes.", god.Name, h.TrialNeed()-h.TrialKills)})
	default:
		g.emit(Event{K: "altar", Actor: h.ID, Target: n.ID, X: n.X, Y: n.Y, FX: god.Name,
			Text: "The altar is warm. " + god.Name + " watches you, and asks nothing yet."})
	}
}

// ---- floor setup: inscription, altar, vault ----

// setupStory places the floor's inscription, altar and vault. It uses its own RNG so the fight's dice don't change.
func (g *Game) setupStory(memo *floorMemo) {
	l := g.Level
	rng := NewRNG(uint64(FloorSeed(g.Seed, g.Depth))*13 + 7)
	if g.InVillage {
		rng = NewRNG(uint64(g.Seed)*17 + 3)
	}
	if !g.InVillage && g.Depth >= 2 {
		g.setupVault(memo, rng) // first: the inscription and the altar stay out of it
	}
	rooms := g.storyRooms(rng)
	key := g.memoKey()
	if _, ok := Lore[key]; ok {
		g.placeNpcIn(&Npc{Name: LoreName, Lore: key + 1}, rooms, 0)
	}
	if g.InVillage || g.Depth >= 2 {
		g.placeNpcIn(&Npc{Name: AltarName, Altar: true}, rooms, 1)
	}
	if !g.InVillage && g.Depth >= 2 && rng.Chance(ShrineChance) {
		def := ShrineOrder[rng.Intn(len(ShrineOrder))]
		g.placeNpcIn(&Npc{Name: def.Name, Shrine: def.ID, Used: memo.Shrine}, rooms, 2)
	}
	_ = l
}

// inVault: the cell is inside the vault room or its walls.
func (g *Game) inVault(x, y int) bool {
	r := g.Vault
	return r != nil && x >= r.X0-1 && x <= r.X1+1 && y >= r.Y0-1 && y <= r.Y1+1
}

// storyRooms: rooms away from the start, in a shuffled order (the village: around the plaza).
func (g *Game) storyRooms(rng *RNG) []Pt {
	l := g.Level
	var out []Pt
	for i, r := range l.Rooms {
		if (i == l.StartRm && !g.InVillage) || g.inVault(r.Center().X, r.Center().Y) {
			continue
		}
		out = append(out, r.Center())
	}
	for i := len(out) - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		out[i], out[j] = out[j], out[i]
	}
	out = append(out, Pt{l.Start.X + 4, l.Start.Y}, Pt{l.Stairs.X - 4, l.Stairs.Y}) // fallbacks for tight caves
	return out
}

// placeNpcIn puts n near one of the spots, trying them from index first on (an open spot first, then any).
func (g *Game) placeNpcIn(n *Npc, spots []Pt, first int) {
	l := g.Level
	free := func(x, y int) bool {
		t := l.At(x, y)
		return t != TStairs && t != TUp && t != TDoor && t != TWater && g.occ[y*LW+x] == 0 && !g.inVault(x, y)
	}
	for _, open := range []bool{true, false} {
		for i := range spots {
			near := spots[(first+i)%len(spots)]
			if p, ok := l.nearestFree(near, func(x, y int) bool { return free(x, y) && (!open || l.openAround(x, y)) }); ok {
				g.placeNpcAt(n, p)
				return
			}
		}
	}
}

func (g *Game) placeNpcAt(n *Npc, p Pt) {
	n.ID, n.X, n.Y, n.Home = g.newID(), p.X, p.Y, p
	g.Npcs = append(g.Npcs, n)
	g.npcByID[n.ID] = n
	g.occ[p.Y*LW+p.X] = int32(n.ID)
}

// vaultRoom finds a room with a single way in that holds nothing the party must reach.
func (g *Game) vaultRoom(rng *RNG) (Rect, Pt, bool) {
	l := g.Level
	var cands []int
	var doors []Pt
	for i, r := range l.Rooms {
		if i == l.StartRm || i == l.BossRoom {
			continue
		}
		in := func(p Pt) bool { return p.X >= r.X0-1 && p.X <= r.X1+1 && p.Y >= r.Y0-1 && p.Y <= r.Y1+1 }
		if in(l.Start) || in(l.Stairs) || (l.HasUp && in(l.Up)) {
			continue
		}
		var open []Pt
		for x := r.X0 - 1; x <= r.X1+1; x++ {
			for _, y := range []int{r.Y0 - 1, r.Y1 + 1} {
				if !Solid(l.At(x, y)) {
					open = append(open, Pt{x, y})
				}
			}
		}
		for y := r.Y0; y <= r.Y1; y++ {
			for _, x := range []int{r.X0 - 1, r.X1 + 1} {
				if !Solid(l.At(x, y)) {
					open = append(open, Pt{x, y})
				}
			}
		}
		if len(open) == 1 {
			cands = append(cands, i)
			doors = append(doors, open[0])
		}
	}
	if len(cands) == 0 {
		return Rect{}, Pt{}, false
	}
	k := rng.Intn(len(cands))
	return l.Rooms[cands[k]], doors[k], true
}

func (g *Game) setupVault(memo *floorMemo, rng *RNG) {
	r, door, ok := g.vaultRoom(rng)
	if !ok {
		return
	}
	g.Vault, g.VaultDoor = &r, door
	if !memo.Vault {
		g.Level.set(door.X, door.Y, TLocked)
	}
	if !memo.VaultLooted {
		c := r.Center()
		if p, ok := g.Level.nearestFree(c, func(x, y int) bool {
			return x >= r.X0 && x <= r.X1 && y >= r.Y0 && y <= r.Y1 && g.occ[y*LW+x] == 0
		}); ok {
			ch := &Chest{ID: g.newID(), X: p.X, Y: p.Y, Vault: true}
			g.Chests = append(g.Chests, ch)
			g.chestByID[ch.ID] = ch
			g.occ[p.Y*LW+p.X] = int32(ch.ID)
		}
	}
	if memo.Vault {
		return
	}
	// the key: an elite outside the vault carries it, or it lies in a far room
	inVault := func(x, y int) bool { return x >= r.X0 && x <= r.X1 && y >= r.Y0 && y <= r.Y1 }
	for _, m := range g.Mons {
		if m.Elite && m.Spawn > 0 && !inVault(m.X, m.Y) {
			m.HasKey = true
			return
		}
	}
	rooms := g.storyRooms(rng)
	for _, c := range rooms {
		if inVault(c.X, c.Y) {
			continue
		}
		if p, ok := g.Level.nearestFree(c, func(x, y int) bool { return g.occ[y*LW+x] == 0 && !inVault(x, y) }); ok {
			g.Items = append(g.Items, &Item{ID: g.newID(), Kind: Items[KeyItem].Glyph, Def: KeyItem, X: p.X, Y: p.Y, Amount: 1})
			return
		}
	}
}

// bumpLocked: walking into the vault door opens it with the floor's key.
func (g *Game) bumpLocked(h *Hero, x, y int) bool {
	if g.Level.At(x, y) != TLocked {
		return false
	}
	slot := h.firstSlot(KeyItem)
	if slot < 0 {
		if g.Now-h.lockedAt >= lockedEvery || h.lockedAt == 0 {
			h.lockedAt = g.Now
			g.emit(Event{K: "locked", Actor: h.ID, X: x, Y: y})
		}
		return true
	}
	h.removeAt(slot, 1)
	g.Level.set(x, y, TDoor)
	g.memo().Vault = true
	g.explNew = append(g.explNew, TileUpdate{x, y, TDoor})
	g.emit(Event{K: "unlock", Actor: h.ID, X: x, Y: y})
	return true
}

// vaultLoot: a vault chest spills more than a chest.
func (g *Game) vaultLoot(x, y int) {
	g.memo().VaultLooted = true
	g.Items = append(g.Items, &Item{ID: g.newID(), Kind: '$', X: x, Y: y, Amount: VaultGold})
	for i := 0; i < VaultRolls; i++ {
		g.chestLoot(x, y)
	}
	for _, h := range g.Heroes {
		if !h.Dead && h.Connected {
			g.Items = append(g.Items, &Item{ID: g.newID(), Kind: Items["ring"].Glyph, Def: "ring", X: x, Y: y, Amount: 1, Owner: h.ID})
			g.dropGear(h, x, y, Rare)
		}
	}
}

// dropKey: the elite carrying the vault key drops it where it fell.
func (g *Game) dropKey(m *Monster) {
	g.Items = append(g.Items, &Item{ID: g.newID(), Kind: Items[KeyItem].Glyph, Def: KeyItem, X: m.X, Y: m.Y, Amount: 1})
	g.emit(Event{K: "keydrop", Actor: m.ID, X: m.X, Y: m.Y, Text: m.Def().Name})
}
