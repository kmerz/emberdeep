package room

import (
	"sort"
	"sync"
	"time"
)

// TickStats collects per-room tick durations (used by the load test).
type TickStats struct {
	mu sync.Mutex
	d  []time.Duration
}

func (s *TickStats) Record(d time.Duration) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.d = append(s.d, d)
	s.mu.Unlock()
}

// Summary returns count, p50, p99 and max.
func (s *TickStats) Summary() (n int, p50, p99, max time.Duration) {
	s.mu.Lock()
	d := append([]time.Duration(nil), s.d...)
	s.mu.Unlock()
	if len(d) == 0 {
		return
	}
	sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
	return len(d), d[len(d)/2], d[min(len(d)-1, len(d)*99/100)], d[len(d)-1]
}
