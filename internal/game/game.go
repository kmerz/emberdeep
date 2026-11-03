// Package game is the pure, deterministic simulation: no goroutines, no clocks, no I/O.
package game

import (
	"errors"
	"math"
)

type Phase int

const (
	PhasePlaying Phase = iota
	PhaseWiped
	PhaseCleared
)

type Settings struct {
	Difficulty   int
	FriendlyFire bool
}

type IntentKind int

const (
	IntMove IntentKind = iota
	IntCast
	IntPotion // drink the first healing potion (the Q key)
	IntRevive
	IntUse     // use the item in Slot (thrown items land at X, Y)
	IntDrop    // drop the stack in Slot
	IntLearn   // spend a point on Skill (learn it or raise its rank)
	IntBind    // put Skill on key Slot
	IntRespec  // reset the tree (only at the stairs)
	IntBuy     // buy one of item Skill from the trader
	IntSell    // sell one item from rucksack Slot to the trader
	IntAccept  // accept quest Skill (quest id) from the giver next to the hero
	IntUnequip // take off the weapon (Slot 0) or trinket (Slot 1)
)

type Intent struct {
	Kind   IntentKind
	Dir    int
	Slot   int
	X, Y   int
	Target int
	Hold   bool
	Seq    int
	Skill  string // learn / bind: skill id
}

var (
	ErrNoHero    = errors.New("no such hero")
	ErrDowned    = errors.New("hero is downed")
	ErrTooFast   = errors.New("moving too fast")
	ErrBlocked   = errors.New("blocked")
	ErrCooldown  = errors.New("on cooldown")
	ErrResource  = errors.New("not enough resource")
	ErrRange     = errors.New("out of range")
	ErrLOS       = errors.New("no line of sight")
	ErrBadTarget = errors.New("bad target")
	ErrBadIntent = errors.New("bad intent")
	ErrNoPotion  = errors.New("no potions")
	ErrNoItem    = errors.New("no such item")
	ErrNoPoints  = errors.New("no skill points")
	ErrMaxRank   = errors.New("skill at max rank")
	ErrLocked    = errors.New("skill locked")
	ErrNotHere   = errors.New("not here")
	ErrNoGold    = errors.New("not enough gold")
	ErrCharmed   = errors.New("charmed")
	ErrHaveIt    = errors.New("already have it")
	ErrPackFull  = errors.New("rucksack full")
	ErrPhase     = errors.New("run is over")
)

type Event struct {
	K      string  `json:"k"`
	Actor  int     `json:"actor,omitempty"`
	Target int     `json:"target,omitempty"`
	X      int     `json:"x"`
	Y      int     `json:"y"`
	TX     int     `json:"tx,omitempty"`
	TY     int     `json:"ty,omitempty"`
	Amount int     `json:"amount,omitempty"`
	FX     string  `json:"fx,omitempty"`
	Text   string  `json:"text,omitempty"`
	R      float64 `json:"r,omitempty"`
}

type TileUpdate struct {
	X, Y int
	T    byte
}

type Item struct {
	ID     int
	Kind   byte   // '$' gold, otherwise the item's glyph
	Def    string // rucksack item id ("" = gold)
	X, Y   int
	Amount int
	Owner  int   // 0 = shared
	Spawn  int   // gold from the level: 1 + its index in Level.Golds
	G      *Gear // a weapon or trinket
}

// Chest is a closed chest. It blocks its cell until a hero bumps it open. Mimics are monsters, not chests.
type Chest struct {
	ID    int
	X, Y  int
	Urn   bool // a breakable urn rather than a treasure chest
	Spot  int  // 1 + its index in Level.Chests
	Vault bool // the vault's treasure chest
}

type Patch struct {
	X, Y  int
	Until int
	Owner int
	Dmg   int // bleed per second
}

type pending struct {
	at     int
	skill  string
	caster int
	x, y   int
	dmg    int // final damage, fixed at cast time (rank and buffs included)
	radius float64
}

