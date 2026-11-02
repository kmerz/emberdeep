package game

import "math"

// The world's dressing: furniture, urns to smash, powder kegs that blow up (and set off each other), hidden
// spike traps, and lava pools. The numbers live in balance.go (Dressings and the constants after it).

// Trap is a hidden spike trap. It shows itself when a hero comes next to it (or when something steps on it).
type Trap struct {
	X, Y    int
	Seen    bool
	ReadyAt int // tick it can fire again
}

// dress places the theme's furniture, urns, kegs, traps and lava. It runs after everything else and uses its
// own generator, so the rest of the floor stays exactly as it was.
func (l *Level) dress(seed uint32, depth int, taken map[Pt]bool, isRoom []bool) {
	d, ok := Dressings[l.Theme.ID]
	if !ok {
		return
	}
	rng := NewRNG(uint64(seed)*613 + 2718)
	pick := func(r Rect, ok func(x, y int) bool) (Pt, bool) {
		for k := 0; k < 30; k++ {
			p := Pt{r.X0 + rng.Intn(r.W()), r.Y0 + rng.Intn(r.H())}
			if !taken[p] && ok(p.X, p.Y) {
				return p, true
			}
		}
		return Pt{}, false
	}
	nearDoor := func(x, y int) bool {
		for dy := -2; dy <= 2; dy++ {
			for dx := -2; dx <= 2; dx++ {
				if t := l.At(x+dx, y+dy); t == TDoor || t == TStairs {
					return true
				}
			}
		}
		return false
	}
	centers := map[Pt]bool{} // corridors run to room centers: keep them clear
	for _, r := range l.Rooms {
		centers[r.Center()] = true
	}
	for i, r := range l.Rooms {
		// furniture stands alone in the open (nothing solid around it), so it can never close a passage
		if d.Furniture != 0 {
			for k, n := 0, rng.Intn(d.FurnPerRoom+1); k < n; k++ {
				p, ok := pick(r, func(x, y int) bool {
					return isRoom[y*LW+x] && l.At(x, y) == TFloor && l.openAround(x, y) && !nearDoor(x, y) &&
						cheb(x, y, l.Start.X, l.Start.Y) > 2 && !centers[Pt{x, y}]
				})
				if !ok {
					break
				}
				l.set(p.X, p.Y, d.Furniture)
				taken[p] = true
				if d.Furniture == TCandle && len(l.Glows) < MaxGlows {
					l.Glows = append(l.Glows, p)
				}
			}
		}
		if i == l.StartRm {
			continue
		}
		// urns and kegs stand against a wall, like chests
		against := func(x, y int) bool { return l.chestSpot(x, y) }
		if rng.Chance(d.UrnChance) {
			for k, n := 0, 1+rng.Intn(2); k < n; k++ {
				if p, ok := pick(r, against); ok {
					taken[p] = true
					l.Chests = append(l.Chests, ChestSpawn{X: p.X, Y: p.Y, Urn: true})
				}
			}
		}
		if rng.Chance(d.KegChance) {
			if p, ok := pick(r, against); ok {
				taken[p] = true
				l.Kegs = append(l.Kegs, p)
			}
		}
	}
	// spike traps: on open floor anywhere but the start room, never in a doorway
	if depth >= 2 {
		for k, tries := 0, 0; k < d.Traps && tries < 400; tries++ {
			r := l.Rooms[rng.Intn(len(l.Rooms))]
			p := Pt{r.X0 + rng.Intn(r.W()), r.Y0 + rng.Intn(r.H())}
			if taken[p] || l.At(p.X, p.Y) != TFloor || nearDoor(p.X, p.Y) || cheb(p.X, p.Y, l.Start.X, l.Start.Y) < 12 {
				continue
			}
			taken[p] = true
			l.Traps = append(l.Traps, p)
			k++
		}
	}
	// lava pools: a small blob of burning ground in a room (walkable, so it never blocks the way; it just hurts)
	for k, tries := 0, 0; k < d.LavaPools && tries < 100; tries++ {
		ri := rng.Intn(len(l.Rooms))
		if ri == l.StartRm {
			continue
		}
		r := l.Rooms[ri]
		p := Pt{r.X0 + rng.Intn(r.W()), r.Y0 + rng.Intn(r.H())}
		lavaOK := func(q Pt) bool {
			t := l.At(q.X, q.Y)
			return !taken[q] && (t == TFloor || t == TEmbers) && !nearDoor(q.X, q.Y)
		}
		if !lavaOK(p) {
			continue
		}
		taken[p] = true
		l.Lava = append(l.Lava, p)
		for _, dd := range dirs8 {
			q := Pt{p.X + dd.X, p.Y + dd.Y}
			if rng.Chance(0.5) && lavaOK(q) {
				taken[q] = true
				l.Lava = append(l.Lava, q)
			}
		}
		k++
	}
}

