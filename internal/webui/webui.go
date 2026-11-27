// Package webui serves the web app built from web/ (see `make web`).
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// static holds the Vite build. It contains only .gitkeep until `make web` runs.
//
//go:embed all:static
var static embed.FS

// Handler serves the embedded web app.
func Handler() http.Handler {
	sub, err := fs.Sub(static, "static")
	if err != nil {
		panic(err) // the embed path is fixed at compile time
	}
	return handler(sub)
}

// handler serves files from fsys. Paths that aren't files get index.html, so
// the app's own addresses (/libraries/1/titles/2) work on reload. Unknown
// /api/ paths get a JSON 404 instead.
func handler(fsys fs.FS) http.Handler {
	files := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if strings.HasPrefix(p, "/api/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error": "no such API endpoint"}` + "\n"))
			return
		}

		name := strings.TrimPrefix(path.Clean(p), "/")
		if name != "" && name != "index.html" {
			if st, err := fs.Stat(fsys, name); err == nil && !st.IsDir() {
				if strings.HasPrefix(name, "assets/") {
					// Vite puts a content hash in every asset name.
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
		}

		index, err := fs.ReadFile(fsys, "index.html")
		if err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(notBuilt))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
}

const notBuilt = `<!doctype html>
<meta charset="utf-8">
<title>CineVault</title>
<body style="font-family:system-ui,sans-serif;background:#10161b;color:#ebe6da;padding:3rem;line-height:1.5">
<h1>The web interface isn't built</h1>
<p>This server was compiled without the web app. Run <code>make web</code> (or <code>make run</code>) in the CineVault folder, then restart the server.</p>
<p>The API works: <a style="color:#f2b84b" href="/api/health">/api/health</a></p>
</body>
`
