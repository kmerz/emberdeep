package room

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"

	"emberdeep/internal/game"
	"emberdeep/internal/proto"
)

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func ceilSec(ticks int) int {
	if ticks <= 0 {
		return 0
	}
	return (ticks + game.TickHz - 1) / game.TickHz
}

func SeedString(seed uint32) string { return fmt.Sprintf("%04X-%02X", seed>>8&0xFFFF, seed&0xFF) }

func heroStatuses(g *game.Game, h *game.Hero) []proto.Status {
	var st []proto.Status
	if g.Now < h.WallUntil {
		st = append(st, proto.Status{K: "wall", T: ceilSec(h.WallUntil - g.Now)})
	}
	if g.Now < h.BellUntil {
		st = append(st, proto.Status{K: "bell", T: ceilSec(h.BellUntil - g.Now)})
	}
	if g.Now < h.SlowUntil {
		st = append(st, proto.Status{K: "slow", T: ceilSec(h.SlowUntil - g.Now)})
	}
	if g.Now < h.CryUntil {
		st = append(st, proto.Status{K: "cry", T: ceilSec(h.CryUntil - g.Now)})
	}
	if g.Now < h.HiddenUntil {
		st = append(st, proto.Status{K: "hidden", T: ceilSec(h.HiddenUntil - g.Now)})
	}
	if h.PoisonCharges > 0 {
		st = append(st, proto.Status{K: "poison", T: h.PoisonCharges})
	}
	if g.Now < h.TorchUntil {
		st = append(st, proto.Status{K: "torch", T: ceilSec(h.TorchUntil - g.Now)})
	}
	return st
}

// keyInfo is the cooldown left, full cooldown and cost of the skill on a key. An empty key reports a cost
// nobody can pay, so the bot brain never presses it.
func keyInfo(g *game.Game, h *game.Hero, key int) (cd, cdMax, cost int) {
	b := h.Binds[key]
	if b < 0 || h.Ranks[b] == 0 {
		return 0, 0, 1 << 20
	}
	sk := h.Def().Skills[b].Ranked(h.Ranks[b])
	return max(0, h.CDReady[b]-g.Now), sk.CD, sk.Cost
}

func LevelMsg(g *game.Game) []byte {
	l := g.Level
	tor := make([][2]int, len(l.Torches))
	for i, t := range l.Torches {
		tor[i] = [2]int{t.X, t.Y}
	}
	glows := make([][2]int, len(l.Glows))
	for i, p := range l.Glows {
		glows[i] = [2]int{p.X, p.Y}
	}
	return mustJSON(proto.Level{T: "level", W: l.W, H: l.H, Seed: SeedString(g.Seed), Torches: tor,
		Stairs: [2]int{l.Stairs.X, l.Stairs.Y}, Start: [2]int{l.Start.X, l.Start.Y}, Depth: g.Depth, MaxDepth: game.MaxDepth,
		Theme: l.Theme.ID, Glows: glows, Dark: l.Theme.Dark, Village: g.InVillage})
}

func ExploreMsg(tiles []game.TileUpdate) []byte {
	out := make([][3]int, len(tiles))
	for i, t := range tiles {
		out[i] = [3]int{t.X, t.Y, int(t.T)}
	}
	return mustJSON(proto.Explore{T: "explore", Tiles: out})
}

func visBox(g *game.Game) proto.Vis {
	x0, y0, x1, y1 := game.LW, game.LH, -1, -1
	for y := 0; y < game.LH; y++ {
		row := g.Visible[y*game.LW : (y+1)*game.LW]
		for x, v := range row {
			if v {
				x0, x1 = min(x0, x), max(x1, x)
				y0, y1 = min(y0, y), max(y1, y)
			}
		}
	}
	if x1 < 0 {
		return proto.Vis{}
	}
	w, h := x1-x0+1, y1-y0+1
	bits := make([]byte, (w*h+7)/8)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if g.Visible[(y0+y)*game.LW+x0+x] {
				i := y*w + x
				bits[i>>3] |= 1 << (i & 7)
			}
		}
	}
	return proto.Vis{X0: x0, Y0: y0, W: w, H: h, B: base64.StdEncoding.EncodeToString(bits)}
}

func heroEnt(g *game.Game, h *game.Hero) proto.Ent {
	e := proto.Ent{ID: h.ID, K: "h", G: "@", X: h.X, Y: h.Y, HP: h.HP, HPMax: h.HPMax, Cls: string(h.Cls), Name: h.Name,
		Downed: h.Downed, Dead: h.Dead, Face: h.Facing, St: heroStatuses(g, h)}
	if h.Downed {
		e.DownT = ceilSec(h.DownedUntil - g.Now)
		for _, r := range g.Heroes {
			if r.ReviveTarget == h.ID && r.ReviveProg > 0 {
				p := float64(r.ReviveProg) / float64(game.ReviveTicks)
				if p > e.Rev {
					e.Rev = p
				}
			}
		}
	}
	return e
}

