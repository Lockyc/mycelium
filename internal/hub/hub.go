package hub

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lockyc/mycelium/internal/graph"
	"github.com/lockyc/mycelium/internal/serve"
	"github.com/lockyc/mycelium/internal/transport"
)

func loadManifests(dir string) ([]graph.Manifest, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var ms []graph.Manifest
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var m graph.Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		ms = append(ms, m)
	}
	return ms, nil
}

// Build reads all manifests + optional overlay, merges, and writes MAP.md
// and graph.json into outDir.
func Build(manifestsDir, overlayPath, outDir string) error {
	ms, err := loadManifests(manifestsDir)
	if err != nil {
		return err
	}
	var ov graph.Overlay
	if overlayPath != "" {
		data, err := os.ReadFile(overlayPath)
		if err != nil {
			return err
		}
		if ov, err = graph.ParseOverlay(data); err != nil {
			return err
		}
	}
	g := graph.Merge(ms, ov)
	// Stamp each doc-graph digest with a self-navigating link to its full payload
	// route, derived from the component id — so a graph.json consumer follows
	// `docGraph.url` instead of reconstructing /repos/<id>/docgraph.json. Serve-tier
	// concern, done at the hub (the node doesn't know the route).
	for i := range g.Components {
		if g.Components[i].DocGraph != nil {
			g.Components[i].DocGraph.URL = graph.RepoDocGraphRoute(g.Components[i].ID)
		}
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	jsonData, err := graph.RenderJSON(g)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, graph.GraphJSONName), jsonData, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, graph.MapName), []byte(graph.RenderMarkdown(g)), 0o644); err != nil {
		return err
	}
	return writeDocGraphs(outDir, ms)
}

// writeDocGraphs writes each component's full docgraph payload to
// <outDir>/repos/<id>/docgraph.json — the on-disk layout mirrors the served URL
// (serve.Handler's /repos/ route). First-seen wins across manifests (matching
// component dedup); the repos subtree is cleared first so a removed repo's stale
// payload never lingers. Non-atomic, consistent with Build's other writes.
func writeDocGraphs(outDir string, ms []graph.Manifest) error {
	reposDir := filepath.Join(outDir, "repos")
	if err := os.RemoveAll(reposDir); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, m := range ms {
		for id, payload := range m.DocGraphs {
			if seen[id] {
				continue
			}
			// Reject an id that isn't a clean relative path (defense against a
			// crafted manifest); canonical ids never contain "." segments or "..".
			// graph.SafeRelID is the single predicate shared with serve's read-time
			// guard on the same id → filesystem-path trust boundary.
			if !graph.SafeRelID(id) {
				continue
			}
			seen[id] = true
			dest := filepath.Join(reposDir, filepath.FromSlash(id), "docgraph.json")
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(dest, payload, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

// Handler builds the mux: the artifact routes (MAP.md, graph.json, and the
// root alias) plus the authenticated ingest endpoint, whose onIngest re-runs
// Build (serialized).
func Handler(manifestsDir, overlayPath, dir, ingestToken string) http.Handler {
	var mu sync.Mutex
	rebuild := func() error {
		mu.Lock()
		defer mu.Unlock()
		if err := Build(manifestsDir, overlayPath, dir); err != nil {
			fmt.Fprintln(os.Stderr, "hub: rebuild failed:", err)
			return err
		}
		return nil
	}
	mux := http.NewServeMux()
	mux.Handle(transport.ManifestPath, transport.IngestHandler(manifestsDir, ingestToken, rebuild))
	// static routes (/MAP.md, /graph.json, /) come last as the fallback.
	mux.Handle("/", serve.Handler(dir))
	return mux
}

// Serve builds the artifacts once then listens on addr, serving its static
// routes and the authenticated ingest endpoint. Blocks until the server exits.
func Serve(manifestsDir, overlayPath, dir, ingestToken, addr string) error {
	if err := Build(manifestsDir, overlayPath, dir); err != nil {
		return err
	}
	if ingestToken == "" {
		fmt.Fprintf(os.Stderr, "WARNING: ingest endpoint on %s is UNAUTHENTICATED "+
			"(no --ingest-token-file); anyone who can reach it can push manifests and trigger rebuilds\n", addr)
	}
	srv := &http.Server{
		Addr:    addr,
		Handler: Handler(manifestsDir, overlayPath, dir, ingestToken),
		// bound the header read so a slow client can't hold a connection open indefinitely.
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv.ListenAndServe()
}
