package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"emberdeep/internal/proto"
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
	depth := flag.Int("depth", 1, "dev: start every run on this floor")
	saves := flag.String("saves", envOr("SAVE_DIR", "saves"), "folder for saved games (env SAVE_DIR; empty = no saving to disk)")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("emberdeep", proto.Version)
		return
	}
	maxRooms, _ := strconv.Atoi(envOr("MAX_ROOMS", "200"))
	public := os.Getenv("PUBLIC_URL")
	hub := room.NewHub(room.Config{
		MaxRooms: maxRooms, PublicURL: public, Origin: room.OriginPolicy{PublicURL: public, Dev: *dev},
		Lag: time.Duration(*lag) * time.Millisecond, Jitter: time.Duration(*jitter) * time.Millisecond, Bots: *bots,
		StartDepth: *depth, SaveDir: *saves,
	})
	log.Printf("emberdeep %s listening on %s (max rooms %d, bots %d, lag %dms±%dms, saves in %q)", proto.Version, *addr, maxRooms, *bots, *lag, *jitter, *saves)
	srv := &http.Server{Addr: *addr, Handler: hub.Handler(web.Handler())}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	// on Ctrl-C or a stop from the service manager, close every room so running games are saved
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Printf("stopping: saving running games")
	hub.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
