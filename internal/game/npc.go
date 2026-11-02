package game

// The gnome village: friendly gnomes, Grimble the trader, and the Scroll of Return's portal.

// Npc is a friendly gnome. Like a chest it fills its cell, so attacks and paths treat it as an obstacle;
// it isn't a monster and can't be hurt.
type Npc struct {
	ID       int
	X, Y     int
	Home     Pt
	Name     string
	Trader   bool
	NextMove int
	said     int
}

type NpcSpawn struct {
	X, Y   int
	Name   string
	Trader bool
}

// Portal is the way back to the village opened by a Scroll of Return.
type Portal struct {
	X, Y  int
	Until int
}

var gnomeLines = []string{
	"Mind the stairs, they bite. Well, the things below do.",
	"Grimble has torches. Never go down without a torch.",
	"We dug this village with spoons. Long story.",
	"Garnets! Grimble pays well for garnets.",
	"The Hollow below is dark as a boot. Take light.",
	"If you see a chest that smiles, don't.",
}

// GenerateVillage builds the gnome village: a plaza, houses around it, the trader's stall and the stairs.
func GenerateVillage(seed uint32) *Level {
	l := &Level{W: LW, H: LH, Tiles: make([]byte, LW*LH), Theme: Themes["village"]}
	for i := range l.Tiles {
		l.Tiles[i] = ' '
	}
	carve := func(r Rect) {
		for y := r.Y0; y <= r.Y1; y++ {
			for x := r.X0; x <= r.X1; x++ {
				l.set(x, y, TFloor)
			}
		}
	}
	plaza := Rect{50, 20, 125, 41}
	houses := []Rect{{56, 7, 68, 14}, {82, 7, 94, 14}, {108, 7, 120, 14}, {64, 47, 78, 54}, {96, 47, 110, 54}}
	l.Rooms = append([]Rect{plaza}, houses...)
	carve(plaza)
	var doors []Pt
	for _, h := range houses {
		carve(h)
		cx := (h.X0 + h.X1) / 2
		if h.Y1 < plaza.Y0 { // above the plaza: a lane down to it
			l.vcorr(cx, h.Y1+1, plaza.Y0-1)
			doors = append(doors, Pt{cx, h.Y1 + 1})
		} else {
			l.vcorr(cx, plaza.Y1+1, h.Y0-1)
			doors = append(doors, Pt{cx, h.Y0 - 1})
		}
	}
	for y := 0; y < LH; y++ { // walls around every floor cell
		for x := 0; x < LW; x++ {
			if l.At(x, y) != TFloor {
				continue
			}
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if l.At(x+dx, y+dy) == ' ' {
						l.set(x+dx, y+dy, TWall)
					}
				}
			}
		}
	}
	for _, d := range doors {
		l.set(d.X, d.Y, TDoor)
	}
	// Grimble's stall: two posts on either side of the trader
	l.set(86, 25, TPillar)
	l.set(90, 25, TPillar)
	// torches along the plaza's top wall and in each house
	for x := plaza.X0 + 3; x <= plaza.X1-3; x += 8 {
		if l.At(x, plaza.Y0-1) == TWall {
			l.set(x, plaza.Y0-1, TTorch)
			l.Torches = append(l.Torches, Pt{x, plaza.Y0 - 1})
		}
	}
	for _, h := range houses {
		x := h.X0 + 2
		if l.At(x, h.Y0-1) == TWall {
			l.set(x, h.Y0-1, TTorch)
			l.Torches = append(l.Torches, Pt{x, h.Y0 - 1})
		}
	}
	rng := NewRNG(uint64(seed) + 4711)
	for y := plaza.Y0; y <= plaza.Y1; y++ { // a little rubble and a few flower-ish bits
		for x := plaza.X0; x <= plaza.X1; x++ {
			if l.At(x, y) == TFloor && rng.Chance(0.01) {
				l.set(x, y, TRubble)
			}
		}
	}
	l.Start = Pt{plaza.X0 + 3, (plaza.Y0 + plaza.Y1) / 2}
	l.Stairs = Pt{plaza.X1 - 2, (plaza.Y0 + plaza.Y1) / 2}
	l.set(l.Stairs.X, l.Stairs.Y, TStairs)
	l.Npcs = []NpcSpawn{
		{X: 88, Y: 25, Name: "Grimble", Trader: true},
		{X: 66, Y: 33, Name: "Pip"},
		{X: 104, Y: 36, Name: "Nottle"},
		{X: 62, Y: 11, Name: "Bramwick"},
		{X: 113, Y: 23, Name: "Wendle"},
	}
	return l
}

