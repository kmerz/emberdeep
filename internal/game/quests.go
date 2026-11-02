package game

// Sidequests: the mayor's notice board on floor 1 and Grimble in the village each offer a few per run.
// Quests are shared by the party; everyone gets the reward.

type QuestDef struct {
	ID, Title, Text string
	Giver           string // "board" or "grimble"
	Kind            string // "hunt", "rescue", "fetch", "collect"
	Depth           int    // the floor the target is on (collect: the first floor its items drop on)
	Name            string // hunt: the named monster; rescue: the gnome
	Base            byte   // hunt: the monster it's a bigger version of
	Item            string // fetch / collect: the item to hand in
	Count           int    // collect: how many
	Sources         string // collect: monster kinds that drop the item
	DropChance      float64
	Gold, XP        int
	Point           bool   // +1 skill point for every hero
	Gift            string // an item the accepting hero gets (the spoon quest's Scroll of Return)
}

// The quest pool. The board offers QuestOffers of its quests per run, Grimble the same of his.
var QuestPool = []QuestDef{
	{ID: "snapjaw", Giver: "board", Kind: "hunt", Depth: 2, Name: "Old Snapjaw", Base: 'r', Gold: 120, XP: 80,
		Title: "WANTED: Old Snapjaw",
		Text:  "A bone rat the size of a pig ate the baker's whole shop, baker included. Last seen on floor 2."},
	{ID: "wick", Giver: "board", Kind: "hunt", Depth: 3, Name: "Sister Wick", Base: 's', Gold: 150, XP: 100,
		Title: "WANTED: Sister Wick",
		Text:  "A skeleton nun on floor 3 still lights the King's black candles. Put her out."},
	{ID: "tibble", Giver: "board", Kind: "rescue", Depth: 3, Name: "Tibble", Gold: 150, XP: 100, Point: true,
		Title: "LOST: Tibble",
		Text:  "Pip's little brother went looking for garnets on floor 3. Find him and lead him to the stairs."},
	{ID: "eggs", Giver: "board", Kind: "collect", Depth: 4, Item: "antegg", Count: 5, Sources: "aAQ", DropChance: 0.5,
		Gold: 160, XP: 120, Title: "WANTED: 5 Ant Eggs",
		Text: "The town alchemist needs ant eggs from the hive on floor 4. Hand them to Grimble, he'll pay."},
	{ID: "ledger", Giver: "board", Kind: "fetch", Depth: 5, Item: "ledger", Gold: 200, XP: 150, Point: true,
		Title: "The Tallow King's Ledger",
		Text:  "The King kept a ledger of every soul he melted down. It's in his hall on floor 5. Bring it to Grimble."},
	{ID: "fizzwick", Giver: "grimble", Kind: "rescue", Depth: 6, Name: "Fizzwick", Gold: 220, XP: 160,
		Title: "Find Fizzwick",
		Text:  "Wendle's cousin went mushroom picking in the caverns below. Bring him to the stairs, he knows the way home."},
	{ID: "palemother", Giver: "grimble", Kind: "hunt", Depth: 7, Name: "The Pale Mother", Base: 'S', Gold: 250, XP: 200, Point: true,
		Title: "The Pale Mother",
		Text:  "Something in the Black Hollow takes gnomes who go down with a lantern. Kill it."},
	{ID: "spoon", Giver: "grimble", Kind: "fetch", Depth: 8, Item: "spoon", Gold: 300, XP: 220, Point: true, Gift: "returnscroll",
		Title: "Grimble's Lucky Spoon",
		Text:  "I dropped my lucky spoon at the Abyss Gate. Take this Scroll of Return and bring it back. Please."},
	{ID: "cinderjaw", Giver: "grimble", Kind: "hunt", Depth: 9, Name: "Cinderjaw", Base: 'd', Gold: 300, XP: 250, Point: true,
		Title: "Cinderjaw",
		Text:  "An old drake burns every gnome scout on floor 9. Grimble wants its head (and its teeth)."},
	{ID: "shards", Giver: "grimble", Kind: "collect", Depth: 9, Item: "embershard", Count: 4, Sources: "Gi", DropChance: 0.4,
		Gold: 350, XP: 250, Title: "4 Ember Shards",
		Text: "Magma golems and imps carry ember shards. Bring Grimble four and he'll make you very rich."},
}

const (
	QuestOffers   = 3   // per giver and run
	NamedHPMult   = 3.0 // a hunt target has this much more HP than its kind
	NamedDmgMult  = 1.5
	FollowerHP    = 40
	FollowerEvery = MoveTicks // ticks between a follower's steps (as fast as a hero)
	RescueReach   = 3         // the follower must be this close to the stairs when the party leaves
)

type QuestState int

const (
	QuestOffered QuestState = iota
	QuestActive
	QuestDone
	QuestFailed
)

type Quest struct {
	Def      *QuestDef
	State    QuestState
	Progress int // collect: the most of the item any hero carries; fetch: 1 once found
}

