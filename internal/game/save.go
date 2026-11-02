package game

import "math"

// A saved game keeps the run (seed, floor, quests) and every hero by player name. Loading builds the floor
// fresh: monsters and chests come back, but heroes keep their level, gold, rucksack and skills. Heroes whose
// player is not there stay in the save for later.

// RunSave is what a save file holds.
type RunSave struct {
	Seed        uint32
	Settings    Settings
	Depth       int
	InVillage   bool
	VillageSeen bool
	ReturnDepth int
	Kills       int
	Quests      []QuestSave
	Heroes      map[string]SavedHero // by player name
}

type QuestSave struct {
	ID       string
	State    QuestState
	Progress int
}

type SavedHero struct {
	Cls  ClassID
	Hero heroSave
}

// Lvl is the saved hero's level.
func (s SavedHero) Lvl() int { return s.Hero.Lvl }

// Save returns the run as it is now. Heroes in prev that are not in the game are kept. A hero who is down
// (or the whole party after a wipe) is saved as they arrived on this floor.
func (g *Game) Save(prev map[string]SavedHero) RunSave {
	s := RunSave{Seed: g.Seed, Settings: g.Settings, Depth: g.Depth, InVillage: g.InVillage, VillageSeen: g.VillageSeen,
		ReturnDepth: g.ReturnDepth, Kills: g.Kills, Heroes: map[string]SavedHero{}}
	for name, h := range prev {
		s.Heroes[name] = h
	}
	for _, q := range g.Quests {
		s.Quests = append(s.Quests, QuestSave{ID: q.Def.ID, State: q.State, Progress: q.Progress})
	}
	for _, h := range g.Heroes {
		s.Heroes[h.Name] = g.SaveHero(h)
	}
	return s
}

// SaveHero is one hero as a save keeps them.
func (g *Game) SaveHero(h *Hero) SavedHero {
	hs := h.save()
	if a, ok := g.arrival[h.ID]; ok && (h.Dead || h.Downed || g.Phase == PhaseWiped) {
		hs = a
	}
	hs.HP = max(hs.HP, int(math.Round(float64(hs.HPMax)*FallenHPFrac)))
	return SavedHero{Cls: h.Cls, Hero: hs}
}

// Load starts a game from a save with n heroes' worth of scaling. Add the heroes with AddSavedHero or AddHero.
func Load(s RunSave, n int) *Game {
	g := &Game{Seed: s.Seed, Settings: s.Settings, N: n, stairsSince: -1}
	g.nextID = monIDBase
	g.heroByID = map[int]*Hero{}
	g.initQuests()
	for _, qs := range s.Quests {
		if q := g.quest(qs.ID); q != nil {
			q.State, q.Progress = qs.State, qs.Progress
		}
	}
	g.VillageSeen, g.ReturnDepth, g.Kills = s.VillageSeen, s.ReturnDepth, s.Kills
	depth := min(max(s.Depth, 1), MaxDepth)
	if s.InVillage {
		g.Depth = depth
		g.loadVillage()
	} else {
		g.loadFloor(depth)
	}
	return g
}

// AddSavedHero brings a saved hero back into the game.
func (g *Game) AddSavedHero(id int, name string, s SavedHero) *Hero {
	h := g.AddHero(id, name, s.Cls)
	h.restore(s.Hero)
	h.HP = min(max(h.HP, 1), h.HPMax)
	g.arrival[id] = h.save()
	return h
}

// CatchUp raises a new hero to the average level of the rest of the party, so someone joining a deep run
// is not stuck at level 1. Each level comes with its skill point and HP, as if earned.
func (g *Game) CatchUp(h *Hero) {
	sum, n := 0, 0
	for _, o := range g.Heroes {
		if o != h {
			sum += o.Lvl
			n++
		}
	}
	if n == 0 {
		return
	}
	target := sum / n
	for h.Lvl < target {
		h.Lvl++
		h.Points++
		h.HPMax += HPPerLevel(h.Def().HP)
	}
	h.HP = h.HPMax
	g.arrival[h.ID] = h.save()
}
