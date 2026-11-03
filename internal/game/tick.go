package game

// Tick advances the simulation by one 20 Hz step and returns the events produced since the last call
// (including events produced by Apply).
func (g *Game) Tick() []Event {
	g.Now++
	if g.Phase == PhasePlaying {
		g.runPending()
		g.tickPatches()
		g.tickNpcs()
		g.chatterNpcs()
		g.tickFollowers()
		g.tickFires()
		g.tickTraps()
		g.tickCharm()
		for _, h := range g.Heroes {
			g.tickHero(h)
			g.tickReviver(h)
			g.tickDowned(h)
		}
		for _, m := range append([]*Monster(nil), g.Mons...) {
			if m.HP > 0 {
				g.monsterAI(m)
			}
		}
	}
	g.updateVision()
	if g.Phase == PhasePlaying {
		g.checkStairs()
		if g.Phase == PhasePlaying {
			g.checkPortal()
		}
		g.checkWipe()
	}
	out := g.events
	g.events = nil
	return out
}

func (g *Game) tickHero(h *Hero) {
	if h.Dead {
		return
	}
	if g.LanternLit(h) {
		h.Oil--
		if h.Oil == OilWarn {
			g.emit(Event{K: "oillow", Actor: h.ID, X: h.X, Y: h.Y})
		}
	}
	def := h.Def()
	if def.Regen != 0 && !h.Downed {
		r := def.Regen
		if r > 0 {
			r = h.resGain(r)
		}
		h.resAcc += r / TickHz
		for h.resAcc >= 1 {
			h.resAcc--
			h.gainRes(1)
		}
		for h.resAcc <= -1 {
			h.resAcc++
			h.gainRes(-1)
		}
	}
}
