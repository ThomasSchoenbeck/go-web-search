package main

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"sync"
)

// The observability SPA is built by Vite into web/dist and embedded here.
//
// BUILD ORDERING: web/dist is gitignored and generated — `pnpm build` in web/
// MUST run before `go build`, which is why Taskfile.yaml chains them. Only
// web/dist/.gitkeep is committed, so a clean checkout still compiles; the
// binary then serves a "not built" notice instead of the UI. The `all:` prefix
// is what makes that work: without it go:embed skips dotfiles and an unbuilt
// web/dist would have no embeddable files at all. The same placeholder lives in
// web/public so that Vite, which empties dist/ on every build, copies it back.
//
//go:embed all:web/dist
var distEmbed embed.FS

// distFS is the embedded tree rooted at web/dist, so paths match request paths.
var distFS = func() fs.FS {
	sub, err := fs.Sub(distEmbed, "web/dist")
	if err != nil {
		// Only reachable if the go:embed directive above stops matching.
		panic("web/dist not embedded: " + err.Error())
	}
	return sub
}()

// spaBuilt reports whether a real Vite build was embedded, as opposed to the
// bare .gitkeep placeholder of a clean checkout.
var spaBuilt = sync.OnceValue(func() bool {
	_, err := fs.Stat(distFS, "index.html")
	return err == nil
})

const spaMissingNotice = `Observability UI not built.

This binary was compiled without a Vite build, so web/dist held only its
placeholder. Build the SPA first, then rebuild:

    pnpm --dir web install --frozen-lockfile
    pnpm --dir web build
    go build .

Or run "task build", which chains them in that order.

The REST and MCP APIs on this listener are unaffected.
`

// backendPath reports whether a path belongs to the API, MCP or health routes.
// Registered handlers already win over the SPA fallback by ServeMux precedence;
// this stops *unregistered* backend paths (a typo'd /api/run) from being
// answered with index.html, which would turn a 404 into a confusing 200.
func backendPath(p string) bool {
	return strings.HasPrefix(p, "/api/") ||
		p == "/mcp" || strings.HasPrefix(p, "/mcp/") ||
		p == "/healthz"
}

// handleSPA serves the embedded SPA: static assets when the path names one,
// index.html otherwise so History-API client routes (/runs, /logs/…) load the
// app instead of 404ing.
func (s *apiServer) handleSPA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if backendPath(r.URL.Path) {
		http.NotFound(w, r)
		return
	}
	if !spaBuilt() {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(spaMissingNotice))
		return
	}

	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name == "" || name == "." {
		name = "index.html"
	}
	if info, err := fs.Stat(distFS, name); err != nil || info.IsDir() {
		name = "index.html" // SPA fallback
	}
	http.ServeFileFS(w, r, distFS, name)
}
