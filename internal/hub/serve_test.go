package hub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/lockyc/mycelium/internal/graph"
	"github.com/lockyc/mycelium/internal/transport"
)

func TestHubHandlerIngestRebuildsAndServes(t *testing.T) {
	man := t.TempDir()
	out := t.TempDir()
	if err := Build(man, "", out); err != nil { // seed empty artifacts
		t.Fatal(err)
	}
	h := Handler(man, "", out, "sekret")
	srv := httptest.NewServer(h)
	defer srv.Close()

	// push a manifest -> hub stores it and rebuilds
	if err := transport.Push(srv.URL, "sekret", graph.Manifest{
		Node: "node-a",
		Components: []graph.Component{{ID: "github.com/acme/widgets", Name: "widgets",
			Sidecar: graph.Sidecar{Name: "widgets", Summary: "w"}}},
	}); err != nil {
		t.Fatal(err)
	}

	// the rebuilt graph.json now contains the pushed component
	resp, err := http.Get(srv.URL + "/graph.json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var g graph.Graph
	if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
		t.Fatal(err)
	}
	if len(g.Components) != 1 || g.Components[0].Name != "widgets" {
		t.Fatalf("graph after ingest = %+v", g.Components)
	}
	// stored keyed by node
	if _, err := os.Stat(filepath.Join(man, "node-a.json")); err != nil {
		t.Fatalf("no node manifest: %v", err)
	}
	// MAP.md served
	md, err := http.Get(srv.URL + "/MAP.md")
	if err != nil {
		t.Fatal(err)
	}
	defer md.Body.Close()
	if md.StatusCode != 200 {
		t.Fatalf("MAP.md status %d", md.StatusCode)
	}
}

// A fresh deployment starts before any node has pushed: Serve must create the
// manifests dir and build an empty graph rather than exit on the missing dir.
func TestServeStartsWithoutManifestsDir(t *testing.T) {
	man := filepath.Join(t.TempDir(), "manifests")
	out := t.TempDir()
	// An unbindable addr makes Serve return right after its startup build.
	err := Serve(man, "", out, "", "127.0.0.1:-1")
	if err == nil {
		t.Fatal("Serve on an unbindable addr returned nil")
	}
	if _, statErr := os.Stat(filepath.Join(out, graph.GraphJSONName)); statErr != nil {
		t.Fatalf("startup build did not run (Serve err = %v): %v", err, statErr)
	}
	if fi, statErr := os.Stat(man); statErr != nil || !fi.IsDir() {
		t.Fatalf("manifests dir not created: %v", statErr)
	}
}
