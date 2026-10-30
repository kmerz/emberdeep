package room

import "sync"

const maxFifo = 512

// Outbox is the per-connection send queue. Control messages and events go into a FIFO that is never
// silently dropped; snapshots go into a single slot so a slow client only ever sees the latest one.
type Outbox struct {
	mu      sync.Mutex
	fifo    [][]byte
	snap    []byte
	closed  bool
	notify  chan struct{}
	Dropped int // snapshots overwritten before being sent
}

func NewOutbox() *Outbox { return &Outbox{notify: make(chan struct{}, 1)} }

func (o *Outbox) wake() {
	select {
	case o.notify <- struct{}{}:
	default:
	}
}

// PushEvent queues a message that must not be lost. It returns false when the client is too slow
// (queue full) or the outbox is closed; the caller should then drop the connection.
func (o *Outbox) PushEvent(b []byte) bool {
	o.mu.Lock()
	if o.closed || len(o.fifo) >= maxFifo {
		o.mu.Unlock()
		return false
	}
	o.fifo = append(o.fifo, b)
	o.mu.Unlock()
	o.wake()
	return true
}

// PushSnap replaces the pending snapshot (drop-oldest).
func (o *Outbox) PushSnap(b []byte) {
	o.mu.Lock()
	if o.closed {
		o.mu.Unlock()
		return
	}
	if o.snap != nil {
		o.Dropped++
	}
	o.snap = b
	o.mu.Unlock()
	o.wake()
}

// Pop returns the next message: queued events first, then the latest snapshot.
func (o *Outbox) Pop() ([]byte, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.fifo) > 0 {
		b := o.fifo[0]
		o.fifo[0] = nil
		o.fifo = o.fifo[1:]
		return b, true
	}
	if o.snap != nil {
		b := o.snap
		o.snap = nil
		return b, true
	}
	return nil, false
}

func (o *Outbox) Notify() <-chan struct{} { return o.notify }

func (o *Outbox) Close() {
	o.mu.Lock()
	o.closed = true
	o.mu.Unlock()
	o.wake()
}

func (o *Outbox) Closed() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.closed
}

func (o *Outbox) Len() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.fifo)
}
