package game

// Barks: short lines monsters and gnomes say out loud (a "bark" event; the client shows a speech bubble).
// Bosses and hunt targets always speak when they notice the party, at half health and when they down a hero;
// ordinary monsters only sometimes, and never more than one every BarkGap. Lines are kept under 40 characters
// so a bubble fits on screen.

type bossLines struct{ Wake, Half, Down []string }

var BossBarks = map[byte]bossLines{
	'K': {
		Wake: []string{"More tallow for my candles!", "Kneel, little wicks."},
		Half: []string{"You dare melt ME?", "My wax... my beautiful wax!"},
		Down: []string{"Another wick snuffed.", "Into the pot with you."},
	},
	'Q': {
		Wake: []string{"Intruders in the brood!", "My children are hungry."},
		Half: []string{"Swarm! SWARM!", "Protect your mother!"},
		Down: []string{"Feed it to the larvae.", "One less soft thing."},
	},
	'D': {
		Wake: []string{"Little embers... come closer.", "I have slept an age. I hunger."},
		Half: []string{"I will burn the world for this!", "You only make me angry."},
		Down: []string{"Ash. As it should be.", "Crisp."},
	},
}

// NamedBarks are for the hunt quests' named monsters.
var NamedBarks = bossLines{
	Wake: []string{"They sent YOU for me?", "Oh good, visitors."},
	Half: []string{"That... actually hurt.", "Not like this!"},
	Down: []string{"Tell the mayor I said hello.", "Next!"},
}

// WakeBarks: what an ordinary monster might say when it notices the party.
var WakeBarks = map[byte][]string{
	'z': {"Uuurrrgh...", "Hnnngh... fresh..."},
	's': {"*rattle rattle*", "Join us..."},
	'f': {"Fresh meat!", "Ha! Easy prey!"},
	'h': {"CRUSH.", "SMASH SMALL ONES."},
	'n': {"More bones for my army!"},
	'A': {"*click click click*"},
	'i': {"Hee hee! Burn, burn!", "Catch!"},
	'd': {"Hsssss!"},
	'G': {"...GRRRMMMBL..."},
	'U': {"Hello, darling.", "Come here, sweetling."},
	'I': {"Your power smells delicious."},
	'o': {"*blorp*"},
}

var MimicBarks = []string{"SURPRISE!", "Finders keepers!", "Mine! All mine!"}
var CallerBarks = []string{"Rise, my pretties!", "Bones, to me!", "Heh heh heh..."}
var FollowerBarks = []string{"Is it much further?", "I hear something...", "Don't leave me!", "Are those the stairs?"}
var TraderBarks = []string{"Potions! Fresh potions!", "Lamp oil! Keeps the dark away!", "Garnets wanted, good prices!"}
var GnomeBarks = []string{"Oh! Adventurers!", "Mind the mud.", "Hello hello!", "Back from below? Brave!"}

const (
	barkWoke = 1 << iota
	barkHalf
)

func (g *Game) barkLine(id, x, y int, kind string, lines []string) {
	if len(lines) == 0 {
		return
	}
	g.emit(Event{K: "bark", Actor: id, X: x, Y: y, Text: lines[g.barkRng.Intn(len(lines))], FX: kind})
}

func (g *Game) bark(m *Monster, lines []string) {
	g.barkLine(m.ID, m.X, m.Y, string(rune(m.Kind)), lines)
}

func bossLinesFor(m *Monster) (bossLines, bool) {
	if m.Named != "" {
		return NamedBarks, true
	}
	b, ok := BossBarks[m.Kind]
	return b, ok
}

// barkWake: a monster notices the party.
func (g *Game) barkWake(m *Monster) {
	if m.Barked&barkWoke != 0 || m.Summoned {
		return
	}
	m.Barked |= barkWoke
	if b, ok := bossLinesFor(m); ok {
		g.bark(m, b.Wake)
		return
	}
	if g.Now < g.nextBark || !g.barkRng.Chance(BarkChance) {
		return
	}
	g.nextBark = g.Now + BarkGap
	g.bark(m, WakeBarks[m.Kind])
}

// barkHurt: a boss drops to half health.
func (g *Game) barkHurt(m *Monster) {
	if m.HP <= 0 || m.HP*2 > m.HPMax || m.Barked&barkHalf != 0 {
		return
	}
	if b, ok := bossLinesFor(m); ok {
		m.Barked |= barkHalf
		g.bark(m, b.Half)
	}
}

// barkDown: a boss knocks a hero down.
func (g *Game) barkDown(srcID int) {
	if m := g.monByID[srcID]; m != nil {
		if b, ok := bossLinesFor(m); ok {
			g.bark(m, b.Down)
		}
	}
}

// barkCaller: a Bone Caller sometimes cackles when it raises a minion.
func (g *Game) barkCaller(m *Monster) {
	if g.Now >= g.nextBark && g.barkRng.Chance(0.3) {
		g.nextBark = g.Now + BarkGap
		g.bark(m, CallerBarks)
	}
}

// chatterNpcs: gnomes now and then say something to a hero walking by; a rescued gnome frets on the way.
func (g *Game) chatterNpcs() {
	if g.Now%TickHz != 0 {
		return
	}
	for _, n := range g.Npcs {
		if n.Board || g.Now < n.chatAt || !g.IsVisible(n.X, n.Y) {
			continue
		}
		lines, reach := GnomeBarks, 3
		switch {
		case n.Trader:
			lines, reach = TraderBarks, 7
		case n.Follower:
			if n.Following == 0 {
				continue
			}
			lines, reach = FollowerBarks, 99
		}
		near := false
		for _, h := range g.Heroes {
			if !h.Dead && !h.Downed && cheb(h.X, h.Y, n.X, n.Y) <= reach && cheb(h.X, h.Y, n.X, n.Y) > 1 {
				near = true
				break
			}
		}
		if near && g.barkRng.Chance(0.25) {
			n.chatAt = g.Now + GnomeChatter + g.barkRng.Intn(GnomeChatter)
			g.barkLine(n.ID, n.X, n.Y, "g", lines)
		}
	}
}
