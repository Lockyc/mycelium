package hub

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestBuildWritesDocGraphPayloads(t *testing.T) {
	manifestsDir := t.TempDir()
	outDir := t.TempDir()
	m := graph.Manifest{
		Node:       "n",
		Components: []graph.Component{{ID: "github.com/x/y", Name: "y", Sidecar: graph.Sidecar{Name: "y", Summary: "s"}}},
		DocGraphs:  map[string]json.RawMessage{"github.com/x/y": json.RawMessage(`{"schemaVersion":1,"nodes":[{"path":"README.md"}]}`)},
	}
	data, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(manifestsDir, "n.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(manifestsDir, "", outDir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(outDir, "repos", "github.com", "x", "y", "docgraph.json"))
	if err != nil {
		t.Fatalf("payload not written: %v", err)
	}
	if !strings.Contains(string(got), `"README.md"`) {
		t.Fatalf("payload content wrong: %s", got)
	}
}

func TestBuildClearsStalePayloads(t *testing.T) {
	manifestsDir := t.TempDir()
	outDir := t.TempDir()
	stale := filepath.Join(outDir, "repos", "github.com", "x", "gone", "docgraph.json")
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte(`{"old":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	m := graph.Manifest{Node: "n"} // no components, no docgraphs
	data, _ := json.MarshalIndent(m, "", "  ")
	os.WriteFile(filepath.Join(manifestsDir, "n.json"), data, 0o644)
	if err := Build(manifestsDir, "", outDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale payload should be cleared, stat err = %v", err)
	}
}
