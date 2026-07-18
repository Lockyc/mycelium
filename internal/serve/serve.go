package serve

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/lockyc/mycelium/internal/graph"
)

func Handler(dir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/"+graph.MapName, func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(dir, graph.MapName))
	})
	mux.HandleFunc("/"+graph.GraphJSONName, func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(dir, graph.GraphJSONName))
	})
	// Per-repo full docgraph payload: GET /repos/<id>/docgraph.json, served from
	// the mirrored on-disk tree <dir>/repos/<id>/docgraph.json (hub writes it).
	// The id spans multiple path segments (canonical ids contain slashes), so this
	// is a prefix route that validates the tail itself. Prefix/suffix come from the
	// one route source (graph.RepoDocGraphRoute stamps the same into each digest url).
	mux.HandleFunc(graph.RepoDocGraphPrefix, func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, graph.RepoDocGraphPrefix)
		if !strings.HasSuffix(rest, graph.RepoDocGraphSuffix) {
			http.NotFound(w, r)
			return
		}
		id := strings.TrimSuffix(rest, graph.RepoDocGraphSuffix)
		// graph.SafeRelID guards no traversal, no leading dot, no empty id — the
		// route's mux already clean-redirects dirty paths before the handler runs,
		// but this is the defense that survives a future router swap.
		if !graph.SafeRelID(id) {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dir, "repos", filepath.FromSlash(id), "docgraph.json"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dir, graph.MapName))
	})
	return mux
}
