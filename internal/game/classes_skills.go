package game

import "math"

// Skills of the Warrior, Rogue and Monk. (Sorcerer skills live in skills.go.)

// facingDir picks the 8-way direction from the hero toward the target cell (the current facing when equal).
func (h *Hero) facingDir(tx, ty int) int {
	return dirIndex(tx-h.X, ty-h.Y, h.Facing)
}

func (g *Game) monsterAt(x, y int) *Monster {
	if x < 0 || y < 0 || x >= LW || y >= LH {
		return nil
	}
	if id := g.occ[y*LW+x]; id >= monIDBase {
		return g.monByID[int(id)]
	}
	return nil
}

// meleeDir returns the aimed direction, or turns toward an adjacent monster when the aimed
// cell(s) are empty, preferring the one angularly closest to the aim.
func (g *Game) meleeDir(h *Hero, tx, ty int, arc bool) int {
	d := h.facingDir(tx, ty)
	ks := []int{0}
	if arc {
		ks = []int{0, 1, -1}
	}
	for _, k := range ks {
		v := dirs8[(d+k+8)%8]
		if g.monsterAt(h.X+v.X, h.Y+v.Y) != nil {
			return d
		}
	}
	best, bd := d, 99
	for i, v := range dirs8 {
		if g.monsterAt(h.X+v.X, h.Y+v.Y) == nil {
			continue
		}
		c := (i - d + 8) % 8
		if c > 4 {
			c = 8 - c
		}
		if c < bd {
			best, bd = i, c
		}
	}
	return best
}

// ---- Warrior ----

// Cleave hits the three cells of a 45-degree arc in front of the hero (adjacent only).
func (g *Game) castCleave(h *Hero, sk SkillDef, tx, ty int) error {
	d := g.meleeDir(h, tx, ty, true)
	h.Facing = d
	front := dirs8[d]
	g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: h.X + front.X, TY: h.Y + front.Y, FX: sk.ID})
	for _, k := range []int{0, 1, -1} {
		v := dirs8[(d+k+8)%8]
		if m := g.monsterAt(h.X+v.X, h.Y+v.Y); m != nil {
			g.damageMonster(m, h.dmgScale(sk.Dmg), h.ID, false)
		}
	}
	return nil
}

// Iron Wall absorbs damage for Dur ticks and draws threat from nearby enemies.
func (g *Game) castIronWall(h *Hero, sk SkillDef) error {
	h.WallUntil = g.Now + sk.Dur
	h.WallAbsorb = sk.Aux
	for _, m := range g.Mons {
		if dist(h.X, h.Y, m.X, m.Y) <= 14 {
			g.addThreat(m, h.ID, IronWallThreat)
		}
	}
	g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: h.X, TY: h.Y, FX: sk.ID})
	return nil
}

// Leap jumps to a free cell in range and line of sight, then hits everything around the landing spot.
func (g *Game) castLeap(h *Hero, sk SkillDef, tx, ty int) error {
	if !g.cellFree(tx, ty) {
		return ErrBlocked
	}
	g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: tx, TY: ty, FX: sk.ID})
	h.Facing = h.facingDir(tx, ty)
	g.moveEntity(int32(h.ID), &h.X, &h.Y, tx, ty)
	g.emit(Event{K: "impact", Actor: h.ID, X: tx, Y: ty, FX: sk.ID, R: sk.Radius})
	g.areaDamage(h, tx, ty, sk.Radius, h.dmgScale(sk.Dmg))
	g.pickup(h)
	return nil
}

// ---- Rogue ----

// behind reports whether the attacker stands behind the monster (opposite to where it faces).
func behind(m *Monster, ax, ay int) bool {
	f := dirs8[m.Facing]
	return f.X*(ax-m.X)+f.Y*(ay-m.Y) < 0
}

func (g *Game) castStab(h *Hero, sk SkillDef, tx, ty int) error {
	d := g.meleeDir(h, tx, ty, false)
	h.Facing = d
	v := dirs8[d]
	g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: h.X + v.X, TY: h.Y + v.Y, FX: sk.ID})
	if m := g.monsterAt(h.X+v.X, h.Y+v.Y); m != nil {
		dmg, crit := h.dmgScale(sk.Dmg), false
		if behind(m, h.X, h.Y) {
			dmg, crit = dmg*2, true
		}
		g.damageMonster(m, dmg, h.ID, crit)
		if h.PoisonCharges > 0 && m.HP > 0 { // Poison Blade
			h.PoisonCharges--
			m.BleedUntil, m.BleedDmg, m.BleedOwner = g.Now+h.PoisonDur, h.PoisonDmg, h.ID
		}
	}
	return nil
}

