// Package bot holds the AI shared by server-side bot teammates and the headless test client.
// A Brain only sees what a real client sees (entities, own resources, known tiles) and answers with
// ordinary client messages, so every action goes through the same server-side validation.
package bot

import (
	"math"

	"emberdeep/internal/proto"
)

type Ent struct {
	ID           int
	K            string // h, m, i
	X, Y         int
	Downed, Dead bool
	Elite        bool
	HP, HPMax    int
}

type View struct {
	Me        int
	Cls       string
	Ents      []Ent
	Walkable  func(x, y int) bool // known, walkable tile
	Unknown   func(x, y int) bool // never explored
	CD        []int
	Res       int
	Cost      []int
	Potions   int
	PotCD     int
	HP, HPMax int
	Humans    map[int]bool // hero ids to follow (nil = anyone)
}

type Brain struct {
	Leader bool // explore on its own instead of following someone
	seq    int
	reviv  int
	wander int
}

func NewBrain(leader bool) *Brain { return &Brain{Leader: leader} }

func (b *Brain) nextSeq() int { b.seq++; return b.seq }

func cheb(ax, ay, bx, by int) int {
	dx, dy := ax-bx, ay-by
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}

func dist(ax, ay, bx, by int) float64 {
	dx, dy := float64(ax-bx), float64(ay-by)*2
	return math.Sqrt(dx*dx + dy*dy)
}

var dirs = [8][2]int{{0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}}

// Think returns the messages to send this step (at most one move).
func (b *Brain) Think(v *View) []proto.ClientMsg {
	var me *Ent
	for i := range v.Ents {
		if v.Ents[i].K == "h" && v.Ents[i].ID == v.Me {
			me = &v.Ents[i]
		}
	}
	if me == nil || me.Dead || me.Downed {
		b.reviv = 0
		return nil
	}
	var out []proto.ClientMsg
	// revive a downed ally
	var down *Ent
	for i := range v.Ents {
		e := &v.Ents[i]
		if e.K == "h" && e.ID != v.Me && e.Downed && !e.Dead && (down == nil || cheb(me.X, me.Y, e.X, e.Y) < cheb(me.X, me.Y, down.X, down.Y)) {
			down = e
		}
	}
	// nearest monster
	var mon *Ent
	md := 1e9
	for i := range v.Ents {
		e := &v.Ents[i]
		if e.K != "m" {
			continue
		}
		if d := dist(me.X, me.Y, e.X, e.Y); d < md {
			mon, md = e, d
		}
	}
	if down != nil && (mon == nil || md > 3) {
		if cheb(me.X, me.Y, down.X, down.Y) <= 1 {
			if b.reviv != down.ID {
				b.reviv = down.ID
				return []proto.ClientMsg{{T: proto.CRevive, Target: down.ID, Hold: true, Seq: b.nextSeq()}}
			}
			return nil // keep holding
		}
		if m := b.stepTo(v, me, down.X, down.Y, true); m != nil {
			out = append(out, *m)
		}
		return out
	}
	b.reviv = 0
	if v.Potions > 0 && v.PotCD == 0 && v.HPMax > 0 && v.HP*100/v.HPMax < 40 {
		out = append(out, proto.ClientMsg{T: proto.CPotion, Seq: b.nextSeq()})
	}
	if mon != nil && md <= 7 {
		adj := cheb(me.X, me.Y, mon.X, mon.Y) <= 1
		out = append(out, b.fight(v, me, mon, md, adj)...)
		ranged := v.Cls == "sorcerer"
		if !adj && !ranged {
			if m := b.stepTo(v, me, mon.X, mon.Y, false); m != nil {
				out = append(out, *m)
			}
		}
		return out
	}
	// follow the nearest human
	var lead *Ent
	ld := 1e9
	for i := range v.Ents {
		e := &v.Ents[i]
		if e.K != "h" || e.ID == v.Me || e.Dead || (v.Humans != nil && !v.Humans[e.ID]) {
			continue
		}
		if d := dist(me.X, me.Y, e.X, e.Y); d < ld {
			lead, ld = e, d
		}
	}
	if !b.Leader && lead != nil {
		if ld > 6 {
			if m := b.stepTo(v, me, lead.X, lead.Y, false); m != nil {
				out = append(out, *m)
			}
		}
		return out
	}
	// leader: explore toward the nearest unexplored frontier
	if m := b.explore(v, me); m != nil {
		out = append(out, *m)
	}
	return out
}

