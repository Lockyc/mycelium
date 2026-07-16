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
	return os.WriteFile(filepath.Join(outDir, graph.MapName), []byte(graph.RenderMarkdown(g)), 0o644)
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
