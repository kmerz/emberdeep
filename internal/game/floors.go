package game

// Floors remember what happened on them, so the party can go back up: monsters that died stay dead, opened
// chests and smashed urns stay open, picked-up gold stays gone and the map stays explored. A floor is still
// rebuilt from the seed when you return; the memo just leaves out what is gone.

// TUp is the staircase up, next to where the party arrives on every floor below the first.
const TUp = '<'

// UpStairsGap keeps the up stairs this many cells (Chebyshev) from the arrival point, so arriving never starts
// the countdown to go back up.
const UpStairsGap = 3

type floorMemo struct {
	Dead        []int  // indices into Level.Mons of monsters that died
	Opened      []int  // indices into Level.Chests of chests and urns opened (or mimics killed)
	Gold        []int  // indices into Level.Golds of gold piles picked up
	Explored    []byte // one bit per cell
	Vault       bool   // the vault door is open
	VaultLooted bool   // the vault chest is open
	Shrine      bool   // the shrine has been used
	Fortune     bool   // a Shrine of Fortune was used: gold counts double
}

func has(list []int, i int) bool {
	for _, v := range list {
		if v == i {
			return true
		}
	}
	return false
}

func (m *floorMemo) clone() *floorMemo {
	return &floorMemo{Dead: append([]int(nil), m.Dead...), Opened: append([]int(nil), m.Opened...),
		Gold: append([]int(nil), m.Gold...), Explored: append([]byte(nil), m.Explored...), Vault: m.Vault, VaultLooted: m.VaultLooted, Shrine: m.Shrine, Fortune: m.Fortune}
}

// memoKey: floors by depth; the gnome village is 0.
func (g *Game) memoKey() int {
	if g.InVillage {
		return 0
	}
	return g.Depth
}

// memo is the current floor's memory (created on first use).
func (g *Game) memo() *floorMemo {
	if g.Floors == nil {
		g.Floors = map[int]*floorMemo{}
	}
	k := g.memoKey()
	if g.Floors[k] == nil {
		g.Floors[k] = &floorMemo{}
	}
	return g.Floors[k]
}

// rememberFloor stores what is explored of the current floor (call before leaving it, and before saving).
func (g *Game) rememberFloor() {
	if g.Level == nil || g.Explored == nil {
		return
	}
	bits := make([]byte, (len(g.Explored)+7)/8)
	for i, e := range g.Explored {
		if e {
			bits[i/8] |= 1 << (i % 8)
		}
	}
	g.memo().Explored = bits
}

// restoreExplored marks the remembered cells explored again (and queues them for the clients).
func (g *Game) restoreExplored(m *floorMemo) {
	for i := range g.Explored {
		if i/8 < len(m.Explored) && m.Explored[i/8]&(1<<(i%8)) != 0 {
			g.Explored[i] = true
			g.explNew = append(g.explNew, TileUpdate{i % LW, i / LW, g.Level.Tiles[i]})
		}
	}
}

// placeUpStairs puts the staircase up a few cells from the arrival point (not on the first floor).
func (g *Game) placeUpStairs() {
	l := g.Level
	l.HasUp = false
	if !g.InVillage && g.Depth <= 1 {
		return
	}
	s := l.Start
	ground := func(x, y int) bool { // plain floor or loose decor, never stairs, doors, water or embers
		t := l.At(x, y)
		return (t == TFloor || t == TRubble || t == TBones || t == TBlood || t == TFungus) &&
			cheb(x, y, s.X, s.Y) >= UpStairsGap && cheb(x, y, l.Stairs.X, l.Stairs.Y) > StairsReach+1
	}
	p, ok := l.nearestFree(Pt{s.X - UpStairsGap, s.Y}, func(x, y int) bool { return ground(x, y) && l.openAround(x, y) })
	if !ok { // tight caves: anywhere walkable will do
		p, ok = l.nearestFree(Pt{s.X - UpStairsGap, s.Y}, ground)
	}
	if !ok {
		return
	}
	l.set(p.X, p.Y, TUp)
	l.Up, l.HasUp = p, true
}

// ascend takes the party up a floor: to the floor above (arriving at its stairs down), or from floor 6 to the
// gnome village and from the village to floor 5.
func (g *Game) ascend() {
	g.rememberFloor()
	g.questsOnLeave()
	g.standUp()
	g.arriveDown = true
	switch {
	case g.InVillage:
		g.loadFloor(VillageAfter)
	case g.Depth == VillageAfter+1 && g.VillageSeen:
		g.ReturnDepth = g.Depth
		g.loadVillage()
	default:
		g.loadFloor(g.Depth - 1)
	}
	g.emit(Event{K: "ascend", Amount: g.Depth})
}

// bossDead: the Tallow King of this floor died on an earlier visit.
func (g *Game) bossDead(m *floorMemo) bool {
	for _, i := range m.Dead {
		if i < len(g.Level.Mons) && g.Level.Mons[i].Kind == 'K' {
			return true
		}
	}
	return false
}

// arrivalPoint is where heroes appear: the start, or the stairs down when the party came up from below.
func (g *Game) arrivalPoint() Pt {
	if g.arrivedDown {
		return g.Level.Stairs
	}
	return g.Level.Start
}
