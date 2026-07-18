package serve

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	// Decoy old-named files: present on disk so that a surviving old ROUTE
	// serves 200 and trips the anti-shim assertion below. Without these, a
	// dual-write shim 404s here only because the fixture lacks the file.
	for _, old := range []string{"CATALOG.md", "catalog.json"} {
		if err := os.WriteFile(filepath.Join(dir, old), []byte("decoy"), 0o644); err != nil {
			t.Fatal(err)
		}
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

func TestServeDocGraphPayload(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "repos", "github.com", "x", "y", "docgraph.json")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte(`{"schemaVersion":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(Handler(dir))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/repos/github.com/x/y/docgraph.json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"schemaVersion":1`) {
		t.Fatalf("bad body: %s", body)
	}

	// non-docgraph.json path under /repos/ → 404 (no directory listing leak)
	bad, _ := http.Get(srv.URL + "/repos/github.com/x/y/")
	if bad.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404 for non-payload path, got %d", bad.StatusCode)
	}
	bad.Body.Close()

	// traversal attempt → 404
	trav, _ := http.Get(srv.URL + "/repos/../../etc/passwd/docgraph.json")
	if trav.StatusCode == http.StatusOK {
		t.Fatalf("traversal must not succeed, got %d", trav.StatusCode)
	}
	trav.Body.Close()
}

// The /repos/<id>/docgraph.json id guard's own unit coverage (rejecting
// traversal/leading-dot/empty ids, accepting well-formed ones) moved to
// graph.TestSafeRelID — this route now delegates to graph.SafeRelID, the
// single predicate shared with the hub's write-time guard.

func TestQueryRoutes(t *testing.T) {
	dir := t.TempDir()
	const g = `{
  "components":[{"id":"github.com/acme/warden","name":"warden","commit":"a","summary":"terminals","kind":"app","status":"active","stack":["rust"],"provides":[{"name":"sidebar","summary":"the sidebar"}]}],
  "capabilities":{"sidebar":["warden"]},
  "edges":[],"dangling_edges":[],"orphans":[]
}`
	if err := os.WriteFile(filepath.Join(dir, "graph.json"), []byte(g), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(Handler(dir))
	defer srv.Close()

	// /q index
	if body, code := get(t, srv.URL+"/q"); code != 200 || !strings.Contains(body, "capabilities") {
		t.Errorf("/q index wrong: %d %s", code, body)
	}
	// capability found
	if body, code := get(t, srv.URL+"/q/capability/sidebar"); code != 200 || !strings.Contains(body, "the sidebar") {
		t.Errorf("/q/capability wrong: %d %s", code, body)
	}
	// unknown → 404 with JSON error, NOT empty 200
	if body, code := get(t, srv.URL+"/q/capability/nope"); code != 404 || !strings.Contains(body, "error") {
		t.Errorf("unknown capability should 404 json: %d %s", code, body)
	}
	// components filter
	if body, code := get(t, srv.URL+"/q/components?kind=app"); code != 200 || !strings.Contains(body, "warden") {
		t.Errorf("/q/components filter wrong: %d %s", code, body)
	}
	// component result is flat (no sidecar wrapper), same as graph.json
	if body, _ := get(t, srv.URL+"/q/component/warden"); strings.Contains(body, `"sidecar"`) {
		t.Errorf("component result must be flat: %s", body)
	}
}

func get(t *testing.T, url string) (string, int) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b), resp.StatusCode
}
