package serve

import (
	"net/http"
	"path"
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
	// is a prefix route that validates the tail itself.
	const docGraphSuffix = "/docgraph.json"
	mux.HandleFunc("/repos/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/repos/")
		if !strings.HasSuffix(rest, docGraphSuffix) {
			http.NotFound(w, r)
			return
		}
		id := strings.TrimSuffix(rest, docGraphSuffix)
		// Reject traversal / non-canonical ids: the id must be a clean relative path.
		if id == "" || id != path.Clean(id) || strings.HasPrefix(id, ".") || strings.Contains(id, "..") {
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
