package game

import "sort"

type Pt struct{ X, Y int }

type Rect struct{ X0, Y0, X1, Y1 int } // interior, inclusive

func (r Rect) Center() Pt { return Pt{(r.X0 + r.X1) / 2, (r.Y0 + r.Y1) / 2} }
func (r Rect) W() int     { return r.X1 - r.X0 + 1 }
func (r Rect) H() int     { return r.Y1 - r.Y0 + 1 }

const (
	TWall   = '#'
	TFloor  = '.'
	TDoor   = '+'
	TTorch  = 'T'
	TPillar = 'O'
	TStairs = '>'
	TBlood  = '~'
	TRubble = ':'
	TBones  = '%'
	TWater  = '=' // shallow water (crypt)
	TFungus = '"' // glowing fungus (caverns)
	TEmbers = '^' // ember cracks (ember depths)
	// furniture: one-cell set pieces that block the way like a pillar
	TCandle = '|' // candle stand (cathedral), gives off light
	TCoffin = '_' // coffin (crypt)
	TStalk  = 'P' // giant mushroom stalk (caverns)
)

func Solid(t byte) bool {
	return t == TWall || t == TTorch || t == TPillar || t == TCandle || t == TCoffin || t == TStalk || t == TLocked || t == ' ' || t == 0
}

type MonSpawn struct {
	Kind  byte
	X, Y  int
	Elite bool
}

type GoldSpawn struct{ X, Y, Amount int }

type Level struct {
	W, H     int
	Tiles    []byte
	Rooms    []Rect
	Start    Pt
	StartRm  int
	Stairs   Pt
	Up       Pt // the stairs up (set when the game loads the floor; HasUp false on floor 1)
	HasUp    bool
	BossRoom int
	Torches  []Pt
	Mons     []MonSpawn
	Golds    []GoldSpawn
	Theme    *Theme
	Glows    []Pt // decor tiles that give off light
	Chests   []ChestSpawn
	Npcs     []NpcSpawn
	Kegs     []Pt // powder kegs (they become monsters that never act)
	Traps    []Pt // hidden spike traps
	Lava     []Pt // burning ground that never goes out
}

type ChestSpawn struct {
	X, Y  int
	Mimic bool
	Urn   bool // a breakable urn: walk into it to smash it
}

func (l *Level) At(x, y int) byte {
	if x < 0 || y < 0 || x >= l.W || y >= l.H {
		return ' '
	}
	return l.Tiles[y*l.W+x]
}

func (l *Level) Walkable(x, y int) bool { return !Solid(l.At(x, y)) }

func (l *Level) set(x, y int, t byte) {
	if x >= 0 && y >= 0 && x < l.W && y < l.H {
		l.Tiles[y*l.W+x] = t
	}
}

// FloorSeed gives each floor of a run its own seed, so a floor is the same every time it is generated.
func FloorSeed(seed uint32, depth int) uint32 {
	if depth <= 1 {
		return seed // floor 1 keeps the run seed's layout
	}
	return seed ^ uint32(depth)*0x9E3779B1
}

