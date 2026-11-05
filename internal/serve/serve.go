package serve

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/lockyc/mycelium/internal/graph"
	"github.com/lockyc/mycelium/internal/query"
)

func Handler(dir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/"+graph.MapName, func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(dir, graph.MapName))
	})
	mux.HandleFunc("/"+graph.GraphJSONName, func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(dir, graph.GraphJSONName))
	})
	mux.HandleFunc("/q", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, query.Descriptors())
	})
	mux.HandleFunc("/q/", func(w http.ResponseWriter, r *http.Request) {
		g, err := loadGraph(dir)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "graph unavailable")
			return
		}
		rest := strings.TrimPrefix(r.URL.Path, "/q/")
		q, name, _ := strings.Cut(rest, "/")
		switch q {
		case "capabilities":
			writeJSON(w, http.StatusOK, query.Capabilities(g))
		case "capability":
			v, ok := query.Capability(g, name)
			if !ok {
				writeErr(w, http.StatusNotFound, "no such capability: "+name)
				return
			}
			writeJSON(w, http.StatusOK, v)
		case "component":
			e, ok := query.Component(g, name)
			if !ok {
				writeErr(w, http.StatusNotFound, "no such component or node: "+name)
				return
			}
			writeJSON(w, http.StatusOK, e)
		case "components":
			qs := r.URL.Query()
			writeJSON(w, http.StatusOK, query.Components(g, query.ComponentFilter{
				Kind: qs.Get("kind"), Stack: qs.Get("stack"), Status: qs.Get("status"), Tag: qs.Get("tag"),
			}))
		case "used-by":
			rels, ok := query.UsedBy(g, name)
			if !ok {
				writeErr(w, http.StatusNotFound, "no such component or capability: "+name)
				return
			}
			writeJSON(w, http.StatusOK, rels)
		case "uses":
			rels, ok := query.Uses(g, name)
			if !ok {
				writeErr(w, http.StatusNotFound, "no such component: "+name)
				return
			}
			writeJSON(w, http.StatusOK, rels)
		case "search":
			writeJSON(w, http.StatusOK, query.Search(g, r.URL.Query().Get("q")))
		default:
			writeErr(w, http.StatusNotFound, "no such query: "+q)
		}
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

func loadGraph(dir string) (graph.Graph, error) {
	data, err := os.ReadFile(filepath.Join(dir, graph.GraphJSONName))
	if err != nil {
		return graph.Graph{}, err
	}
	var g graph.Graph
	if err := json.Unmarshal(data, &g); err != nil {
		return graph.Graph{}, err
	}
	return g, nil
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
