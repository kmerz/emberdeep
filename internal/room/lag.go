package room

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

// Lagger adds artificial one-way latency. Each call to Wait blocks until
// max(prevDeliver, now+lag+rand(0..jitter)), so jitter never reorders messages.
type Lagger struct {
	Lag, Jitter time.Duration
	mu          sync.Mutex
	prev        time.Time
}

func NewLagger(lag, jitter time.Duration) *Lagger {
	if lag <= 0 && jitter <= 0 {
		return nil
	}
	return &Lagger{Lag: lag, Jitter: jitter}
}

func (l *Lagger) deliverAt(now time.Time) time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()
	d := l.Lag
	if l.Jitter > 0 {
		d += time.Duration(rand.Int63n(int64(l.Jitter) + 1))
	}
	t := now.Add(d)
	if t.Before(l.prev) {
		t = l.prev
	}
	l.prev = t
	return t
}

// Wait blocks until the message may be delivered.
func (l *Lagger) Wait(ctx context.Context) {
	if l == nil {
		return
	}
	d := time.Until(l.deliverAt(time.Now()))
	if d <= 0 {
		return
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
