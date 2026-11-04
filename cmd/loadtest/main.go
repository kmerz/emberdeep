// Command loadtest runs an in-process server with N rooms of 4 headless clients and reports tick-time percentiles.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"emberdeep/internal/bot"
	"emberdeep/internal/room"
)

func main() {
	rooms := flag.Int("rooms", 50, "number of rooms")
	per := flag.Int("clients", 4, "headless clients per room")
	dur := flag.Duration("duration", 30*time.Second, "how long to play")
	flag.Parse()

	stats := &room.TickStats{}
	hub := room.NewHub(room.Config{MaxRooms: *rooms + 10, Stats: stats, Quiet: true})
	defer hub.Close()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	srv := &http.Server{Handler: hub.Handler(nil)}
	go srv.Serve(ln)
	defer srv.Close()
	url := "http://" + ln.Addr().String()

	ctx, cancel := context.WithTimeout(context.Background(), *dur)
	defer cancel()
	classes := []string{"warrior", "rogue", "sorcerer", "monk"}
	var clients []*bot.Client
	var wg sync.WaitGroup
	var runErrs sync.Map
	for i := 0; i < *rooms; i++ {
		rm, err := hub.Create()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		for j := 0; j < *per; j++ {
			c := &bot.Client{URL: url, Room: rm.Code(), Name: fmt.Sprintf("r%dp%d", i, j), Cls: classes[j%4], Leader: true}
			clients = append(clients, c)
			wg.Add(1)
			go func(c *bot.Client) {
				defer wg.Done()
				if err := c.Run(ctx); err != nil {
					runErrs.Store(c.Name, err)
				}
			}(c)
		}
	}
	wg.Wait()
	var snaps, errs int64
	starved := 0
	for _, c := range clients {
		snaps += c.Snaps.Load()
		errs += c.Errors.Load()
		if c.Snaps.Load() == 0 {
			starved++
		}
	}
	failed := 0
	runErrs.Range(func(k, v any) bool { failed++; return true })
	n, p50, p99, max := stats.Summary()
	fmt.Printf("rooms=%d clients=%d duration=%v\n", *rooms, len(clients), *dur)
	fmt.Printf("snapshots received=%d protocol errors=%d clients without snaps=%d failed connections=%d\n", snaps, errs, starved, failed)
	fmt.Printf("tick time (sim + snapshot build, per room tick): count=%d p50=%v p99=%v max=%v (budget 50ms)\n", n, p50, p99, max)
	if p99 > 50*time.Millisecond {
		fmt.Println("FAIL: p99 exceeds the 50 ms tick budget")
		os.Exit(1)
	}
}
