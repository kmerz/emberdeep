package game

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

// ---- Warrior ----

// Cleave hits the three cells of a 45-degree arc in front of the hero (adjacent only).
func (g *Game) castCleave(h *Hero, sk SkillDef, tx, ty int) error {
	d := h.facingDir(tx, ty)
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
	d := h.facingDir(tx, ty)
	h.Facing = d
	v := dirs8[d]
	g.emit(Event{K: "cast", Actor: h.ID, X: h.X, Y: h.Y, TX: h.X + v.X, TY: h.Y + v.Y, FX: sk.ID})
	if m := g.monsterAt(h.X+v.X, h.Y+v.Y); m != nil {
		dmg, crit := h.dmgScale(sk.Dmg), false
		if behind(m, h.X, h.Y) {
			dmg, crit = dmg*2, true
		}
		g.damageMonster(m, dmg, h.ID, crit)
	}
	return nil
}

// Caltrops drops a 3x3 patch that slows and bleeds enemies standing in it.
func (g *Game) castCaltrops(h *Hero, sk SkillDef, tx, ty int) error {
	g.Patches = append(g.Patches, &Patch{X: tx, Y: ty, Until: g.Now + sk.Dur, Owner: h.ID})
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
		def := Classes[Rogue].Skills[1]
		for _, m := range g.Mons {
			if abs(m.X-p.X) <= 1 && abs(m.Y-p.Y) <= 1 {
				m.SlowUntil = g.Now + 10
				m.BleedUntil = g.Now + Sec(3)
				m.BleedDmg = def.Dmg
				m.BleedOwner = p.Owner
			}
		}
	}
	g.Patches = g.Patches[:n]
}

// ---- Monk ----

// Palm Strike hits the adjacent enemy and knocks it back Aux cells, stopping at walls and other bodies.
func (g *Game) castPalm(h *Hero, sk SkillDef, tx, ty int) error {
	d := h.facingDir(tx, ty)
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
