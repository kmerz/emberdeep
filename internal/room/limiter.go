package room

import "time"

// Bucket is a token bucket (rate tokens/s, capacity burst).
type Bucket struct {
	rate, burst float64
	tokens      float64
	last        time.Time
}

func NewBucket(rate, burst float64, now time.Time) *Bucket {
	return &Bucket{rate: rate, burst: burst, tokens: burst, last: now}
}

func (b *Bucket) Allow(now time.Time) bool {
	if dt := now.Sub(b.last).Seconds(); dt > 0 {
		b.tokens = min(b.burst, b.tokens+dt*b.rate)
		b.last = now
	}
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}
