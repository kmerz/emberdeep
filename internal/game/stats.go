package game

import "strings"

// RunStats counts what one hero did this run, for the summary when the party wipes or reaches the bottom.
// They are kept by player name outside the hero, so retrying a floor after a wipe does not undo them, and
// they are saved with the run.
type RunStats struct {
	Dmg        int    `json:"dmg"`                  // damage dealt to monsters (overkill not counted)
	Kills      int    `json:"kills"`                // killing blows
	Taken      int    `json:"taken"`                // damage taken
	Downs      int    `json:"downs"`                // times downed
	Revives    int    `json:"revives"`              // allies brought back
	Potions    int    `json:"potions"`              // healing potions drunk
	Gold       int    `json:"gold"`                 // gold picked up (before it is shared)
	BestHit    int    `json:"bestHit"`              // biggest single hit
	LastDowned string `json:"lastDowned,omitempty"` // what downed them last: "an Imp's firebolt"
}

// HeroSummary is one line of the run summary.
type HeroSummary struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Cls   string `json:"cls"`
	Lvl   int    `json:"lvl"`
	Title string `json:"title"`
	RunStats
}

// stats is h's run record (a throwaway one for nil, so callers need no checks).
func (g *Game) stats(h *Hero) *RunStats {
	if h == nil {
		return &RunStats{}
	}
	if g.Stats == nil {
		g.Stats = map[string]*RunStats{}
	}
	s := g.Stats[h.Name]
	if s == nil {
		s = &RunStats{}
		g.Stats[h.Name] = s
	}
	return s
}

// titleRules hand out the summary titles in this order: each goes to the best hero still without one.
var titleRules = []struct {
	Title string
	Val   func(s *RunStats) int
}{
	{"the Butcher", func(s *RunStats) int { return s.Kills }},
	{"the Furnace", func(s *RunStats) int { return s.Dmg }},
	{"the Lifeline", func(s *RunStats) int { return s.Revives }},
	{"the Anvil", func(s *RunStats) int { return s.Taken }},
	{"the Floor Inspector", func(s *RunStats) int { return s.Downs }},
	{"the Hammer", func(s *RunStats) int { return s.BestHit }},
	{"the Thirsty", func(s *RunStats) int { return s.Potions }},
	{"the Magpie", func(s *RunStats) int { return s.Gold }},
}

// Summary lists every hero's run stats with a title each (ties go to the hero who joined first).
func (g *Game) Summary() []HeroSummary {
	out := make([]HeroSummary, len(g.Heroes))
	for i, h := range g.Heroes {
		out[i] = HeroSummary{ID: h.ID, Name: h.Name, Cls: string(h.Cls), Lvl: h.Lvl, RunStats: *g.stats(h)}
	}
	for _, r := range titleRules {
		best, bv := -1, 0
		for i := range out {
			if v := r.Val(&out[i].RunStats); out[i].Title == "" && v > bv {
				best, bv = i, v
			}
		}
		if best >= 0 {
			out[best].Title = r.Title
		}
	}
	for i := range out {
		if out[i].Title == "" {
			out[i].Title = "the Tourist"
		}
	}
	return out
}

// shotNames are what the ranged monsters shoot.
var shotNames = map[byte]string{'i': "firebolt", 'd': "fire spit"}

// monName is how the log names a monster mid-sentence: "an Imp", "the Tallow King", or a hunt target's name.
func monName(m *Monster) string {
	if m.Named != "" {
		return m.Named
	}
	n := m.Def().Name
	if strings.HasPrefix(n, "The ") {
		return "the " + n[4:]
	}
	if strings.ContainsRune("AEIOU", rune(n[0])) {
		return "an " + n
	}
	return "a " + n
}

// cause says what hurt a hero: a monster (and how), an ally, or the floor itself (how alone).
func (g *Game) cause(srcID int, how string) string {
	if m := g.monByID[srcID]; m != nil {
		if how == "" {
			return monName(m)
		}
		return monName(m) + "'s " + how
	}
	if o := g.heroByID[srcID]; o != nil {
		return o.Name + " (friendly fire)"
	}
	return how
}

// PlayTicks is how long the run has been played, over every session of it.
func (g *Game) PlayTicks() int { return g.Played + g.Now }
