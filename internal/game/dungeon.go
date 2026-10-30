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
)

func Solid(t byte) bool { return t == TWall || t == TTorch || t == TPillar || t == ' ' || t == 0 }

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
	BossRoom int
	Torches  []Pt
	Mons     []MonSpawn
	Golds    []GoldSpawn
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

// Generate builds a deterministic level for the seed. n is the party size at run start.
func Generate(seed uint32, n int) *Level {
	if n < 1 {
		n = 1
	}
	rng := NewRNG(uint64(seed) + 77)
	l := &Level{W: LW, H: LH, Tiles: make([]byte, LW*LH)}
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
	// doors where a corridor enters a room
	for y := 1; y < LH-1; y++ {
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
		if i == l.StartRm || r.W() < 10 || r.H() < 7 {
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
	l.set(l.Stairs.X, l.Stairs.Y, TStairs)
	// torches on top walls
	for _, r := range l.Rooms {
		for x := r.X0 + 2; x <= r.X1-2; x += 6 + rng.Intn(3) {
			if l.At(x, r.Y0-1) == TWall {
				l.set(x, r.Y0-1, TTorch)
				l.Torches = append(l.Torches, Pt{x, r.Y0 - 1})
			}
		}
		if rng.Chance(0.6) && l.At(r.X0-1, r.Y1-1) == TWall {
			l.set(r.X0-1, r.Y1-1, TTorch)
			l.Torches = append(l.Torches, Pt{r.X0 - 1, r.Y1 - 1})
		}
	}
	// decor
	for y := 0; y < LH; y++ {
		for x := 0; x < LW; x++ {
			if l.At(x, y) == TFloor && rng.Chance(0.03) {
				l.set(x, y, []byte{TBlood, TRubble, TBones}[rng.Intn(3)])
			}
		}
	}
	// monsters and gold
	taken := map[Pt]bool{l.Stairs: true}
	free := func(r Rect) Pt {
		for k := 0; k < 40; k++ {
			p := Pt{r.X0 + rng.Intn(r.W()), r.Y0 + rng.Intn(r.H())}
			if l.Walkable(p.X, p.Y) && !taken[p] {
				taken[p] = true
				return p
			}
		}
		return r.Center()
	}
	kinds := []byte("zzzssfb")
	for i, r := range l.Rooms {
		if i == l.StartRm {
			continue
		}
		base := 2 + rng.Intn(3)
		if i == l.BossRoom {
			c := r.Center()
			taken[c] = true
			l.Mons = append(l.Mons, MonSpawn{Kind: 'K', X: c.X, Y: c.Y, Elite: true})
			base = 3
		}
		for k := 0; k < PackSize(base, n); k++ {
			p := free(r)
			l.Mons = append(l.Mons, MonSpawn{Kind: kinds[rng.Intn(len(kinds))], X: p.X, Y: p.Y})
		}
	}
	for _, r := range l.Rooms {
		if rng.Chance(0.6) {
			for k := 0; k < 1+rng.Intn(2); k++ {
				p := free(r)
				l.Golds = append(l.Golds, GoldSpawn{p.X, p.Y, GoldBase + rng.Intn(20)})
			}
		}
	}
	return l
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
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
