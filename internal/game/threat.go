package game

// addThreat records damage (or a flat bonus) a hero dealt to a monster.
func (g *Game) addThreat(m *Monster, hero, dmg int) {
	m.Threat = append(m.Threat, ThreatEntry{Hero: hero, Dmg: dmg, Tick: g.Now})
}

// pickTarget returns the hero with the most damage dealt to m in the last ThreatWindow ticks,
// otherwise the nearest eligible hero. Downed, dead and disconnected heroes are never targeted.
func (g *Game) pickTarget(m *Monster) *Hero {
	// prune old entries
	n := 0
	for _, e := range m.Threat {
		if g.Now-e.Tick <= ThreatWindow {
			m.Threat[n] = e
			n++
		}
	}
	m.Threat = m.Threat[:n]
	var best *Hero
	bestDmg := 0
	for _, h := range g.Heroes { // deterministic order
		if !g.eligibleHero(h) {
			continue
		}
		sum := 0
		for _, e := range m.Threat {
			if e.Hero == h.ID {
				sum += e.Dmg
			}
		}
		if sum > bestDmg {
			best, bestDmg = h, sum
		}
	}
	if best != nil {
		return best
	}
	return g.nearestHero(m.X, m.Y, 1e9)
}
