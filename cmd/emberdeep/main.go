package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"emberdeep/internal/room"
	"emberdeep/internal/web"
)

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func main() {
	addr := flag.String("addr", envOr("ADDR", ":8080"), "listen address (env ADDR)")
	dev := flag.Bool("dev", false, "allow the Vite dev server origin (http://localhost:5173)")
	bots := flag.Int("bots", 0, "fill N slots of every new room with AI teammates (joining players replace them)")
	lag := flag.Int("lag", 0, "artificial one-way latency in ms")
	jitter := flag.Int("jitter", 0, "artificial jitter in ms")
	flag.Parse()
	maxRooms, _ := strconv.Atoi(envOr("MAX_ROOMS", "200"))
	public := os.Getenv("PUBLIC_URL")
	hub := room.NewHub(room.Config{
		MaxRooms: maxRooms, PublicURL: public, Origin: room.OriginPolicy{PublicURL: public, Dev: *dev},
		Lag: time.Duration(*lag) * time.Millisecond, Jitter: time.Duration(*jitter) * time.Millisecond, Bots: *bots,
	})
	defer hub.Close()
	log.Printf("emberdeep listening on %s (max rooms %d, bots %d, lag %dms±%dms)", *addr, maxRooms, *bots, *lag, *jitter)
	log.Fatal(http.ListenAndServe(*addr, hub.Handler(web.Handler())))
}
