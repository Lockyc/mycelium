package hub

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lockyc/mycelium/internal/catalog"
	"github.com/lockyc/mycelium/internal/serve"
	"github.com/lockyc/mycelium/internal/transport"
)

func loadManifests(dir string) ([]catalog.Manifest, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var ms []catalog.Manifest
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var m catalog.Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		ms = append(ms, m)
	}
	return ms, nil
}

// Build reads all manifests + optional overlay, merges, and writes CATALOG.md
// and catalog.json into outDir.
func Build(manifestsDir, overlayPath, outDir string) error {
	ms, err := loadManifests(manifestsDir)
	if err != nil {
		return err
	}
	var ov catalog.Overlay
	if overlayPath != "" {
		data, err := os.ReadFile(overlayPath)
		if err != nil {
			return err
		}
		if ov, err = catalog.ParseOverlay(data); err != nil {
			return err
		}
	}
	cat := catalog.Merge(ms, ov)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	jsonData, err := catalog.RenderJSON(cat)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "catalog.json"), jsonData, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "CATALOG.md"), []byte(catalog.RenderMarkdown(cat)), 0o644)
}

// Handler builds the mux: static catalog routes plus the authenticated ingest
// endpoint, whose onIngest re-runs Build (serialized).
func Handler(manifestsDir, overlayPath, catalogDir, ingestToken string) http.Handler {
	var mu sync.Mutex
	rebuild := func() error {
		mu.Lock()
		defer mu.Unlock()
		if err := Build(manifestsDir, overlayPath, catalogDir); err != nil {
			fmt.Fprintln(os.Stderr, "hub: rebuild failed:", err)
			return err
		}
		return nil
	}
	mux := http.NewServeMux()
	mux.Handle(transport.ManifestPath, transport.IngestHandler(manifestsDir, ingestToken, rebuild))
	// static catalog routes (/CATALOG.md, /catalog.json, /) come last as the fallback.
	mux.Handle("/", serve.Handler(catalogDir))
	return mux
}

// Serve builds the catalog once then listens on addr, serving static catalog
// routes and the authenticated ingest endpoint. Blocks until the server exits.
func Serve(manifestsDir, overlayPath, catalogDir, ingestToken, addr string) error {
	if err := Build(manifestsDir, overlayPath, catalogDir); err != nil {
		return err
	}
	if ingestToken == "" {
		fmt.Fprintf(os.Stderr, "WARNING: ingest endpoint on %s is UNAUTHENTICATED "+
			"(no --ingest-token-file); anyone who can reach it can push manifests and trigger rebuilds\n", addr)
	}
	return http.ListenAndServe(addr, Handler(manifestsDir, overlayPath, catalogDir, ingestToken))
}
