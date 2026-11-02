package game

import "math"

type ThreatEntry struct {
	Hero int
	Dmg  int
	Tick int
}

type Monster struct {
	ID        int
	Kind      byte
	X, Y      int
	HP, HPMax int
	Elite     bool
	Facing    int
	NextMove  int
	NextAtk   int
	Awake     bool
	Disguised bool // a mimic still pretending to be a chest
	Neutral   bool // an ant worker that hasn't been provoked

	NextAbility      int // breath, charm or blink ready again
	BreathAt         int // tick the dragon's breath lands (0 = not winding up)
	BreathX, BreathY int
	Threat           []ThreatEntry

	SlowUntil  int
	BleedUntil int
	BleedDmg   int
	BleedOwner int
	StunUntil  int

	Gen       int  // blobs: 0 = whole, 1 = a blobling (doesn't split again)
	Parent    int  // the caller that raised this minion (0 = none)
	Summoned  bool // minions give little XP and no loot
	NextSpawn int  // callers: tick of the next summon
}

func (m *Monster) Def() MonDef { return MonDefs[m.Kind] }

// damageMonster applies damage from a hero, awards resource, handles death and loot.
func (g *Game) damageMonster(m *Monster, dmg int, actor int, crit bool) {
	if m.HP <= 0 || dmg <= 0 {
		return
	}
	if m.Disguised { // hitting a "chest" gives the mimic away
		m.Disguised = false
		g.emit(Event{K: "mimic", Actor: actor, Target: m.ID, X: m.X, Y: m.Y})
	}
	if m.Def().Faction != "" {
		g.alarm(m, actor)
	}
	m.HP -= dmg
	m.Awake = true
	if m.HP > 0 && m.Def().Role == "drainer" {
		defer g.maybeBlink(m)
	}
	if a := g.heroByID[actor]; a != nil {
		g.addThreat(m, actor, dmg)
		if gd := a.Def().GainDeal; gd > 0 {
			a.gainRes(gd)
		}
	}
	fx := ""
	if crit {
		fx = "crit"
	}
	g.emit(Event{K: "hit", Actor: actor, Target: m.ID, X: m.X, Y: m.Y, Amount: dmg, FX: fx})
	if m.HP <= 0 {
		g.killMonster(m, actor)
	}
}

func (g *Game) killMonster(m *Monster, killer int) {
	m.HP = 0
	if g.occ[m.Y*LW+m.X] == int32(m.ID) {
		g.occ[m.Y*LW+m.X] = 0
	}
	delete(g.monByID, m.ID)
	for i, o := range g.Mons {
		if o == m {
			g.Mons = append(g.Mons[:i], g.Mons[i+1:]...)
			break
		}
	}
	g.Kills++
	def := m.Def()
	if def.Role == "splitter" && m.Gen == 0 {
		g.split(m)
	}
	if m.Summoned { // no loot, a little XP
		g.emit(Event{K: "kill", Actor: killer, Target: m.ID, X: m.X, Y: m.Y, Text: def.Name, FX: string(rune(m.Kind))})
		g.gainXP(1)
		return
	}
	g.emit(Event{K: "kill", Actor: killer, Target: m.ID, X: m.X, Y: m.Y, Text: def.Name, FX: string(rune(m.Kind))})
	if m.Elite {
		g.KingKilled = true
	}
	if m.Kind == 'M' { // a mimic keeps the chest's treasure inside
		g.chestLoot(m.X, m.Y)
	}
	xp := int(math.Round(float64(def.XP) * DepthXPMult(g.Depth)))
	g.gainXP(xp)
	// gold is shared; potions are personal drops rolled per player
	if m.Elite || g.rng.Chance(GoldDropChance) {
		amt := GoldBase + g.rng.Intn(14)
		if m.Elite {
			amt *= 8
		}
		g.Items = append(g.Items, &Item{ID: g.newID(), Kind: '$', X: m.X, Y: m.Y, Amount: amt})
	}
	for _, h := range g.Heroes {
		if h.Dead || !h.Connected {
			continue
		}
		for _, id := range ItemOrder {
			def := Items[id]
			if (m.Elite && def.Use == "heal") || g.rng.Chance(def.DropChance) {
				g.Items = append(g.Items, &Item{ID: g.newID(), Kind: def.Glyph, Def: id, X: m.X, Y: m.Y, Amount: 1, Owner: h.ID})
			}
		}
	}
}

