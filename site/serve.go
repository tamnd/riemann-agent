package site

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Handler serves a built site from dir as Cloudflare Pages does: a path that ends in / serves its index.html, and a missing file serves 404.html with the status 404.
func Handler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := path.Clean("/" + r.URL.Path)
		name := filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(p, "/")))
		if strings.HasSuffix(r.URL.Path, "/") {
			name = filepath.Join(name, "index.html")
		}
		if _, err := os.Stat(name); err != nil {
			data, rerr := os.ReadFile(filepath.Join(dir, "404.html")) // #nosec G304 -- the page of the built site
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			if rerr == nil {
				_, _ = w.Write(data)
			}
			return
		}
		if strings.HasSuffix(p, ".md") {
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		}
		files.ServeHTTP(w, r)
	})
}