type Game struct {
	Seed     uint32
	Level    *Level
	Depth    int // current floor, 1..MaxDepth
	Settings Settings
	N        int // party size at run start (drives scaling)
	Now      int
	Phase    Phase

	Heroes  []*Hero
	Mons    []*Monster
	Items   []*Item
	Patches []*Patch
	Chests  []*Chest
	Orbs    []Orb
	Npcs    []*Npc
	Portal  *Portal
	Quests  []*Quest // the run's sidequests (offered, active, done, failed)
	Fires   []Fire
	Traps   []*Trap

	InVillage   bool
	VillageSeen bool
	ReturnDepth int // where the village stairs lead after a Scroll of Return (0 = the floor after the village)
	PortalAt    int
	PortalNeed  int
	portalSince int

	Explored []bool
	Visible  []bool

	KingKilled  bool
	Kills       int
	StairsAt    int // heroes currently at the stairs
	StairsNeed  int
	stairsSince int // tick when the full-party window started, -1 when not running
	StairsLeft  int // ticks remaining in the window (0 when not running)
	ClearedTick int
	WipedTick   int

	rng       *RNG
	barkRng   *RNG // only for what monsters say, so lines never change the fight's dice
	nextBark  int  // no ordinary monster line before this tick
	nextID    int
	occ       []int32
	heroByID  map[int]*Hero
	monByID   map[int]*Monster
	chestByID map[int]*Chest
	npcByID   map[int]*Npc
	events    []Event
	explNew   []TileUpdate
	pend      []pending
	bfsStamp  []int32
	bfsPar    []int32
	stamp     int32

	arrival       map[int]heroSave   // each hero as they arrived on this floor (a wipe retries from here)
	Floors        map[int]*floorMemo // what each visited floor remembers (by depth; the village is 0)
	arrivalMemo   *floorMemo         // this floor's memory as the party arrived (a wipe goes back to it)
	arriveDown    bool               // the party came up: they arrive at the stairs down, not the start
	arrivedDown   bool               // ...and did on this floor (a wipe retry puts them there again)
	upArmed       bool               // the stairs count only once nobody stands at them (arriving next to
	downArmed     bool               // them must not start the countdown)
	StairsUp      bool               // the party is gathering at the stairs up
	Vault         *Rect              // this floor's vault room (nil = none)
	VaultDoor     Pt
	questsChanged bool
	levelChanged  bool
}

// heroSave is the part of a hero that carries over between floors.
type heroSave struct {
	Lvl, XP, HP, HPMax, Res, ResMax, Gold int
	Weapon, Trinket                       *Gear `json:",omitempty"`
	Pack                                  [PackSlots]Stack
	PackCap                               int
	Ranks                                 [TreeSize]int
	Binds                                 [3]int
	Points                                int
	Oil                                   int
	Blessed, TrialKills                   int
	TrialOn                               bool
}

const monIDBase = 10000

func New(seed uint32, st Settings, n int) *Game { return NewAt(seed, st, n, 1) }

// NewAt starts a run on the given floor (the -depth dev flag uses it to look at deeper floors).
func NewAt(seed uint32, st Settings, n, depth int) *Game {
	g := &Game{Seed: seed, Settings: st, N: n, stairsSince: -1}
	g.nextID = monIDBase
	g.heroByID = map[int]*Hero{}
	g.initQuests()
	g.loadFloor(min(max(depth, 1), MaxDepth))
	return g
}

// loadFloor builds floor depth and resets everything that belongs to a floor. Heroes are kept and placed
// around the start; their state is saved so a wipe can retry the floor.
func (g *Game) loadFloor(depth int) {
	g.InVillage = false
	g.setupFloor(depth, Generate(g.Seed, depth, g.N), FloorSeed(g.Seed, depth))
}

// loadVillage loads the gnome village (it keeps the depth number of the floor the party came from).
func (g *Game) loadVillage() {
	g.InVillage, g.VillageSeen = true, true
	g.setupFloor(g.Depth, GenerateVillage(g.Seed), g.Seed^0x6E0BE)
}