// Caltrops drops a 3x3 patch that slows and bleeds enemies standing in it.
func (g *Game) castCaltrops(h *Hero, sk SkillDef, tx, ty int) error {
	g.Patches = append(g.Patches, &Patch{X: tx, Y: ty, Until: g.Now + sk.Dur, Owner: h.ID, Dmg: sk.Dmg})
	g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: tx, TY: ty, FX: sk.ID, R: sk.Radius})
	return nil
}

func (g *Game) tickPatches() {
	n := 0
	for _, p := range g.Patches {
		if g.Now >= p.Until {
			continue
		}
		g.Patches[n] = p
		n++
		for _, m := range g.Mons {
			if abs(m.X-p.X) <= 1 && abs(m.Y-p.Y) <= 1 {
				m.SlowUntil = g.Now + 10
				m.BleedUntil = g.Now + Sec(3)
				m.BleedDmg = p.Dmg
				m.BleedOwner = p.Owner
			}
		}
	}
	g.Patches = g.Patches[:n]
}

// ---- Monk ----

// Palm Strike hits the adjacent enemy and knocks it back Aux cells, stopping at walls and other bodies.
func (g *Game) castPalm(h *Hero, sk SkillDef, tx, ty int) error {
	d := g.meleeDir(h, tx, ty, false)
	h.Facing = d
	v := dirs8[d]
	g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: h.X + v.X, TY: h.Y + v.Y, FX: sk.ID})
	if m := g.monsterAt(h.X+v.X, h.Y+v.Y); m != nil {
		g.damageMonster(m, h.dmgScale(sk.Dmg), h.ID, false)
		if m.HP > 0 {
			g.knockback(m, v, sk.Aux)
		}
	}
	return nil
}

func (g *Game) knockback(m *Monster, v Pt, cells int) {
	for i := 0; i < cells; i++ {
		nx, ny := m.X+v.X, m.Y+v.Y
		if !g.cellFree(nx, ny) {
			return
		}
		g.moveEntity(int32(m.ID), &m.X, &m.Y, nx, ny)
	}
}

// Dash moves up to Aux cells in a line, passing through enemies (and hurting them) but not walls.
func (g *Game) castDash(h *Hero, sk SkillDef, tx, ty int) error {
	d := h.facingDir(tx, ty)
	v := dirs8[d]
	x, y := h.X, h.Y
	lastFree := Pt{h.X, h.Y}
	var hit []*Monster
	for i := 0; i < sk.Aux; i++ {
		nx, ny := x+v.X, y+v.Y
		if Solid(g.Level.At(nx, ny)) {
			break
		}
		x, y = nx, ny
		if m := g.monsterAt(x, y); m != nil {
			hit = append(hit, m)
		} else if g.occ[y*LW+x] == 0 {
			lastFree = Pt{x, y}
		}
	}
	if lastFree.X == h.X && lastFree.Y == h.Y {
		return ErrBlocked
	}
	h.Facing = d
	g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: lastFree.X, TY: lastFree.Y, FX: sk.ID})
	g.moveEntity(int32(h.ID), &h.X, &h.Y, lastFree.X, lastFree.Y)
	for _, m := range hit {
		g.damageMonster(m, h.dmgScale(sk.Dmg), h.ID, false)
	}
	g.pickup(h)
	return nil
}

// Bell of Warding gives every ally within the radius reduced damage taken for Dur ticks.
func (g *Game) castBell(h *Hero, sk SkillDef) error {
	g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: h.X, TY: h.Y, FX: sk.ID, R: sk.Radius})
	for _, o := range g.Heroes {
		if !o.Dead && dist(h.X, h.Y, o.X, o.Y) <= sk.Radius {
			o.BellUntil = g.Now + sk.Dur
		}
	}
	return nil
}

// ---- Skill-tree skills ----

// firstHit walks the line toward (tx, ty) for up to rng cells and returns the end point and the first monster.
func (g *Game) firstHit(x0, y0, tx, ty int, rng float64) (Pt, *Monster) {
	end := Pt{x0, y0}
	for i, p := range Line(x0, y0, tx, ty) {
		if i == 0 {
			continue
		}
		if Solid(g.Level.At(p.X, p.Y)) || dist(x0, y0, p.X, p.Y) > rng+0.5 {
			break
		}
		end = p
		if m := g.monsterAt(p.X, p.Y); m != nil {
			return end, m
		}
	}
	return end, nil
}