func (b *Brain) fight(v *View, me, mon *Ent, md float64, adj bool) []proto.ClientMsg {
	cast := func(slot int) proto.ClientMsg {
		return proto.ClientMsg{T: proto.CCast, Slot: slot, X: mon.X, Y: mon.Y, Seq: b.nextSeq()}
	}
	ready := func(slot int) bool {
		return slot < len(v.CD) && v.CD[slot] == 0 && v.Res >= v.Cost[slot]
	}
	switch v.Cls {
	case "warrior":
		if adj && ready(1) {
			return []proto.ClientMsg{cast(1)}
		}
		if adj && ready(0) {
			return []proto.ClientMsg{cast(0)}
		}
		if md > 4 && md < 12 && ready(2) {
			return []proto.ClientMsg{cast(2)}
		}
	case "rogue":
		if md > 3 && md < 14 && ready(2) {
			return []proto.ClientMsg{cast(2)}
		}
		if md > 2 && md < 10 && ready(1) {
			return []proto.ClientMsg{cast(1)}
		}
		if adj && ready(0) {
			return []proto.ClientMsg{cast(0)}
		}
	case "sorcerer":
		if md > 4 && ready(1) {
			return []proto.ClientMsg{cast(1)}
		}
		if ready(0) {
			return []proto.ClientMsg{cast(0)}
		}
	case "monk":
		if md <= 6 && ready(2) {
			return []proto.ClientMsg{cast(2)}
		}
		if adj && ready(0) {
			return []proto.ClientMsg{cast(0)}
		}
	}
	return nil
}

type node struct{ x, y int }

// bfs finds the first step toward the nearest cell satisfying goal.
func (b *Brain) bfs(v *View, me *Ent, goal func(x, y int) bool) *proto.ClientMsg {
	occ := map[node]bool{}
	for _, e := range v.Ents {
		if e.K != "i" && e.ID != v.Me && !(e.K == "h" && e.Downed) {
			occ[node{e.X, e.Y}] = true
		}
	}
	type qe struct {
		n     node
		first int
	}
	seen := map[node]bool{{me.X, me.Y}: true}
	q := []qe{{node{me.X, me.Y}, -1}}
	for i := 0; i < len(q) && i < 2500; i++ {
		c := q[i]
		if c.first >= 0 && goal(c.n.x, c.n.y) {
			return &proto.ClientMsg{T: proto.CMove, Dir: c.first, Seq: b.nextSeq()}
		}
		for d, dd := range dirs {
			n := node{c.n.x + dd[0], c.n.y + dd[1]}
			if seen[n] || !v.Walkable(n.x, n.y) || (occ[n] && !goal(n.x, n.y)) {
				continue
			}
			seen[n] = true
			f := c.first
			if f < 0 {
				f = d
			}
			q = append(q, qe{n, f})
		}
	}
	return nil
}

func (b *Brain) stepTo(v *View, me *Ent, tx, ty int, adjacentOK bool) *proto.ClientMsg {
	return b.bfs(v, me, func(x, y int) bool { return cheb(x, y, tx, ty) <= 1 })
}

func (b *Brain) explore(v *View, me *Ent) *proto.ClientMsg {
	return b.bfs(v, me, func(x, y int) bool {
		for _, d := range dirs {
			if v.Unknown(x+d[0], y+d[1]) {
				return true
			}
		}
		return false
	})
}
