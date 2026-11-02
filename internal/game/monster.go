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
	Threat    []ThreatEntry

	SlowUntil  int
	BleedUntil int
	BleedDmg   int
	BleedOwner int
	StunUntil  int
}

func (m *Monster) Def() MonDef { return MonDefs[m.Kind] }

// damageMonster applies damage from a hero, awards resource, handles death and loot.
func (g *Game) damageMonster(m *Monster, dmg int, actor int, crit bool) {
	if m.HP <= 0 || dmg <= 0 {
		return
	}
	m.HP -= dmg
	m.Awake = true
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
	g.emit(Event{K: "kill", Actor: killer, Target: m.ID, X: m.X, Y: m.Y, Text: def.Name, FX: string(rune(m.Kind))})
	if m.Elite {
		g.KingKilled = true
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

func (g *Game) monsterAI(m *Monster) {
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
		g.moveEntity(int32(m.ID), &m.X, &m.Y, step.X, step.Y)
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
