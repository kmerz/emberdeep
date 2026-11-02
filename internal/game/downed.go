package game

import "math"

// applyRevive starts or stops holding E on a downed ally. Progress accumulates in Tick while adjacency holds.
func (g *Game) applyRevive(h *Hero, in Intent) error {
	if !in.Hold {
		h.ReviveTarget, h.ReviveProg = 0, 0
		return nil
	}
	t := g.heroByID[in.Target]
	if t == nil || t == h || !t.Downed || t.Dead {
		return ErrBadTarget
	}
	if cheb(h.X, h.Y, t.X, t.Y) > 1 {
		return ErrRange
	}
	if h.ReviveTarget != t.ID {
		h.ReviveTarget, h.ReviveProg = t.ID, 0
	}
	return nil
}

// tickReviver advances (or resets) h's revive progress.
func (g *Game) tickReviver(h *Hero) {
	if h.ReviveTarget == 0 {
		return
	}
	t := g.heroByID[h.ReviveTarget]
	if t == nil || !t.Downed || h.Dead || h.Downed || !h.Connected || cheb(h.X, h.Y, t.X, t.Y) > 1 {
		h.ReviveTarget, h.ReviveProg = 0, 0
		return
	}
	h.ReviveProg++
	if h.ReviveProg >= ReviveTicks {
		t.Downed = false
		t.HP = max(1, int(math.Round(float64(t.HPMax)*ReviveHPFrac)))
		g.emit(Event{K: "revive", Actor: h.ID, Target: t.ID, X: t.X, Y: t.Y})
		for _, o := range g.Heroes {
			if o.ReviveTarget == t.ID {
				o.ReviveTarget, o.ReviveProg = 0, 0
			}
		}
	}
}

// tickDowned turns an expired downed hero into a dead spectator.
func (g *Game) tickDowned(h *Hero) {
	if !h.Downed || g.Now < h.DownedUntil {
		return
	}
	h.Downed = false
	h.Dead = true
	h.HP = 0
	if g.occ[h.Y*LW+h.X] == int32(h.ID) {
		g.occ[h.Y*LW+h.X] = 0
	}
	for _, o := range g.Heroes {
		if o.ReviveTarget == h.ID {
			o.ReviveTarget, o.ReviveProg = 0, 0
		}
	}
	g.emit(Event{K: "dead", Target: h.ID, X: h.X, Y: h.Y})
}

// checkWipe ends the run when no connected hero is still standing. Heroes held for a reconnect are
// invulnerable and idle, so they do not count either way.
func (g *Game) checkWipe() {
	up, connected := 0, 0
	for _, h := range g.Heroes {
		if !h.Connected {
			continue
		}
		connected++
		if !h.Dead && !h.Downed {
			up++
		}
	}
	if connected > 0 && up == 0 {
		g.Phase = PhaseWiped
		g.WipedTick = g.Now
		g.emit(Event{K: "wipe"})
	}
}

// checkStairs: every living, connected hero must be on or next to '>' for StairsWindow ticks.
// partyAt counts who must gather at a staircase (living, connected heroes and a gnome being rescued) and how
// many of them stand next to it.
func (g *Game) partyAt(st Pt) (need, at int) {
	for _, h := range g.Heroes {
		if h.Dead || !h.Connected {
			continue
		}
		need++
		if cheb(h.X, h.Y, st.X, st.Y) <= StairsReach {
			at++
		}
	}
	for _, n := range g.Npcs { // a gnome being rescued has to come along too
		if n.Follower && n.Following != 0 {
			need++
			if cheb(n.X, n.Y, st.X, st.Y) <= RescueReach {
				at++
			}
		}
	}
	return need, at
}

// checkStairs runs the countdown when the whole party stands at the stairs down (or up). A staircase only
// counts once nobody stands at it, so arriving next to one never sends the party straight back.
func (g *Game) checkStairs() {
	need, at := g.partyAt(g.Level.Stairs)
	if at == 0 {
		g.downArmed = true
	}
	upNeed, upAt := 0, 0
	if g.Level.HasUp {
		upNeed, upAt = g.partyAt(g.Level.Up)
		if upAt == 0 {
			g.upArmed = true
		}
	}
	up := upAt > at
	if up {
		need, at = upNeed, upAt
	}
	if up != g.StairsUp { // the party changed its mind: start over
		g.StairsUp, g.stairsSince = up, -1
	}
	st := g.Level.Stairs
	armed := g.downArmed
	if up {
		st, armed = g.Level.Up, g.upArmed
	}
	g.StairsNeed, g.StairsAt = need, at
	if need == 0 || at < need || !armed {
		g.stairsSince, g.StairsLeft = -1, 0
		return
	}
	if g.stairsSince < 0 {
		g.stairsSince = g.Now
	}
	g.StairsLeft = max(1, StairsWindow-(g.Now-g.stairsSince))
	if g.Now-g.stairsSince >= StairsWindow {
		if up {
			g.ascend()
			return
		}
		if g.Depth < MaxDepth || g.InVillage {
			g.descend()
			return
		}
		g.Phase = PhaseCleared
		g.ClearedTick = g.Now
		g.StairsLeft = 0
		g.emit(Event{K: "cleared", X: st.X, Y: st.Y})
	}
}