func (g *Game) setupFloor(depth int, lvl *Level, seed uint32) {
	n := g.N
	g.Depth = depth
	g.Level = lvl
	g.placeUpStairs()
	memo := g.memo()
	g.arrivedDown, g.arriveDown = g.arriveDown, false
	// coming up, the party arrives on the stairs down: those count only once everyone has stepped away
	g.upArmed, g.downArmed, g.StairsUp = true, !g.arrivedDown, false
	g.rng = NewRNG(uint64(seed)*31 + 5)
	g.barkRng = NewRNG(uint64(seed)*17 + 99)
	g.nextBark = 0
	g.Mons, g.Items, g.Patches, g.pend, g.Chests, g.Orbs, g.Npcs, g.Portal, g.Fires = nil, nil, nil, nil, nil, nil, nil, nil, nil
	g.npcByID = map[int]*Npc{}
	g.portalSince, g.PortalAt, g.PortalNeed = -1, 0, 0
	g.KingKilled = false
	g.stairsSince, g.StairsLeft, g.StairsAt, g.StairsNeed = -1, 0, 0, 0
	g.explNew = nil
	g.Explored = make([]bool, LW*LH)
	g.Visible = make([]bool, LW*LH)
	g.occ = make([]int32, LW*LH)
	g.bfsStamp = make([]int32, LW*LH)
	g.bfsPar = make([]int32, LW*LH)
	g.monByID = map[int]*Monster{}
	for si, s := range g.Level.Mons {
		if has(memo.Dead, si) {
			continue
		}
		def := MonDefs[s.Kind]
		hp := float64(def.HP) * EnemyHPMult(n)
		if s.Elite {
			hp = float64(def.HP) * EliteHPMult(n)
		}
		hp *= DepthHPMult(depth) * Difficulties[g.Settings.Difficulty].HPMult
		m := &Monster{ID: g.newID(), Kind: s.Kind, X: s.X, Y: s.Y, HP: int(math.Round(hp)), Elite: s.Elite, Facing: 6, Neutral: def.Neutral, Spawn: si + 1}
		m.HPMax = m.HP
		g.Mons = append(g.Mons, m)
		g.monByID[m.ID] = m
		g.occ[m.Y*LW+m.X] = int32(m.ID)
	}
	for gi, gs := range g.Level.Golds {
		if has(memo.Gold, gi) {
			continue
		}
		g.Items = append(g.Items, &Item{ID: g.newID(), Kind: '$', X: gs.X, Y: gs.Y, Amount: gs.Amount, Spawn: gi + 1})
	}
	for _, ns := range lvl.Npcs {
		c := &Npc{ID: g.newID(), X: ns.X, Y: ns.Y, Home: Pt{ns.X, ns.Y}, Name: ns.Name, Trader: ns.Trader}
		g.Npcs = append(g.Npcs, c)
		g.npcByID[c.ID] = c
		g.occ[c.Y*LW+c.X] = int32(c.ID)
	}
	if depth == 1 && !g.InVillage { // the mayor's notice board stands in the first room
		s := lvl.Start
		if p, ok := lvl.nearestFree(Pt{s.X + 2, s.Y - 2}, func(x, y int) bool { return g.occ[y*LW+x] == 0 && (x != s.X || y != s.Y) }); ok {
			b := &Npc{ID: g.newID(), X: p.X, Y: p.Y, Home: p, Name: "Notice Board", Board: true}
			g.Npcs = append(g.Npcs, b)
			g.npcByID[b.ID] = b
			g.occ[p.Y*LW+p.X] = int32(b.ID)
		}
	}
	g.chestByID = map[int]*Chest{}
	for ci, cs := range g.Level.Chests {
		if has(memo.Opened, ci) {
			continue
		}
		if cs.Mimic {
			def := MonDefs['M']
			hp := int(math.Round(float64(def.HP) * EnemyHPMult(n) * DepthHPMult(depth) * Difficulties[g.Settings.Difficulty].HPMult))
			m := &Monster{ID: g.newID(), Kind: 'M', X: cs.X, Y: cs.Y, HP: hp, HPMax: hp, Facing: 6, Disguised: true, ChestSpot: ci + 1}
			g.Mons = append(g.Mons, m)
			g.monByID[m.ID] = m
			g.occ[m.Y*LW+m.X] = int32(m.ID)
			continue
		}
		c := &Chest{ID: g.newID(), X: cs.X, Y: cs.Y, Urn: cs.Urn, Spot: ci + 1}
		g.Chests = append(g.Chests, c)
		g.chestByID[c.ID] = c
		g.occ[c.Y*LW+c.X] = int32(c.ID)
	}
	g.Vault = nil
	g.setupWorld()
	g.questsOnFloor()
	g.setupStory(memo)
	g.restoreExplored(memo)
	g.arrivalMemo = memo.clone()
	g.KingKilled = g.KingKilled || g.bossDead(memo)
	g.arrival = map[int]heroSave{}
	for _, h := range g.Heroes {
		h.freshOnFloor()
		for i, s := range h.Pack { // a vault key only fits its own floor's door
			if s.Item == KeyItem {
				h.Pack[i] = Stack{}
			}
		}
		p, _ := g.Level.nearestFree(g.arrivalPoint(), func(x, y int) bool { return g.occ[y*LW+x] == 0 })
		h.X, h.Y = p.X, p.Y
		g.occ[h.Y*LW+h.X] = int32(h.ID)
		g.arrival[h.ID] = h.save()
	}
	g.levelChanged = true
}

