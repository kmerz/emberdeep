package room

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"emberdeep/internal/bot"
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
	brain          *bot.Brain
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
	ctl     chan func()
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
	saved      *game.RunSave // the saved game the next start continues (nil = a new run)
}

func newRoom(h *Hub, code string, seed uint32) *Room {
	r := &Room{hub: h, code: code, cfg: &h.cfg, seed: seed,
		intents: make(chan inMsg, 1024), joins: make(chan joinReq), leaves: make(chan *Conn, 16), ctl: make(chan func(), 16), done: make(chan struct{})}
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

// Inspect runs f inside the room goroutine (which owns the Game) and waits for it. Used by tests and tools.
func (r *Room) Inspect(f func(g *game.Game, players int)) {
	done := make(chan struct{})
	select {
	case r.ctl <- func() { f(r.g, len(r.players)); close(done) }:
	case <-r.done:
		return
	}
	select {
	case <-done:
	case <-r.done:
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
		case f := <-r.ctl:
			f()
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
	if r.g != nil && r.g.Phase != game.PhaseCleared {
		r.persist() // the room closes (empty, or the server stops): keep the run
	}
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
		lvl, saved := 1, false
		if r.g != nil {
			if h := r.g.Hero(p.ID); h != nil {
				lvl = h.Lvl
			}
		} else if sh, ok := r.savedHero(p); ok {
			lvl, saved = sh.Lvl(), true
		}
		l.Players = append(l.Players, proto.LobbyPlayer{Slot: i + 1, ID: p.ID, Name: p.Name, Cls: string(p.Cls), Lvl: lvl,
			Ready: p.Ready || p.Bot, PingMs: p.PingMs, Host: p.Host, Connected: p.connected(), Bot: p.Bot, Saved: saved})
	}
	if r.saved != nil {
		si := &proto.SaveInfo{Depth: r.saved.Depth, Village: r.saved.InVillage, Heroes: []proto.SaveHero{}}
		for name, h := range r.saved.Heroes {
			si.Heroes = append(si.Heroes, proto.SaveHero{Name: name, Cls: string(h.Cls), Lvl: h.Lvl()})
		}
		sort.Slice(si.Heroes, func(i, j int) bool {
			return si.Heroes[i].Lvl > si.Heroes[j].Lvl || si.Heroes[i].Lvl == si.Heroes[j].Lvl && si.Heroes[i].Name < si.Heroes[j].Name
		})
		l.Save = si
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
		// a human takes the slot of the most recently added bot
		bot := r.lastBot()
		if bot == nil {
			j.reply <- joinResp{err: ErrRoomFull}
			return
		}
		r.removePlayer(bot)
	}
	cls := game.ClassID(h.Cls)
	if !game.ValidClass(h.Cls) {
		cls = game.Warrior
	}
	if r.saved != nil {
		if sh, ok := r.saved.Heroes[name]; ok {
			cls = sh.Cls // back for a saved hero: same class, so they continue it
		}
	}
	r.nextPID++
	p := &Player{ID: r.nextPID, Name: name, Cls: cls, Token: newToken(), Conn: j.conn, PingMs: -1, hbSent: map[int]time.Time{}}
	j.conn.PlayerID = p.ID
	r.players = append(r.players, p)
	r.emptySince = time.Time{}
	r.assignHostIfNone()
	if r.g != nil {
		r.addHero(p) // join in progress: spawns next to the party
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
		c.Send(QuestsMsg(r.g))
		if ex := r.g.ExploredTiles(); len(ex) > 0 {
			c.Send(ExploreMsg(ex))
		}
		if r.overlay != "" {
			c.Send(mustJSON(proto.Overlay{T: "overlay", Kind: r.overlay, Summary: r.g.Summary()}))
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

// quit removes a player who left on purpose (/leave): the hero leaves the world at once instead of
// waiting out the reconnect window, and the slot is free for someone else.
func (r *Room) quit(p *Player) {
	r.removePlayer(p)
	if r.g != nil {
		r.sendAll(mustJSON(proto.Events{T: "events", Tick: r.g.Now, List: []game.Event{{K: "msg", Text: p.Name + " left the dungeon"}}}))
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
		if h := r.g.Hero(p.ID); h != nil && r.saved != nil {
			r.saved.Heroes[h.Name] = r.g.SaveHero(h) // the hero waits in the save for their player
		}
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
			// retry the floor the party fell on, with heroes as they arrived on it
			r.g.RetryFloor()
			r.overlay = "" // the client drops its overlay when a level arrives
			r.sendLevel()
		} else if r.g.Phase == game.PhaseCleared {
			r.endRun() // done reading the summary
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
		case "addBot":
			if !r.addBot() {
				return
			}
		case "removeBot":
			b := r.lastBot()
			if b == nil {
				return
			}
			r.removePlayer(b)
		case "newRun": // forget the saved game: the next start is a fresh descent
			if r.saved == nil {
				return
			}
			r.saved = nil
			deleteSave(r.cfg.SaveDir, r.code)
			r.seed = r.hub.nextSeed()
		default:
			return
		}
		r.broadcastLobby()
	case proto.CLeave:
		r.quit(p)
	case proto.CSave:
		if !p.Host || r.g == nil || r.g.Phase == game.PhaseCleared {
			return
		}
		r.persist()
		r.sendAll(mustJSON(proto.Events{T: "events", Tick: r.g.Now, List: []game.Event{{K: "msg", Text: p.Name + " saved the game"}}}))
		r.endRun()
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
	if r.saved != nil {
		run := *r.saved
		run.Settings = r.settings // the lobby may have changed the difficulty
		r.g = game.Load(run, len(r.players))
	} else {
		r.g = game.NewAt(r.seed, r.settings, len(r.players), r.cfg.StartDepth)
	}
	for _, p := range r.players {
		if _, ok := r.savedHero(p); !ok {
			continue
		}
		r.addHero(p).Connected = p.connected()
	}
	for _, p := range r.players { // new heroes after the saved ones, so they catch up to the party's level
		if _, ok := r.savedHero(p); !ok {
			r.addHero(p).Connected = p.connected()
		}
	}
	r.persist()
	r.g.Tick()
	r.sendAll(mustJSON(proto.Phase{T: "phase", Phase: "game"}))
	r.sendLevel()
	r.broadcastLobby()
}

// sendLevel sends the current floor and what is explored of it. The level message comes first because
// the client clears its map when it arrives.
func (r *Room) sendLevel() {
	r.g.TakeLevelChanged()
	r.g.TakeQuestsChanged()
	r.sendAll(LevelMsg(r.g))
	r.sendAll(QuestsMsg(r.g))
	if ex := r.g.TakeExplored(); len(ex) > 0 {
		r.sendAll(ExploreMsg(ex))
	}
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
	if r.g.TakeLevelChanged() {
		r.sendAll(LevelMsg(r.g)) // the party went down the stairs
		r.persist()
	}
	if r.g.TakeQuestsChanged() {
		r.sendAll(QuestsMsg(r.g))
	}
	if ex := r.g.TakeExplored(); len(ex) > 0 {
		r.sendAll(ExploreMsg(ex))
	}
	if len(evs) > 0 {
		r.sendAll(mustJSON(proto.Events{T: "events", Tick: r.g.Now, List: evs}))
	}
	r.afterTick()
	if r.g == nil { // the run just ended
		return
	}
	shared := BuildShared(r.g)
	for _, p := range r.players {
		if p.Conn != nil {
			p.Conn.SendSnap(BuildPrivate(r.g, shared, p.ID))
			r.Metrics.Snaps.Add(1)
		}
	}
	r.cfg.Stats.Record(time.Since(t0))
}

// afterTick handles run-level state changes: the wipe and cleared overlays, and the return to the lobby.
func (r *Room) afterTick() {
	switch r.g.Phase {
	case game.PhaseWiped:
		if r.overlay != "wipe" {
			r.overlay = "wipe"
			r.sendAll(mustJSON(proto.Overlay{T: "overlay", Kind: "wipe", Summary: r.g.Summary()}))
		}
	case game.PhaseCleared:
		if r.overlay != "cleared" {
			r.overlay = "cleared"
			r.sendAll(mustJSON(proto.Overlay{T: "overlay", Kind: "cleared", Summary: r.g.Summary()}))
			r.recordFame()
		}
		if r.g.Now-r.g.ClearedTick >= game.ClearedReturn {
			r.endRun()
		}
	}
}

// recordFame puts a won run in the Hall of Fame and tells the party if it made the list. Runs of bots alone,
// and runs that started below the top (a dev flag), do not count.
func (r *Room) recordFame() {
	if r.humans() == 0 || r.cfg.StartDepth > 1 {
		return
	}
	if rank := r.hub.fame.add(fameRun(r.g, r.players, r.cfg.Clock())); rank > 0 {
		r.sendAll(mustJSON(proto.Events{T: "events", Tick: r.g.Now, List: []game.Event{{K: "fame", Amount: rank}}}))
	}
}

// endRun returns the room to the lobby (a new seed for the next descent).
func (r *Room) endRun() {
	if r.g.Phase == game.PhaseCleared { // the run is won: nothing to continue
		r.saved = nil
		deleteSave(r.cfg.SaveDir, r.code)
	}
	r.g = nil
	r.overlay = ""
	if r.saved != nil {
		r.seed = r.saved.Seed
	} else {
		r.seed = r.hub.nextSeed()
	}
	for _, p := range r.players {
		p.Ready = false
	}
	r.sendAll(mustJSON(proto.Phase{T: "phase", Phase: "lobby"}))
	r.broadcastLobby()
}

// savedHero is the player's hero in the saved game, if there is one of the class they picked.
func (r *Room) savedHero(p *Player) (game.SavedHero, bool) {
	if r.saved == nil {
		return game.SavedHero{}, false
	}
	sh, ok := r.saved.Heroes[p.Name]
	return sh, ok && sh.Cls == p.Cls
}

// addHero puts a player's hero into the running game: their saved hero if they have one, else a new hero
// at the party's level.
func (r *Room) addHero(p *Player) *game.Hero {
	if sh, ok := r.savedHero(p); ok {
		return r.g.AddSavedHero(p.ID, p.Name, sh)
	}
	h := r.g.AddHero(p.ID, p.Name, p.Cls)
	r.g.CatchUp(h)
	return h
}

// persist saves the running game: in memory for the lobby, and to disk when a save dir is set.
func (r *Room) persist() {
	var prev map[string]game.SavedHero
	if r.saved != nil {
		prev = r.saved.Heroes
	}
	run := r.g.Save(prev)
	r.saved = &run
	if err := writeSave(r.cfg.SaveDir, r.code, run, r.cfg.Clock()); err != nil {
		log.Printf("room %s: save failed: %v", r.code, err)
	}
}

var botNames = []string{"bot-ash", "bot-vex", "bot-orm"}
var botClasses = []game.ClassID{game.Rogue, game.Monk, game.Sorcerer}

// lastBot returns the most recently added bot, or nil if the room has none.
func (r *Room) lastBot() *Player {
	for i := len(r.players) - 1; i >= 0; i-- {
		if r.players[i].Bot {
			return r.players[i]
		}
	}
	return nil
}

// addBots fills n slots with AI teammates (always ready); joining humans replace them.
func (r *Room) addBots(n int) {
	for i := 0; i < n && r.addBot(); i++ {
	}
}

// addBot adds one AI teammate with the first unused bot name. It reports false if the room is full
// or every bot name is taken.
func (r *Room) addBot() bool {
	if len(r.players) >= game.MaxHeroes {
		return false
	}
	for i, name := range botNames {
		taken := false
		for _, p := range r.players {
			taken = taken || p.Name == name
		}
		if taken {
			continue
		}
		r.nextPID++
		r.players = append(r.players, &Player{ID: r.nextPID, Name: name, Cls: botClasses[i], Bot: true, Ready: true,
			PingMs: -1, brain: bot.NewBrain(false)})
		return true
	}
	return false
}

// botThink runs every third tick (the hero step interval); intents go through the normal validation path.
func (r *Room) botThink() {
	if r.g.Now%game.MoveTicks != 0 {
		return
	}
	humans := map[int]bool{}
	for _, p := range r.players {
		if !p.Bot && p.Conn != nil {
			humans[p.ID] = true
		}
	}
	for _, p := range r.players {
		if !p.Bot || p.brain == nil {
			continue
		}
		h := r.g.Hero(p.ID)
		if h == nil {
			continue
		}
		// a god's trial: bots don't seek out altars, so a finished trial counts as prayed
		if h.TrialDone() {
			r.g.Bless(h)
		}
		r.g.EquipBetter(h) // wear the best gear in the rucksack
		// spend skill points (new skills land on the first free key, so keys 1-3 hold skills 0-2)
		if h.Points > 0 {
			if idx := game.NextSkill(h.Ranks, h.Lvl); idx >= 0 {
				r.applyIntent(p.ID, game.Intent{Kind: game.IntLearn, Skill: h.Def().Skills[idx].ID})
			}
		}
		v := r.botView(h, humans)
		for _, m := range p.brain.Think(v) {
			if it, ok := ToIntent(m); ok {
				r.applyIntent(p.ID, it)
			}
		}
	}
}

func (r *Room) botView(h *game.Hero, humans map[int]bool) *bot.View {
	g := r.g
	v := &bot.View{Me: h.ID, Cls: string(h.Cls), Res: h.Res, Potions: h.Count("potion"), HP: h.HP, HPMax: h.HPMax, Humans: humans,
		PotCD: max(0, h.PotionReady-g.Now), CD: make([]int, 3), Cost: make([]int, 3)}
	for i := 0; i < 3; i++ {
		v.CD[i], _, v.Cost[i] = keyInfo(g, h, i)
	}
	for _, o := range g.Heroes {
		v.Ents = append(v.Ents, bot.Ent{ID: o.ID, K: "h", X: o.X, Y: o.Y, Downed: o.Downed, Dead: o.Dead})
	}
	for _, m := range g.Mons {
		if m.Def().Role == "keg" { // a keg is in the way, but bots don't shoot it
			v.Ents = append(v.Ents, bot.Ent{ID: m.ID, K: "n", X: m.X, Y: m.Y})
			continue
		}
		if g.IsVisible(m.X, m.Y) && !m.Disguised && !m.Neutral && g.Revealed(m) { // bots spot no mimics or hidden shades, and leave ant workers be
			v.Ents = append(v.Ents, bot.Ent{ID: m.ID, K: "m", X: m.X, Y: m.Y, Elite: m.Elite, HP: m.HP, HPMax: m.HPMax})
		}
	}
	// gnomes and the notice board block the way: without them a bot paths through the board and bumps it forever
	for _, n := range g.Npcs {
		v.Ents = append(v.Ents, bot.Ent{ID: n.ID, K: "n", X: n.X, Y: n.Y})
	}
	v.Walkable = func(x, y int) bool {
		return x >= 0 && y >= 0 && x < game.LW && y < game.LH && g.Explored[y*game.LW+x] && g.Level.Walkable(x, y) && !g.HazardAt(x, y)
	}
	v.Unknown = func(x, y int) bool {
		return x >= 0 && y >= 0 && x < game.LW && y < game.LH && !g.Explored[y*game.LW+x]
	}
	return v
}
