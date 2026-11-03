package game

import "math"

// Deep monsters: ranged shooters, burning ground, the dragon's breath, charm, drain and blink, and shades.

// Fire is a burning cell that hurts heroes standing in it.
type Fire struct {
	X, Y  int
	Until int
	Dmg   int
}

func (g *Game) addFire(x, y, ticks, dmg int) {
	if Solid(g.Level.At(x, y)) {
		return
	}
	for i := range g.Fires { // refresh a fire that's already there
		if g.Fires[i].X == x && g.Fires[i].Y == y {
			g.Fires[i].Until = max(g.Fires[i].Until, g.Now+ticks)
			return
		}
	}
	g.Fires = append(g.Fires, Fire{X: x, Y: y, Until: g.Now + ticks, Dmg: dmg})
}

// tickFires burns out old fires and, once a second, hurts heroes standing in one.
func (g *Game) tickFires() {
	n := 0
	for _, f := range g.Fires {
		if f.Until > g.Now {
			g.Fires[n] = f
			n++
		}
	}
	g.Fires = g.Fires[:n]
	if g.Now%TickHz != 0 || len(g.Fires) == 0 {
		return
	}
	for _, h := range g.Heroes {
		for _, f := range g.Fires {
			if f.X == h.X && f.Y == h.Y {
				g.damageHero(h, f.Dmg, 0, "burning ground")
				break
			}
		}
	}
}

// monsterDmg is a monster's hit after difficulty and depth.
func (g *Game) monsterDmg(m *Monster, base float64) int {
	if m.Named != "" {
		base *= NamedDmgMult
	}
	if g.Event == "cursed" {
		base *= CursedDmgMult
	}
	dmg := int(math.Round(base * Difficulties[g.Settings.Difficulty].DmgMult * DepthDmgMult(g.Depth)))
	if m.Elite {
		dmg += 2 * (g.N - 1)
	}
	return dmg
}

// rangedAct: shoot when in range and sight, back away if it's a skirmisher and a hero gets close.
// Reports whether it used the turn.
func (g *Game) rangedAct(m *Monster, tgt *Hero) bool {
	def := m.Def()
	d := dist(m.X, m.Y, tgt.X, tgt.Y)
	close := cheb(m.X, m.Y, tgt.X, tgt.Y)
	if def.KeepOff && close <= 2 && g.Now >= m.NextMove && g.stepAway(m, tgt) {
		return true
	}
	if close <= 1 && !def.KeepOff {
		return false // bite like anyone else
	}
	los := g.Level.LineClear(m.X, m.Y, tgt.X, tgt.Y)
	if d <= def.Range && los {
		if g.Now >= m.NextAtk {
			m.NextAtk = g.Now + def.AtkTicks
			m.Facing = dirIndex(tgt.X-m.X, tgt.Y-m.Y, m.Facing)
			g.emit(Event{K: "shot", Actor: m.ID, X: m.X, Y: m.Y, TX: tgt.X, TY: tgt.Y, FX: string(rune(m.Kind))})
			g.damageHero(tgt, g.monsterDmg(m, float64(def.Dmg)), m.ID, shotName(m))
		}
		return true // hold position while in range
	}
	return false
}

// stepAway moves m to the free neighbour farthest from the hero, if that's farther than where it is.
func (g *Game) stepAway(m *Monster, tgt *Hero) bool {
	d := dist(m.X, m.Y, tgt.X, tgt.Y)
	best, bd := Pt{}, d
	for _, v := range dirs8 {
		n := Pt{m.X + v.X, m.Y + v.Y}
		if nd := dist(n.X, n.Y, tgt.X, tgt.Y); nd > bd && g.cellFree(n.X, n.Y) {
			best, bd = n, nd
		}
	}
	if bd <= d {
		return false
	}
	m.NextMove = g.Now + m.Def().StepTicks()
	g.moveEntity(int32(m.ID), &m.X, &m.Y, best.X, best.Y)
	return true
}

// dragonAct: wind up a breath at a hero in reach, then breathe a burning cone. Reports whether it used the turn.
func (g *Game) dragonAct(m *Monster, tgt *Hero) bool {
	if m.BreathAt > 0 {
		if g.Now >= m.BreathAt {
			g.breathe(m)
		}
		return true // stands still while it breathes
	}
	if g.Now >= m.NextAbility && dist(m.X, m.Y, tgt.X, tgt.Y) <= BreathRange && g.Level.LineClear(m.X, m.Y, tgt.X, tgt.Y) {
		m.BreathAt, m.BreathX, m.BreathY = g.Now+BreathWind, tgt.X, tgt.Y
		m.NextAbility = g.Now + BreathCD
		m.Facing = dirIndex(tgt.X-m.X, tgt.Y-m.Y, m.Facing)
		g.emit(Event{K: "breath", Actor: m.ID, X: m.X, Y: m.Y, TX: tgt.X, TY: tgt.Y, R: BreathRange, Amount: BreathWind})
		return true
	}
	return false
}

