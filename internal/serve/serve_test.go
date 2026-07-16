package serve

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeArtifacts(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "MAP.md"), []byte("# hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "graph.json"), []byte(`{"components":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHandlerServesArtifacts(t *testing.T) {
	dir := t.TempDir()
	writeArtifacts(t, dir)
	srv := httptest.NewServer(Handler(dir))
	defer srv.Close()

	// /MAP.md, /graph.json, and the root alias all serve 200.
	for _, path := range []string{"/MAP.md", "/graph.json", "/"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("GET %s: status %d, want 200", path, resp.StatusCode)
		}
	}

	// the old names are gone, not shadowed — a 200 here means a compat shim survived.
	for _, path := range []string{"/CATALOG.md", "/catalog.json"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s status %d, want 404 — old route still served", path, resp.StatusCode)
		}
	}
}

func TestHandlerNotFound(t *testing.T) {
	dir := t.TempDir()
	writeArtifacts(t, dir)
	srv := httptest.NewServer(Handler(dir))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/nope")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("GET /nope: status %d, want 404", resp.StatusCode)
	}
}