// descend takes the party to the next floor. The fallen stand up with FallenHPFrac of their HP.
func (g *Game) descend() {
	g.rememberFloor()
	g.questsOnLeave()
	g.standUp()
	switch {
	case g.InVillage: // back down: to the floor the party came from, or the one after the village
		next := g.ReturnDepth
		if next == 0 {
			next = VillageAfter + 1
		}
		g.ReturnDepth = 0
		g.loadFloor(next)
	case g.Depth == VillageAfter: // the village is the floor between 5 and 6, both ways
		g.loadVillage()
		g.emit(Event{K: "village"})
		return
	default:
		g.loadFloor(g.Depth + 1)
	}
	g.emit(Event{K: "descend", Amount: g.Depth})
}

// standUp brings the fallen back with FallenHPFrac of their HP (on every change of floor).
func (g *Game) standUp() {
	for _, h := range g.Heroes {
		if h.Dead || h.Downed {
			h.Dead, h.Downed = false, false
			h.HP = max(1, int(math.Round(float64(h.HPMax)*FallenHPFrac)))
		}
	}
}

// RetryFloor restarts the current floor after a wipe, with every hero as they arrived on it.
// Heroes who joined during the floor start over at level 1.
func (g *Game) RetryFloor() {
	saved := g.arrival
	for _, h := range g.Heroes {
		if s, ok := saved[h.ID]; ok {
			h.restore(s)
		} else {
			*h = *newHero(h.ID, h.Name, h.Cls, h.Connected)
		}
	}
	g.Phase = PhasePlaying
	g.WipedTick = 0
	g.Floors[g.memoKey()] = g.arrivalMemo.clone()
	g.arriveDown = g.arrivedDown
	if g.InVillage {
		g.loadVillage()
	} else {
		g.loadFloor(g.Depth)
	}
}

// TakeLevelChanged reports whether a new floor was loaded since the last call.
func (g *Game) TakeLevelChanged() bool {
	c := g.levelChanged
	g.levelChanged = false
	return c
}

func (g *Game) newID() int { g.nextID++; return g.nextID }

func (g *Game) Hero(id int) *Hero   { return g.heroByID[id] }
func (g *Game) Mon(id int) *Monster { return g.monByID[id] }

func (g *Game) emit(e Event) { g.events = append(g.events, e) }

// TakeExplored returns tiles newly explored since the last call.
func (g *Game) TakeExplored() []TileUpdate {
	out := g.explNew
	g.explNew = nil
	return out
}

// ExploredTiles returns every explored tile (for late joiners).
func (g *Game) ExploredTiles() []TileUpdate {
	var out []TileUpdate
	for i, e := range g.Explored {
		if e {
			out = append(out, TileUpdate{i % LW, i / LW, g.Level.Tiles[i]})
		}
	}
	return out
}

func (g *Game) idx(x, y int) int { return y*LW + x }

func (g *Game) cellFree(x, y int) bool {
	return g.Level.Walkable(x, y) && g.occ[y*LW+x] == 0
}

func (g *Game) anyAlive() bool {
	for _, h := range g.Heroes {
		if !h.Dead {
			return true
		}
	}
	return false
}

// AddHero creates a hero next to the party (or in the start room if nobody is alive).
func (g *Game) AddHero(id int, name string, cls ClassID) *Hero {
	h := newHero(id, name, cls, true)
	at := g.Level.Start
	for _, o := range g.Heroes {
		if !o.Dead && !o.Downed {
			at = Pt{o.X, o.Y}
			break
		}
	}
	p, ok := g.Level.nearestFree(at, func(x, y int) bool { return g.occ[y*LW+x] == 0 })
	if !ok {
		p, _ = g.Level.nearestFree(g.Level.Start, func(x, y int) bool { return g.occ[y*LW+x] == 0 })
	}
	h.X, h.Y = p.X, p.Y
	g.Heroes = append(g.Heroes, h)
	g.heroByID[id] = h
	g.occ[h.Y*LW+h.X] = int32(id)
	if _, ok := g.arrival[id]; !ok {
		g.arrival[id] = h.save()
	}
	return h
}

