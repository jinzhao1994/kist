// Package web serves the embedded browser application without external assets.
package web

import (
	"embed"
	"html"
	"io/fs"
	"net/http"
	"path"
	"strconv"
	"strings"
)

//go:embed all:static
var assets embed.FS

func Handler(primaryColor func() (string, error)) http.Handler {
	root, err := fs.Sub(assets, "static/build")
	if err != nil {
		return unavailable()
	}
	if _, err := fs.Stat(root, "index.html"); err != nil {
		return unavailable()
	}
	return serve(root, primaryColor)
}
func unavailable() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Web assets are missing. Build with make build.", http.StatusServiceUnavailable)
	})
}
func serve(root fs.FS, primaryColor func() (string, error)) http.Handler {
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
			// Readable archive routes can end in file extensions or dotted folder names.
			archiveRoute := strings.HasPrefix(requested, "boxes/")
			if (!archiveRoute && path.Ext(requested) != "") || strings.HasPrefix(requested, "assets/") {
				http.NotFound(w, r)
				return
			}
			// Route fallbacks return the HTML document while asset misses stay 404.
			requested = "index.html"
		}
		if requested == "index.html" {
			document, err := fs.ReadFile(root, "index.html")
			if err != nil {
				http.Error(w, "Web assets are unavailable.", http.StatusServiceUnavailable)
				return
			}
			color, err := primaryColor()
			if err != nil {
				http.Error(w, "Instance appearance is unavailable.", http.StatusServiceUnavailable)
				return
			}
			// A data-only attribute works with the existing script CSP.
			document = []byte(strings.Replace(string(document), `name="kist-primary-color" content=""`, `name="kist-primary-color" content="`+html.EscapeString(color)+`"`, 1))
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Length", strconv.Itoa(len(document)))
			if r.Method == http.MethodGet {
				_, _ = w.Write(document)
			}
			return
		}
		files.ServeHTTP(w, r)
	})
}