// addMon puts a new monster on the floor at x, y (scaled for the party and depth).
func (g *Game) addMon(kind byte, x, y int, hpScale float64) *Monster {
	def := MonDefs[kind]
	hp := max(1, int(math.Round(float64(def.HP)*EnemyHPMult(g.N)*DepthHPMult(g.Depth)*hpScale)))
	m := &Monster{ID: g.newID(), Kind: kind, X: x, Y: y, HP: hp, HPMax: hp, Facing: 6, Awake: true}
	g.Mons = append(g.Mons, m)
	g.monByID[m.ID] = m
	g.occ[y*LW+x] = int32(m.ID)
	return m
}

// freeNear finds a free cell next to x, y, or reports false.
func (g *Game) freeNear(x, y int) (Pt, bool) {
	start := g.rng.Intn(8)
	for k := 0; k < 8; k++ {
		d := dirs8[(start+k)%8]
		if g.cellFree(x+d.X, y+d.Y) {
			return Pt{x + d.X, y + d.Y}, true
		}
	}
	return Pt{}, false
}

// split turns a dying blob into bloblings with half its strength each.
func (g *Game) split(m *Monster) {
	for k := 0; k < SplitInto; k++ {
		p, ok := g.freeNear(m.X, m.Y)
		if !ok {
			break
		}
		b := g.addMon(m.Kind, p.X, p.Y, 0.5)
		b.Gen = 1
		b.Threat = append([]ThreatEntry(nil), m.Threat...)
	}
	g.emit(Event{K: "split", Target: m.ID, X: m.X, Y: m.Y})
}

// alarm turns every monster of m's faction within AlarmRange against the attacker.
func (g *Game) alarm(m *Monster, actor int) {
	fac := m.Def().Faction
	woke := 0
	for _, o := range g.Mons {
		if o.HP <= 0 || o.Def().Faction != fac || dist(m.X, m.Y, o.X, o.Y) > AlarmRange {
			continue
		}
		if o.Neutral {
			woke++
		}
		o.Neutral, o.Awake = false, true
		if g.heroByID[actor] != nil {
			g.addThreat(o, actor, 1)
		}
	}
	if woke > 0 {
		g.emit(Event{K: "alarm", Actor: actor, Target: m.ID, X: m.X, Y: m.Y, Amount: woke})
	}
}

// queenAct lays a soldier next to the queen while she sees a hero (up to QueenMax alive).
func (g *Game) queenAct(m *Monster, tgt *Hero) {
	def := m.Def()
	if g.Now < m.NextSpawn || dist(m.X, m.Y, tgt.X, tgt.Y) > QueenRange || !g.Level.LineClear(m.X, m.Y, tgt.X, tgt.Y) {
		return
	}
	alive := 0
	for _, o := range g.Mons {
		if o.Parent == m.ID && o.HP > 0 {
			alive++
		}
	}
	if alive >= QueenMax {
		return
	}
	if p, ok := g.freeNear(m.X, m.Y); ok {
		s := g.addMon(def.Summon, p.X, p.Y, 1)
		s.Parent, s.Summoned = m.ID, true
		g.addThreat(s, tgt.ID, 1)
		m.NextSpawn = g.Now + QueenEvery
		g.emit(Event{K: "summon", Actor: m.ID, Target: s.ID, X: p.X, Y: p.Y, Text: MonDefs[def.Summon].Name, FX: string(rune(m.Kind))})
	}
}

