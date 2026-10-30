package room

import (
	"context"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Conn is one client connection. Reader/writer goroutines belong to it; the room only ever touches the
// outbox (mutex-guarded, never blocking) and Close.
type Conn struct {
	ws       *websocket.Conn
	Out      *Outbox
	PlayerID int // written by the room goroutine before Join returns
	cancel   context.CancelFunc

	once sync.Once

	bucket     *Bucket
	lastDrop   time.Time
	floodStart time.Time
	Dropped    int
}

func newConn(ws *websocket.Conn, cancel context.CancelFunc) *Conn {
	return &Conn{ws: ws, Out: NewOutbox(), cancel: cancel, bucket: NewBucket(30, 30, time.Now())}
}

// NewTestConn makes a connection without a websocket (for tests and bots); read its Out directly.
func NewTestConn() *Conn { return newConn(nil, nil) }

// Send queues a must-not-lose message. A client whose queue overflows is dropped.
func (c *Conn) Send(b []byte) bool {
	if !c.Out.PushEvent(b) {
		c.Close()
		return false
	}
	return true
}

func (c *Conn) SendSnap(b []byte) { c.Out.PushSnap(b) }

// Kick sends a final error and closes once it has been flushed.
func (c *Conn) Kick(msg string) {
	c.Out.PushEvent(mustJSON(struct {
		T   string `json:"t"`
		Msg string `json:"msg"`
	}{"error", msg}))
	c.Out.Close()
}

func (c *Conn) Close() {
	c.once.Do(func() {
		c.Out.Close()
		if c.ws != nil {
			c.ws.CloseNow()
		}
		if c.cancel != nil {
			c.cancel()
		}
	})
}

// allow applies the 30 intents/s limit; it returns (allowed, kick). Sustained flooding for 5 s kicks.
func (c *Conn) allow(now time.Time) (bool, bool) {
	if c.bucket.Allow(now) {
		return true, false
	}
	c.Dropped++
	if c.lastDrop.IsZero() || now.Sub(c.lastDrop) > time.Second {
		c.floodStart = now
	}
	c.lastDrop = now
	return false, now.Sub(c.floodStart) >= 5*time.Second
}