func newHero(id int, name string, cls ClassID, connected bool) *Hero {
	def := Classes[cls]
	h := &Hero{ID: id, Name: name, Cls: cls, Lvl: 1, HPMax: def.HP, HP: def.HP, ResMax: def.ResMax, Res: def.ResStart,
		Connected: connected, Facing: 2, PackCap: StartPackCap}
	h.AddItem("potion", StartPotions)
	h.Oil = StartOil
	h.resetTree() // the first skill on key 1
	h.Points = 1  // and one point to spend
	return h
}

func (g *Game) RemoveHero(id int) {
	h := g.heroByID[id]
	if h == nil {
		return
	}
	if g.occ[h.Y*LW+h.X] == int32(id) {
		g.occ[h.Y*LW+h.X] = 0
	}
	delete(g.heroByID, id)
	for i, o := range g.Heroes {
		if o == h {
			g.Heroes = append(g.Heroes[:i], g.Heroes[i+1:]...)
			break
		}
	}
	for _, o := range g.Heroes {
		if o.ReviveTarget == id {
			o.ReviveTarget, o.ReviveProg = 0, 0
		}
	}
}

func (g *Game) SetConnected(id int, c bool) {
	if h := g.heroByID[id]; h != nil {
		h.Connected = c
		if !c {
			h.ReviveTarget, h.ReviveProg = 0, 0
		}
	}
}

// Apply validates and applies one player intent. A rejected intent returns an error and changes nothing
// (except that the sequence number is still acknowledged).
func (g *Game) Apply(pid int, in Intent) error {
	h := g.heroByID[pid]
	if h == nil {
		return ErrNoHero
	}
	if in.Seq > h.LastSeq {
		h.LastSeq = in.Seq
	}
	if g.Phase != PhasePlaying {
		return ErrPhase
	}
	if h.Dead {
		return ErrDowned
	}
	if h.Downed {
		return ErrDowned
	}
	if !h.Connected {
		return ErrBadIntent
	}
	if g.Now < h.CharmUntil {
		switch in.Kind {
		case IntMove, IntCast, IntPotion, IntUse, IntRevive, IntDrop:
			return ErrCharmed
		}
	}
	switch in.Kind {
	case IntMove:
		return g.applyMove(h, in)
	case IntCast:
		return g.applyCast(h, in)
	case IntPotion:
		return g.applyPotion(h)
	case IntUse:
		return g.applyUse(h, in)
	case IntDrop:
		return g.applyDrop(h, in)
	case IntLearn:
		idx := h.SkillIndex(in.Skill)
		if err := h.canLearn(idx); err != nil {
			return err
		}
		h.Points--
		h.Ranks[idx]++
		if h.Ranks[idx] == 1 { // a new skill goes on the first free key
			for k, b := range h.Binds {
				if b < 0 {
					h.Binds[k] = idx
					break
				}
			}
		}
		g.emit(Event{K: "learn", Actor: h.ID, X: h.X, Y: h.Y, FX: h.Def().Skills[idx].ID, Amount: h.Ranks[idx]})
		return nil
	case IntBind:
		idx := h.SkillIndex(in.Skill)
		if in.Slot < 0 || in.Slot > 2 || idx < 0 || idx >= TreeSize || h.Ranks[idx] == 0 {
			return ErrBadIntent
		}
		for k, b := range h.Binds { // already on another key: swap
			if b == idx {
				h.Binds[k] = h.Binds[in.Slot]
			}
		}
		h.Binds[in.Slot] = idx
		return nil
	case IntBuy:
		return g.applyBuy(h, in.Skill)
	case IntSell:
		return g.applySell(h, in.Slot)
	case IntAccept:
		return g.applyAccept(h, in.Skill)
	case IntUnequip:
		return g.unequip(h, in.Slot)
	case IntRespec:
		if !g.AtStairs(h) {
			return ErrNotHere
		}
		h.resetTree()
		g.emit(Event{K: "respec", Actor: h.ID, X: h.X, Y: h.Y})
		return nil
	case IntRevive:
		return g.applyRevive(h, in)
	}
	return ErrBadIntent
}