// setupWorld turns the level's traps and lava into live state (called when a floor loads).
func (g *Game) setupWorld() {
	for _, p := range g.Level.Kegs {
		if g.occ[p.Y*LW+p.X] != 0 {
			continue
		}
		m := &Monster{ID: g.newID(), Kind: KegKind, X: p.X, Y: p.Y, HP: 1, HPMax: 1, Facing: 6}
		g.Mons = append(g.Mons, m)
		g.monByID[m.ID] = m
		g.occ[m.Y*LW+m.X] = int32(m.ID)
	}
	g.Traps = nil
	for _, p := range g.Level.Traps {
		g.Traps = append(g.Traps, &Trap{X: p.X, Y: p.Y})
	}
	for _, p := range g.Level.Lava {
		g.addFire(p.X, p.Y, 1<<30, LavaDmg)
	}
}

// HazardAt reports burning ground or a trap someone has seen (bots walk around them).
func (g *Game) HazardAt(x, y int) bool {
	for _, f := range g.Fires {
		if f.X == x && f.Y == y {
			return true
		}
	}
	for _, t := range g.Traps {
		if t.Seen && t.X == x && t.Y == y {
			return true
		}
	}
	return false
}

// tickTraps shows traps to heroes who come close and springs them on whatever stands on them.
func (g *Game) tickTraps() {
	for _, t := range g.Traps {
		if !t.Seen {
			for _, h := range g.Heroes {
				if !h.Dead && !h.Downed && cheb(h.X, h.Y, t.X, t.Y) <= 1 {
					t.Seen = true
					g.emit(Event{K: "trapseen", Actor: h.ID, X: t.X, Y: t.Y})
					break
				}
			}
		}
		id := int(g.occ[t.Y*LW+t.X])
		if id == 0 || g.Now < t.ReadyAt {
			continue
		}
		if h := g.heroByID[id]; h != nil {
			if h.Dead || h.Downed {
				continue
			}
			t.Seen, t.ReadyAt = true, g.Now+TrapRearm
			g.emit(Event{K: "trap", Target: h.ID, X: t.X, Y: t.Y})
			g.damageHero(h, int(math.Round(TrapDmg*DepthDmgMult(g.Depth)*Difficulties[g.Settings.Difficulty].DmgMult)), 0)
		} else if m := g.monByID[id]; m != nil && m.Def().Role != "keg" && !m.Disguised {
			t.ReadyAt = g.Now + TrapRearm
			if g.IsVisible(t.X, t.Y) {
				t.Seen = true
			}
			g.emit(Event{K: "trap", Target: m.ID, X: t.X, Y: t.Y})
			g.damageMonster(m, int(math.Round(TrapMonDmg*DepthHPMult(g.Depth))), 0, false)
		}
	}
}

// smashUrn breaks an urn a hero walked into: a little gold now and then, rarely an item for that hero.
func (g *Game) smashUrn(h *Hero, c *Chest) {
	g.emit(Event{K: "urn", Actor: h.ID, Target: c.ID, X: c.X, Y: c.Y})
	if g.rng.Chance(UrnGoldChance) {
		g.Items = append(g.Items, &Item{ID: g.newID(), Kind: '$', X: c.X, Y: c.Y, Amount: GoldBase/2 + g.rng.Intn(8)})
	}
	if g.rng.Chance(UrnItemChance) {
		total := 0.0
		for _, id := range ItemOrder {
			total += Items[id].DropChance
		}
		r := g.rng.Float() * total
		for _, id := range ItemOrder {
			def := Items[id]
			if r -= def.DropChance; r < 0 {
				g.Items = append(g.Items, &Item{ID: g.newID(), Kind: def.Glyph, Def: id, X: c.X, Y: c.Y, Amount: 1, Owner: h.ID})
				break
			}
		}
	}
}

// breakKeg lights a keg that was hit: it blows up KegFuse ticks later.
func (g *Game) breakKeg(m *Monster, by int) {
	g.emit(Event{K: "fuse", Actor: by, Target: m.ID, X: m.X, Y: m.Y})
	g.pend = append(g.pend, pending{at: g.Now + KegFuse, skill: "keg", caster: by, x: m.X, y: m.Y})
}

// kegBlast hurts everything around a keg: monsters (credited to whoever lit it), heroes, and other kegs,
// which then go off too. It leaves the ground burning for a moment.
func (g *Game) kegBlast(p pending) {
	g.emit(Event{K: "impact", Actor: p.caster, X: p.x, Y: p.y, FX: "keg", R: KegRadius})
	dmg := int(math.Round(KegDmg * DepthHPMult(g.Depth)))
	for _, m := range append([]*Monster(nil), g.Mons...) {
		if m.HP > 0 && dist(p.x, p.y, m.X, m.Y) <= KegRadius {
			g.damageMonster(m, dmg, p.caster, false)
		}
	}
	hd := int(math.Round(KegHeroDmg * DepthDmgMult(g.Depth) * Difficulties[g.Settings.Difficulty].DmgMult))
	for _, h := range g.Heroes {
		if !h.Dead && dist(p.x, p.y, h.X, h.Y) <= KegRadius {
			g.damageHero(h, hd, 0)
		}
	}
	g.addFire(p.x, p.y, KegFireTicks, FireDmg)
	for i, d := range dirs8 {
		if i%2 == 0 {
			g.addFire(p.x+d.X, p.y+d.Y, KegFireTicks, FireDmg)
		}
	}
}