// callerAct runs a caller's turn: summon when it can, back away from heroes that get close, and hold its
// distance. It reports whether it used the turn (false = act like any other monster).
func (g *Game) callerAct(m *Monster, tgt *Hero) bool {
	def := m.Def()
	d := dist(m.X, m.Y, tgt.X, tgt.Y)
	if d <= CallerRange && g.Now >= m.NextSpawn && g.Level.LineClear(m.X, m.Y, tgt.X, tgt.Y) {
		alive := 0
		for _, o := range g.Mons {
			if o.Parent == m.ID && o.HP > 0 {
				alive++
			}
		}
		if alive < CallerMax {
			if p, ok := g.freeNear(m.X, m.Y); ok {
				r := g.addMon(def.Summon, p.X, p.Y, 1)
				r.Parent, r.Summoned = m.ID, true
				m.NextSpawn = g.Now + CallerEvery
				g.emit(Event{K: "summon", Actor: m.ID, Target: r.ID, X: p.X, Y: p.Y, Text: MonDefs[def.Summon].Name, FX: string(rune(m.Kind))})
				return true
			}
		}
	}
	if cheb(m.X, m.Y, tgt.X, tgt.Y) <= CallerKeepOff {
		if g.Now >= m.NextMove { // back off to the free neighbour farthest from the hero
			best, bd := Pt{}, d
			for _, v := range dirs8 {
				n := Pt{m.X + v.X, m.Y + v.Y}
				if nd := dist(n.X, n.Y, tgt.X, tgt.Y); nd > bd && g.cellFree(n.X, n.Y) {
					best, bd = n, nd
				}
			}
			if bd > d {
				m.NextMove = g.Now + def.MoveTicks
				g.moveEntity(int32(m.ID), &m.X, &m.Y, best.X, best.Y)
				return true
			}
		}
		return false // cornered: fight
	}
	return d <= CallerHold
}

// revealMimic wakes a mimic that a hero bumped: it bites at once.
func (g *Game) revealMimic(m *Monster, h *Hero) {
	m.Disguised, m.Awake = false, true
	g.addThreat(m, h.ID, 1)
	g.emit(Event{K: "mimic", Actor: h.ID, Target: m.ID, X: m.X, Y: m.Y})
	dmg := int(math.Round(float64(m.Def().Dmg) * MimicBite * Difficulties[g.Settings.Difficulty].DmgMult * DepthDmgMult(g.Depth)))
	m.Facing = dirIndex(h.X-m.X, h.Y-m.Y, m.Facing)
	m.NextAtk = g.Now + m.Def().AtkTicks
	g.damageHero(h, dmg, m.ID)
}