func (g *Game) npcAt(x, y int) *Npc {
	if x < 0 || y < 0 || x >= LW || y >= LH {
		return nil
	}
	return g.npcByID[int(g.occ[y*LW+x])]
}

// tickNpcs lets the gnomes potter about near their spot (the trader stays at the stall).
func (g *Game) tickNpcs() {
	for _, n := range g.Npcs {
		if n.Trader || g.Now < n.NextMove {
			continue
		}
		n.NextMove = g.Now + NpcMoveTicks + g.rng.Intn(NpcMoveTicks)
		d := dirs8[g.rng.Intn(8)]
		x, y := n.X+d.X, n.Y+d.Y
		if g.cellFree(x, y) && cheb(x, y, n.Home.X, n.Home.Y) <= NpcWander {
			g.moveEntity(int32(n.ID), &n.X, &n.Y, x, y)
		}
	}
}

// bumpNpc: walking into a gnome. The trader opens the shop for that hero; the others say a line.
func (g *Game) bumpNpc(h *Hero, x, y int) bool {
	n := g.npcAt(x, y)
	if n == nil {
		return false
	}
	if n.Trader {
		g.emit(Event{K: "shop", Actor: h.ID, Target: n.ID, X: n.X, Y: n.Y, Text: n.Name})
		return true
	}
	line := gnomeLines[(n.said+n.ID)%len(gnomeLines)]
	n.said++
	g.emit(Event{K: "say", Actor: h.ID, Target: n.ID, X: n.X, Y: n.Y, Text: n.Name + ": " + line})
	return true
}

// nextToTrader reports whether the hero stands next to a trader in the village.
func (g *Game) nextToTrader(h *Hero) bool {
	if !g.InVillage {
		return false
	}
	for _, n := range g.Npcs {
		if n.Trader && cheb(h.X, h.Y, n.X, n.Y) <= 1 {
			return true
		}
	}
	return false
}

func (g *Game) applyBuy(h *Hero, id string) error {
	if !g.nextToTrader(h) {
		return ErrNotHere
	}
	price := -1
	for _, s := range ShopStock {
		if s.Item == id {
			price = s.Price
		}
	}
	if price < 0 {
		return ErrNoItem
	}
	if h.Gold < price {
		return ErrNoGold
	}
	if h.AddItem(id, 1) != 1 {
		return ErrPackFull
	}
	h.Gold -= price
	g.emit(Event{K: "trade", Actor: h.ID, X: h.X, Y: h.Y, FX: id, Amount: -price})
	return nil
}

func (g *Game) applySell(h *Hero, slot int) error {
	if !g.nextToTrader(h) {
		return ErrNotHere
	}
	if slot < 0 || slot >= PackSlots || h.Pack[slot].N <= 0 {
		return ErrNoItem
	}
	id := h.Pack[slot].Item
	price := SellPrice(id)
	h.removeAt(slot, 1)
	h.Gold += price
	g.emit(Event{K: "trade", Actor: h.ID, X: h.X, Y: h.Y, FX: id, Amount: price})
	return nil
}

// openPortal reads a Scroll of Return: it only works below the village, once the party has been there.
func (g *Game) openPortal(h *Hero) error {
	if !g.VillageSeen || g.InVillage || g.Depth <= VillageAfter {
		return ErrNotHere
	}
	g.Portal = &Portal{X: h.X, Y: h.Y, Until: g.Now + PortalTicks}
	g.portalSince = -1
	g.emit(Event{K: "portal", Actor: h.ID, X: h.X, Y: h.Y})
	return nil
}

// checkPortal: like the stairs, every living connected hero must stand in or next to the portal for
// PortalWindow ticks; then the party goes to the village and the village stairs lead back here.
func (g *Game) checkPortal() {
	p := g.Portal
	if p == nil {
		return
	}
	if g.Now >= p.Until {
		g.Portal, g.PortalAt, g.PortalNeed = nil, 0, 0
		return
	}
	g.PortalNeed, g.PortalAt = g.partyNear(Pt{p.X, p.Y})
	if g.PortalNeed == 0 || g.PortalAt < g.PortalNeed {
		g.portalSince = -1
		return
	}
	if g.portalSince < 0 {
		g.portalSince = g.Now
	}
	if g.Now-g.portalSince >= PortalWindow {
		g.ReturnDepth = g.Depth
		g.standUp()
		g.loadVillage()
		g.emit(Event{K: "village"})
	}
}

// partyNear counts the living, connected heroes and how many of them are within StairsReach of p.
func (g *Game) partyNear(p Pt) (need, at int) {
	for _, h := range g.Heroes {
		if h.Dead || !h.Connected {
			continue
		}
		need++
		if cheb(h.X, h.Y, p.X, p.Y) <= StairsReach {
			at++
		}
	}
	return need, at
}
