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

	HP, HPMax   int
	Res         int
	ResMax      int
	resAcc      float64
	Gold        int
	Pack        [PackSlots]Stack
	PackCap     int // usable slots (StartPackCap, more with the Gnome Satchel)
	packFullAt  int // last tick a "rucksack full" message was sent
	Weapon      *Gear
	Trinket     *Gear
	leechAcc    float64
	regenAcc    int    // regen gear: HP owed, in 1/TickHz steps
	Shrine      string // the shrine blessing the hero is under (shrine.go)
	ShrineUntil int
	shrineOn    string // Shrine while it lasts, else "" (kept by tickHero)

	CDReady [TreeSize]int // per skill, so moving a skill to another key doesn't reset it
	Ranks   [TreeSize]int // 0 = not learned
	Binds   [3]int        // skill index on keys 1-3, -1 = empty
	Points  int           // unspent skill points

	CryUntil      int // War Cry damage buff
	HiddenUntil   int // Smoke Step: monsters can't target this hero
	PoisonCharges int // Poison Blade: stabs left that poison
	PoisonDmg     int
	PoisonDur     int
	buff          float64 // damage multiplier while a skill is being cast (War Cry)
	PotionReady   int
	NextMove      int
	LastSeq       int

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
	Oil        int  // ticks of light left in the lantern (it burns by itself on dark floors)
	NextStrike int  // tick the next free weapon strike is ready
	Blessed    int  // the highest trial level the hero's god has blessed
	TrialOn    bool // a god's trial is running (the hero is held at a trial level)
	TrialKills int  // foes slain for it
	lockedAt   int  // last "the door is locked" message
	CharmUntil int  // charmed by a succubus: can't act, drifts toward her
	CharmBy    int
	LastDamage int // tick of last damage taken (for display)

	fov      []int32
	fovX     int
	fovY     int
	fovValid bool
}

func (h *Hero) Def() *ClassDef { return Classes[h.Cls] }

func (h *Hero) save() heroSave {
	return heroSave{Lvl: h.Lvl, XP: h.XP, HP: h.HP, HPMax: h.HPMax, Res: h.Res, ResMax: h.ResMax, Gold: h.Gold, Pack: h.Pack, PackCap: h.PackCap,
		Ranks: h.Ranks, Binds: h.Binds, Points: h.Points, Oil: h.Oil, Blessed: h.Blessed, TrialOn: h.TrialOn, TrialKills: h.TrialKills,
		Weapon: h.Weapon, Trinket: h.Trinket}
}

func (h *Hero) restore(s heroSave) {
	h.Lvl, h.XP, h.HP, h.HPMax, h.Res, h.ResMax, h.Gold, h.Pack = s.Lvl, s.XP, s.HP, s.HPMax, s.Res, s.ResMax, s.Gold, s.Pack
	h.Ranks, h.Binds, h.Points, h.PackCap, h.Oil = s.Ranks, s.Binds, s.Points, s.PackCap, s.Oil
	h.Blessed, h.TrialOn, h.TrialKills = s.Blessed, s.TrialOn, s.TrialKills
	h.Weapon, h.Trinket = s.Weapon, s.Trinket
	for i, st := range h.Pack { // saves from before the lantern: torches become oil, unknown items go
		if st.Item == "torch" {
			h.Oil = min(OilMax, h.Oil+st.N*OilFlask)
		}
		if st.N > 0 && Items[st.Item] == nil {
			h.Pack[i] = Stack{}
		}
	}
	h.Dead, h.Downed = false, false
}

// resetTree puts a hero back to the starting tree: the first skill at rank 1 on key 1, every other point unspent.
func (h *Hero) resetTree() {
	spent := 0
	for _, r := range h.Ranks {
		spent += r
	}
	h.Ranks = [TreeSize]int{1}
	h.Binds = [3]int{0, -1, -1}
	h.Points += max(0, spent-1)
}

// canLearn reports why a skill can't be learned or ranked up (nil = it can).
func (h *Hero) canLearn(idx int) error {
	if idx < 0 || idx >= TreeSize {
		return ErrBadIntent
	}
	if h.Points <= 0 {
		return ErrNoPoints
	}
	if h.Ranks[idx] >= MaxRank {
		return ErrMaxRank
	}
	tier := SkillTier(idx)
	if h.Lvl < SkillTiers[tier-1] {
		return ErrLocked
	}
	if tier > 1 && h.Ranks[idx] == 0 && h.Ranks[tier-2] == 0 && h.Ranks[tier-2+3] == 0 {
		return ErrLocked // needs a skill of the tier above
	}
	return nil
}

