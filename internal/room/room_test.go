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
	// readiness was reset
	l := lastLobby(a)
	r.Inspect(func(g *game.Game, _ int) {
		if g != nil {
			t.Error("game should be gone")
		}
	})
	_ = l
}