func (g *Game) applyMove(h *Hero, in Intent) error {
	if in.Dir < 0 || in.Dir > 7 {
		return ErrBadIntent
	}
	if g.Now+MoveTolerance < h.NextMove {
		return ErrTooFast
	}
	d := dirs8[in.Dir]
	nx, ny := h.X+d.X, h.Y+d.Y
	h.Facing = in.Dir
	// moving cancels any revive in progress
	h.ReviveTarget, h.ReviveProg = 0, 0
	// bumping a chest opens it (or wakes the mimic); it costs a step
	if g.bumpChest(h, nx, ny) || g.bumpNpc(h, nx, ny) {
		g.spendStep(h)
		return nil
	}
	if m := g.monByID[int(g.occ[ny*LW+nx])]; m != nil && !m.Neutral && m.HP > 0 && m.Def().Role != "keg" { // walking into an enemy hits it
		g.strike(h, nx, ny)
		return nil
	}
	if g.bumpLocked(h, nx, ny) {
		return nil
	}
	if !g.cellFree(nx, ny) {
		return ErrBlocked
	}
	g.moveEntity(int32(h.ID), &h.X, &h.Y, nx, ny)
	g.spendStep(h)
	g.pickup(h)
	return nil
}

func (g *Game) spendStep(h *Hero) {
	cost := MoveTicks
	if g.Now < h.SlowUntil {
		cost = MoveTicks * 2
	}
	base := h.NextMove
	if g.Now > base {
		base = g.Now
	}
	h.NextMove = base + cost
}

// bumpChest opens the chest at x, y or reveals a mimic there. It reports whether there was one.
func (g *Game) bumpChest(h *Hero, x, y int) bool {
	id := int(g.occ[y*LW+x])
	if c := g.chestByID[id]; c != nil {
		g.openChest(h, c)
		return true
	}
	if m := g.monByID[id]; m != nil && m.Disguised {
		g.revealMimic(m, h)
		return true
	}
	return false
}

func (g *Game) openChest(h *Hero, c *Chest) {
	if c.Spot > 0 {
		g.memo().Opened = append(g.memo().Opened, c.Spot-1)
	}
	delete(g.chestByID, c.ID)
	for i, o := range g.Chests {
		if o == c {
			g.Chests = append(g.Chests[:i], g.Chests[i+1:]...)
			break
		}
	}
	if g.occ[c.Y*LW+c.X] == int32(c.ID) {
		g.occ[c.Y*LW+c.X] = 0
	}
	if c.Urn {
		g.smashUrn(h, c)
		return
	}
	g.emit(Event{K: "chest", Actor: h.ID, Target: c.ID, X: c.X, Y: c.Y})
	if c.Vault {
		g.vaultLoot(c.X, c.Y)
		return
	}
	g.chestLoot(c.X, c.Y)
}

// chestLoot spills a chest's treasure on a cell: shared gold, and ChestRolls personal item rolls per hero
// (weighted by each item's drop chance).
func (g *Game) chestLoot(x, y int) {
	g.Items = append(g.Items, &Item{ID: g.newID(), Kind: '$', X: x, Y: y, Amount: ChestGold + g.rng.Intn(ChestGold+1)})
	total := 0.0
	for _, id := range ItemOrder {
		total += Items[id].DropChance
	}
	for _, h := range g.Heroes {
		if h.Dead || !h.Connected {
			continue
		}
		if g.rng.Chance(GearChestChance) {
			g.dropGear(h, x, y, Common)
		}
		for k := 0; k < ChestRolls; k++ {
			r := g.rng.Float() * total
			for _, id := range ItemOrder {
				def := Items[id]
				if r -= def.DropChance; r < 0 {
					g.Items = append(g.Items, &Item{ID: g.newID(), Kind: def.Glyph, Def: id, X: x, Y: y, Amount: 1, Owner: h.ID})
					break
				}
			}
		}
	}
}

func (g *Game) moveEntity(id int32, px, py *int, nx, ny int) {
	if g.occ[*py*LW+*px] == id {
		g.occ[*py*LW+*px] = 0
	}
	*px, *py = nx, ny
	g.occ[ny*LW+nx] = id
}