func (g *Game) monsterAI(m *Monster) {
	if m.Disguised {
		return
	}
	if m.Neutral { // busy carrying food: an aimless step now and then
		if g.Now >= m.NextMove && g.rng.Chance(0.3) {
			d := dirs8[g.rng.Intn(8)]
			if g.cellFree(m.X+d.X, m.Y+d.Y) {
				m.Facing = dirIndex(d.X, d.Y, m.Facing)
				g.moveEntity(int32(m.ID), &m.X, &m.Y, m.X+d.X, m.Y+d.Y)
			}
		}
		if g.Now >= m.NextMove {
			m.NextMove = g.Now + m.Def().MoveTicks*2
		}
		return
	}
	if g.Now < m.BleedUntil && g.Now%TickHz == 0 {
		g.damageMonster(m, m.BleedDmg, m.BleedOwner, false)
		if m.HP <= 0 {
			return
		}
	}
	if g.Now < m.StunUntil {
		return
	}
	tgt := g.pickTarget(m)
	if tgt == nil {
		return
	}
	if !m.Awake {
		d := dist(m.X, m.Y, tgt.X, tgt.Y)
		if d > AggroRange || !g.Level.LineClear(m.X, m.Y, tgt.X, tgt.Y) {
			return
		}
		m.Awake = true
	}
	if dist(m.X, m.Y, tgt.X, tgt.Y) > LeashRange {
		m.Awake = false
		return
	}
	if m.Def().Role == "caller" && g.callerAct(m, tgt) {
		return
	}
	switch m.Def().Role {
	case "ranged":
		if g.rangedAct(m, tgt) {
			return
		}
	case "dragon":
		if g.dragonAct(m, tgt) {
			return
		}
	case "charmer":
		if g.charmerAct(m, tgt) {
			return
		}
	}
	if m.Def().Role == "queen" {
		g.queenAct(m, tgt)
		if cheb(m.X, m.Y, tgt.X, tgt.Y) > 1 { // she never walks; she bites only what comes close
			return
		}
	}
	if cheb(m.X, m.Y, tgt.X, tgt.Y) <= 1 {
		if g.Now >= m.NextAtk {
			def := m.Def()
			m.NextAtk = g.Now + def.AtkTicks
			dmg := int(math.Round(float64(def.Dmg) * Difficulties[g.Settings.Difficulty].DmgMult * DepthDmgMult(g.Depth)))
			if m.Elite {
				dmg += 2 * (g.N - 1)
			}
			dx, dy := tgt.X-m.X, tgt.Y-m.Y
			m.Facing = dirIndex(dx, dy, m.Facing)
			g.damageHero(tgt, dmg, m.ID)
			if def.Role == "drainer" {
				g.drainBite(m, tgt)
			}
		}
		return
	}
	if g.Now < m.NextMove {
		return
	}
	cost := m.Def().MoveTicks
	if g.Now < m.SlowUntil {
		cost *= 2
	}
	m.NextMove = g.Now + cost
	var step Pt
	var ok bool
	if m.Kind == 'b' && g.rng.Chance(0.4) {
		d := dirs8[g.rng.Intn(8)]
		step, ok = Pt{m.X + d.X, m.Y + d.Y}, true
		if !g.cellFree(step.X, step.Y) {
			ok = false
		}
	} else {
		step, ok = g.stepToward(m.X, m.Y, tgt.X, tgt.Y)
	}
	if ok {
		m.Facing = dirIndex(step.X-m.X, step.Y-m.Y, m.Facing)
		px, py := m.X, m.Y
		g.moveEntity(int32(m.ID), &m.X, &m.Y, step.X, step.Y)
		if m.Def().Trail {
			g.addFire(px, py, FireTicks, FireDmg)
		}
	}
}

// dirIndex maps a delta to one of the 8 directions (keeps def when delta is zero).
func dirIndex(dx, dy, def int) int {
	sx, sy := sign(dx), sign(dy)
	for i, d := range dirs8 {
		if d.X == sx && d.Y == sy {
			return i
		}
	}
	return def
}

func sign(v int) int {
	if v < 0 {
		return -1
	}
	if v > 0 {
		return 1
	}
	return 0
}

// stepToward runs a bounded BFS to a cell adjacent to the target and returns the first step.
func (g *Game) stepToward(sx, sy, tx, ty int) (Pt, bool) {
	g.stamp++
	st := g.stamp
	start := int32(sy*LW + sx)
	g.bfsStamp[start] = st
	g.bfsPar[start] = -1
	q := []int32{start}
	expanded := 0
	for len(q) > 0 && expanded < 700 {
		c := q[0]
		q = q[1:]
		expanded++
		cx, cy := int(c)%LW, int(c)/LW
		if cheb(cx, cy, tx, ty) <= 1 && c != start {
			// walk back to the first step
			for g.bfsPar[c] != start {
				c = g.bfsPar[c]
			}
			return Pt{int(c) % LW, int(c) / LW}, true
		}
		for _, d := range dirs8 {
			nx, ny := cx+d.X, cy+d.Y
			if nx < 0 || ny < 0 || nx >= LW || ny >= LH {
				continue
			}
			ni := int32(ny*LW + nx)
			if g.bfsStamp[ni] == st || !g.cellFree(nx, ny) {
				continue
			}
			g.bfsStamp[ni] = st
			g.bfsPar[ni] = c
			q = append(q, ni)
		}
	}
	return Pt{}, false
}