// inCone reports whether (x, y) lies in the dragon's 90-degree cone toward (tx, ty), within BreathRange.
func inCone(m *Monster, tx, ty, x, y int) bool {
	if x == m.X && y == m.Y {
		return false
	}
	ax, ay := float64(tx-m.X), float64(ty-m.Y)*2
	bx, by := float64(x-m.X), float64(y-m.Y)*2
	la, lb := math.Hypot(ax, ay), math.Hypot(bx, by)
	if la == 0 || lb > BreathRange {
		return false
	}
	return (ax*bx+ay*by)/(la*lb) >= math.Cos(math.Pi/4)
}

func (g *Game) breathe(m *Monster) {
	tx, ty := m.BreathX, m.BreathY
	m.BreathAt = 0
	g.emit(Event{K: "breathfire", Actor: m.ID, X: m.X, Y: m.Y, TX: tx, TY: ty, R: BreathRange})
	r := int(BreathRange) + 1
	for y := m.Y - r/2 - 1; y <= m.Y+r/2+1; y++ {
		for x := m.X - r; x <= m.X+r; x++ {
			if inCone(m, tx, ty, x, y) && !Solid(g.Level.At(x, y)) && g.Level.LineClear(m.X, m.Y, x, y) {
				g.addFire(x, y, BreathFire, FireDmg)
			}
		}
	}
	dmg := g.monsterDmg(m, float64(m.Def().Dmg)*1.5)
	for _, h := range g.Heroes {
		if !h.Dead && inCone(m, tx, ty, h.X, h.Y) && g.Level.LineClear(m.X, m.Y, h.X, h.Y) {
			g.damageHero(h, dmg, m.ID, "fire breath")
		}
	}
}

// charmerAct: charm the hero if she can (they freeze and drift toward her for CharmTicks).
func (g *Game) charmerAct(m *Monster, tgt *Hero) bool {
	if g.Now < m.NextAbility || g.Now < tgt.CharmUntil || dist(m.X, m.Y, tgt.X, tgt.Y) > m.Def().Range ||
		!g.Level.LineClear(m.X, m.Y, tgt.X, tgt.Y) {
		return false
	}
	m.NextAbility = g.Now + CharmCD
	tgt.CharmUntil, tgt.CharmBy = g.Now+CharmTicks, m.ID
	tgt.ReviveTarget, tgt.ReviveProg = 0, 0
	g.emit(Event{K: "charm", Actor: m.ID, Target: tgt.ID, X: tgt.X, Y: tgt.Y})
	return true
}

// tickCharm walks charmed heroes toward the one who charmed them.
func (g *Game) tickCharm() {
	for _, h := range g.Heroes {
		if g.Now >= h.CharmUntil || h.Dead || h.Downed || g.Now < h.NextMove {
			continue
		}
		c := g.monByID[h.CharmBy]
		if c == nil || c.HP <= 0 {
			h.CharmUntil = 0
			continue
		}
		if cheb(h.X, h.Y, c.X, c.Y) <= 1 {
			continue
		}
		if p, ok := g.stepToward(h.X, h.Y, c.X, c.Y); ok {
			g.moveEntity(int32(h.ID), &h.X, &h.Y, p.X, p.Y)
			h.NextMove = g.Now + MoveTicks*2
		}
	}
}

// drainBite: an incubus bite also drains resource and heals the incubus by as much.
func (g *Game) drainBite(m *Monster, h *Hero) {
	d := min(h.Res, DrainRes)
	if d <= 0 {
		return
	}
	h.Res -= d
	m.HP = min(m.HPMax, m.HP+d)
	g.emit(Event{K: "drain", Actor: m.ID, Target: h.ID, X: h.X, Y: h.Y, Amount: d})
}

// maybeBlink: a hit incubus sometimes blinks to a free cell 3-6 cells away.
func (g *Game) maybeBlink(m *Monster) {
	if g.Now < m.NextAbility || !g.rng.Chance(BlinkChance) {
		return
	}
	for try := 0; try < 12; try++ {
		x, y := m.X+g.rng.Intn(13)-6, m.Y+g.rng.Intn(7)-3
		if cheb(x, y, m.X, m.Y) >= 3 && g.cellFree(x, y) && g.Level.LineClear(m.X, m.Y, x, y) {
			g.emit(Event{K: "blink", Actor: m.ID, X: m.X, Y: m.Y, TX: x, TY: y})
			g.moveEntity(int32(m.ID), &m.X, &m.Y, x, y)
			m.NextAbility = g.Now + BlinkCD
			return
		}
	}
}

// Revealed reports whether a stealthy monster is close enough to a living hero to be seen.
func (g *Game) Revealed(m *Monster) bool {
	if !m.Def().Stealth {
		return true
	}
	for _, h := range g.Heroes {
		if !h.Dead && cheb(h.X, h.Y, m.X, m.Y) <= StealthReveal {
			return true
		}
	}
	return false
}

// shotName is what a ranged monster shoots ("shot" for one without a name of its own).
func shotName(m *Monster) string {
	if n, ok := shotNames[m.Kind]; ok {
		return n
	}
	return "shot"
}
