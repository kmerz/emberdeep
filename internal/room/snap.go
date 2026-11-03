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
	if h.Shrine != "" && g.Now < h.ShrineUntil {
		st = append(st, proto.Status{K: h.Shrine, T: ceilSec(h.ShrineUntil - g.Now)}) // haste, fury or blood
	}
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
	if g.Now < h.CharmUntil {
		st = append(st, proto.Status{K: "charmed", T: ceilSec(h.CharmUntil - g.Now)})
	}
	if g.LanternLit(h) {
		st = append(st, proto.Status{K: "torch", T: ceilSec(h.Oil)}) // the lantern burns
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
	var up *[2]int
	if l.HasUp {
		up = &[2]int{l.Up.X, l.Up.Y}
	}
	return mustJSON(proto.Level{T: "level", W: l.W, H: l.H, Seed: SeedString(g.Seed), Torches: tor, Up: up,
		Stairs: [2]int{l.Stairs.X, l.Stairs.Y}, Start: [2]int{l.Start.X, l.Start.Y}, Depth: g.Depth, MaxDepth: game.MaxDepth,
		Theme: l.Theme.ID, Glows: glows, Dark: l.Theme.Dark, Village: g.InVillage, Event: g.Event, EventTxt: game.FloorEvents[g.Event]})
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
			gl := "g"
			if n.Altar {
				gl = "A" // an altar of the Four
			} else if n.Shrine != "" {
				gl = "S" // a shrine ("s" once it is dark)
				if n.Used {
					gl = "s"
				}
			} else if n.Lore > 0 {
				gl = "i" // an inscription
			}
			e := proto.Ent{ID: n.ID, K: "n", G: gl, X: n.X, Y: n.Y, Name: n.Name, Trader: n.Trader, Smith: n.Smith, Board: n.Board, Follow: n.Follower}
			if n.Follower {
				e.HP, e.HPMax = n.HP, n.HPMax
			}
			s.Ents = append(s.Ents, e)
		}
	}
	for _, f := range g.Fires {
		if g.IsVisible(f.X, f.Y) {
			s.Fires = append(s.Fires, [2]int{f.X, f.Y})
		}
	}
	if p := g.Portal; p != nil {
		s.Portal = &[3]int{p.X, p.Y, ceilSec(p.Until - g.Now)}
	}
	for _, c := range g.Chests {
		if g.IsVisible(c.X, c.Y) {
			gl := "C"
			if c.Urn {
				gl = "u"
			} else if c.Vault {
				gl = "V"
			}
			s.Ents = append(s.Ents, proto.Ent{ID: c.ID, K: "i", G: gl, X: c.X, Y: c.Y})
		}
	}
	for _, t := range g.Traps {
		if t.Seen && g.IsVisible(t.X, t.Y) {
			armed := 0
			if g.Now >= t.ReadyAt {
				armed = 1
			}
			s.Traps = append(s.Traps, [3]int{t.X, t.Y, armed})
		}
	}
	for _, m := range g.Mons {
		if !g.IsVisible(m.X, m.Y) {
			continue
		}
		if !g.Revealed(m) { // a shade stays unseen until a hero is close
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
		if m.Named != "" { // a hunt target: labelled like a boss
			e.Name, e.Elite = m.Named, true
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
			s.Ents = append(s.Ents, itemEnt(it))
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
	s.Obj = proto.Objectives{Boss: boss != 0, BossName: game.MonDefs[boss].Name, Village: g.InVillage, PortalAt: g.PortalAt, PortalNeed: g.PortalNeed, KingAlive: !g.KingKilled, StairsAt: g.StairsAt, StairsNeed: g.StairsNeed, StairsLeft: g.StairsLeft, StairsUp: g.StairsUp,
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
		me.Pack[i] = proto.PackSlot{Item: s.Item, N: s.N, G: proto.ViewGear(s.G)}
	}
	me.Weapon, me.Trinket = proto.ViewGear(h.Weapon), proto.ViewGear(h.Trinket)
	if g.InVillage {
		for _, w := range g.Wares[pid] {
			v := proto.ViewGear(w)
			if v != nil {
				v.Price = game.WarePrice(w)
			}
			me.Wares = append(me.Wares, v)
		}
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
	me.Points, me.Respec, me.PackCap = h.Points, g.AtStairs(h), h.PackCap
	me.Oil, me.OilMax = ceilSec(h.Oil), game.OilMax/game.TickHz
	if h.TrialOn {
		me.Trial = &proto.Trial{God: game.Gods[h.Cls].Name, Need: h.TrialNeed(), Kills: h.TrialKills}
	}
	out = append(out, `,"ackSeq":`...)
	out = strconv.AppendInt(out, int64(h.LastSeq), 10)
	out = append(out, `,"me":`...)
	out = append(out, mustJSON(me)...)
	drops := []proto.Ent{}
	for _, it := range g.Items {
		if it.Owner == pid && g.IsVisible(it.X, it.Y) {
			drops = append(drops, itemEnt(it))
		}
	}
	out = append(out, `,"drops":`...)
	out = append(out, mustJSON(drops)...)
	return append(out, '}')
}

var questStates = map[game.QuestState]string{game.QuestOffered: "offered", game.QuestActive: "active", game.QuestDone: "done", game.QuestFailed: "failed"}

// QuestsMsg is the party's quest list.
func QuestsMsg(g *game.Game) []byte {
	out := proto.QuestList{T: "quests", List: []proto.QuestInfo{}}
	for _, q := range g.Quests {
		d := q.Def
		out.List = append(out.List, proto.QuestInfo{ID: d.ID, Title: d.Title, Text: d.Text, Giver: d.Giver, Kind: d.Kind, Depth: d.Depth,
			State: questStates[q.State], Progress: q.Progress, Count: max(1, d.Count), Gold: d.Gold, XP: d.XP, Point: d.Point})
	}
	return mustJSON(out)
}

// itemEnt is an item on the floor; gear carries its name and rarity, and rare things get a loot beam.
func itemEnt(it *game.Item) proto.Ent {
	e := proto.Ent{ID: it.ID, K: "i", G: string(rune(it.Kind)), X: it.X, Y: it.Y, HPMax: it.Amount, It: it.Def}
	if it.G != nil {
		e.Rar, e.Nm = it.G.Rarity, it.G.Name
	} else if d := game.Items[it.Def]; d != nil {
		e.Rar = d.Rarity
	}
	return e
}
