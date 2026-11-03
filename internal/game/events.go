package game

// Floor events: about one floor in four gets something special. Which one is fixed by the run seed, so a
// retry or a return trip finds the same event.
//
//   goblin  a Treasure Goblin runs from the party and escapes after a while; killed, it spills gold and
//           drops a rare for everyone
//   stir    the dead stir: every so often skeletons and zombies claw their way up near the party
//   cursed  gold counts double, but monsters hit harder

const (
	FloorEventChance = 0.25
	GoblinKind       = 'T'
	GoblinEscape     = 30 * TickHz // after it first sees the party
	GoblinGold       = 40          // per floor of depth
	GoblinMinDist    = 30.0        // it starts this far from the stairs up
	StirFirst        = 20 * TickHz // the first rising after arriving
	StirEvery        = 25 * TickHz
	StirMax          = 8 // risen dead alive at once
	StirNear         = 8 // they rise about this far from a hero
	CursedGoldMult   = 2
	CursedDmgMult    = 1.25
)

// FloorEvents are the event ids with what the client shows on arrival.
var FloorEvents = map[string]string{
	"goblin": "A Treasure Goblin is somewhere on this floor. Catch it before it escapes!",
	"stir":   "The dead stir. They will not stay in the ground.",
	"cursed": "This floor is cursed: gold is worth double, but monsters hit harder.",
}

var eventOrder = []string{"goblin", "stir", "cursed"}

// FloorEvent is the event on floor depth of a run with this seed ("" = none). Boss floors, the first floor
// and the last one never get one.
func FloorEvent(seed uint32, depth int) string {
	if depth <= 1 || depth >= MaxDepth || Floor(depth).Boss != 0 {
		return ""
	}
	r := NewRNG(uint64(FloorSeed(seed, depth))*13 + 7)
	if !r.Chance(FloorEventChance) {
		return ""
	}
	return eventOrder[r.Intn(len(eventOrder))]
}

// setupEvent starts the floor's event (called from setupFloor).
func (g *Game) setupEvent(depth int, memo *floorMemo) {
	g.Event, g.nextStir = "", 0
	if g.InVillage {
		return
	}
	g.Event = FloorEvent(g.Seed, depth)
	g.eventRng = NewRNG(uint64(FloorSeed(g.Seed, depth))*29 + 3)
	switch g.Event {
	case "goblin":
		if !memo.GoblinGone {
			g.placeGoblin()
		}
	case "stir":
		g.nextStir = g.Now + StirFirst
	}
}

// placeGoblin hides the goblin on a free cell far from where the party arrives.
func (g *Game) placeGoblin() {
	from := g.Level.Start
	for try := 0; try < 400; try++ {
		x, y := 1+g.eventRng.Intn(LW-2), 1+g.eventRng.Intn(LH-2)
		if g.cellFree(x, y) && dist(x, y, from.X, from.Y) >= GoblinMinDist {
			m := g.addMon(GoblinKind, x, y, 1)
			m.Awake = false
			return
		}
	}
}

// goblinAct: once it has seen the party, run from the nearest hero until the time is up, then vanish.
// It never fights back. Reports true: a goblin never acts like other monsters.
func (g *Game) goblinAct(m *Monster, tgt *Hero) bool {
	if m.EscapeAt == 0 {
		m.EscapeAt = g.Now + GoblinEscape
		g.emit(Event{K: "goblin", Target: m.ID, X: m.X, Y: m.Y})
	}
	if g.Now >= m.NextMove {
		g.stepAway(m, tgt)
	}
	return true
}

// goblinEscaped removes a goblin whose time ran out: its loot is gone with it.
func (g *Game) goblinEscaped(m *Monster) {
	g.emit(Event{K: "escape", Target: m.ID, X: m.X, Y: m.Y})
	g.removeMon(m)
	g.memo().GoblinGone = true
}

// goblinLoot: a killed goblin spills a big pile of gold and a rare for every hero.
func (g *Game) goblinLoot(m *Monster) {
	g.memo().GoblinGone = true
	g.Items = append(g.Items, &Item{ID: g.newID(), Kind: '$', X: m.X, Y: m.Y, Amount: GoblinGold * g.Depth})
	for _, h := range g.Heroes {
		if !h.Dead && h.Connected {
			g.dropGear(h, m.X, m.Y, Rare)
		}
	}
}

// tickStir raises a few dead near a random standing hero now and then (the "stir" event).
func (g *Game) tickStir() {
	if g.Event != "stir" || g.Now < g.nextStir {
		return
	}
	g.nextStir = g.Now + StirEvery
	risen := 0
	for _, m := range g.Mons {
		if m.Risen && m.HP > 0 {
			risen++
		}
	}
	var up []*Hero
	for _, h := range g.Heroes {
		if !h.Dead && !h.Downed && h.Connected {
			up = append(up, h)
		}
	}
	if len(up) == 0 {
		return
	}
	h := up[g.eventRng.Intn(len(up))]
	want := min(2+g.N/2, StirMax-risen)
	n := 0
	for try := 0; try < 60 && n < want; try++ {
		x := h.X + g.eventRng.Intn(2*StirNear+1) - StirNear
		y := h.Y + g.eventRng.Intn(StirNear+1) - StirNear/2
		if x < 1 || y < 1 || x >= LW-1 || y >= LH-1 || !g.cellFree(x, y) || cheb(x, y, h.X, h.Y) < 3 {
			continue
		}
		kind := byte('z')
		if g.eventRng.Chance(0.5) {
			kind = 's'
		}
		m := g.addMon(kind, x, y, 1)
		m.Summoned, m.Risen = true, true
		g.addThreat(m, h.ID, 1)
		g.emit(Event{K: "summon", Target: m.ID, X: x, Y: y, Text: m.Def().Name, FX: "stir"})
		n++
	}
	if n > 0 {
		g.emit(Event{K: "stir", Target: h.ID, X: h.X, Y: h.Y, Amount: n})
	}
}

// eventGoldMult is how many times gold picked up on this floor counts (a cursed floor: double).
func (g *Game) eventGoldMult() int {
	if g.Event == "cursed" {
		return CursedGoldMult
	}
	return 1
}
