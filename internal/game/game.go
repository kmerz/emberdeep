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
	IntPotion
	IntRevive
)

type Intent struct {
	Kind   IntentKind
	Dir    int
	Slot   int
	X, Y   int
	Target int
	Hold   bool
	Seq    int
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
	Kind   byte // '$' gold, '!' potion
	X, Y   int
	Amount int
	Owner  int // 0 = shared
}

type Patch struct {
	X, Y  int
	Until int
	Owner int
}

type pending struct {
	at     int
	skill  string
	caster int
	x, y   int
}

type Game struct {
	Seed     uint32
	Level    *Level
	Settings Settings
	N        int // party size at run start (drives scaling)
	Now      int
	Phase    Phase

	Heroes  []*Hero
	Mons    []*Monster
	Items   []*Item
	Patches []*Patch

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

	rng      *RNG
	nextID   int
	occ      []int32
	heroByID map[int]*Hero
	monByID  map[int]*Monster
	events   []Event
	explNew  []TileUpdate
	pend     []pending
	bfsStamp []int32
	bfsPar   []int32
	stamp    int32
}

const monIDBase = 10000

func New(seed uint32, st Settings, n int) *Game {
	g := &Game{Seed: seed, Settings: st, N: n, stairsSince: -1}
	g.Level = Generate(seed, n)
	g.rng = NewRNG(uint64(seed)*31 + 5)
	g.nextID = monIDBase
	g.Explored = make([]bool, LW*LH)
	g.Visible = make([]bool, LW*LH)
	g.occ = make([]int32, LW*LH)
	g.bfsStamp = make([]int32, LW*LH)
	g.bfsPar = make([]int32, LW*LH)
	g.heroByID = map[int]*Hero{}
	g.monByID = map[int]*Monster{}
	for _, s := range g.Level.Mons {
		def := MonDefs[s.Kind]
		hp := float64(def.HP) * EnemyHPMult(n)
		if s.Elite {
			hp = float64(def.HP) * EliteHPMult(n)
		}
		m := &Monster{ID: g.newID(), Kind: s.Kind, X: s.X, Y: s.Y, HP: int(math.Round(hp)), Elite: s.Elite, Facing: 6}
		m.HPMax = m.HP
		g.Mons = append(g.Mons, m)
		g.monByID[m.ID] = m
		g.occ[m.Y*LW+m.X] = int32(m.ID)
	}
	for _, gs := range g.Level.Golds {
		g.Items = append(g.Items, &Item{ID: g.newID(), Kind: '$', X: gs.X, Y: gs.Y, Amount: gs.Amount})
	}
	return g
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
	def := Classes[cls]
	h := &Hero{ID: id, Name: name, Cls: cls, Lvl: 1, HPMax: def.HP, HP: def.HP, ResMax: def.ResMax, Res: def.ResStart,
		Potions: StartPotions, Connected: true, Facing: 2}
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
	switch in.Kind {
	case IntMove:
		return g.applyMove(h, in)
	case IntCast:
		return g.applyCast(h, in)
	case IntPotion:
		return g.applyPotion(h)
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
	if !g.cellFree(nx, ny) {
		return ErrBlocked
	}
	g.moveEntity(int32(h.ID), &h.X, &h.Y, nx, ny)
	cost := MoveTicks
	if g.Now < h.SlowUntil {
		cost = MoveTicks * 2
	}
	base := h.NextMove
	if g.Now > base {
		base = g.Now
	}
	h.NextMove = base + cost
	g.pickup(h)
	return nil
}

func (g *Game) moveEntity(id int32, px, py *int, nx, ny int) {
	if g.occ[*py*LW+*px] == id {
		g.occ[*py*LW+*px] = 0
	}
	*px, *py = nx, ny
	g.occ[ny*LW+nx] = id
}

func (g *Game) applyPotion(h *Hero) error {
	if h.Potions <= 0 {
		return ErrNoPotion
	}
	if g.Now < h.PotionReady {
		return ErrCooldown
	}
	h.Potions--
	h.PotionReady = g.Now + PotionCD
	heal := int(float64(h.HPMax) * PotionHealFrac)
	h.HP = min(h.HPMax, h.HP+heal)
	g.emit(Event{K: "potion", Actor: h.ID, X: h.X, Y: h.Y, Amount: heal})
	return nil
}

func (g *Game) pickup(h *Hero) {
	for i := 0; i < len(g.Items); i++ {
		it := g.Items[i]
		if it.X != h.X || it.Y != h.Y || (it.Owner != 0 && it.Owner != h.ID) {
			continue
		}
		g.Items = append(g.Items[:i], g.Items[i+1:]...)
		i--
		switch it.Kind {
		case '$':
			g.splitGold(h, it.Amount)
			g.emit(Event{K: "pickup", Actor: h.ID, X: h.X, Y: h.Y, Amount: it.Amount, FX: "gold", Text: "gold"})
		case '!':
			h.Potions = min(MaxPotions, h.Potions+1)
			g.emit(Event{K: "pickup", Actor: h.ID, X: h.X, Y: h.Y, Amount: 1, FX: "potion", Text: "potion"})
		}
	}
}

// ---- helpers shared by skills / AI ----

func (g *Game) eligibleHero(h *Hero) bool { return !h.Dead && !h.Downed && h.Connected }

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
