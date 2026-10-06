package activity

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// webappFS holds the static frontend (index.html, style.css, app.js, api.js and
// vendor/). all: pulls in files the default embed would skip (e.g. dotfiles);
// only the vendored SDK is guaranteed present at build time, and a missing
// index.html simply yields a 404 from the FileServer.
//
//go:embed all:webapp
var webappFS embed.FS

// webappHandler serves the embedded frontend. .mjs gets an explicit JS
// content-type (Go's mime table lacks it on some platforms), HTML is revalidated
// each load, and the immutable vendored SDK is cached for a day.
func webappHandler() (http.Handler, error) {
	sub, err := fs.Sub(webappFS, "webapp")
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, ".mjs") {
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		}
		// Only the vendored SDK is long-cached; app files revalidate on every
		// load so a redeploy is picked up immediately (no build hashes here).
		if strings.HasPrefix(path, "/vendor/") {
			w.Header().Set("Cache-Control", "max-age=86400")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		fileServer.ServeHTTP(w, r)
	}), nil
}
