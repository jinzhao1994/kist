// Package web serves the embedded browser application without external assets.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:static
var assets embed.FS

func Handler() http.Handler {
	root, err := fs.Sub(assets, "static/build")
	if err != nil {
		return unavailable()
	}
	if _, err := fs.Stat(root, "index.html"); err != nil {
		return unavailable()
	}
	return serve(root)
}
func unavailable() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Web assets are missing. Build with make build.", http.StatusServiceUnavailable)
	})
}
func serve(root fs.FS) http.Handler {
	files := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", 405)
			return
		}
		requested := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if requested == "." {
			requested = "index.html"
		}
		if info, err := fs.Stat(root, requested); err != nil || info.IsDir() {
			if path.Ext(requested) != "" || strings.HasPrefix(requested, "assets/") {
				http.NotFound(w, r)
				return
			}
			// Route fallbacks return the HTML document while asset misses stay 404.
			clone := r.Clone(r.Context())
			u := *r.URL
			u.Path = "/"
			clone.URL = &u
			files.ServeHTTP(w, clone)
			return
		}
		files.ServeHTTP(w, r)
	})
}
