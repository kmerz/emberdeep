package room

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"

	"emberdeep/internal/game"
	"emberdeep/internal/proto"
)

// OriginPolicy decides which websocket Origin headers are accepted.
type OriginPolicy struct {
	PublicURL string // e.g. https://play.example.com
	Dev       bool   // also allow the Vite dev server
}

// Check accepts a missing Origin (non-browser clients), the request Host, the PUBLIC_URL origin and,
// in dev mode, http://localhost:5173.
func (p OriginPolicy) Check(r *http.Request) bool {
	o := r.Header.Get("Origin")
	if o == "" {
		return true
	}
	u, err := url.Parse(o)
	if err != nil || u.Host == "" {
		return false
	}
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}
	if p.PublicURL != "" {
		if pu, err := url.Parse(p.PublicURL); err == nil && strings.EqualFold(pu.Host, u.Host) && pu.Scheme == u.Scheme {
			return true
		}
	}
	if p.Dev && (o == "http://localhost:5173" || o == "http://127.0.0.1:5173") {
		return true
	}
	return false
}

func acceptWS(w http.ResponseWriter, r *http.Request, p OriginPolicy) (*websocket.Conn, error) {
	if !p.Check(r) {
		http.Error(w, "bad origin", http.StatusForbidden)
		return nil, http.ErrAbortHandler
	}
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true}) // origin verified above
	if err != nil {
		return nil, err
	}
	c.SetReadLimit(8 << 10)
	return c, nil
}

// writePump drains the outbox to the websocket until ctx ends or the outbox closes.
func writePump(ctx context.Context, ws *websocket.Conn, ob *Outbox, lag *Lagger) error {
	for {
		for {
			b, at, ok := ob.PopTimed()
			if !ok {
				break
			}
			if lag != nil {
				SleepUntil(ctx, lag.DeliverAt(at))
			}
			wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := ws.Write(wctx, websocket.MessageText, b)
			cancel()
			if err != nil {
				return err
			}
		}
		if ob.Closed() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ob.Notify():
		}
	}
}

// ToIntent converts a gameplay client message into a game intent.
func ToIntent(m proto.ClientMsg) (game.Intent, bool) {
	switch m.T {
	case proto.CMove:
		return game.Intent{Kind: game.IntMove, Dir: m.Dir, Seq: m.Seq}, true
	case proto.CCast:
		return game.Intent{Kind: game.IntCast, Slot: m.Slot, X: m.X, Y: m.Y, Seq: m.Seq}, true
	case proto.CPotion:
		return game.Intent{Kind: game.IntPotion, Seq: m.Seq}, true
	case proto.CUse:
		return game.Intent{Kind: game.IntUse, Slot: m.Slot, X: m.X, Y: m.Y, Seq: m.Seq}, true
	case proto.CLearn:
		return game.Intent{Kind: game.IntLearn, Skill: m.Skill, Seq: m.Seq}, true
	case proto.CBind:
		return game.Intent{Kind: game.IntBind, Slot: m.Slot, Skill: m.Skill, Seq: m.Seq}, true
	case proto.CBuy:
		return game.Intent{Kind: game.IntBuy, Skill: m.Skill, Seq: m.Seq}, true
	case proto.CSell:
		return game.Intent{Kind: game.IntSell, Slot: m.Slot, Seq: m.Seq}, true
	case proto.CRespec:
		return game.Intent{Kind: game.IntRespec, Seq: m.Seq}, true
	case proto.CDrop:
		return game.Intent{Kind: game.IntDrop, Slot: m.Slot, Seq: m.Seq}, true
	case proto.CRevive:
		return game.Intent{Kind: game.IntRevive, Target: m.Target, Hold: m.Hold, Seq: m.Seq}, true
	}
	return game.Intent{}, false
}
