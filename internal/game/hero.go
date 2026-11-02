package game

import "math"

type Hero struct {
	ID     int
	Name   string
	Cls    ClassID
	Lvl    int
	XP     int
	X, Y   int
	Facing int

	HP, HPMax  int
	Res        int
	ResMax     int
	resAcc     float64
	Gold       int
	Pack       [PackSlots]Stack
	packFullAt int // last tick a "rucksack full" message was sent

	CDReady     [3]int
	PotionReady int
	NextMove    int
	LastSeq     int

	Connected bool
	Dead      bool

	Downed       bool
	DownedUntil  int
	ReviveTarget int
	ReviveProg   int

	WallUntil  int
	WallAbsorb int
	BellUntil  int
	SlowUntil  int
	BleedUntil int
	LastDamage int // tick of last damage taken (for display)

	fov      []int32
	fovX     int
	fovY     int
	fovValid bool
}

func (h *Hero) Def() *ClassDef { return Classes[h.Cls] }

func (h *Hero) save() heroSave {
	return heroSave{Lvl: h.Lvl, XP: h.XP, HP: h.HP, HPMax: h.HPMax, Res: h.Res, ResMax: h.ResMax, Gold: h.Gold, Pack: h.Pack}
}

func (h *Hero) restore(s heroSave) {
	h.Lvl, h.XP, h.HP, h.HPMax, h.Res, h.ResMax, h.Gold, h.Pack = s.Lvl, s.XP, s.HP, s.HPMax, s.Res, s.ResMax, s.Gold, s.Pack
	h.Dead, h.Downed = false, false
}

// Stack is one rucksack slot.
type Stack struct {
	Item string
	N    int
}

// AddItem puts up to n of an item into the rucksack: existing stacks first, then empty slots.
// It returns how many fit.
func (h *Hero) AddItem(id string, n int) int {
	def := Items[id]
	if def == nil || n <= 0 {
		return 0
	}
	added := 0
	for pass := 0; pass < 2 && added < n; pass++ {
		for i := range h.Pack {
			s := &h.Pack[i]
			if (pass == 0 && s.Item != id) || (pass == 1 && s.N != 0) {
				continue
			}
			k := min(n-added, def.MaxStack-s.N)
			if k <= 0 {
				continue
			}
			s.Item, s.N = id, s.N+k
			added += k
			if added == n {
				break
			}
		}
	}
	return added
}

// removeAt takes n items out of a slot.
func (h *Hero) removeAt(slot, n int) {
	s := &h.Pack[slot]
	s.N -= n
	if s.N <= 0 {
		*s = Stack{}
	}
}

// Count is how many of an item the hero carries.
func (h *Hero) Count(id string) int {
	n := 0
	for _, s := range h.Pack {
		if s.Item == id {
			n += s.N
		}
	}
	return n
}

// firstSlot is the first slot holding the item, or -1.
func (h *Hero) firstSlot(id string) int {
	for i, s := range h.Pack {
		if s.Item == id && s.N > 0 {
			return i
		}
	}
	return -1
}

// PackUsed is the number of occupied slots.
func (h *Hero) PackUsed() int {
	n := 0
	for _, s := range h.Pack {
		if s.N > 0 {
			n++
		}
	}
	return n
}

// freshOnFloor clears everything tied to the old floor: timers, statuses, revives and the vision cache.
func (h *Hero) freshOnFloor() {
	h.DownedUntil, h.ReviveTarget, h.ReviveProg = 0, 0, 0
	h.WallUntil, h.WallAbsorb, h.BellUntil, h.SlowUntil, h.BleedUntil = 0, 0, 0, 0, 0
	h.CDReady = [3]int{}
	h.PotionReady = 0
	h.fovValid = false
}

func (h *Hero) gainRes(n int) {
	h.Res = min(h.ResMax, max(0, h.Res+n))
}

// SetClass changes class (lobby only: resets resource/HP).
func (h *Hero) SetClass(c ClassID) {
	def := Classes[c]
	h.Cls = c
	h.HPMax, h.HP = def.HP+HPPerLevel(def.HP)*(h.Lvl-1), def.HP+HPPerLevel(def.HP)*(h.Lvl-1)
	h.ResMax, h.Res = def.ResMax, def.ResStart
}