// initQuests picks this run's offers from the seed: QuestOffers per giver.
func (g *Game) initQuests() {
	rng := NewRNG(uint64(g.Seed)*53 + 977)
	for _, giver := range []string{"board", "grimble"} {
		var pool []*QuestDef
		for i := range QuestPool {
			if QuestPool[i].Giver == giver {
				pool = append(pool, &QuestPool[i])
			}
		}
		for i := len(pool) - 1; i > 0; i-- { // shuffle, then keep the first QuestOffers in pool order
			j := rng.Intn(i + 1)
			pool[i], pool[j] = pool[j], pool[i]
		}
		pick := pool[:min(QuestOffers, len(pool))]
		for _, d := range QuestPool {
			for _, p := range pick {
				if p.ID == d.ID {
					g.Quests = append(g.Quests, &Quest{Def: p})
				}
			}
		}
	}
	g.questsChanged = true
}

func (g *Game) quest(id string) *Quest {
	for _, q := range g.Quests {
		if q.Def.ID == id {
			return q
		}
	}
	return nil
}

// TakeQuestsChanged reports whether the quest list changed since the last call.
func (g *Game) TakeQuestsChanged() bool {
	c := g.questsChanged
	g.questsChanged = false
	return c
}

// nextToGiver reports whether the hero stands next to the notice board or Grimble.
func (g *Game) nextToGiver(h *Hero, giver string) bool {
	for _, n := range g.Npcs {
		if ((giver == "board" && n.Board) || (giver == "grimble" && n.Trader)) && cheb(h.X, h.Y, n.X, n.Y) <= 1 {
			return true
		}
	}
	return false
}

func (g *Game) applyAccept(h *Hero, id string) error {
	q := g.quest(id)
	if q == nil || q.State != QuestOffered {
		return ErrBadIntent
	}
	if !g.nextToGiver(h, q.Def.Giver) {
		return ErrNotHere
	}
	q.State = QuestActive
	g.questsChanged = true
	if q.Def.Gift != "" {
		h.AddItem(q.Def.Gift, 1)
	}
	g.emit(Event{K: "questnew", Actor: h.ID, X: h.X, Y: h.Y, Text: q.Def.Title})
	if !g.InVillage && g.Depth == q.Def.Depth {
		g.spawnQuest(q)
	}
	return nil
}

// questsOnFloor places what the active quests need on the floor that just loaded.
func (g *Game) questsOnFloor() {
	if g.InVillage {
		return
	}
	for _, q := range g.Quests {
		if q.State == QuestActive && q.Def.Depth == g.Depth {
			g.spawnQuest(q)
		}
	}
}

// farSpot is a free cell in a room far from the start (the boss room for fetch quests, since the
// ledger lies in the King's hall).
func (g *Game) farSpot(q *QuestDef) Pt {
	l := g.Level
	rng := NewRNG(uint64(FloorSeed(g.Seed, g.Depth))*7 + uint64(len(q.ID))*131 + uint64(q.ID[0]))
	room := l.BossRoom
	if q.Kind != "fetch" && len(l.Rooms) > 2 {
		dists := l.bfsDist(l.Start)
		best := -1
		for k := 0; k < 6; k++ { // the farthest of a few random rooms that aren't the start or the boss room
			i := rng.Intn(len(l.Rooms))
			if i == l.StartRm || i == l.BossRoom {
				continue
			}
			c := l.Rooms[i].Center()
			if best < 0 || dists[c.Y*LW+c.X] > dists[l.Rooms[best].Center().Y*LW+l.Rooms[best].Center().X] {
				best = i
			}
		}
		if best >= 0 {
			room = best
		}
	}
	p, _ := l.nearestFree(l.Rooms[room].Center(), func(x, y int) bool { return g.occ[y*LW+x] == 0 })
	return p
}

func (g *Game) spawnQuest(q *Quest) {
	d := q.Def
	switch d.Kind {
	case "hunt":
		for _, m := range g.Mons {
			if m.Named == d.Name {
				return
			}
		}
		p := g.farSpot(d)
		m := g.addMon(d.Base, p.X, p.Y, NamedHPMult)
		m.Named, m.Awake = d.Name, false
	case "rescue":
		for _, n := range g.Npcs {
			if n.Quest == d.ID {
				return
			}
		}
		p := g.farSpot(d)
		n := &Npc{ID: g.newID(), X: p.X, Y: p.Y, Home: p, Name: d.Name, Follower: true, Quest: d.ID, HP: FollowerHP, HPMax: FollowerHP}
		g.Npcs = append(g.Npcs, n)
		g.npcByID[n.ID] = n
		g.occ[p.Y*LW+p.X] = int32(n.ID)
	case "fetch":
		for _, h := range g.Heroes {
			if h.Count(d.Item) > 0 {
				return // already carried
			}
		}
		p := g.farSpot(d)
		g.Items = append(g.Items, &Item{ID: g.newID(), Kind: Items[d.Item].Glyph, Def: d.Item, X: p.X, Y: p.Y, Amount: 1})
	}
}

