package bot

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"emberdeep/internal/game"

	"emberdeep/internal/proto"
)

const (
	lw = 176
	lh = 62
)

// Client is a headless websocket player driven by a Brain.
type Client struct {
	URL, Room, Name, Cls string
	Origin               string
	Leader               bool
	learn                string // skill to spend the next point on (guarded by mu)

	Snaps, Events, Explores atomic.Int64
	Errors                  atomic.Int64
	LastTick                atomic.Int64
	LastErr                 atomic.Value // string

	mu    sync.Mutex
	tiles [lw * lh]byte
	vis   [lw * lh]bool
	you   int
	view  View
}

func (c *Client) Run(ctx context.Context) error {
	u := "ws" + strings.TrimPrefix(c.URL, "http") + "/ws?room=" + c.Room
	opts := &websocket.DialOptions{}
	if c.Origin != "" {
		opts.HTTPHeader = http.Header{"Origin": {c.Origin}}
	}
	ws, _, err := websocket.Dial(ctx, u, opts)
	if err != nil {
		return err
	}
	defer ws.CloseNow()
	ws.SetReadLimit(1 << 22)
	send := func(m proto.ClientMsg) error {
		b, _ := json.Marshal(m)
		wctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		return ws.Write(wctx, websocket.MessageText, b)
	}
	if err := send(proto.ClientMsg{T: proto.CHello, Name: c.Name, Cls: c.Cls}); err != nil {
		return err
	}
	brain := NewBrain(c.Leader)
	var inGame atomic.Bool
	errc := make(chan error, 1)
	go func() {
		for {
			_, data, err := ws.Read(ctx)
			if err != nil {
				errc <- err
				return
			}
			var m struct {
				T string `json:"t"`
			}
			if json.Unmarshal(data, &m) != nil {
				c.Errors.Add(1)
				continue
			}
			switch m.T {
			case "hb":
				var h proto.Hb
				_ = json.Unmarshal(data, &h)
				_ = send(proto.ClientMsg{T: proto.CHbAck, N: h.N})
			case "welcome":
				var w proto.Welcome
				_ = json.Unmarshal(data, &w)
				c.mu.Lock()
				c.you = w.You
				c.mu.Unlock()
				_ = send(proto.ClientMsg{T: proto.CReady})
			case "phase":
				var p proto.Phase
				_ = json.Unmarshal(data, &p)
				inGame.Store(p.Phase == "game")
			case "explore":
				var e proto.Explore
				_ = json.Unmarshal(data, &e)
				c.Explores.Add(1)
				c.mu.Lock()
				for _, t := range e.Tiles {
					c.tiles[t[1]*lw+t[0]] = byte(t[2])
				}
				c.mu.Unlock()
			case "events":
				c.Events.Add(1)
			case "error":
				var e proto.Error
				_ = json.Unmarshal(data, &e)
				c.LastErr.Store(e.Msg)
				c.Errors.Add(1)
			case "snap":
				c.onSnap(data)
			}
		}
	}()
	tk := time.NewTicker(150 * time.Millisecond)
	var lastLearn time.Time
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errc:
			if ctx.Err() != nil {
				return nil
			}
			return err
		case <-tk.C:
			if !inGame.Load() {
				continue
			}
			c.mu.Lock()
			v := c.view
			learn := c.learn
			v.Walkable = c.walkable
			v.Unknown = c.unknown
			c.mu.Unlock()
			if v.Me == 0 {
				continue
			}
			if learn != "" && time.Since(lastLearn) > 500*time.Millisecond {
				lastLearn = time.Now()
				if err := send(proto.ClientMsg{T: proto.CLearn, Skill: learn}); err != nil {
					return err
				}
			}
			for _, m := range brain.Think(&v) {
				if err := send(m); err != nil {
					return err
				}
			}
		}
	}
}

func (c *Client) walkable(x, y int) bool {
	if x < 0 || y < 0 || x >= lw || y >= lh {
		return false
	}
	t := c.tiles[y*lw+x]
	return t != 0 && t != '#' && t != 'T' && t != 'O' && t != ' '
}

func (c *Client) unknown(x, y int) bool {
	return x >= 0 && y >= 0 && x < lw && y < lh && c.tiles[y*lw+x] == 0
}

func (c *Client) onSnap(data []byte) {
	var s struct {
		proto.SnapShared
		AckSeq int       `json:"ackSeq"`
		Me     *proto.Me `json:"me"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		c.Errors.Add(1)
		c.LastErr.Store(fmt.Sprint(err))
		return
	}
	c.Snaps.Add(1)
	c.LastTick.Store(int64(s.Tick))
	_ = base64.StdEncoding
	c.mu.Lock()
	defer c.mu.Unlock()
	v := View{Me: c.you, Ents: make([]Ent, 0, len(s.Ents))}
	for _, e := range s.Ents {
		v.Ents = append(v.Ents, Ent{ID: e.ID, K: e.K, X: e.X, Y: e.Y, Downed: e.Downed, Dead: e.Dead, Elite: e.Elite, HP: e.HP, HPMax: e.HPMax})
		if e.K == "h" && e.ID == c.you {
			v.Cls, v.HP, v.HPMax = e.Cls, e.HP, e.HPMax
		}
	}
	if s.Me != nil {
		v.CD, v.Cost, v.Res, v.Potions, v.PotCD = s.Me.CD, s.Me.Cost, s.Me.Res, s.Me.Potions, s.Me.PotCD
		c.learn = ""
		if s.Me.Points > 0 && len(s.Me.Tree) == game.TreeSize {
			var ranks [game.TreeSize]int
			for i, t := range s.Me.Tree {
				ranks[i] = t.Rank
			}
			if idx := game.NextSkill(ranks, s.Me.Lvl); idx >= 0 {
				c.learn = s.Me.Tree[idx].ID
			}
		}
	}
	c.view = v
}
