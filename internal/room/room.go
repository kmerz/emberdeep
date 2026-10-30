package room

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"emberdeep/internal/game"
	"emberdeep/internal/proto"
)

var (
	ErrRoomFull   = errors.New("room: room full")
	ErrBadName    = errors.New("room: name must be 2-12 letters, digits, _ or -")
	ErrRoomClosed = errors.New("room: room closed")
)

type Player struct {
	ID       int
	Name     string
	Cls      game.ClassID
	Ready    bool
	Token    string
	Host     bool
	Bot      bool
	Conn     *Conn
	PingMs   int
	hbSent   map[int]time.Time
	lastPing time.Time // last map ping, for the 0.5 s limit
	// DisconnectedAt is set while the hero is held for reconnection.
	DisconnectedAt time.Time
	brain          *botBrain
}

func (p *Player) connected() bool { return p.Bot || p.Conn != nil }

type inMsg struct {
	conn *Conn
	m    proto.ClientMsg
}

type joinReq struct {
	conn  *Conn
	hello proto.ClientMsg
	reply chan joinResp
}

type joinResp struct {
	res JoinResult
	err error
}

type JoinResult struct {
	PlayerID int
	Token    string
	IsHost   bool
}

// Metrics are atomic counters readable from any goroutine.
type Metrics struct {
	Applied  atomic.Int64
	Rejected atomic.Int64
	Ticks    atomic.Int64
	Snaps    atomic.Int64
}

// Room owns one Game exclusively; only run() touches game, players and settings.
type Room struct {
	hub  *Hub
	code string
	cfg  *Config

	intents chan inMsg
	joins   chan joinReq
	leaves  chan *Conn
	done    chan struct{}
	Metrics Metrics

	// owned by the room goroutine
	players    []*Player
	nextPID    int
	settings   game.Settings
	seed       uint32
	g          *game.Game
	hbN        int
	emptySince time.Time
	clearedAt  int
	overlay    string
	closed     bool
}

func newRoom(h *Hub, code string, seed uint32) *Room {
	r := &Room{hub: h, code: code, cfg: &h.cfg, seed: seed,
		intents: make(chan inMsg, 1024), joins: make(chan joinReq), leaves: make(chan *Conn, 16), done: make(chan struct{})}
	r.emptySince = r.cfg.Clock()
	return r
}

func (r *Room) Code() string          { return r.code }
func (r *Room) Done() <-chan struct{} { return r.done }

// Join blocks until the room goroutine has processed the join.
func (r *Room) Join(c *Conn, hello proto.ClientMsg) (JoinResult, error) {
	req := joinReq{conn: c, hello: hello, reply: make(chan joinResp, 1)}
	select {
	case r.joins <- req:
	case <-r.done:
		return JoinResult{}, ErrRoomClosed
	}
	select {
	case resp := <-req.reply:
		return resp.res, resp.err
	case <-r.done:
		return JoinResult{}, ErrRoomClosed
	}
}

func (r *Room) Submit(c *Conn, m proto.ClientMsg) {
	select {
	case r.intents <- inMsg{c, m}:
	case <-r.done:
	}
}

func (r *Room) Leave(c *Conn) {
	select {
	case r.leaves <- c:
	case <-r.done:
	}
}

func (r *Room) run(ctx context.Context) {
	tick := time.NewTicker(r.cfg.TickEvery)
	hb := time.NewTicker(r.cfg.HbEvery)
	defer tick.Stop()
	defer hb.Stop()
	defer r.shutdown()
	for {
		select {
		case <-ctx.Done():
			return
		case j := <-r.joins:
			r.handleJoin(j)
		case c := <-r.leaves:
			r.handleLeave(c)
		case m := <-r.intents:
			r.handleMsg(m)
		case <-tick.C:
			r.onTick(r.cfg.Clock())
			if r.closed {
				return
			}
		case <-hb.C:
			r.onHb(r.cfg.Clock())
		}
	}
}

func (r *Room) shutdown() {
	r.hub.remove(r.code)
	close(r.done)
	for _, p := range r.players {
		if p.Conn != nil {
			p.Conn.Close()
		}
	}
}

