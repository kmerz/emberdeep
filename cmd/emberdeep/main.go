package main

import (
	"flag"
	"log"
	"net/http"
	"os"

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
	addr := flag.String("addr", envOr("ADDR", ":8080"), "listen address")
	dev := flag.Bool("dev", false, "allow the Vite dev server origin")
	flag.Parse()
	op := room.OriginPolicy{PublicURL: os.Getenv("PUBLIC_URL"), Dev: *dev}
	mux := http.NewServeMux()
	mux.Handle("/ws", room.SoloHandler(op))
	mux.Handle("/", web.Handler())
	log.Printf("emberdeep listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
