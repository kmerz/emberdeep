package game

// The strike is every hero's free fallback attack: walking into an enemy, or pressing a skill key without
// enough resource, hits an adjacent enemy with your weapon. It is weak but never costs anything, so a hero
// drained dry (an Incubus, an empty mana bar) can still fight. Rage and ki fill up from it as from any hit.

const (
	StrikeDmg   = 9               // base damage, scaled by level like a skill
	StrikeTicks = TickHz * 6 / 10 // time between strikes
)

// strike hits the enemy at (tx, ty) if it is next to the hero, else the adjacent enemy closest to that cell.
// It reports whether there was anything to hit.
func (g *Game) strike(h *Hero, tx, ty int) bool {
	if g.Now < h.NextStrike {
		return true // swinging already: swallow the key, don't report "no resource"
	}
	var best *Monster
	bestD := 1 << 30
	for _, m := range g.Mons {
		if m.HP <= 0 || m.Neutral || m.Def().Role == "keg" || cheb(h.X, h.Y, m.X, m.Y) != 1 { // kegs: only on purpose, with a skill
			continue
		}
		if d := cheb(m.X, m.Y, tx, ty); d < bestD {
			best, bestD = m, d
		}
	}
	if best == nil {
		return false
	}
	h.NextStrike = g.Now + StrikeTicks
	h.Facing = dirIndex(best.X-h.X, best.Y-h.Y, h.Facing)
	h.ReviveTarget, h.ReviveProg = 0, 0
	g.emit(Event{K: "strike", Actor: h.ID, X: h.X, Y: h.Y, TX: best.X, TY: best.Y})
	g.damageMonster(best, h.dmgScale(StrikeDmg), h.ID, false)
	return true
}
