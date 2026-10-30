// Package web serves the embedded client build.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// Handler serves the SPA: real files from dist, index.html for everything else (e.g. /r/ASH-7Q2).
func Handler() http.Handler {
	sub, _ := fs.Sub(distFS, "dist")
	files := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" || strings.HasPrefix(r.URL.Path, "/r/") {
			p = "index.html"
		}
		if _, err := fs.Stat(sub, p); err != nil {
			p = "index.html"
		}
		if p == "index.html" {
			w.Header().Set("Cache-Control", "no-cache")
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			files.ServeHTTP(w, r2)
			return
		}
		files.ServeHTTP(w, r)
	})
}
