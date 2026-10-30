package room

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

// Lagger adds artificial one-way latency. A message that arrived at time t is delivered at
// max(prevDeliver, t+lag+rand(0..jitter)), so jitter never reorders messages.
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

// DeliverAt returns the delivery time for a message that arrived at t.
func (l *Lagger) DeliverAt(t time.Time) time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()
	d := l.Lag
	if l.Jitter > 0 {
		d += time.Duration(rand.Int63n(int64(l.Jitter) + 1))
	}
	at := t.Add(d)
	if at.Before(l.prev) {
		at = l.prev
	}
	l.prev = at
	return at
}

// SleepUntil blocks until t or ctx ends.
func SleepUntil(ctx context.Context, t time.Time) {
	d := time.Until(t)
	if d <= 0 {
		return
	}
	tm := time.NewTimer(d)
	defer tm.Stop()
	select {
	case <-ctx.Done():
	case <-tm.C:
	}
}
