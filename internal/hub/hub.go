package hub

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/lockyc/mycelium/internal/catalog"
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
