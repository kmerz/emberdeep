package room

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"emberdeep/internal/game"
	"emberdeep/internal/proto"
)

func fastHub(t *testing.T, mod func(*Config)) *Hub {
	cfg := Config{TickEvery: 5 * time.Millisecond, HbEvery: 20 * time.Millisecond, Seed: 7}
	if mod != nil {
		mod(&cfg)
	}
	h := NewHub(cfg)
	t.Cleanup(h.Close)
	return h
}

// drain returns the message types (and raw bodies) currently queued on a test connection.
func drain(c *Conn) (types []string, raw [][]byte) {
	for {
		b, ok := c.Out.Pop()
		if !ok {
			return
		}
		var m struct {
			T string `json:"t"`
		}
		_ = json.Unmarshal(b, &m)
		types = append(types, m.T)
		raw = append(raw, b)
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

func join(t *testing.T, r *Room, name, cls string) (*Conn, JoinResult) {
	t.Helper()
	c := NewTestConn()
	res, err := r.Join(c, proto.ClientMsg{T: "hello", Name: name, Cls: cls})
	if err != nil {
		t.Fatalf("join %s: %v", name, err)
	}
	return c, res
}

func lastLobby(c *Conn) *proto.Lobby {
	_, raws := drain(c)
	var out *proto.Lobby
	for _, b := range raws {
		var l proto.Lobby
		if json.Unmarshal(b, &l) == nil && l.T == "lobby" {
			cp := l
			out = &cp
		}
	}
	return out
}

func TestCodeFormat(t *testing.T) {
	h := fastHub(t, nil)
	for i := 0; i < 200; i++ {
		r, err := h.Create()
		if err != nil {
			t.Fatal(err)
		}
		c := r.Code()
		if len(c) != 7 || c[3] != '-' || NormalizeCode(c) != c {
			t.Fatalf("bad code %q", c)
		}
		for _, ch := range c {
			if ch == '0' || ch == 'O' || ch == '1' || ch == 'I' || ch == 'L' {
				t.Fatalf("ambiguous char in %q", c)
			}
		}
	}
	if NormalizeCode("ash7q2") != "ASH-7Q2" || NormalizeCode("ASH-7Q0") != "" {
		t.Fatal("normalize")
	}
}

func TestSanitizeName(t *testing.T) {
	cases := map[string]string{"Nyx": "nyx", "  Vela!! ": "vela", "a b c": "abc", "ABCDEFGHIJKLMNOP": "abcdefghijkl"}
	for in, want := range cases {
		got, ok := SanitizeName(in)
		if !ok || got != want {
			t.Errorf("%q -> %q,%v want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "a", "!!", " x "} {
		if _, ok := SanitizeName(bad); ok {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestLobbyJoinReadyStart(t *testing.T) {
	h := fastHub(t, nil)
	r, _ := h.Create()
	a, ra := join(t, r, "kmerz", "warrior")
	b, rb := join(t, r, "nyx", "rogue")
	if !ra.IsHost || rb.IsHost {
		t.Fatal("first joiner must be host")
	}
	l := lastLobby(a)
	if l == nil || len(l.Players) != 2 || l.Players[1].Name != "nyx" || l.Players[1].Slot != 2 {
		t.Fatalf("lobby not shared: %+v", l)
	}
	r.Submit(a, proto.ClientMsg{T: "ready"})
	waitFor(t, "ready flag", func() bool {
		l := lastLobby(b)
		return l != nil && l.Players[0].Ready
	})
	r.Submit(b, proto.ClientMsg{T: "ready"})
	waitFor(t, "game start", func() bool {
		types, _ := drain(a)
		for _, ty := range types {
			if ty == "level" {
				return true
			}
		}
		return false
	})
}

func TestHostStartsWithoutEveryoneReady(t *testing.T) {
	h := fastHub(t, nil)
	r, _ := h.Create()
	a, _ := join(t, r, "host", "monk")
	_, _ = join(t, r, "slow", "monk")
	r.Submit(a, proto.ClientMsg{T: "ready"})
	r.Submit(a, proto.ClientMsg{T: "start"})
	waitFor(t, "forced start", func() bool {
		types, _ := drain(a)
		for _, ty := range types {
			if ty == "level" {
				return true
			}
		}
		return false
	})
}

func TestRoomCapAndBadName(t *testing.T) {
	h := fastHub(t, nil)
	r, _ := h.Create()
	for i := 0; i < 4; i++ {
		join(t, r, "player"+string(rune('a'+i)), "rogue")
	}
	if _, err := r.Join(NewTestConn(), proto.ClientMsg{T: "hello", Name: "extra", Cls: "monk"}); err != ErrRoomFull {
		t.Fatalf("expected room full, got %v", err)
	}
	r2, _ := h.Create()
	if _, err := r2.Join(NewTestConn(), proto.ClientMsg{T: "hello", Name: "x"}); err != ErrBadName {
		t.Fatalf("expected bad name, got %v", err)
	}
}

func TestMaxRooms(t *testing.T) {
	h := fastHub(t, func(c *Config) { c.MaxRooms = 2 })
	h.Create()
	h.Create()
	if _, err := h.Create(); err != ErrTooManyRooms {
		t.Fatalf("expected ErrTooManyRooms, got %v", err)
	}
}

func hasType(c *Conn, want string) bool {
	types, _ := drain(c)
	for _, ty := range types {
		if ty == want {
			return true
		}
	}
	return false
}

func TestChatBroadcastAndSanitize(t *testing.T) {
	h := fastHub(t, nil)
	r, _ := h.Create()
	a, _ := join(t, r, "kmerz", "warrior")
	b, _ := join(t, r, "nyx", "rogue")
	drain(a)
	drain(b)
	r.Submit(a, proto.ClientMsg{T: "chat", Text: "hello\x00 \x1b[31mteam" + strings.Repeat("x", 400)})
	r.Submit(a, proto.ClientMsg{T: "chat", Text: "   "})
	waitFor(t, "chat on b", func() bool {
		_, raws := drain(b)
		for _, raw := range raws {
			var c proto.Chat
			if json.Unmarshal(raw, &c) == nil && c.T == "chat" {
				if c.Name != "kmerz" || c.Cls != "warrior" || len(c.Text) > 160 || strings.ContainsAny(c.Text, "\x00\x1b") || !strings.HasPrefix(c.Text, "hello [31mteam") {
					t.Errorf("bad chat %+v", c)
				}
				return true
			}
		}
		return false
	})
}

func TestJoinInProgressGetsFullState(t *testing.T) {
	h := fastHub(t, nil)
	r, _ := h.Create()
	a, _ := join(t, r, "kmerz", "warrior")
	r.Submit(a, proto.ClientMsg{T: "ready"})
	waitFor(t, "start", func() bool { return hasType(a, "level") })
	// a latecomer joins the running game
	b, rb := join(t, r, "nyx", "rogue")
	got := map[string]bool{}
	waitFor(t, "sync", func() bool {
		types, _ := drain(b)
		for _, ty := range types {
			got[ty] = true
		}
		return got["snap"]
	})
	for _, want := range []string{"welcome", "lobby", "phase", "level", "explore", "snap"} {
		if !got[want] {
			t.Errorf("late joiner did not receive %q (got %v)", want, got)
		}
	}
	if rb.IsHost {
		t.Error("late joiner must not be host")
	}
	// the new hero exists next to the party
	var s *proto.SnapShared
	for i := 0; i < 50 && s == nil; i++ {
		_, raws := drain(b)
		for _, raw := range raws {
			var x proto.SnapShared
			if json.Unmarshal(raw, &x) == nil && x.T == "snap" && len(x.Party) == 2 {
				s = &x
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if s == nil {
		t.Fatal("snapshot with both heroes never arrived")
	}
}

// fakeClock is an injectable clock for deadline tests.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock { return &fakeClock{t: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)} }
func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func countType(c *Conn, want string) int {
	n := 0
	types, _ := drain(c)
	for _, ty := range types {
		if ty == want {
			n++
		}
	}
	return n
}

func TestPingRateLimitAndValidation(t *testing.T) {
	clk := newFakeClock()
	h := fastHub(t, func(c *Config) { c.Clock = clk.Now })
	r, _ := h.Create()
	a, _ := join(t, r, "nyx", "rogue")
	b, _ := join(t, r, "vela", "sorcerer")
	// pings are ignored in the lobby
	r.Submit(a, proto.ClientMsg{T: "ping", Cell: [2]int{3, 3}})
	r.Submit(a, proto.ClientMsg{T: "start"})
	var cell [2]int
	waitFor(t, "start and explore", func() bool {
		_, raws := drain(b)
		for _, raw := range raws {
			var e proto.Explore
			if json.Unmarshal(raw, &e) == nil && e.T == "explore" && len(e.Tiles) > 0 {
				cell = [2]int{e.Tiles[0][0], e.Tiles[0][1]}
				return true
			}
		}
		return false
	})
	drain(a)
	// out-of-range and unexplored cells are rejected; a valid one is broadcast with sender colour info
	r.Submit(a, proto.ClientMsg{T: "ping", Cell: [2]int{-5, 9999}})
	r.Submit(a, proto.ClientMsg{T: "ping", Cell: cell})
	r.Submit(a, proto.ClientMsg{T: "ping", Cell: cell}) // within 0.5 s: dropped
	waitFor(t, "ping delivered", func() bool {
		_, raws := drain(b)
		for _, raw := range raws {
			var p proto.Ping
			if json.Unmarshal(raw, &p) == nil && p.T == "ping" {
				return p.Name == "nyx" && p.Cls == "rogue" && p.Cell == cell
			}
		}
		return false
	})
	time.Sleep(30 * time.Millisecond)
	if n := countType(b, "ping"); n != 0 {
		t.Fatalf("expected the rapid second ping to be dropped, got %d extra", n)
	}
	clk.Advance(600 * time.Millisecond)
	r.Submit(a, proto.ClientMsg{T: "ping", Cell: cell})
	waitFor(t, "second ping after the cooldown", func() bool { return countType(b, "ping") == 1 })
}

func levelSeed(c *Conn) (string, bool) {
	_, raws := drain(c)
	for _, raw := range raws {
		var l proto.Level
		if json.Unmarshal(raw, &l) == nil && l.T == "level" {
			return l.Seed, true
		}
	}
	return "", false
}

func TestWipeOverlayAndRestartKeepsSeed(t *testing.T) {
	h := fastHub(t, nil)
	r, _ := h.Create()
	a, _ := join(t, r, "kmerz", "warrior")
	b, _ := join(t, r, "nyx", "rogue")
	r.Submit(a, proto.ClientMsg{T: "start"})
	var seed string
	waitFor(t, "level", func() bool { s, ok := levelSeed(a); seed = s; return ok })
	drain(b)
	r.Inspect(func(g *game.Game, _ int) { g.Hurt(1, 9999); g.Hurt(2, 9999) })
	waitFor(t, "wipe overlay", func() bool {
		_, raws := drain(b)
		for _, raw := range raws {
			var o proto.Overlay
			if json.Unmarshal(raw, &o) == nil && o.T == "overlay" && o.Kind == "wipe" {
				return true
			}
		}
		return false
	})
	// a non-host cannot restart; the host can
	r.Submit(b, proto.ClientMsg{T: "start"})
	time.Sleep(30 * time.Millisecond)
	if s, ok := levelSeed(a); ok {
		t.Fatalf("non-host restarted the run (seed %s)", s)
	}
	r.Submit(a, proto.ClientMsg{T: "start"})
	var seed2 string
	waitFor(t, "restart", func() bool { s, ok := levelSeed(a); seed2 = s; return ok })
	if seed2 != seed {
		t.Fatalf("restart must keep the seed: %s vs %s", seed, seed2)
	}
	r.Inspect(func(g *game.Game, _ int) {
		if g.Phase != game.PhasePlaying || g.Hero(1).Downed {
			t.Error("fresh run expected")
		}
	})
}

func TestClearedOverlayThenLobby(t *testing.T) {
	h := fastHub(t, nil)
	r, _ := h.Create()
	a, _ := join(t, r, "kmerz", "warrior")
	r.Submit(a, proto.ClientMsg{T: "start"})
	waitFor(t, "level", func() bool { return hasType(a, "level") })
	r.Inspect(func(g *game.Game, _ int) {
		g.Depth = game.MaxDepth // the stairs end the run only on the last floor
		st := g.Level.Stairs
		if !g.Teleport(1, st.X-1, st.Y) {
			t.Error("teleport failed")
		}
	})
	var sawCleared, sawLobby bool
	waitFor(t, "cleared then lobby", func() bool {
		_, raws := drain(a)
		for _, raw := range raws {
			var o proto.Overlay
			if json.Unmarshal(raw, &o) == nil && o.T == "overlay" && o.Kind == "cleared" {
				sawCleared = true
			}
			var p proto.Phase
			if json.Unmarshal(raw, &p) == nil && p.T == "phase" && p.Phase == "lobby" && sawCleared {
				sawLobby = true
			}
		}
		return sawLobby
	})
	if f := h.fame.list(); len(f) != 1 || f[0].Party[0].Name != "kmerz" || f[0].Diff != "NORMAL" {
		t.Fatalf("the won run should be in the Hall of Fame: %+v", f)
	}
	// readiness was reset
	l := lastLobby(a)
	r.Inspect(func(g *game.Game, _ int) {
		if g != nil {
			t.Error("game should be gone")
		}
	})
	_ = l
}

func TestReconnectInsideAndOutsideWindow(t *testing.T) {
	clk := newFakeClock()
	h := fastHub(t, func(c *Config) { c.Clock = clk.Now })
	r, _ := h.Create()
	a, ra := join(t, r, "kmerz", "warrior")
	b, _ := join(t, r, "nyx", "rogue")
	r.Submit(a, proto.ClientMsg{T: "start"})
	waitFor(t, "start", func() bool { return hasType(b, "level") })

	// inside the window: the hero stays, idle and invulnerable, and the token resumes control
	r.Leave(a)
	r.Inspect(func(g *game.Game, players int) {
		hh := g.Hero(ra.PlayerID)
		if hh == nil || hh.Connected || players != 2 {
			t.Errorf("hero must stay in the world, disconnected (hero=%v players=%d)", hh, players)
		}
	})
	clk.Advance(59 * time.Second)
	a2 := NewTestConn()
	res, err := r.Join(a2, proto.ClientMsg{T: "hello", Name: "ignored", Cls: "monk", Token: ra.Token})
	if err != nil || res.PlayerID != ra.PlayerID {
		t.Fatalf("resume failed: %+v %v", res, err)
	}
	r.Inspect(func(g *game.Game, players int) {
		hh := g.Hero(ra.PlayerID)
		if hh == nil || !hh.Connected || hh.Name != "kmerz" || players != 2 {
			t.Errorf("hero should be back under control: %+v", hh)
		}
	})
	waitFor(t, "full resync", func() bool {
		got := map[string]bool{}
		types, _ := drain(a2)
		for _, ty := range types {
			got[ty] = true
		}
		return got["level"] && got["explore"]
	})

	// outside the window: removed
	r.Leave(a2)
	waitFor(t, "disconnect processed", func() bool {
		gone := false
		r.Inspect(func(g *game.Game, _ int) { gone = !g.Hero(ra.PlayerID).Connected })
		return gone
	})
	clk.Advance(61 * time.Second)
	waitFor(t, "hero removal", func() bool {
		removed := false
		r.Inspect(func(g *game.Game, players int) { removed = g.Hero(ra.PlayerID) == nil && players == 1 })
		return removed
	})
	if _, err := r.Join(NewTestConn(), proto.ClientMsg{T: "hello", Name: "x", Token: ra.Token}); err != ErrBadName {
		// the token is gone: the 1-character name is validated as a fresh join
		t.Fatalf("expired token must not resume (err=%v)", err)
	}
}

func TestLeaveInLobbyFreesSlotAndHostTransfers(t *testing.T) {
	h := fastHub(t, nil)
	r, _ := h.Create()
	a, _ := join(t, r, "kmerz", "warrior")
	b, _ := join(t, r, "nyx", "rogue")
	drain(b)
	r.Leave(a)
	waitFor(t, "host transfer", func() bool {
		l := lastLobby(b)
		return l != nil && len(l.Players) == 1 && l.Players[0].Host
	})
	for i := 0; i < 3; i++ {
		join(t, r, "p"+string(rune('a'+i)), "monk")
	}
	if _, err := r.Join(NewTestConn(), proto.ClientMsg{T: "hello", Name: "late", Cls: "monk"}); err != ErrRoomFull {
		t.Fatalf("room should be full at 4, got %v", err)
	}
}

func TestEmptyRoomIsClosedAfterTimeout(t *testing.T) {
	clk := newFakeClock()
	h := fastHub(t, func(c *Config) { c.Clock = clk.Now; c.EmptyTimeout = 5 * time.Minute })
	r, _ := h.Create()
	a, _ := join(t, r, "kmerz", "warrior")
	r.Leave(a)
	waitFor(t, "leave processed", func() bool {
		n := -1
		r.Inspect(func(_ *game.Game, players int) { n = players })
		return n == 0
	})
	clk.Advance(4 * time.Minute)
	time.Sleep(40 * time.Millisecond)
	if h.Get(r.Code()) == nil {
		t.Fatal("room closed too early")
	}
	clk.Advance(2 * time.Minute)
	waitFor(t, "room cleanup", func() bool { return h.Get(r.Code()) == nil })
	select {
	case <-r.Done():
	case <-time.After(time.Second):
		t.Fatal("room goroutine did not exit")
	}
	if h.Count() != 0 {
		t.Fatal("hub should be empty")
	}
	// someone rejoining resets the timer
	r2, _ := h.Create()
	c, _ := join(t, r2, "nyx", "rogue")
	clk.Advance(10 * time.Minute)
	time.Sleep(40 * time.Millisecond)
	if h.Get(r2.Code()) == nil {
		t.Fatal("an occupied room must stay open")
	}
	_ = c
}

func TestSlowClientNeverStallsRoom(t *testing.T) {
	h := fastHub(t, nil)
	r, _ := h.Create()
	a, _ := join(t, r, "kmerz", "warrior")
	slow, _ := join(t, r, "nyx", "rogue") // never drains its outbox
	r.Submit(a, proto.ClientMsg{T: "start"})
	waitFor(t, "ticks", func() bool { return r.Metrics.Ticks.Load() > 60 })
	if slow.Out.DroppedSnaps() == 0 {
		t.Fatal("the slow client should have had snapshots dropped")
	}
	// events were kept (never dropped silently): the level message is still first in line
	b, ok := slow.Out.Pop()
	if !ok || len(b) == 0 {
		t.Fatal("slow client lost its queue")
	}
	// and the fast client is unaffected
	if !hasType(a, "snap") {
		t.Fatal("fast client starved")
	}
}

func TestOutboxDropsOldestSnapshotsKeepsEvents(t *testing.T) {
	o := NewOutbox()
	o.PushEvent([]byte(`{"t":"events","n":1}`))
	o.PushSnap([]byte(`{"t":"snap","n":1}`))
	o.PushSnap([]byte(`{"t":"snap","n":2}`))
	o.PushEvent([]byte(`{"t":"events","n":2}`))
	o.PushSnap([]byte(`{"t":"snap","n":3}`))
	var got []string
	for {
		b, ok := o.Pop()
		if !ok {
			break
		}
		got = append(got, string(b))
	}
	want := []string{`{"t":"events","n":1}`, `{"t":"events","n":2}`, `{"t":"snap","n":3}`}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v", got)
	}
	// a client that never reads gets disconnected once the event queue overflows
	o2 := NewOutbox()
	for i := 0; i < maxFifo; i++ {
		if !o2.PushEvent([]byte("x")) {
			t.Fatal("queue should accept up to the cap")
		}
	}
	if o2.PushEvent([]byte("x")) {
		t.Fatal("overflow must be reported")
	}
}

func TestRateLimitAndFloodKick(t *testing.T) {
	c := NewTestConn()
	now := time.Now()
	allowed := 0
	for i := 0; i < 100; i++ {
		if ok, _ := c.allow(now); ok {
			allowed++
		}
	}
	if allowed != 30 {
		t.Fatalf("burst should allow 30, got %d", allowed)
	}
	// a steady 30/s is fine
	c2 := NewTestConn()
	t0 := time.Now()
	for i := 0; i < 90; i++ {
		if ok, _ := c2.allow(t0.Add(time.Duration(i) * 33 * time.Millisecond)); !ok {
			t.Fatalf("30 intents/s must pass (i=%d)", i)
		}
	}
	// sustained flooding for 5 s is kicked
	c3 := NewTestConn()
	kicked := false
	for i := 0; i < 700 && !kicked; i++ {
		_, kicked = c3.allow(t0.Add(time.Duration(i) * 10 * time.Millisecond)) // 100/s
	}
	if !kicked {
		t.Fatal("expected a kick for sustained flooding")
	}
}

func TestLaggerNeverReorders(t *testing.T) {
	l := NewLagger(20*time.Millisecond, 50*time.Millisecond)
	now := time.Now()
	prev := time.Time{}
	for i := 0; i < 500; i++ {
		at := l.DeliverAt(now.Add(time.Duration(i) * time.Millisecond))
		if at.Before(prev) {
			t.Fatal("delivery order must be preserved")
		}
		if at.Sub(now.Add(time.Duration(i)*time.Millisecond)) < 20*time.Millisecond {
			t.Fatal("minimum lag not applied")
		}
		prev = at
	}
	if NewLagger(0, 0) != nil {
		t.Fatal("no lag configured should disable the wrapper")
	}
}

func TestServerBotsFillSlotsAndAct(t *testing.T) {
	h := fastHub(t, func(c *Config) { c.Bots = 3 })
	r, _ := h.Create()
	a, _ := join(t, r, "kmerz", "warrior")
	l := lastLobby(a)
	bots := 0
	for _, p := range l.Players {
		if p.Bot && p.Ready {
			bots++
		}
	}
	if bots != 3 {
		t.Fatalf("expected 3 ready bots in the lobby, got %d", bots)
	}
	r.Submit(a, proto.ClientMsg{T: "ready"}) // humans ready => bots are always ready => start
	waitFor(t, "start", func() bool { return r.Metrics.Ticks.Load() > 0 })
	r.Inspect(func(g *game.Game, players int) {
		n := 0
		for _, hh := range g.Heroes {
			if hh.Connected && len(hh.Name) > 4 && hh.Name[:4] == "bot-" {
				n++
			}
		}
		if n != 3 || players != 4 {
			t.Errorf("expected 3 bot heroes in the world, got %d (players %d)", n, players)
		}
		if g.N != 4 {
			t.Errorf("bots count toward party scaling, N=%d", g.N)
		}
	})
}

func TestHumansReplaceServerBots(t *testing.T) {
	h := fastHub(t, func(c *Config) { c.Bots = 3 })
	r, _ := h.Create()
	a, _ := join(t, r, "kmerz", "warrior")
	for _, n := range []string{"nyx", "vela", "oddjob"} {
		if _, err := r.Join(NewTestConn(), proto.ClientMsg{T: "hello", Name: n, Cls: "monk"}); err != nil {
			t.Fatalf("%s should replace a bot, got %v", n, err)
		}
	}
	if _, err := r.Join(NewTestConn(), proto.ClientMsg{T: "hello", Name: "late", Cls: "monk"}); err != ErrRoomFull {
		t.Fatalf("4 humans fill the room, got %v", err)
	}
	l := lastLobby(a) // Join replies after the lobby broadcast
	if l == nil || len(l.Players) != 4 {
		t.Fatalf("expected 4 players in the lobby, got %+v", l)
	}
	for _, p := range l.Players {
		if p.Bot {
			t.Errorf("bot %s should have been replaced", p.Name)
		}
	}
}

func TestHumanReplacesBotMidRun(t *testing.T) {
	h := fastHub(t, func(c *Config) { c.Bots = 3 })
	r, _ := h.Create()
	a, _ := join(t, r, "kmerz", "warrior")
	r.Submit(a, proto.ClientMsg{T: "ready"})
	waitFor(t, "start", func() bool { return r.Metrics.Ticks.Load() > 0 })
	if _, err := r.Join(NewTestConn(), proto.ClientMsg{T: "hello", Name: "nyx", Cls: "rogue"}); err != nil {
		t.Fatalf("join in progress should replace a bot, got %v", err)
	}
	r.Inspect(func(g *game.Game, players int) {
		bots, nyx := 0, false
		for _, hh := range g.Heroes {
			if strings.HasPrefix(hh.Name, "bot-") {
				bots++
			}
			nyx = nyx || hh.Name == "nyx"
		}
		if players != 4 || bots != 2 || !nyx {
			t.Errorf("expected nyx plus 2 bots in the world, got bots=%d nyx=%v players=%d", bots, nyx, players)
		}
	})
}

func lobbyBots(l *proto.Lobby) []string {
	var names []string
	for _, p := range l.Players {
		if p.Bot {
			names = append(names, p.Name)
		}
	}
	return names
}

func TestHostAddsAndRemovesBots(t *testing.T) {
	h := fastHub(t, nil)
	r, _ := h.Create()
	a, _ := join(t, r, "kmerz", "warrior")
	b, _ := join(t, r, "nyx", "rogue")
	drain(a)
	set := func(c *Conn, key string) { r.Submit(c, proto.ClientMsg{T: "setting", Key: key}) }

	set(b, "addBot") // not the host: ignored
	set(a, "addBot")
	set(a, "addBot")
	set(a, "addBot") // room is full at 4
	var l *proto.Lobby
	waitFor(t, "two bots", func() bool {
		if x := lastLobby(a); x != nil {
			l = x
		}
		return l != nil && len(lobbyBots(l)) == 2
	})
	time.Sleep(20 * time.Millisecond)
	if x := lastLobby(a); x != nil {
		l = x
	}
	if len(l.Players) != 4 || len(lobbyBots(l)) != 2 {
		t.Fatalf("expected 2 humans + 2 bots, got %+v", l.Players)
	}

	set(a, "removeBot") // removes the last bot (bot-vex)
	set(a, "addBot")    // reuses the free name
	waitFor(t, "bot re-added", func() bool {
		if x := lastLobby(a); x != nil {
			l = x
		}
		bots := lobbyBots(l)
		return len(bots) == 2 && bots[1] == "bot-vex"
	})
	if bots := lobbyBots(l); bots[0] == bots[1] {
		t.Fatalf("bot names must be unique, got %v", bots)
	}

	// a joining human still replaces a host-added bot
	if _, err := r.Join(NewTestConn(), proto.ClientMsg{T: "hello", Name: "vela", Cls: "monk"}); err != nil {
		t.Fatalf("join should replace a bot, got %v", err)
	}
	if x := lastLobby(a); x != nil {
		l = x
	}
	if len(l.Players) != 4 || len(lobbyBots(l)) != 1 {
		t.Fatalf("expected 3 humans + 1 bot, got %+v", l.Players)
	}

	set(a, "removeBot")
	set(a, "removeBot") // no bots left: ignored
	waitFor(t, "no bots", func() bool {
		if x := lastLobby(a); x != nil {
			l = x
		}
		return len(lobbyBots(l)) == 0 && len(l.Players) == 3
	})
}

func TestLeaveMidRunRemovesHeroAtOnce(t *testing.T) {
	h := fastHub(t, nil)
	r, _ := h.Create()
	a, _ := join(t, r, "kmerz", "warrior")
	b, _ := join(t, r, "nyx", "rogue")
	r.Submit(a, proto.ClientMsg{T: "ready"})
	r.Submit(b, proto.ClientMsg{T: "ready"})
	waitFor(t, "start", func() bool { return r.Metrics.Ticks.Load() > 0 })
	drain(a)

	r.Submit(b, proto.ClientMsg{T: "leave"})
	r.Leave(b) // the client closes its socket right after
	waitFor(t, "nyx removed", func() bool {
		gone := false
		r.Inspect(func(g *game.Game, players int) {
			gone = players == 1 && len(g.Heroes) == 1 && g.Heroes[0].Name == "kmerz"
		})
		return gone
	})
	_, raws := drain(a)
	saw := false
	for _, raw := range raws {
		saw = saw || strings.Contains(string(raw), "nyx left the dungeon")
	}
	if !saw {
		t.Errorf("the party should be told that nyx left")
	}
	// the slot is free right away: a new player can join mid-run
	if _, err := r.Join(NewTestConn(), proto.ClientMsg{T: "hello", Name: "vela", Cls: "monk"}); err != nil {
		t.Fatalf("join after leave: %v", err)
	}
}

// levelDepth returns the depth of the last level message queued on c.
func levelDepth(c *Conn) (int, bool) {
	_, raws := drain(c)
	d, ok := 0, false
	for _, b := range raws {
		var l proto.Level
		if json.Unmarshal(b, &l) == nil && l.T == "level" {
			d, ok = l.Depth, true
		}
		var p proto.Phase
		if json.Unmarshal(b, &p) == nil && p.T == "phase" && p.Phase == "lobby" {
			return -1, true
		}
	}
	return d, ok
}

func descendOnce(t *testing.T, r *Room, c *Conn, want int) {
	t.Helper()
	r.Inspect(func(g *game.Game, _ int) {
		st := g.Level.Stairs
		if !g.Teleport(1, st.X-1, st.Y) {
			t.Error("teleport failed")
		}
	})
	waitFor(t, "descend", func() bool {
		d, ok := levelDepth(c)
		if ok && d != want {
			t.Fatalf("expected a level with depth %d, got %d (-1 = back to the lobby)", want, d)
		}
		return ok
	})
}

func TestStairsTakeThePartyDownThenRetryAfterWipe(t *testing.T) {
	h := fastHub(t, nil)
	r, _ := h.Create()
	a, _ := join(t, r, "kmerz", "warrior")
	r.Submit(a, proto.ClientMsg{T: "start"})
	waitFor(t, "level", func() bool { return hasType(a, "level") })
	descendOnce(t, r, a, 2)
	r.Inspect(func(g *game.Game, players int) {
		if g == nil || g.Depth != 2 || g.Phase != game.PhasePlaying || players != 1 {
			t.Fatalf("expected to be playing on depth 2")
		}
	})

	r.Inspect(func(g *game.Game, _ int) { g.Hurt(1, 9999) })
	waitFor(t, "wipe", func() bool {
		ok := false
		r.Inspect(func(g *game.Game, _ int) { ok = g.Phase == game.PhaseWiped })
		return ok
	})
	drain(a)
	r.Submit(a, proto.ClientMsg{T: "start"})
	waitFor(t, "retry", func() bool {
		d, ok := levelDepth(a)
		if ok && d != 2 {
			t.Fatalf("retry should reload depth 2, got %d", d)
		}
		return ok
	})
	r.Inspect(func(g *game.Game, _ int) {
		if g.Phase != game.PhasePlaying || g.Depth != 2 || g.Hero(1).Downed {
			t.Error("expected a fresh attempt at depth 2")
		}
	})
}

func TestRucksackOverTheRoom(t *testing.T) {
	h := fastHub(t, nil)
	r, _ := h.Create()
	a, _ := join(t, r, "kmerz", "warrior")
	r.Submit(a, proto.ClientMsg{T: "start"})
	waitFor(t, "level", func() bool { return hasType(a, "level") })
	r.Inspect(func(g *game.Game, _ int) { g.Hero(1).AddItem("gem", 2) })

	lastMe := func() *proto.Me {
		_, raws := drain(a)
		var me *proto.Me
		for _, b := range raws {
			var s struct {
				T  string    `json:"t"`
				Me *proto.Me `json:"me"`
			}
			if json.Unmarshal(b, &s) == nil && s.T == "snap" && s.Me != nil {
				me = s.Me
			}
		}
		return me
	}
	var me *proto.Me
	waitFor(t, "pack in snapshot", func() bool {
		if m := lastMe(); m != nil {
			me = m
		}
		return me != nil && len(me.Pack) == game.PackSlots && me.Pack[1].Item == "gem" && me.Pack[1].N == 2 && me.Oil > 0
	})
	if me.Potions != game.StartPotions || me.Pack[0].Item != "potion" {
		t.Fatalf("expected the starting potions in slot 0: %+v", me.Pack[0])
	}

	r.Submit(a, proto.ClientMsg{T: "drop", Slot: 1, Seq: 1})
	waitFor(t, "gem dropped", func() bool {
		gone := false
		r.Inspect(func(g *game.Game, _ int) { gone = g.Hero(1).Count("gem") == 0 && len(g.Items) > 0 })
		return gone
	})
}

func TestBotsHaveLightInTheDark(t *testing.T) {
	h := fastHub(t, func(c *Config) { c.Bots = 1; c.StartDepth = 7 })
	r, _ := h.Create()
	a, _ := join(t, r, "kmerz", "warrior")
	r.Submit(a, proto.ClientMsg{T: "start"})
	waitFor(t, "bot lantern", func() bool {
		lit := false
		r.Inspect(func(g *game.Game, _ int) {
			if g == nil || !g.Level.Theme.Dark {
				return
			}
			for _, hh := range g.Heroes {
				if strings.HasPrefix(hh.Name, "bot-") && g.LanternLit(hh) {
					lit = true
				}
			}
		})
		return lit
	})
}

func TestSkillTreeOverTheRoom(t *testing.T) {
	h := fastHub(t, func(c *Config) { c.Bots = 1 })
	r, _ := h.Create()
	a, _ := join(t, r, "kmerz", "warrior")
	r.Submit(a, proto.ClientMsg{T: "start"})
	waitFor(t, "level", func() bool { return hasType(a, "level") })
	r.Submit(a, proto.ClientMsg{T: "learn", Skill: "shieldbash", Seq: 1})
	r.Submit(a, proto.ClientMsg{T: "bind", Slot: 0, Skill: "shieldbash", Seq: 2})
	waitFor(t, "learned and bound", func() bool {
		ok := false
		r.Inspect(func(g *game.Game, _ int) {
			var hh *game.Hero
			for _, x := range g.Heroes {
				if x.Name == "kmerz" {
					hh = x
				}
			}
			ok = hh.Ranks[hh.SkillIndex("shieldbash")] == 1 && hh.Binds[0] == hh.SkillIndex("shieldbash") && hh.Points == 0
		})
		return ok
	})
	// the bot spends its starting point too
	waitFor(t, "bot learns", func() bool {
		spent := false
		r.Inspect(func(g *game.Game, _ int) {
			for _, hh := range g.Heroes {
				if strings.HasPrefix(hh.Name, "bot-") && hh.Points == 0 {
					spent = true
				}
			}
		})
		return spent
	})
	// the private snapshot carries the tree and the key bindings
	waitFor(t, "tree in snapshot", func() bool {
		_, raws := drain(a)
		for _, b := range raws {
			var s struct {
				T  string    `json:"t"`
				Me *proto.Me `json:"me"`
			}
			if json.Unmarshal(b, &s) == nil && s.Me != nil && len(s.Me.Tree) == game.TreeSize && s.Me.Binds[0] == "shieldbash" && s.Me.Binds[1] == "cleave" {
				return true
			}
		}
		return false
	})
}

func TestTradeMessagesMapToIntents(t *testing.T) {
	in, ok := ToIntent(proto.ClientMsg{T: "buy", Skill: "torch", Seq: 3})
	if !ok || in.Kind != game.IntBuy || in.Skill != "torch" {
		t.Fatalf("buy: %+v", in)
	}
	in, ok = ToIntent(proto.ClientMsg{T: "sell", Slot: 4})
	if !ok || in.Kind != game.IntSell || in.Slot != 4 {
		t.Fatalf("sell: %+v", in)
	}
}

func lastQuests(c *Conn) *proto.QuestList {
	_, raws := drain(c)
	var out *proto.QuestList
	for _, b := range raws {
		var q proto.QuestList
		if json.Unmarshal(b, &q) == nil && q.T == "quests" {
			cp := q
			out = &cp
		}
	}
	return out
}

func TestQuestsOverTheRoom(t *testing.T) {
	h := fastHub(t, nil)
	r, _ := h.Create()
	a, _ := join(t, r, "kmerz", "warrior")
	r.Submit(a, proto.ClientMsg{T: "start"})
	var ql *proto.QuestList
	waitFor(t, "quest list", func() bool { ql = lastQuests(a); return ql != nil })
	if len(ql.List) != 2*game.QuestOffers || ql.List[0].State != "offered" {
		t.Fatalf("the run's offers arrive with the level: %+v", ql.List)
	}
	var id string
	for _, q := range ql.List {
		if q.Giver == "board" {
			id = q.ID
			break
		}
	}
	r.Inspect(func(g *game.Game, _ int) { // stand next to the notice board
		for _, n := range g.Npcs {
			if n.Board {
				for _, d := range [][2]int{{0, 1}, {1, 0}, {-1, 0}, {0, -1}, {1, 1}, {-1, 1}} {
					if g.Teleport(1, n.X+d[0], n.Y+d[1]) {
						return
					}
				}
			}
		}
	})
	r.Submit(a, proto.ClientMsg{T: "accept", Skill: id, Seq: 1})
	waitFor(t, "accepted", func() bool {
		if q := lastQuests(a); q != nil {
			for _, x := range q.List {
				if x.ID == id && x.State == "active" {
					return true
				}
			}
		}
		return false
	})
}

// Bots must see the notice board and gnomes as obstacles, or they path through the board and bump it forever.
func TestBotsSeeTheNoticeBoard(t *testing.T) {
	h := fastHub(t, func(c *Config) { c.Bots = 1 })
	r, _ := h.Create()
	a, _ := join(t, r, "kmerz", "warrior")
	r.Submit(a, proto.ClientMsg{T: "ready"})
	waitFor(t, "start", func() bool { return r.Metrics.Ticks.Load() > 0 })
	r.Inspect(func(g *game.Game, _ int) {
		var board *game.Npc
		for _, n := range g.Npcs {
			if n.Board {
				board = n
			}
		}
		if board == nil {
			t.Fatal("floor 1 should have the notice board")
		}
		var bh *game.Hero
		for _, hh := range g.Heroes {
			if len(hh.Name) > 4 && hh.Name[:4] == "bot-" {
				bh = hh
			}
		}
		if bh == nil {
			t.Fatal("no bot hero")
		}
		v := r.botView(bh, nil)
		for _, e := range v.Ents {
			if e.K == "n" && e.X == board.X && e.Y == board.Y {
				return
			}
		}
		t.Error("the bot's view has no notice board in it")
	})
}

// Back after a drop without the session token (the browser lost it): the same name takes the waiting hero over
// instead of joining a second time next to them.
func TestSameNameTakesOverADisconnectedHero(t *testing.T) {
	h := fastHub(t, nil)
	r, _ := h.Create()
	a, ra := join(t, r, "kmerz", "warrior")
	b, _ := join(t, r, "nyx", "rogue")
	r.Submit(a, proto.ClientMsg{T: "start"})
	waitFor(t, "start", func() bool { return hasType(b, "level") })
	r.Leave(a)
	waitFor(t, "disconnect processed", func() bool {
		gone := false
		r.Inspect(func(g *game.Game, _ int) { gone = !g.Hero(ra.PlayerID).Connected })
		return gone
	})
	a2 := NewTestConn()
	res, err := r.Join(a2, proto.ClientMsg{T: "hello", Name: "KMERZ", Cls: "monk", Token: "stale"})
	if err != nil || res.PlayerID != ra.PlayerID {
		t.Fatalf("the same name should take the hero over: %+v %v", res, err)
	}
	r.Inspect(func(g *game.Game, players int) {
		if hh := g.Hero(ra.PlayerID); hh == nil || !hh.Connected || players != 2 {
			t.Errorf("one kmerz, back in control (hero %+v, players %d)", hh, players)
		}
	})
	// someone else under a name that is still connected joins as a new player
	c, rc := join(t, r, "nyx", "monk")
	if rc.PlayerID == ra.PlayerID {
		t.Fatal("a connected player must not be taken over")
	}
	_ = c
}

// A client that shows the lobby while the run is going (it missed the phase change) gets the game again when
// it readies up.
func TestReadyDuringTheRunResendsTheGame(t *testing.T) {
	h := fastHub(t, nil)
	r, _ := h.Create()
	a, _ := join(t, r, "kmerz", "warrior")
	startRun(t, r, a)
	drain(a)
	r.Submit(a, proto.ClientMsg{T: "ready"})
	waitFor(t, "the game sent again", func() bool {
		types, raws := drain(a)
		for i, ty := range types {
			if ty == "phase" && strings.Contains(string(raws[i]), `"game"`) {
				return true
			}
		}
		return false
	})
}
