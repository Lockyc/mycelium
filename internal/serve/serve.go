package serve

import (
	"net/http"
	"path/filepath"

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
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dir, graph.MapName))
	})
	return mux
}
