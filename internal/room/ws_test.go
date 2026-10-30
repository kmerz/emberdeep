package room

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"emberdeep/internal/proto"
)

type wsClient struct {
	t  *testing.T
	ws *websocket.Conn
}

func dialRoom(t *testing.T, srv *httptest.Server, code, name, cls, token string) *wsClient {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?room=" + code
	ws, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ws.CloseNow() })
	c := &wsClient{t, ws}
	c.send(proto.ClientMsg{T: "hello", Name: name, Cls: cls, Token: token})
	return c
}

func (c *wsClient) send(m proto.ClientMsg) {
	b, _ := json.Marshal(m)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.ws.Write(ctx, websocket.MessageText, b); err != nil {
		c.t.Fatal(err)
	}
}

// until reads messages until pred returns true or the timeout hits.
func (c *wsClient) until(what string, pred func(t string, raw []byte) bool) {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		_, b, err := c.ws.Read(ctx)
		if err != nil {
			c.t.Fatalf("waiting for %s: %v", what, err)
		}
		var m struct {
			T string `json:"t"`
		}
		_ = json.Unmarshal(b, &m)
		if m.T == "hb" {
			c.send(proto.ClientMsg{T: "hbAck", N: func() int { var h proto.Hb; _ = json.Unmarshal(b, &h); return h.N }()})
		}
		if pred(m.T, b) {
			return
		}
	}
}

func startServer(t *testing.T, mod func(*Config)) (*Hub, *httptest.Server) {
	h := fastHub(t, mod)
	srv := httptest.NewServer(h.Handler(nil))
	t.Cleanup(srv.Close)
	return h, srv
}

func TestWebsocketLobbyAndStart(t *testing.T) {
	h, srv := startServer(t, nil)
	rm, _ := h.Create()
	a := dialRoom(t, srv, rm.Code(), "kmerz", "warrior", "")
	b := dialRoom(t, srv, rm.Code(), "nyx", "rogue", "")
	// each tab sees the other in the lobby, with a measured ping
	a.until("two players", func(ty string, raw []byte) bool {
		var l proto.Lobby
		return ty == "lobby" && json.Unmarshal(raw, &l) == nil && len(l.Players) == 2
	})
	b.until("ping measured", func(ty string, raw []byte) bool {
		var l proto.Lobby
		return ty == "lobby" && json.Unmarshal(raw, &l) == nil && len(l.Players) == 2 && l.Players[1].PingMs >= 0 && l.Players[1].Connected
	})
	a.send(proto.ClientMsg{T: "ready"})
	b.send(proto.ClientMsg{T: "ready"})
	a.until("level", func(ty string, _ []byte) bool { return ty == "level" })
	b.until("snap", func(ty string, raw []byte) bool {
		var s proto.SnapShared
		return ty == "snap" && json.Unmarshal(raw, &s) == nil && len(s.Party) == 2
	})
}

func TestBadOriginRejected(t *testing.T) {
	h, srv := startServer(t, func(c *Config) { c.Origin = OriginPolicy{PublicURL: "https://play.example.com"} })
	rm, _ := h.Create()
	u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?room=" + rm.Code()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, resp, err := websocket.Dial(ctx, u, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": {"https://evil.example"}}})
	if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for a foreign origin, got err=%v resp=%v", err, resp)
	}
	ws, _, err := websocket.Dial(ctx, u, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": {"https://play.example.com"}}})
	if err != nil {
		t.Fatalf("PUBLIC_URL origin must be accepted: %v", err)
	}
	ws.CloseNow()
}

func TestOriginPolicy(t *testing.T) {
	mk := func(origin string) *http.Request {
		r := httptest.NewRequest("GET", "http://game.local/ws", nil)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		return r
	}
	p := OriginPolicy{PublicURL: "https://play.example.com", Dev: true}
	for origin, want := range map[string]bool{
		"": true, "http://game.local": true, "https://play.example.com": true, "http://localhost:5173": true,
		"https://evil.example": false, "http://play.example.com": false, "null": false,
	} {
		if got := p.Check(mk(origin)); got != want {
			t.Errorf("origin %q: got %v want %v", origin, got, want)
		}
	}
	if (OriginPolicy{}).Check(mk("http://localhost:5173")) {
		t.Error("vite origin must only be allowed with -dev")
	}
}

func TestCreateAndUnknownRoom(t *testing.T) {
	_, srv := startServer(t, nil)
	resp, err := http.Post(srv.URL+"/api/rooms", "application/json", nil)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("create: %v %v", err, resp)
	}
	var out struct{ Code string }
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if NormalizeCode(out.Code) != out.Code {
		t.Fatalf("bad code %q", out.Code)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws?room=ZZZ-ZZZ", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseNow()
	_, b, err := ws.Read(ctx)
	if err != nil || !strings.Contains(string(b), "no such room") {
		t.Fatalf("expected a no-such-room error, got %q %v", b, err)
	}
}