func (g *Game) gainXP(amount int) {
	for _, h := range g.Heroes {
		if h.Dead {
			continue
		}
		h.XP += amount
		for h.XP >= XPNeed(h.Lvl) {
			h.XP -= XPNeed(h.Lvl)
			h.Lvl++
			inc := HPPerLevel(h.Def().HP)
			h.HPMax += inc
			if !h.Downed {
				h.HP = min(h.HPMax, h.HP+inc+h.HPMax/4)
			}
			g.emit(Event{K: "levelup", Actor: h.ID, X: h.X, Y: h.Y, Amount: h.Lvl})
		}
	}
}

// splitGold divides gold evenly between connected heroes; the remainder goes to the picker.
func (g *Game) splitGold(picker *Hero, amount int) {
	var rec []*Hero
	for _, h := range g.Heroes {
		if h.Connected && !h.Dead {
			rec = append(rec, h)
		}
	}
	if len(rec) == 0 {
		picker.Gold += amount
		return
	}
	share := amount / len(rec)
	rem := amount - share*len(rec)
	for _, h := range rec {
		h.Gold += share
	}
	picker.Gold += rem
}

// PartyGold is the total gold collected by the whole party.
func (g *Game) PartyGold() int {
	t := 0
	for _, h := range g.Heroes {
		t += h.Gold
	}
	return t
}

// damageHero applies monster (or ally) damage to a hero: Bell, Iron Wall absorb, downed transition.
func (g *Game) damageHero(h *Hero, dmg int, srcID int) {
	if h.Dead || h.Downed || !h.Connected || dmg <= 0 {
		return
	}
	if g.Now < h.BellUntil {
		dmg = int(math.Round(float64(dmg) * (1 - float64(Classes[Monk].Skills[2].Aux)/100)))
	}
	if g.Now < h.WallUntil && h.WallAbsorb > 0 {
		a := min(h.WallAbsorb, dmg)
		h.WallAbsorb -= a
		dmg -= a
		g.emit(Event{K: "absorb", Actor: srcID, Target: h.ID, X: h.X, Y: h.Y, Amount: a})
	}
	if dmg < 1 && g.Now >= h.WallUntil {
		dmg = 1
	}
	if dmg <= 0 {
		return
	}
	h.HP -= dmg
	h.LastDamage = g.Now
	if gt := h.Def().GainTaken; gt > 0 {
		h.gainRes(gt)
	}
	g.emit(Event{K: "hit", Actor: srcID, Target: h.ID, X: h.X, Y: h.Y, Amount: dmg})
	if h.HP <= 0 {
		h.HP = 0
		g.downHero(h)
	}
}

func (g *Game) downHero(h *Hero) {
	h.Downed = true
	h.DownedUntil = g.Now + DownedTicks
	h.ReviveTarget, h.ReviveProg = 0, 0
	g.emit(Event{K: "down", Target: h.ID, X: h.X, Y: h.Y})
}

// updateVision unions every hero's FOV into Visible and records newly explored tiles.
func (g *Game) updateVision() {
	clear(g.Visible)
	for _, h := range g.Heroes {
		if h.Dead {
			continue
		}
		if !h.fovValid || h.fovX != h.X || h.fovY != h.Y {
			h.fov = h.fov[:0]
			hh := h
			ComputeFOV(g.Level, h.X, h.Y, VisionRadius, func(x, y int) { hh.fov = append(hh.fov, int32(y*LW+x)) })
			h.fovX, h.fovY, h.fovValid = h.X, h.Y, true
		}
		for _, i := range h.fov {
			g.Visible[i] = true
		}
	}
	for i, v := range g.Visible {
		if v && !g.Explored[i] {
			g.Explored[i] = true
			g.explNew = append(g.explNew, TileUpdate{i % LW, i / LW, g.Level.Tiles[i]})
		}
	}
}

func (g *Game) IsVisible(x, y int) bool {
	return x >= 0 && y >= 0 && x < LW && y < LH && g.Visible[y*LW+x]
}
