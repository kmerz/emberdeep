package game

// NextSkill picks the skill a bot spends its next point on: the original three in order as their tiers
// open, then the lowest-ranked skill it may raise. It returns -1 when nothing can be learned.
func NextSkill(ranks [TreeSize]int, lvl int) int {
	h := &Hero{Ranks: ranks, Lvl: lvl, Points: 1}
	for idx := 0; idx < 3; idx++ {
		if ranks[idx] == 0 && h.canLearn(idx) == nil {
			return idx
		}
	}
	best := -1
	for idx := 0; idx < TreeSize; idx++ {
		if h.canLearn(idx) == nil && (best < 0 || ranks[idx] < ranks[best]) {
			best = idx
		}
	}
	return best
}