// Generate builds a deterministic level for the run seed and depth. n is the party size at run start.
func Generate(seed uint32, depth, n int) *Level {
	if n < 1 {
		n = 1
	}
	fl := Floor(depth)
	th := fl.Theme
	seed = FloorSeed(seed, depth)
	// extra randomness for themes comes from its own generator, so floors 1-2 keep their exact old layout
	trng := NewRNG(uint64(seed)*7 + 4242)
	rng := NewRNG(uint64(seed) + 77)
	l := &Level{W: LW, H: LH, Tiles: make([]byte, LW*LH), Theme: th}
	for i := range l.Tiles {
		l.Tiles[i] = ' '
	}
	isRoom := make([]bool, LW*LH)

	// rooms
	for try := 0; try < 600 && len(l.Rooms) < 24; try++ {
		w := 6 + rng.Intn(13)
		h := 5 + rng.Intn(7)
		x := 3 + rng.Intn(LW-w-6)
		y := 3 + rng.Intn(LH-h-6)
		r := Rect{x, y, x + w - 1, y + h - 1}
		ok := true
		for _, o := range l.Rooms {
			if r.X0-4 <= o.X1 && r.X1+4 >= o.X0 && r.Y0-4 <= o.Y1 && r.Y1+4 >= o.Y0 {
				ok = false
				break
			}
		}
		if ok {
			l.Rooms = append(l.Rooms, r)
		}
	}
	for _, r := range l.Rooms {
		if th.Caves {
			l.carveCave(r, trng, isRoom)
			continue
		}
		for y := r.Y0; y <= r.Y1; y++ {
			for x := r.X0; x <= r.X1; x++ {
				l.set(x, y, TFloor)
				isRoom[y*LW+x] = true
			}
		}
	}
	// corridors: connect each room to its nearest earlier room
	for i := 1; i < len(l.Rooms); i++ {
		a := l.Rooms[i].Center()
		best, bd := 0, 1<<30
		for j := 0; j < i; j++ {
			c := l.Rooms[j].Center()
			d := abs(a.X-c.X) + abs(a.Y-c.Y)*2
			if d < bd {
				best, bd = j, d
			}
		}
		b := l.Rooms[best].Center()
		if rng.Intn(2) == 0 {
			l.hcorr(a.X, b.X, a.Y)
			l.vcorr(b.X, a.Y, b.Y)
		} else {
			l.vcorr(a.X, a.Y, b.Y)
			l.hcorr(a.X, b.X, b.Y)
		}
	}
	// walls
	for y := 0; y < LH; y++ {
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
	// doors where a corridor enters a room (caves have none)
	for y := 1; y < LH-1 && !th.Caves; y++ {
		for x := 1; x < LW-1; x++ {
			if l.At(x, y) != TFloor || isRoom[y*LW+x] {
				continue
			}
			vert := isRoom[(y-1)*LW+x] || isRoom[(y+1)*LW+x]
			horz := isRoom[y*LW+x-1] || isRoom[y*LW+x+1]
			if vert && l.At(x-1, y) == TWall && l.At(x+1, y) == TWall {
				l.set(x, y, TDoor)
			} else if horz && l.At(x, y-1) == TWall && l.At(x, y+1) == TWall {
				l.set(x, y, TDoor)
			}
		}
	}
	// start room: leftmost; boss room: farthest by walking distance
	l.StartRm = 0
	for i, r := range l.Rooms {
		if r.X0 < l.Rooms[l.StartRm].X0 {
			l.StartRm = i
		}
	}
	l.Start = l.Rooms[l.StartRm].Center()
	dists := l.bfsDist(l.Start)
	l.BossRoom = l.StartRm
	bd := -1
	for i, r := range l.Rooms {
		c := r.Center()
		if d := dists[c.Y*LW+c.X]; d > bd {
			bd, l.BossRoom = d, i
		}
	}
	// pillars in big rooms (never adjacent to each other, always fully surrounded by floor)
	for i, r := range l.Rooms {
		if th.Caves || i == l.StartRm || r.W() < 10 || r.H() < 7 {
			continue
		}
		c := r.Center()
		for y := r.Y0 + 2; y <= r.Y1-2; y += 3 {
			for x := r.X0 + 2; x <= r.X1-2; x += 3 {
				if x == c.X || y == c.Y || !l.openAround(x, y) {
					continue
				}
				l.set(x, y, TPillar)
			}
		}
	}
	// stairs
	br := l.Rooms[l.BossRoom]
	l.Stairs = Pt{br.X1 - 1, br.Y1 - 1}
	if th.Caves { // the carved cell of the boss room farthest from the start
		bd := -1
		for y := br.Y0; y <= br.Y1; y++ {
			for x := br.X0; x <= br.X1; x++ {
				if d := dists[y*LW+x]; isRoom[y*LW+x] && d > bd && (Pt{x, y} != br.Center()) {
					bd, l.Stairs = d, Pt{x, y}
				}
			}
		}
	}
	l.set(l.Stairs.X, l.Stairs.Y, TStairs)
	// torches on top walls
	for _, r := range l.Rooms {
		for x := r.X0 + 2; x <= r.X1-2; x += 6 + rng.Intn(3) {
			if l.At(x, r.Y0-1) == TWall && (th.TorchRate >= 1 || trng.Chance(th.TorchRate)) {
				l.set(x, r.Y0-1, TTorch)
				l.Torches = append(l.Torches, Pt{x, r.Y0 - 1})
			}
		}
		if rng.Chance(0.6) && l.At(r.X0-1, r.Y1-1) == TWall && (th.TorchRate >= 1 || trng.Chance(th.TorchRate)) {
			l.set(r.X0-1, r.Y1-1, TTorch)
			l.Torches = append(l.Torches, Pt{r.X0 - 1, r.Y1 - 1})
		}
	}
	// decor
	for y := 0; y < LH; y++ {
		for x := 0; x < LW; x++ {
			if l.At(x, y) == TFloor && rng.Chance(th.DecorChance) {
				t := th.Decor[rng.Intn(len(th.Decor))]
				if t == th.Glow && len(l.Glows) >= MaxGlows {
					t = TRubble
				}
				l.set(x, y, t)
				if t == th.Glow {
					l.Glows = append(l.Glows, Pt{x, y})
				}
			}
		}
	}
	// monsters and gold
	taken := map[Pt]bool{l.Stairs: true}
	free := func(rg *RNG, r Rect) Pt {
		for k := 0; k < 40; k++ {
			p := Pt{r.X0 + rg.Intn(r.W()), r.Y0 + rg.Intn(r.H())}
			if l.Walkable(p.X, p.Y) && !taken[p] {
				taken[p] = true
				return p
			}
		}
		return r.Center()
	}
	kinds := []byte(fl.Kinds)
	for i, r := range l.Rooms {
		if i == l.StartRm {
			continue
		}
		// per-room generators keep the base layout identical whatever the party size
		rg := NewRNG(uint64(seed)*131 + uint64(i) + 9000)
		base := 2 + rg.Intn(3)
		if i == l.BossRoom && fl.Boss != 0 {
			c := r.Center()
			taken[c] = true
			l.Mons = append(l.Mons, MonSpawn{Kind: fl.Boss, X: c.X, Y: c.Y, Elite: true})
			base = 3
		}
		for k := 0; k < PackSize(base, n)+fl.PackBonus; k++ {
			p := free(rg, r)
			l.Mons = append(l.Mons, MonSpawn{Kind: kinds[rg.Intn(len(kinds))], X: p.X, Y: p.Y})
		}
	}
	l.placeChests(seed, depth, taken)
	for i, r := range l.Rooms {
		rg := NewRNG(uint64(seed)*131 + uint64(i) + 5000)
		if rg.Chance(0.6) {
			for k := 0; k < 1+rg.Intn(2); k++ {
				p := free(rg, r)
				l.Golds = append(l.Golds, GoldSpawn{p.X, p.Y, GoldBase + rg.Intn(20)})
			}
		}
	}
	l.dress(seed, depth, taken, isRoom) // last, with its own generator: nothing above moves
	return l
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

// placeChests puts a chest against a wall in some rooms. A chest never sits next to a door or the stairs and
// leaves at least 5 open neighbours, so it can't block a passage. It uses its own generator, so adding
// chests doesn't move anything else.
func (l *Level) placeChests(seed uint32, depth int, taken map[Pt]bool) {
	rng := NewRNG(uint64(seed)*977 + 31337)
	for i, r := range l.Rooms {
		if i == l.StartRm || !rng.Chance(ChestRoomChance) {
			continue
		}
		var cand []Pt
		for y := r.Y0; y <= r.Y1; y++ {
			for x := r.X0; x <= r.X1; x++ {
				if l.chestSpot(x, y) && !taken[Pt{x, y}] {
					cand = append(cand, Pt{x, y})
				}
			}
		}
		if len(cand) == 0 {
			continue
		}
		p := cand[rng.Intn(len(cand))]
		taken[p] = true
		l.Chests = append(l.Chests, ChestSpawn{X: p.X, Y: p.Y, Mimic: rng.Chance(MimicChance(depth))})
	}
}

func (l *Level) chestSpot(x, y int) bool {
	if l.At(x, y) != TFloor {
		return false
	}
	wall, open := false, 0
	for i, d := range dirs8 {
		t := l.At(x+d.X, y+d.Y)
		if t == TDoor || t == TStairs {
			return false
		}
		if Solid(t) {
			wall = wall || i%2 == 0 // a wall straight up, down, left or right
		} else {
			open++
		}
	}
	// two cells out must not be a door either (keeps door mouths clear)
	for _, d := range dirs8 {
		if l.At(x+2*d.X, y+2*d.Y) == TDoor {
			return false
		}
	}
	return wall && open >= 5
}

// carveCave carves an irregular blob inside r: a random walk from the centre fills about 55% of the
// rectangle, then one smoothing pass rounds it off. The centre is always floor, so corridors still connect.
func (l *Level) carveCave(r Rect, rng *RNG, isRoom []bool) {
	in := func(x, y int) bool { return x >= r.X0 && x <= r.X1 && y >= r.Y0 && y <= r.Y1 }
	cave := map[Pt]bool{}
	c := r.Center()
	p := c
	want := r.W() * r.H() * 55 / 100
	for steps := 0; len(cave) < want && steps < want*30; steps++ {
		cave[p] = true
		d := dirs8[rng.Intn(4)*2] // orthogonal steps keep the blob 4-connected
		if q := (Pt{p.X + d.X, p.Y + d.Y}); in(q.X, q.Y) {
			p = q
		}
		if rng.Chance(0.02) {
			p = c // return to the centre now and then so the blob stays round
		}
	}
	// smoothing: fill cells with at least 5 cave neighbours
	add := []Pt{}
	for y := r.Y0; y <= r.Y1; y++ {
		for x := r.X0; x <= r.X1; x++ {
			if cave[Pt{x, y}] {
				continue
			}
			n := 0
			for _, d := range dirs8 {
				if cave[Pt{x + d.X, y + d.Y}] {
					n++
				}
			}
			if n >= 5 {
				add = append(add, Pt{x, y})
			}
		}
	}
	for _, q := range add {
		cave[q] = true
	}
	for q := range cave {
		l.set(q.X, q.Y, TFloor)
		isRoom[q.Y*LW+q.X] = true
	}
}

func (l *Level) hcorr(x0, x1, y int) {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	for x := x0; x <= x1; x++ {
		if l.At(x, y) == ' ' {
			l.set(x, y, TFloor)
		}
	}
}

func (l *Level) vcorr(x, y0, y1 int) {
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	for y := y0; y <= y1; y++ {
		if l.At(x, y) == ' ' {
			l.set(x, y, TFloor)
		}
	}
}

func (l *Level) openAround(x, y int) bool {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if Solid(l.At(x+dx, y+dy)) {
				return false
			}
		}
	}
	return true
}