// questKill: a named monster died, or a collect source may drop its item.
func (g *Game) questKill(m *Monster) {
	for _, q := range g.Quests {
		if q.State != QuestActive {
			continue
		}
		d := q.Def
		switch d.Kind {
		case "hunt":
			if m.Named == d.Name {
				g.completeQuest(q)
			}
		case "collect":
			for i := 0; i < len(d.Sources); i++ {
				if d.Sources[i] == m.Kind && g.rng.Chance(d.DropChance) {
					g.Items = append(g.Items, &Item{ID: g.newID(), Kind: Items[d.Item].Glyph, Def: d.Item, X: m.X, Y: m.Y, Amount: 1})
					break
				}
			}
		}
	}
}

// questPickup updates fetch and collect progress after a hero picked something up.
func (g *Game) questPickup(item string) {
	for _, q := range g.Quests {
		if q.State != QuestActive || q.Def.Item != item {
			continue
		}
		best := 0
		for _, h := range g.Heroes {
			best = max(best, h.Count(item))
		}
		if best != q.Progress {
			q.Progress = best
			g.questsChanged = true
			g.emit(Event{K: "questprogress", Text: q.Def.Title, Amount: best})
		}
	}
}

// questTurnIn: walking into Grimble hands in the fetch and collect quests whose items this hero carries.
func (g *Game) questTurnIn(h *Hero) {
	for _, q := range g.Quests {
		d := q.Def
		if q.State != QuestActive || (d.Kind != "fetch" && d.Kind != "collect") {
			continue
		}
		need := max(1, d.Count)
		if h.Count(d.Item) < need {
			continue
		}
		left := need
		for i := range h.Pack {
			if left > 0 && h.Pack[i].Item == d.Item {
				k := min(left, h.Pack[i].N)
				h.removeAt(i, k)
				left -= k
			}
		}
		g.completeQuest(q)
	}
}

// questsOnLeave runs when the party leaves a floor (stairs or portal): rescues end here.
func (g *Game) questsOnLeave() {
	for _, q := range g.Quests {
		if q.State != QuestActive || q.Def.Kind != "rescue" || q.Def.Depth != g.Depth || g.InVillage {
			continue
		}
		saved := false
		for _, n := range g.Npcs {
			if n.Quest == q.Def.ID && n.Following != 0 && cheb(n.X, n.Y, g.Level.Stairs.X, g.Level.Stairs.Y) <= RescueReach {
				saved = true
			}
		}
		if saved {
			g.completeQuest(q)
		} else {
			g.failQuest(q, q.Def.Name+" was left behind")
		}
	}
}

func (g *Game) completeQuest(q *Quest) {
	d := q.Def
	q.State = QuestDone
	g.questsChanged = true
	for _, h := range g.Heroes {
		h.Gold += d.Gold
		if d.Point {
			h.Points++
		}
	}
	g.gainXP(d.XP)
	pt := 0
	if d.Point {
		pt = 1
	}
	g.emit(Event{K: "questdone", Text: d.Title, Amount: d.Gold, TX: d.XP, TY: pt})
}

func (g *Game) failQuest(q *Quest, why string) {
	q.State = QuestFailed
	g.questsChanged = true
	g.emit(Event{K: "questfail", Text: q.Def.Title + ": " + why})
}

// tickFollowers walks rescued gnomes after the hero they follow.
func (g *Game) tickFollowers() {
	for _, n := range g.Npcs {
		if !n.Follower || n.Following == 0 || g.Now < n.NextMove {
			continue
		}
		n.NextMove = g.Now + FollowerEvery
		h := g.heroByID[n.Following]
		if h == nil || h.Dead || !h.Connected {
			h = g.nearestHero(n.X, n.Y, 1e9)
			if h == nil {
				continue
			}
			n.Following = h.ID
		}
		if cheb(n.X, n.Y, h.X, h.Y) <= 1 {
			continue
		}
		if p, ok := g.stepToward(n.X, n.Y, h.X, h.Y); ok {
			g.moveEntity(int32(n.ID), &n.X, &n.Y, p.X, p.Y)
		}
	}
}

// adjacentFollower is a following gnome next to the monster, if any (monsters with no hero in reach bite it).
func (g *Game) adjacentFollower(m *Monster) *Npc {
	for _, n := range g.Npcs {
		if n.Follower && n.HP > 0 && cheb(n.X, n.Y, m.X, m.Y) <= 1 {
			return n
		}
	}
	return nil
}

func (g *Game) hurtFollower(n *Npc, dmg, src int) {
	n.HP -= dmg
	g.emit(Event{K: "hit", Actor: src, Target: n.ID, X: n.X, Y: n.Y, Amount: dmg})
	if n.HP > 0 {
		return
	}
	g.removeNpc(n)
	if q := g.quest(n.Quest); q != nil && q.State == QuestActive {
		g.failQuest(q, n.Name+" was killed")
	}
}

func (g *Game) removeNpc(n *Npc) {
	if g.occ[n.Y*LW+n.X] == int32(n.ID) {
		g.occ[n.Y*LW+n.X] = 0
	}
	delete(g.npcByID, n.ID)
	for i, o := range g.Npcs {
		if o == n {
			g.Npcs = append(g.Npcs[:i], g.Npcs[i+1:]...)
			break
		}
	}
}
