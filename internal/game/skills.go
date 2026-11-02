package game

import "math"

func (h *Hero) dmgScale(base int) int {
	b := h.buff
	if b == 0 {
		b = 1
	}
	return int(math.Round(float64(base) * (1 + 0.1*float64(h.Lvl-1)) * b))
}

func (g *Game) applyCast(h *Hero, in Intent) error {
	if in.Slot < 0 || in.Slot > 2 {
		return ErrBadIntent
	}
	idx := h.Binds[in.Slot]
	if idx < 0 || h.Ranks[idx] == 0 {
		return ErrLocked
	}
	sk := h.Def().Skills[idx].Ranked(h.Ranks[idx])
	if g.Now < h.CDReady[idx] {
		return ErrCooldown
	}
	if h.Res < sk.Cost {
		return ErrResource
	}
	if in.X < 0 || in.Y < 0 || in.X >= LW || in.Y >= LH {
		return ErrBadTarget
	}
	if sk.Range > 0 && !sk.Melee {
		if dist(h.X, h.Y, in.X, in.Y) > sk.Range+0.5 {
			return ErrRange
		}
		if (in.X != h.X || in.Y != h.Y) && !g.Level.LineClear(h.X, h.Y, in.X, in.Y) {
			return ErrLOS
		}
		if Solid(g.Level.At(in.X, in.Y)) {
			return ErrBlocked
		}
	}
	h.buff = 1
	if g.Now < h.CryUntil {
		h.buff = 1 + CryBuffPct/100.0
	}
	err := g.castSkill(h, sk, in.X, in.Y)
	h.buff = 1
	if err != nil {
		return err
	}
	h.Res -= sk.Cost
	h.CDReady[idx] = g.Now + sk.CD
	h.ReviveTarget, h.ReviveProg = 0, 0
	return nil
}

func (g *Game) castSkill(h *Hero, sk SkillDef, tx, ty int) error {
	switch sk.ID {
	case "spark":
		return g.castSpark(h, sk, tx, ty)
	case "nova":
		return g.castDelayed(h, sk, tx, ty)
	case "blink":
		return g.castBlink(h, sk, tx, ty)
	case "cleave":
		return g.castCleave(h, sk, tx, ty)
	case "ironwall":
		return g.castIronWall(h, sk)
	case "leap":
		return g.castLeap(h, sk, tx, ty)
	case "stab":
		return g.castStab(h, sk, tx, ty)
	case "caltrops":
		return g.castCaltrops(h, sk, tx, ty)
	case "shadowbomb":
		return g.castDelayed(h, sk, tx, ty)
	case "palm":
		return g.castPalm(h, sk, tx, ty)
	case "dash":
		return g.castDash(h, sk, tx, ty)
	case "bell":
		return g.castBell(h, sk)
	case "shieldbash":
		return g.castShieldBash(h, sk, tx, ty)
	case "warcry":
		return g.castWarCry(h, sk)
	case "whirlwind":
		return g.castWhirlwind(h, sk)
	case "poisonblade":
		return g.castPoisonBlade(h, sk)
	case "smokestep":
		return g.castSmokeStep(h, sk)
	case "knives":
		return g.castKnives(h, sk, tx, ty)
	case "frostring":
		return g.castFrostRing(h, sk)
	case "light":
		return g.castLight(h, sk, tx, ty)
	case "chainspark":
		return g.castChainSpark(h, sk, tx, ty)
	case "staffsweep":
		return g.castStaffSweep(h, sk, tx, ty)
	case "healbreath":
		return g.castHealBreath(h, sk)
	case "fists":
		return g.castFists(h, sk, tx, ty)
	}
	return ErrBadIntent
}

func (g *Game) castSpark(h *Hero, sk SkillDef, tx, ty int) error {
	pts := Line(h.X, h.Y, tx, ty)
	ex, ey := tx, ty
	var hit *Monster
	for i := 1; i < len(pts); i++ {
		p := pts[i]
		if Solid(g.Level.At(p.X, p.Y)) {
			break
		}
		ex, ey = p.X, p.Y
		if id := g.occ[p.Y*LW+p.X]; id >= monIDBase {
			hit = g.monByID[int(id)]
			break
		}
	}
	h.Facing = dirIndex(tx-h.X, ty-h.Y, h.Facing)
	g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: ex, TY: ey, FX: sk.ID})
	if hit != nil {
		g.damageMonster(hit, h.dmgScale(sk.Dmg), h.ID, false)
	}
	return nil
}

// castDelayed schedules an area impact Delay ticks later (charge + projectile on the client).
func (g *Game) castDelayed(h *Hero, sk SkillDef, tx, ty int) error {
	h.Facing = dirIndex(tx-h.X, ty-h.Y, h.Facing)
	g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: tx, TY: ty, FX: sk.ID, R: sk.Radius, Amount: sk.Delay})
	g.pend = append(g.pend, pending{at: g.Now + sk.Delay, skill: sk.ID, caster: h.ID, x: tx, y: ty, dmg: h.dmgScale(sk.Dmg), radius: sk.Radius})
	return nil
}

func (g *Game) runPending() {
	// split first: an impact may queue new effects (a keg blast lights the next keg), and those must survive
	var due []pending
	n := 0
	for _, p := range g.pend {
		if p.at > g.Now {
			g.pend[n] = p
			n++
		} else {
			due = append(due, p)
		}
	}
	g.pend = g.pend[:n]
	for _, p := range due {
		g.impact(p)
	}
}

func (g *Game) impact(p pending) {
	if p.skill == "keg" { // a powder keg going off (it may have been lit by a trap, so no hero needed)
		g.kegBlast(p)
		return
	}
	h := g.heroByID[p.caster]
	if h == nil {
		return
	}
	if def := Items[p.skill]; def != nil { // a thrown item
		g.emit(Event{K: "impact", Actor: h.ID, X: p.x, Y: p.y, FX: def.ID, R: def.Radius})
		g.areaDamage(h, p.x, p.y, def.Radius, h.dmgScale(int(def.Amount)))
		return
	}
	if p.skill == "fists" { // one blow of Hundred Fists on whatever stands there now
		if m := g.monsterAt(p.x, p.y); m != nil {
			g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: p.x, TY: p.y, FX: "fist"})
			g.damageMonster(m, p.dmg, h.ID, false)
		}
		return
	}
	g.emit(Event{K: "impact", Actor: h.ID, X: p.x, Y: p.y, FX: p.skill, R: p.radius})
	g.areaDamage(h, p.x, p.y, p.radius, p.dmg)
}

// areaDamage hits every monster within radius (aspect metric) and, with friendly fire on, other heroes.
func (g *Game) areaDamage(h *Hero, cx, cy int, radius float64, dmg int) {
	for _, m := range append([]*Monster(nil), g.Mons...) {
		if m.HP > 0 && dist(cx, cy, m.X, m.Y) <= radius {
			g.damageMonster(m, dmg, h.ID, false)
		}
	}
	if g.Settings.FriendlyFire {
		for _, o := range g.Heroes {
			if o != h && !o.Dead && o.Connected && dist(cx, cy, o.X, o.Y) <= radius {
				g.damageHero(o, max(1, dmg/2), h.ID)
			}
		}
	}
}

func (g *Game) castBlink(h *Hero, sk SkillDef, tx, ty int) error {
	if !g.cellFree(tx, ty) {
		return ErrBlocked
	}
	g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: tx, TY: ty, FX: sk.ID})
	g.moveEntity(int32(h.ID), &h.X, &h.Y, tx, ty)
	g.pickup(h)
	return nil
}
