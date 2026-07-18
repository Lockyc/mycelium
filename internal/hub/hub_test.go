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

// TestBuildRejectsUnsafeDocGraphID proves the write-time trust boundary in
// writeDocGraphs: a manifest is untrusted input (pushed by any node over the
// ingest endpoint), so a crafted DocGraphs key must never let Build write
// outside outDir/repos. Mirrors serve's read-time guard test
// (TestServeDocGraphPayload's traversal case), but for the write side.
func TestBuildRejectsUnsafeDocGraphID(t *testing.T) {
	manifestsDir := t.TempDir()
	outDir := t.TempDir()
	m := graph.Manifest{
		Node: "n",
		Components: []graph.Component{
			{ID: "github.com/x/y", Name: "y", Sidecar: graph.Sidecar{Name: "y", Summary: "s"}},
		},
		DocGraphs: map[string]json.RawMessage{
			"github.com/x/y":   json.RawMessage(`{"schemaVersion":1,"nodes":[{"path":"README.md"}]}`),
			"../evil":          json.RawMessage(`{"schemaVersion":1,"nodes":[{"path":"pwned.md"}]}`),
			"../../etc/passwd": json.RawMessage(`{"schemaVersion":1,"nodes":[{"path":"pwned2.md"}]}`),
		},
	}
	data, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(manifestsDir, "n.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(manifestsDir, "", outDir); err != nil {
		t.Fatal(err)
	}

	// the safe id is written as usual.
	if _, err := os.Stat(filepath.Join(outDir, "repos", "github.com", "x", "y", "docgraph.json")); err != nil {
		t.Fatalf("safe id payload not written: %v", err)
	}

	// no file lands outside outDir/repos as a result of the unsafe ids.
	outside := []string{
		filepath.Join(outDir, "..", "evil", "docgraph.json"),
		filepath.Join(outDir, "evil", "docgraph.json"),
		filepath.Join(filepath.Dir(outDir), "evil"),
		filepath.Join(filepath.Dir(filepath.Dir(outDir)), "etc", "passwd", "docgraph.json"),
	}
	for _, p := range outside {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("unsafe id must not produce a file at %s (stat err = %v)", p, err)
		}
	}

	// the unsafe ids produced no docgraph.json anywhere under outDir beyond the
	// one expected safe payload.
	var found []string
	filepath.WalkDir(outDir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if filepath.Base(p) == "docgraph.json" {
			found = append(found, p)
		}
		return nil
	})
	if len(found) != 1 {
		t.Fatalf("want exactly 1 docgraph.json under outDir, got %d: %v", len(found), found)
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