// applyPotion is the Q key: a healing potion, or a Resource Draught when that is what the hero needs (HP is
// fine but the resource is low, or there are no potions left and the resource isn't full).
func (g *Game) applyPotion(h *Hero) error {
	pot, dr := h.firstSlot("potion"), h.firstSlot("draught")
	hpOK := float64(h.HP) >= float64(h.HPMax)*DraughtHPOK
	resLow := float64(h.Res) < float64(h.ResMax)*DraughtResLow
	slot := pot
	if dr >= 0 && ((hpOK && resLow) || (pot < 0 && h.Res < h.ResMax)) {
		slot = dr
	}
	if slot < 0 {
		return ErrNoPotion
	}
	return g.applyUse(h, Intent{Kind: IntUse, Slot: slot})
}

func (g *Game) applyUse(h *Hero, in Intent) error {
	if in.Slot < 0 || in.Slot >= PackSlots || h.Pack[in.Slot].N <= 0 {
		return ErrNoItem
	}
	def := Items[h.Pack[in.Slot].Item]
	switch def.Use {
	case "equip":
		return g.equip(h, in.Slot)
	case "heal", "resource":
		if g.Now < h.PotionReady {
			return ErrCooldown
		}
		h.PotionReady = g.Now + PotionCD
		if def.Use == "heal" {
			heal := int(float64(h.HPMax) * def.Amount)
			h.HP = min(h.HPMax, h.HP+heal)
			g.emit(Event{K: "potion", Actor: h.ID, X: h.X, Y: h.Y, Amount: heal})
		} else {
			n := int(float64(h.ResMax) * def.Amount)
			h.gainRes(n)
			g.emit(Event{K: "use", Actor: h.ID, X: h.X, Y: h.Y, Amount: n, FX: def.ID})
		}
	case "portal":
		if err := g.openPortal(h); err != nil {
			return err
		}
	case "orb":
		if err := g.checkAim(h, in, def.Range); err != nil {
			return err
		}
		if len(g.Orbs) >= MaxOrbs {
			g.Orbs = g.Orbs[1:]
		}
		g.Orbs = append(g.Orbs, Orb{X: in.X, Y: in.Y, Until: g.Now + OrbTicks})
		g.emit(Event{K: "use", Actor: h.ID, X: h.X, Y: h.Y, TX: in.X, TY: in.Y, FX: def.ID})
	case "throw":
		if err := g.checkAim(h, in, def.Range); err != nil {
			return err
		}
		h.Facing = dirIndex(in.X-h.X, in.Y-h.Y, h.Facing)
		g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: in.X, TY: in.Y, FX: def.ID, R: def.Radius, Amount: ThrowDelay})
		g.pend = append(g.pend, pending{at: g.Now + ThrowDelay, skill: def.ID, caster: h.ID, x: in.X, y: in.Y})
	default:
		return ErrBadIntent
	}
	h.removeAt(in.Slot, 1)
	h.ReviveTarget, h.ReviveProg = 0, 0
	return nil
}

// checkAim validates a cursor target: inside the map, in range, in line of sight, not a wall.
func (g *Game) checkAim(h *Hero, in Intent, rng float64) error {
	if in.X < 0 || in.Y < 0 || in.X >= LW || in.Y >= LH {
		return ErrBadTarget
	}
	if dist(h.X, h.Y, in.X, in.Y) > rng+0.5 {
		return ErrRange
	}
	if (in.X != h.X || in.Y != h.Y) && !g.Level.LineClear(h.X, h.Y, in.X, in.Y) {
		return ErrLOS
	}
	if Solid(g.Level.At(in.X, in.Y)) {
		return ErrBlocked
	}
	return nil
}

// applyDrop puts a whole stack on the hero's cell; anyone can pick it up.
func (g *Game) applyDrop(h *Hero, in Intent) error {
	if in.Slot < 0 || in.Slot >= PackSlots || h.Pack[in.Slot].N <= 0 {
		return ErrNoItem
	}
	if Items[h.Pack[in.Slot].Item].Quest {
		return ErrBadIntent // quest items stay in the rucksack
	}
	s := h.Pack[in.Slot]
	def := Items[s.Item]
	h.Pack[in.Slot] = Stack{}
	g.Items = append(g.Items, &Item{ID: g.newID(), Kind: def.Glyph, Def: def.ID, X: h.X, Y: h.Y, Amount: s.N, G: s.G})
	e := Event{K: "drop", Actor: h.ID, X: h.X, Y: h.Y, Amount: s.N, FX: def.ID}
	if s.G != nil {
		e.Text = s.G.Name
	}
	g.emit(e)
	return nil
}