// BuildShared marshals the room-wide part of a snapshot. Entities are filtered to party vision.
func BuildShared(g *game.Game) []byte {
	s := proto.SnapShared{T: "snap", Tick: g.Now, Vis: visBox(g), Ents: []proto.Ent{}, Party: []proto.PartyEntry{}}
	for _, h := range g.Heroes {
		s.Ents = append(s.Ents, heroEnt(g, h))
		s.Party = append(s.Party, proto.PartyEntry{ID: h.ID, Name: h.Name, Cls: string(h.Cls), Lvl: h.Lvl, HP: h.HP, HPMax: h.HPMax,
			Res: h.Res, ResMax: h.ResMax, St: heroStatuses(g, h), Downed: h.Downed, Dead: h.Dead, Connected: h.Connected,
			DownT: ceilSec(h.DownedUntil - g.Now)})
		if !h.Downed {
			s.Party[len(s.Party)-1].DownT = 0
		}
	}
	for _, n := range g.Npcs {
		if g.IsVisible(n.X, n.Y) {
			s.Ents = append(s.Ents, proto.Ent{ID: n.ID, K: "n", G: "g", X: n.X, Y: n.Y, Name: n.Name, Trader: n.Trader})
		}
	}
	if p := g.Portal; p != nil {
		s.Portal = &[3]int{p.X, p.Y, ceilSec(p.Until - g.Now)}
	}
	for _, c := range g.Chests {
		if g.IsVisible(c.X, c.Y) {
			s.Ents = append(s.Ents, proto.Ent{ID: c.ID, K: "i", G: "C", X: c.X, Y: c.Y})
		}
	}
	for _, m := range g.Mons {
		if !g.IsVisible(m.X, m.Y) {
			continue
		}
		if m.Disguised { // looks exactly like a chest
			s.Ents = append(s.Ents, proto.Ent{ID: m.ID, K: "i", G: "C", X: m.X, Y: m.Y})
			continue
		}
		e := proto.Ent{ID: m.ID, K: "m", G: string(rune(m.Kind)), X: m.X, Y: m.Y, HP: m.HP, HPMax: m.HPMax, Elite: m.Elite, Face: m.Facing,
			Name: m.Def().Name}
		if m.Gen > 0 {
			e.Name = "Blobling"
		}
		if g.Now < m.SlowUntil {
			e.St = append(e.St, proto.Status{K: "slow", T: ceilSec(m.SlowUntil - g.Now)})
		}
		if g.Now < m.BleedUntil {
			e.St = append(e.St, proto.Status{K: "bleed", T: ceilSec(m.BleedUntil - g.Now)})
		}
		s.Ents = append(s.Ents, e)
	}
	for _, it := range g.Items {
		if it.Owner == 0 && g.IsVisible(it.X, it.Y) {
			s.Ents = append(s.Ents, proto.Ent{ID: it.ID, K: "i", G: string(rune(it.Kind)), X: it.X, Y: it.Y, HPMax: it.Amount, It: it.Def})
		}
	}
	for _, o := range g.Orbs {
		s.Orbs = append(s.Orbs, [3]int{o.X, o.Y, ceilSec(o.Until - g.Now)})
	}
	for _, p := range g.Patches {
		if g.IsVisible(p.X, p.Y) {
			s.Patches = append(s.Patches, [2]int{p.X, p.Y})
		}
	}
	st := g.Level.Stairs
	boss := game.Floor(g.Depth).Boss
	if g.InVillage {
		boss = 0
	}
	s.Obj = proto.Objectives{Boss: boss != 0, BossName: game.MonDefs[boss].Name, Village: g.InVillage, PortalAt: g.PortalAt, PortalNeed: g.PortalNeed, KingAlive: !g.KingKilled, StairsAt: g.StairsAt, StairsNeed: g.StairsNeed, StairsLeft: g.StairsLeft,
		StairsFound: g.Explored[st.Y*game.LW+st.X], Gold: g.PartyGold(), Kills: g.Kills}
	return mustJSON(s)
}

// BuildPrivate appends the client's own data (resource, cooldowns, potions, personal drops, ack) to the shared part.
func BuildPrivate(g *game.Game, shared []byte, pid int) []byte {
	h := g.Hero(pid)
	out := make([]byte, 0, len(shared)+400)
	out = append(out, shared[:len(shared)-1]...)
	if h == nil {
		return append(out, '}')
	}
	def := h.Def()
	me := proto.Me{Res: h.Res, ResMax: h.ResMax, Potions: h.Count("potion"), Pack: make([]proto.PackSlot, game.PackSlots), Gold: h.Gold, XP: h.XP, XPNeed: game.XPNeed(h.Lvl), Lvl: h.Lvl,
		CD: make([]int, 3), CDMax: make([]int, 3), Cost: make([]int, 3), PotCD: max(0, h.PotionReady-g.Now)}
	for i, s := range h.Pack {
		me.Pack[i] = proto.PackSlot{Item: s.Item, N: s.N}
	}
	me.Binds = make([]string, 3)
	for i := 0; i < 3; i++ {
		me.CD[i], me.CDMax[i], me.Cost[i] = keyInfo(g, h, i)
		if b := h.Binds[i]; b >= 0 {
			me.Binds[i] = def.Skills[b].ID
		}
	}
	me.Tree = make([]proto.TreeSkill, game.TreeSize)
	for i, s := range def.Skills {
		me.Tree[i] = proto.TreeSkill{ID: s.ID, Rank: h.Ranks[i]}
	}
	me.Points, me.Respec = h.Points, g.AtStairs(h)
	out = append(out, `,"ackSeq":`...)
	out = strconv.AppendInt(out, int64(h.LastSeq), 10)
	out = append(out, `,"me":`...)
	out = append(out, mustJSON(me)...)
	drops := []proto.Ent{}
	for _, it := range g.Items {
		if it.Owner == pid && g.IsVisible(it.X, it.Y) {
			drops = append(drops, proto.Ent{ID: it.ID, K: "i", G: string(rune(it.Kind)), X: it.X, Y: it.Y, HPMax: it.Amount, It: it.Def})
		}
	}
	out = append(out, `,"drops":`...)
	out = append(out, mustJSON(drops)...)
	return append(out, '}')
}
