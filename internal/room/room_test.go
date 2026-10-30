package room

import (
	"encoding/json"
	"testing"
	"time"

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
