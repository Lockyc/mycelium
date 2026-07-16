package hub

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lockyc/mycelium/internal/graph"
)

func TestBuildWritesArtifacts(t *testing.T) {
	man := t.TempDir()
	out := t.TempDir()
	m := graph.Manifest{Node: "n", Components: []graph.Component{
		{ID: "github.com/acme/widgets", Name: "widgets",
			Sidecar: graph.Sidecar{Name: "widgets", Summary: "w"}},
	}}
	data, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(man, "n.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(man, "", out); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"MAP.md", "graph.json"} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Fatalf("missing %s: %v", f, err)
		}
	}
}