// ---- helpers (room goroutine only) ----

func (r *Room) humans() int {
	n := 0
	for _, p := range r.players {
		if !p.Bot {
			n++
		}
	}
	return n
}

func (r *Room) connectedHumans() int {
	n := 0
	for _, p := range r.players {
		if !p.Bot && p.Conn != nil {
			n++
		}
	}
	return n
}

func (r *Room) byConn(c *Conn) *Player {
	for _, p := range r.players {
		if p.Conn == c {
			return p
		}
	}
	return nil
}

func (r *Room) byID(id int) *Player {
	for _, p := range r.players {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func (r *Room) sendAll(b []byte) {
	for _, p := range r.players {
		if p.Conn != nil {
			p.Conn.Send(b)
		}
	}
}

func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (r *Room) inviteURL() string {
	base := strings.TrimRight(r.cfg.PublicURL, "/")
	return base + "/r/" + r.code
}

func (r *Room) assignHost() {
	for _, p := range r.players {
		p.Host = false
	}
	for _, p := range r.players {
		if !p.Bot && p.Conn != nil {
			p.Host = true
			return
		}
	}
	for _, p := range r.players {
		if !p.Bot {
			p.Host = true
			return
		}
	}
}

// ---- lobby ----

func (r *Room) lobbyMsg() []byte {
	l := proto.Lobby{T: "lobby", Code: r.code, InGame: r.g != nil, Players: []proto.LobbyPlayer{}}
	for i, p := range r.players {
		lvl := 1
		if r.g != nil {
			if h := r.g.Hero(p.ID); h != nil {
				lvl = h.Lvl
			}
		}
		l.Players = append(l.Players, proto.LobbyPlayer{Slot: i + 1, ID: p.ID, Name: p.Name, Cls: string(p.Cls), Lvl: lvl,
			Ready: p.Ready || p.Bot, PingMs: p.PingMs, Host: p.Host, Connected: p.connected(), Bot: p.Bot})
	}
	l.Settings = proto.LobbySettings{Difficulty: game.Difficulties[r.settings.Difficulty].Name, Seed: SeedString(r.seed),
		Loot: "personal drops · shared gold", FriendlyFire: r.settings.FriendlyFire}
	return mustJSON(l)
}

func (r *Room) broadcastLobby() { r.sendAll(r.lobbyMsg()) }

func (r *Room) handleJoin(j joinReq) {
	h := j.hello
	// reconnect with a session token
	if h.Token != "" {
		for _, p := range r.players {
			if p.Token == h.Token && !p.Bot {
				if p.Conn != nil && p.Conn != j.conn {
					p.Conn.Kick("session resumed elsewhere")
				}
				p.Conn = j.conn
				p.DisconnectedAt = time.Time{}
				j.conn.PlayerID = p.ID
				if r.g != nil {
					r.g.SetConnected(p.ID, true)
				}
				r.emptySince = time.Time{}
				r.assignHostIfNone()
				r.syncConn(p)
				r.broadcastLobby()
				j.reply <- joinResp{res: JoinResult{PlayerID: p.ID, Token: p.Token, IsHost: p.Host}}
				return
			}
		}
	}
	name, ok := SanitizeName(h.Name)
	if !ok {
		j.reply <- joinResp{err: ErrBadName}
		return
	}
	if len(r.players) >= game.MaxHeroes {
		j.reply <- joinResp{err: ErrRoomFull}
		return
	}
	cls := game.ClassID(h.Cls)
	if !game.ValidClass(h.Cls) {
		cls = game.Warrior
	}
	r.nextPID++
	p := &Player{ID: r.nextPID, Name: name, Cls: cls, Token: newToken(), Conn: j.conn, PingMs: -1, hbSent: map[int]time.Time{}}
	j.conn.PlayerID = p.ID
	r.players = append(r.players, p)
	r.emptySince = time.Time{}
	r.assignHostIfNone()
	if r.g != nil {
		r.g.AddHero(p.ID, p.Name, p.Cls) // join in progress: spawns next to the party
	}
	r.syncConn(p)
	r.broadcastLobby()
	j.reply <- joinResp{res: JoinResult{PlayerID: p.ID, Token: p.Token, IsHost: p.Host}}
}

func (r *Room) assignHostIfNone() {
	for _, p := range r.players {
		if p.Host && p.connected() {
			return
		}
	}
	r.assignHost()
}

// syncConn sends everything a (re)joining client needs. All of it travels as events, never as snapshots.
func (r *Room) syncConn(p *Player) {
	c := p.Conn
	c.Send(mustJSON(proto.Welcome{T: "welcome", You: p.ID, Token: p.Token, Code: r.code, InviteURL: r.inviteURL(), IsHost: p.Host}))
	c.Send(r.lobbyMsg())
	if r.g != nil {
		c.Send(mustJSON(proto.Phase{T: "phase", Phase: "game"}))
		c.Send(LevelMsg(r.g))
		if ex := r.g.ExploredTiles(); len(ex) > 0 {
			c.Send(ExploreMsg(ex))
		}
		if r.overlay != "" {
			c.Send(mustJSON(proto.Overlay{T: "overlay", Kind: r.overlay}))
		}
	} else {
		c.Send(mustJSON(proto.Phase{T: "phase", Phase: "lobby"}))
	}
}

func (r *Room) handleLeave(c *Conn) {
	p := r.byConn(c)
	if p == nil {
		return
	}
	p.Conn = nil
	p.PingMs = -1
	if r.g == nil {
		r.removePlayer(p)
	} else {
		p.DisconnectedAt = r.cfg.Clock()
		r.g.SetConnected(p.ID, false) // the hero stays: idle and invulnerable
		if p.Host {
			p.Host = false
			r.assignHost()
		}
	}
	if r.connectedHumans() == 0 && r.emptySince.IsZero() {
		r.emptySince = r.cfg.Clock()
	}
	r.broadcastLobby()
}

func (r *Room) removePlayer(p *Player) {
	for i, o := range r.players {
		if o == p {
			r.players = append(r.players[:i], r.players[i+1:]...)
			break
		}
	}
	if r.g != nil {
		r.g.RemoveHero(p.ID)
	}
	if p.Host {
		r.assignHost()
	}
	if r.humans() == 0 && r.emptySince.IsZero() {
		r.emptySince = r.cfg.Clock()
	}
}

func (r *Room) allReady() bool {
	if len(r.players) == 0 {
		return false
	}
	for _, p := range r.players {
		if !p.Bot && (!p.Ready || p.Conn == nil) {
			return false
		}
	}
	return r.connectedHumans() > 0
}

func (r *Room) handleMsg(in inMsg) {
	p := r.byConn(in.conn)
	if p == nil {
		return
	}
	m := in.m
	switch m.T {
	case proto.CReady:
		if r.g != nil {
			return
		}
		p.Ready = !p.Ready
		r.broadcastLobby()
		if r.allReady() {
			r.startGame()
		}
	case proto.CSetClass:
		if r.g != nil || !game.ValidClass(m.Cls) {
			return
		}
		p.Cls = game.ClassID(m.Cls)
		p.Ready = false
		r.broadcastLobby()
	case proto.CStart:
		if !p.Host {
			return
		}
		if r.g == nil {
			r.startGame()
		} else if r.g.Phase == game.PhaseWiped {
			r.startGame() // restart with the same seed
		}
	case proto.CSetting:
		if !p.Host || r.g != nil {
			return
		}
		switch m.Key {
		case "friendlyFire":
			r.settings.FriendlyFire = !r.settings.FriendlyFire
		case "difficulty":
			r.settings.Difficulty = (r.settings.Difficulty + 1) % len(game.Difficulties)
		default:
			return
		}
		r.broadcastLobby()
	case proto.CHbAck:
		if t, ok := p.hbSent[m.N]; ok {
			p.PingMs = int(r.cfg.Clock().Sub(t).Milliseconds())
			delete(p.hbSent, m.N)
		}
	case proto.CChat:
		text := SanitizeChat(m.Text)
		if text == "" {
			return
		}
		r.sendAll(mustJSON(proto.Chat{T: "chat", From: p.ID, Name: p.Name, Cls: string(p.Cls), Text: text}))
	case proto.CPing:
		now := r.cfg.Clock()
		if r.g == nil || now.Sub(p.lastPing) < 500*time.Millisecond {
			return
		}
		x, y := m.Cell[0], m.Cell[1]
		if x < 0 || y < 0 || x >= game.LW || y >= game.LH || !r.g.Explored[y*game.LW+x] {
			return
		}
		p.lastPing = now
		r.sendAll(mustJSON(proto.Ping{T: "ping", From: p.ID, Name: p.Name, Cls: string(p.Cls), Cell: m.Cell}))
	default:
		if r.g == nil {
			return
		}
		if it, ok := ToIntent(m); ok {
			r.applyIntent(p.ID, it)
		}
	}
}

func (r *Room) applyIntent(pid int, it game.Intent) {
	if err := r.g.Apply(pid, it); err != nil {
		r.Metrics.Rejected.Add(1)
	} else {
		r.Metrics.Applied.Add(1)
	}
}

// ---- game lifecycle ----

func (r *Room) startGame() {
	r.overlay = ""
	r.g = game.New(r.seed, r.settings, len(r.players))
	for _, p := range r.players {
		h := r.g.AddHero(p.ID, p.Name, p.Cls)
		h.Connected = p.connected()
	}
	r.g.Tick()
	r.sendAll(mustJSON(proto.Phase{T: "phase", Phase: "game"}))
	r.sendAll(LevelMsg(r.g))
	if ex := r.g.TakeExplored(); len(ex) > 0 {
		r.sendAll(ExploreMsg(ex))
	}
	r.broadcastLobby()
}

func (r *Room) onHb(now time.Time) {
	r.hbN++
	msg := mustJSON(proto.Hb{T: "hb", N: r.hbN})
	for _, p := range r.players {
		if p.Conn == nil {
			continue
		}
		p.hbSent[r.hbN] = now
		for n, t := range p.hbSent { // forget unanswered pings
			if now.Sub(t) > 30*time.Second {
				delete(p.hbSent, n)
			}
		}
		p.Conn.Send(msg)
	}
	r.broadcastLobby()
}

func (r *Room) checkTimeouts(now time.Time) {
	for _, p := range append([]*Player(nil), r.players...) {
		if !p.Bot && p.Conn == nil && !p.DisconnectedAt.IsZero() && now.Sub(p.DisconnectedAt) >= r.cfg.ReconnectWindow {
			r.removePlayer(p)
			r.broadcastLobby()
		}
	}
	if r.connectedHumans() == 0 {
		if r.emptySince.IsZero() {
			r.emptySince = now
		}
		if now.Sub(r.emptySince) >= r.cfg.EmptyTimeout {
			r.closed = true
		}
	} else {
		r.emptySince = time.Time{}
	}
}

func (r *Room) onTick(now time.Time) {
	r.checkTimeouts(now)
	if r.closed || r.g == nil || r.connectedHumans() == 0 {
		return
	}
	t0 := time.Now()
	r.botThink()
	evs := r.g.Tick()
	r.Metrics.Ticks.Add(1)
	if ex := r.g.TakeExplored(); len(ex) > 0 {
		r.sendAll(ExploreMsg(ex))
	}
	if len(evs) > 0 {
		r.sendAll(mustJSON(proto.Events{T: "events", Tick: r.g.Now, List: evs}))
	}
	r.afterTick()
	shared := BuildShared(r.g)
	for _, p := range r.players {
		if p.Conn != nil {
			p.Conn.SendSnap(BuildPrivate(r.g, shared, p.ID))
			r.Metrics.Snaps.Add(1)
		}
	}
	r.cfg.Stats.Record(time.Since(t0))
}

// afterTick handles run-level state changes (wipe, cleared). Filled in by later milestones.
func (r *Room) afterTick() {}

func (r *Room) botThink() {}

type botBrain struct{}

var _ = fmt.Sprintf
