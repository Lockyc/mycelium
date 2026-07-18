package hub

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lockyc/mycelium/internal/graph"
)

// TestBuildStampsDocGraphURL locks that graph.json's per-component digest carries
// a self-navigating `url` to its full doc-graph payload, derived from the id — so
// a consumer follows the link instead of reconstructing the route.
func TestBuildStampsDocGraphURL(t *testing.T) {
	manifestsDir := t.TempDir()
	outDir := t.TempDir()
	m := graph.Manifest{
		Node: "n",
		Components: []graph.Component{{
			ID:       "github.com/x/y",
			Name:     "y",
			Sidecar:  graph.Sidecar{Name: "y", Summary: "s"},
			DocGraph: &graph.DocGraphDigest{SchemaVersion: 1, DocCount: 3},
		}},
	}
	data, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(manifestsDir, "n.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(manifestsDir, "", outDir); err != nil {
		t.Fatal(err)
	}
	gj, err := os.ReadFile(filepath.Join(outDir, graph.GraphJSONName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gj), `"url": "/repos/github.com/x/y/docgraph.json"`) {
		t.Fatalf("graph.json digest missing self-navigating url:\n%s", gj)
	}
}