// Shield Bash hits the enemy in front and stuns it for Dur ticks.
func (g *Game) castShieldBash(h *Hero, sk SkillDef, tx, ty int) error {
	d := g.meleeDir(h, tx, ty, false)
	h.Facing = d
	v := dirs8[d]
	g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: h.X + v.X, TY: h.Y + v.Y, FX: sk.ID})
	if m := g.monsterAt(h.X+v.X, h.Y+v.Y); m != nil {
		g.damageMonster(m, h.dmgScale(sk.Dmg), h.ID, false)
		if m.HP > 0 {
			m.StunUntil = g.Now + sk.Dur
		}
	}
	return nil
}

// War Cry pulls every enemy in the radius onto the warrior and makes nearby heroes hit harder.
func (g *Game) castWarCry(h *Hero, sk SkillDef) error {
	for _, m := range g.Mons {
		if !m.Disguised && dist(h.X, h.Y, m.X, m.Y) <= sk.Radius {
			m.Awake = true
			g.addThreat(m, h.ID, IronWallThreat)
		}
	}
	for _, o := range g.Heroes {
		if !o.Dead && dist(h.X, h.Y, o.X, o.Y) <= sk.Radius {
			o.CryUntil = g.Now + sk.Dur
		}
	}
	g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: h.X, TY: h.Y, FX: sk.ID, R: sk.Radius})
	return nil
}

// Whirlwind hits every adjacent enemy.
func (g *Game) castWhirlwind(h *Hero, sk SkillDef) error {
	g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: h.X, TY: h.Y, FX: sk.ID, R: 1.5})
	for _, v := range dirs8 {
		if m := g.monsterAt(h.X+v.X, h.Y+v.Y); m != nil {
			g.damageMonster(m, h.dmgScale(sk.Dmg), h.ID, false)
		}
	}
	return nil
}

// Poison Blade makes the next Aux stabs poison the target (Dmg per second for Dur ticks).
func (g *Game) castPoisonBlade(h *Hero, sk SkillDef) error {
	h.PoisonCharges, h.PoisonDmg, h.PoisonDur = sk.Aux, h.dmgScale(sk.Dmg), sk.Dur
	g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: h.X, TY: h.Y, FX: sk.ID})
	return nil
}

// Smoke Step hides the rogue for Dur ticks: monsters forget them and can't pick them as a target.
func (g *Game) castSmokeStep(h *Hero, sk SkillDef) error {
	h.HiddenUntil = g.Now + sk.Dur
	for _, m := range g.Mons {
		n := 0
		for _, e := range m.Threat {
			if e.Hero != h.ID {
				m.Threat[n] = e
				n++
			}
		}
		m.Threat = m.Threat[:n]
	}
	g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: h.X, TY: h.Y, FX: sk.ID})
	return nil
}

// Fan of Knives throws Aux knives in a cone toward the aim; each hits the first enemy on its line.
func (g *Game) castKnives(h *Hero, sk SkillDef, tx, ty int) error {
	d := h.facingDir(tx, ty)
	h.Facing = d
	n := sk.Aux
	for k := -(n / 2); k <= n/2; k++ {
		// aim each knife at a point Range cells away, spread across neighbouring directions
		a := math.Atan2(float64(dirs8[d].Y)*2, float64(dirs8[d].X)) + float64(k)*0.3
		ex := h.X + int(math.Round(math.Cos(a)*sk.Range))
		ey := h.Y + int(math.Round(math.Sin(a)*sk.Range/2))
		end, m := g.firstHit(h.X, h.Y, ex, ey, sk.Range)
		g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: end.X, TY: end.Y, FX: sk.ID})
		if m != nil {
			g.damageMonster(m, h.dmgScale(sk.Dmg), h.ID, false)
		}
	}
	return nil
}

// Frost Ring slows every enemy in the radius around the sorcerer and nips them.
func (g *Game) castFrostRing(h *Hero, sk SkillDef) error {
	g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: h.X, TY: h.Y, FX: sk.ID, R: sk.Radius})
	for _, m := range append([]*Monster(nil), g.Mons...) {
		if m.HP > 0 && !m.Disguised && dist(h.X, h.Y, m.X, m.Y) <= sk.Radius {
			m.SlowUntil = g.Now + sk.Dur
			g.damageMonster(m, h.dmgScale(sk.Dmg), h.ID, false)
		}
	}
	return nil
}

