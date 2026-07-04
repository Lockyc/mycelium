package transport

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/lockyc/mycelium/internal/catalog"
)

func TestPushSendsAuthedManifest(t *testing.T) {
	var gotAuth, gotPath, gotNode string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		var m catalog.Manifest
		_ = json.Unmarshal(body, &m)
		gotNode = m.Node
		w.WriteHeader(200)
	}))
	defer srv.Close()

	err := Push(srv.URL, "sekret", catalog.Manifest{Node: "forgejo"})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer sekret" || gotPath != ManifestPath || gotNode != "forgejo" {
		t.Fatalf("auth=%q path=%q node=%q", gotAuth, gotPath, gotNode)
	}
}

func TestIngestStoresPerNodeAndRebuilds(t *testing.T) {
	dir := t.TempDir()
	rebuilt := 0
	h := IngestHandler(dir, "sekret", func() { rebuilt++ })
	srv := httptest.NewServer(h)
	defer srv.Close()

	// wrong token -> 401, no write
	if err := Push(srv.URL, "wrong", catalog.Manifest{Node: "forgejo"}); err == nil {
		t.Fatal("want error on bad token")
	}
	// good token -> stored keyed by node, rebuild called
	if err := Push(srv.URL, "sekret", catalog.Manifest{Node: "forgejo"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "forgejo.json")); err != nil {
		t.Fatalf("expected forgejo.json: %v", err)
	}
	if rebuilt != 1 {
		t.Fatalf("rebuilt=%d, want 1", rebuilt)
	}
	// re-push same node replaces (still one file)
	if err := Push(srv.URL, "sekret", catalog.Manifest{Node: "forgejo"}); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("want 1 manifest file, got %d", len(entries))
	}
}

func TestIngestRejectsEmptyNode(t *testing.T) {
	dir := t.TempDir()
	srv := httptest.NewServer(IngestHandler(dir, "", func() {}))
	defer srv.Close()
	// no token configured; empty node -> 400
	if err := Push(srv.URL, "", catalog.Manifest{Node: ""}); err == nil {
		t.Fatal("want error on empty node")
	}
}