// SkillIndex finds a skill of the hero's class by id (-1 = none).
func (h *Hero) SkillIndex(id string) int {
	for i, s := range h.Def().Skills {
		if s.ID == id {
			return i
		}
	}
	return -1
}

// Stack is one rucksack slot.
type Stack struct {
	Item string
	N    int
	G    *Gear `json:",omitempty"` // a weapon or trinket (N is 1)
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
		for i := range h.Pack[:h.PackCap] {
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
	h.CDReady = [TreeSize]int{}
	h.CryUntil, h.HiddenUntil, h.PoisonCharges = 0, 0, 0
	h.CharmUntil = 0
	h.Shrine, h.ShrineUntil, h.shrineOn = "", 0, "" // a blessing stays on its floor
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
	h.HPMax = def.HP + HPPerLevel(def.HP)*(h.Lvl-1) + h.Weapon.Stat("hp") + h.Trinket.Stat("hp")
	h.HP = h.HPMax
	h.ResMax, h.Res = def.ResMax, def.ResStart
}

func (g *Game) gainXP(amount int) {
	for _, h := range g.Heroes {
		if h.Dead {
			continue
		}
		h.XP += amount
		g.levelUps(h)
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

// damageHero applies monster (or ally) damage to a hero: Bell, Iron Wall absorb, downed transition. how names
// the attack for the death recap ("firebolt", "a spike trap"); "" for a plain melee hit.
func (g *Game) damageHero(h *Hero, dmg int, srcID int, how string) {
	if h.Dead || h.Downed || !h.Connected || dmg <= 0 {
		return
	}
	if a := h.GearStat("armor"); a > 0 {
		dmg = int(math.Round(float64(dmg) * (1 - float64(a)/100)))
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
	g.stats(h).Taken += dmg
	if t := h.GearStat("thorns"); t > 0 && g.monByID[srcID] != nil {
		g.thorns = append(g.thorns, thorn{h.ID, srcID, max(1, dmg*t/100)})
	}
	if gt := h.Def().GainTaken; gt > 0 {
		h.gainRes(int(math.Round(h.resGain(float64(gt)))))
	}
	g.emit(Event{K: "hit", Actor: srcID, Target: h.ID, X: h.X, Y: h.Y, Amount: dmg})
	if h.HP <= 0 {
		h.HP = 0
		s := g.stats(h)
		s.LastDowned = g.cause(srcID, how)
		g.downHero(h, s.LastDowned, dmg)
		g.barkDown(srcID)
	}
}

// downHero knocks h down; by and dmg say what did it (the death recap in the log).
func (g *Game) downHero(h *Hero, by string, dmg int) {
	h.Downed = true
	h.DownedUntil = g.Now + DownedTicks
	h.ReviveTarget, h.ReviveProg = 0, 0
	g.stats(h).Downs++
	g.emit(Event{K: "down", Target: h.ID, X: h.X, Y: h.Y, Text: by, Amount: dmg})
}

// updateVision unions every hero's FOV into Visible and records newly explored tiles.
// Light is a light source on a dark floor.
type Light struct {
	X, Y int
	R    float64
}

// Orb is a light cast from a Scroll of Light.
type Orb struct {
	X, Y  int
	Until int
}

// Lights lists every light on the floor: each living hero's own light (bigger with a torch) and the orbs.
func (g *Game) Lights() []Light {
	var out []Light
	for _, h := range g.Heroes {
		if h.Dead {
			continue
		}
		r := BareLight
		if g.LanternLit(h) {
			r = TorchLight
		}
		out = append(out, Light{h.X, h.Y, r})
	}
	for _, o := range g.Orbs {
		out = append(out, Light{o.X, o.Y, OrbLight})
	}
	return out
}

func (g *Game) lit(lights []Light, x, y int) bool {
	for _, l := range lights {
		if dist(x, y, l.X, l.Y) <= l.R {
			return true
		}
	}
	return false
}

func (g *Game) updateVision() {
	clear(g.Visible)
	n := 0
	for _, o := range g.Orbs { // orbs burn out
		if o.Until > g.Now {
			g.Orbs[n] = o
			n++
		}
	}
	g.Orbs = g.Orbs[:n]
	dark := g.Level.Theme != nil && g.Level.Theme.Dark
	var lights []Light
	if dark {
		lights = g.Lights()
	}
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
		if dark {
			for _, i := range h.fov {
				if g.lit(lights, int(i)%LW, int(i)/LW) {
					g.Visible[i] = true
				}
			}
			continue
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

// LanternLit: every hero carries a lantern; it burns by itself on dark floors while it has oil.
func (g *Game) LanternLit(h *Hero) bool {
	return (h.Oil > 0 || h.GearStat("lantern") > 0) && !h.Dead && g.Level.Theme != nil && g.Level.Theme.Dark && !g.InVillage
}
