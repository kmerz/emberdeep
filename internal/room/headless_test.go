package room

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"emberdeep/internal/bot"
	"emberdeep/internal/game"
)

// TestHeadlessFourBotsSixtySeconds plays a real 4-player run over websockets for 60 s (use -race).
func TestHeadlessFourBotsSixtySeconds(t *testing.T) {
	if testing.Short() {
		t.Skip("60 s headless run skipped in -short mode")
	}
	stats := &TickStats{}
	h := NewHub(Config{Stats: stats, Seed: 99})
	defer h.Close()
	srv := httptest.NewServer(h.Handler(nil))
	defer srv.Close()
	rm, err := h.Create()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	classes := []string{"warrior", "rogue", "sorcerer", "monk"}
	clients := make([]*bot.Client, 4)
	errs := make(chan error, 4)
	for i := range clients {
		clients[i] = &bot.Client{URL: srv.URL, Room: rm.Code(), Name: "hb" + string(rune('a'+i)), Cls: classes[i], Leader: true}
		go func(c *bot.Client) { errs <- c.Run(ctx) }(clients[i])
	}
	<-ctx.Done()
	for range clients {
		if err := <-errs; err != nil {
			t.Errorf("client error: %v", err)
		}
	}
	for i, c := range clients {
		if c.Snaps.Load() < 400 {
			t.Errorf("client %d received only %d snapshots", i, c.Snaps.Load())
		}
		if c.Errors.Load() != 0 {
			t.Errorf("client %d protocol errors: %d (%v)", i, c.Errors.Load(), c.LastErr.Load())
		}
		if c.Explores.Load() == 0 || c.Events.Load() == 0 {
			t.Errorf("client %d got no explore/events", i)
		}
	}
	if rm.Metrics.Ticks.Load() < 900 {
		t.Errorf("only %d ticks in 60 s (expected ~1200)", rm.Metrics.Ticks.Load())
	}
	var moved, explored int
	rm.Inspect(func(g *game.Game, _ int) {
		for _, hh := range g.Heroes {
			if hh.X != g.Level.Start.X || hh.Y != g.Level.Start.Y {
				moved++
			}
		}
		for _, e := range g.Explored {
			if e {
				explored++
			}
		}
	})
	if moved == 0 || explored < 400 {
		t.Errorf("bots should have explored: moved=%d explored=%d", moved, explored)
	}
	n, p50, p99, max := stats.Summary()
	t.Logf("ticks=%d p50=%v p99=%v max=%v explored=%d applied=%d rejected=%d", n, p50, p99, max, explored, rm.Metrics.Applied.Load(), rm.Metrics.Rejected.Load())
}