var dirs8 = [8]Pt{{0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}}

// bfsDist returns walking distance (8-connected) from p; -1 when unreachable.
func (l *Level) bfsDist(p Pt) []int {
	d := make([]int, l.W*l.H)
	for i := range d {
		d[i] = -1
	}
	q := []Pt{p}
	d[p.Y*l.W+p.X] = 0
	for len(q) > 0 {
		c := q[0]
		q = q[1:]
		for _, dd := range dirs8 {
			nx, ny := c.X+dd.X, c.Y+dd.Y
			if !l.Walkable(nx, ny) || d[ny*l.W+nx] >= 0 {
				continue
			}
			d[ny*l.W+nx] = d[c.Y*l.W+c.X] + 1
			q = append(q, Pt{nx, ny})
		}
	}
	return d
}

// Reachable reports whether every room center can be reached from the start.
func (l *Level) Reachable() bool {
	d := l.bfsDist(l.Start)
	for _, r := range l.Rooms {
		c := r.Center()
		if d[c.Y*l.W+c.X] < 0 {
			return false
		}
	}
	return d[l.Stairs.Y*l.W+l.Stairs.X] >= 0
}

// SpawnCells returns walkable cells closest to p, ordered by distance, for placing heroes and loot.
func (l *Level) nearestFree(p Pt, ok func(x, y int) bool) (Pt, bool) {
	type cand struct {
		p Pt
		d int
	}
	var cs []cand
	for dy := -6; dy <= 6; dy++ {
		for dx := -6; dx <= 6; dx++ {
			x, y := p.X+dx, p.Y+dy
			if l.Walkable(x, y) && ok(x, y) {
				cs = append(cs, cand{Pt{x, y}, dx*dx + dy*dy*2})
			}
		}
	}
	if len(cs) == 0 {
		return Pt{}, false
	}
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].d < cs[j].d })
	return cs[0].p, true
}