// Light puts a light orb at the cursor for Dur ticks (it lets the party see on the dark floor).
func (g *Game) castLight(h *Hero, sk SkillDef, tx, ty int) error {
	if len(g.Orbs) >= MaxOrbs {
		g.Orbs = g.Orbs[1:]
	}
	g.Orbs = append(g.Orbs, Orb{X: tx, Y: ty, Until: g.Now + sk.Dur})
	g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: tx, TY: ty, FX: sk.ID})
	return nil
}

// Chain Spark hits the first enemy on the line, then jumps up to Aux times to the nearest enemy within
// Radius that it hasn't hit yet, losing 20% each jump.
func (g *Game) castChainSpark(h *Hero, sk SkillDef, tx, ty int) error {
	h.Facing = dirIndex(tx-h.X, ty-h.Y, h.Facing)
	end, m := g.firstHit(h.X, h.Y, tx, ty, sk.Range)
	g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: end.X, TY: end.Y, FX: sk.ID})
	hit := map[int]bool{}
	dmg := float64(h.dmgScale(sk.Dmg))
	for jumps := 0; m != nil; jumps++ {
		hit[m.ID] = true
		from := Pt{m.X, m.Y}
		g.damageMonster(m, int(math.Round(dmg)), h.ID, false)
		if jumps == sk.Aux {
			break
		}
		dmg *= 0.8
		var next *Monster
		bd := sk.Radius
		for _, o := range g.Mons {
			if o.HP > 0 && !o.Disguised && !hit[o.ID] {
				if d := dist(from.X, from.Y, o.X, o.Y); d <= bd && g.Level.LineClear(from.X, from.Y, o.X, o.Y) {
					next, bd = o, d
				}
			}
		}
		if next != nil {
			g.emit(Event{K: "cast", Actor: h.ID, X: from.X, Y: from.Y, TX: next.X, TY: next.Y, FX: sk.ID})
		}
		m = next
	}
	return nil
}

// Staff Sweep swings the monk's staff in a 3-direction arc that reaches 2 cells.
func (g *Game) castStaffSweep(h *Hero, sk SkillDef, tx, ty int) error {
	d := g.meleeDir(h, tx, ty, true)
	h.Facing = d
	front := dirs8[d]
	g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: h.X + front.X*2, TY: h.Y + front.Y*2, FX: sk.ID})
	seen := map[int]bool{}
	for _, k := range []int{0, 1, -1} {
		v := dirs8[(d+k+8)%8]
		for reach := 1; reach <= 2; reach++ {
			x, y := h.X+v.X*reach, h.Y+v.Y*reach
			if Solid(g.Level.At(x, y)) {
				break
			}
			if m := g.monsterAt(x, y); m != nil && !seen[m.ID] {
				seen[m.ID] = true
				g.damageMonster(m, h.dmgScale(sk.Dmg), h.ID, false)
			}
		}
	}
	return nil
}

// Healing Breath heals the monk and every ally in the radius by Dmg percent of their max HP.
func (g *Game) castHealBreath(h *Hero, sk SkillDef) error {
	g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: h.X, TY: h.Y, FX: sk.ID, R: sk.Radius})
	for _, o := range g.Heroes {
		if o.Dead || o.Downed || dist(h.X, h.Y, o.X, o.Y) > sk.Radius {
			continue
		}
		heal := o.HPMax * sk.Dmg / 100
		o.HP = min(o.HPMax, o.HP+heal)
		g.emit(Event{K: "potion", Actor: o.ID, X: o.X, Y: o.Y, Amount: heal})
	}
	return nil
}

// Hundred Fists lands Aux blows on the cell in front, Delay ticks apart.
func (g *Game) castFists(h *Hero, sk SkillDef, tx, ty int) error {
	d := g.meleeDir(h, tx, ty, false)
	h.Facing = d
	v := dirs8[d]
	x, y := h.X+v.X, h.Y+v.Y
	if g.monsterAt(x, y) == nil {
		return ErrBadTarget // nothing to punch: don't waste the Ki
	}
	g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: x, TY: y, FX: sk.ID})
	dmg := h.dmgScale(sk.Dmg)
	for k := 0; k < sk.Aux; k++ {
		g.pend = append(g.pend, pending{at: g.Now + k*sk.Delay, skill: "fists", caster: h.ID, x: x, y: y, dmg: dmg})
	}
	return nil
}
