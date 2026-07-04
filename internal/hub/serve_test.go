package hub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/lockyc/mycelium/internal/catalog"
	"github.com/lockyc/mycelium/internal/transport"
)

func TestHubHandlerIngestRebuildsAndServes(t *testing.T) {
	man := t.TempDir()
	out := t.TempDir()
	if err := Build(man, "", out); err != nil { // seed empty catalog files
		t.Fatal(err)
	}
	h := Handler(man, "", out, "sekret")
	srv := httptest.NewServer(h)
	defer srv.Close()

	// push a manifest -> hub stores it and rebuilds
	if err := transport.Push(srv.URL, "sekret", catalog.Manifest{
		Node: "forgejo",
		Components: []catalog.Component{{ID: "github.com/acme/widgets", Name: "widgets",
			Sidecar: catalog.Sidecar{Name: "widgets", Summary: "w"}}},
	}); err != nil {
		t.Fatal(err)
	}

	// the rebuilt catalog.json now contains the pushed component
	resp, err := http.Get(srv.URL + "/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var cat catalog.Catalog
	if err := json.NewDecoder(resp.Body).Decode(&cat); err != nil {
		t.Fatal(err)
	}
	if len(cat.Components) != 1 || cat.Components[0].Name != "widgets" {
		t.Fatalf("catalog after ingest = %+v", cat.Components)
	}
	// stored keyed by node
	if _, err := os.Stat(filepath.Join(man, "forgejo.json")); err != nil {
		t.Fatalf("no node manifest: %v", err)
	}
	// CATALOG.md served
	md, _ := http.Get(srv.URL + "/CATALOG.md")
	defer md.Body.Close()
	if md.StatusCode != 200 {
		t.Fatalf("CATALOG.md status %d", md.StatusCode)
	}
}