func (g *Game) pickup(h *Hero) {
	for i := 0; i < len(g.Items); i++ {
		it := g.Items[i]
		if it.X != h.X || it.Y != h.Y || (it.Owner != 0 && it.Owner != h.ID) {
			continue
		}
		if it.Def == "" { // gold
			if it.Spawn > 0 {
				g.memo().Gold = append(g.memo().Gold, it.Spawn-1)
			}
			g.Items = append(g.Items[:i], g.Items[i+1:]...)
			i--
			g.splitGold(h, it.Amount)
			g.emit(Event{K: "pickup", Actor: h.ID, X: h.X, Y: h.Y, Amount: it.Amount, FX: "gold", Text: "gold"})
			continue
		}
		if it.Def == "oil" { // into the lantern, not the rucksack; a full lantern leaves it lying
			if h.Oil+OilFlask > OilMax+OilFlask/2 {
				continue
			}
			h.Oil = min(OilMax, h.Oil+OilFlask*it.Amount)
			g.emit(Event{K: "pickup", Actor: h.ID, X: h.X, Y: h.Y, Amount: it.Amount, FX: it.Def, Text: Items[it.Def].Name})
			g.Items = append(g.Items[:i], g.Items[i+1:]...)
			i--
			continue
		}
		if it.G != nil {
			if g.pickGear(h, it) {
				g.Items = append(g.Items[:i], g.Items[i+1:]...)
				i--
			} else if h.packFullAt == 0 || g.Now-h.packFullAt >= PackFullEvery {
				h.packFullAt = g.Now
				g.emit(Event{K: "packfull", Actor: h.ID, X: h.X, Y: h.Y})
			}
			continue
		}
		n := h.AddItem(it.Def, it.Amount)
		if n > 0 {
			g.emit(Event{K: "pickup", Actor: h.ID, X: h.X, Y: h.Y, Amount: n, FX: it.Def, Text: Items[it.Def].Name})
		}
		if n > 0 && Items[it.Def].Quest {
			g.questPickup(it.Def)
		}
		if n < it.Amount { // the rest stays on the floor
			it.Amount -= n
			if h.packFullAt == 0 || g.Now-h.packFullAt >= PackFullEvery {
				h.packFullAt = g.Now
				g.emit(Event{K: "packfull", Actor: h.ID, X: h.X, Y: h.Y})
			}
			continue
		}
		g.Items = append(g.Items[:i], g.Items[i+1:]...)
		i--
	}
}

// ---- helpers shared by skills / AI ----

func (g *Game) eligibleHero(h *Hero) bool {
	return !h.Dead && !h.Downed && h.Connected && g.Now >= h.HiddenUntil
}

// AtStairs reports whether the hero stands on or next to the stairs (where the tree can be reset).
func (g *Game) AtStairs(h *Hero) bool {
	return cheb(h.X, h.Y, g.Level.Stairs.X, g.Level.Stairs.Y) <= StairsReach
}

func (g *Game) nearestHero(x, y int, maxD float64) *Hero {
	var best *Hero
	bd := maxD
	for _, h := range g.Heroes {
		if !g.eligibleHero(h) {
			continue
		}
		if d := dist(x, y, h.X, h.Y); d <= bd {
			best, bd = h, d
		}
	}
	return best
}

func (g *Game) findFree(p Pt) (Pt, bool) {
	return g.Level.nearestFree(p, func(x, y int) bool { return g.occ[y*LW+x] == 0 })
}

// Teleport moves a hero to a free walkable cell (tests, tools).
func (g *Game) Teleport(id, x, y int) bool {
	h := g.heroByID[id]
	if h == nil || !g.cellFree(x, y) {
		return false
	}
	g.moveEntity(int32(h.ID), &h.X, &h.Y, x, y)
	return true
}

// Hurt applies raw damage to a hero as if a monster had hit it (tests, tools).
func (g *Game) Hurt(id, dmg int) {
	if h := g.heroByID[id]; h != nil {
		g.damageHero(h, dmg, monIDBase)
	}
}
