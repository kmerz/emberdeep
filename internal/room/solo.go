package room

import (
	"context"
	"encoding/json"
	"math/rand"
	"net/http"
	"time"

	"emberdeep/internal/game"
	"emberdeep/internal/proto"
)

// SoloHandler (Step 0 only) runs one private single-player game per websocket.
func SoloHandler(op OriginPolicy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws, err := acceptWS(w, r, op)
		if err != nil {
			return
		}
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		defer ws.CloseNow()
		_, data, err := ws.Read(ctx)
		if err != nil {
			return
		}
		var hello proto.ClientMsg
		if json.Unmarshal(data, &hello) != nil || hello.T != proto.CHello {
			return
		}
		cls := game.ClassID(hello.Cls)
		if !game.ValidClass(hello.Cls) {
			cls = game.Sorcerer
		}
		g := game.New(uint32(rand.Intn(1<<24)), game.Settings{}, 1)
		h := g.AddHero(1, "solo", cls)
		ob := NewOutbox()
		go writePump(ctx, ws, ob, nil)
		ob.PushEvent(mustJSON(proto.Welcome{T: "welcome", You: h.ID, Code: "SOLO", IsHost: true}))
		ob.PushEvent(mustJSON(proto.Phase{T: "phase", Phase: "game"}))
		ob.PushEvent(LevelMsg(g))
		in := make(chan proto.ClientMsg, 64)
		go func() {
			defer close(in)
			for {
				_, d, err := ws.Read(ctx)
				if err != nil {
					return
				}
				var m proto.ClientMsg
				if json.Unmarshal(d, &m) == nil {
					in <- m
				}
			}
		}()
		tk := time.NewTicker(50 * time.Millisecond)
		defer tk.Stop()
		for {
			select {
			case m, ok := <-in:
				if !ok {
					return
				}
				if it, ok := ToIntent(m); ok {
					_ = g.Apply(h.ID, it)
				}
			case <-tk.C:
				evs := g.Tick()
				if ex := g.TakeExplored(); len(ex) > 0 {
					ob.PushEvent(ExploreMsg(ex))
				}
				if len(evs) > 0 {
					ob.PushEvent(mustJSON(proto.Events{T: "events", Tick: g.Now, List: evs}))
				}
				ob.PushSnap(BuildPrivate(g, BuildShared(g), h.ID))
			}
		}
	}
}
