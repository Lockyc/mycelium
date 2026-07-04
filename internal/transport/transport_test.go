package transport

import (
	"encoding/json"
	"errors"
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

	err := Push(srv.URL, "sekret", catalog.Manifest{Node: "node-a"})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer sekret" || gotPath != ManifestPath || gotNode != "node-a" {
		t.Fatalf("auth=%q path=%q node=%q", gotAuth, gotPath, gotNode)
	}
}

func TestIngestStoresPerNodeAndRebuilds(t *testing.T) {
	dir := t.TempDir()
	rebuilt := 0
	h := IngestHandler(dir, "sekret", func() error { rebuilt++; return nil })
	srv := httptest.NewServer(h)
	defer srv.Close()

	// wrong token -> 401, no write
	if err := Push(srv.URL, "wrong", catalog.Manifest{Node: "node-a"}); err == nil {
		t.Fatal("want error on bad token")
	}
	// good token -> stored keyed by node, rebuild called
	if err := Push(srv.URL, "sekret", catalog.Manifest{Node: "node-a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "node-a.json")); err != nil {
		t.Fatalf("expected node-a.json: %v", err)
	}
	if rebuilt != 1 {
		t.Fatalf("rebuilt=%d, want 1", rebuilt)
	}
	// re-push same node replaces (still one file)
	if err := Push(srv.URL, "sekret", catalog.Manifest{Node: "node-a"}); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("want 1 manifest file, got %d", len(entries))
	}
	if rebuilt != 2 {
		t.Fatalf("rebuilt=%d after re-push, want 2", rebuilt)
	}
}

func TestIngestRejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	h := IngestHandler(dir, "", nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	// Count files before attempt
	entriesBefore, _ := os.ReadDir(dir)
	countBefore := len(entriesBefore)

	// Try to push with path traversal node id
	if err := Push(srv.URL, "", catalog.Manifest{Node: "../evil"}); err == nil {
		t.Fatal("want error on path traversal node id")
	}

	// Verify no new file was created
	entriesAfter, _ := os.ReadDir(dir)
	countAfter := len(entriesAfter)
	if countAfter != countBefore {
		t.Fatalf("want %d files after rejection, got %d", countBefore, countAfter)
	}

	// Also test other unsafe node ids
	unsafeIds := []string{".", "..", "foo/bar", "baz\\qux"}
	for _, unsafeId := range unsafeIds {
		err := Push(srv.URL, "", catalog.Manifest{Node: unsafeId})
		if err == nil {
			t.Fatalf("want error on node id %q", unsafeId)
		}
	}
}

func TestIngestRejectsEmptyNode(t *testing.T) {
	dir := t.TempDir()
	srv := httptest.NewServer(IngestHandler(dir, "", func() error { return nil }))
	defer srv.Close()
	// no token configured; empty node -> 400
	if err := Push(srv.URL, "", catalog.Manifest{Node: ""}); err == nil {
		t.Fatal("want error on empty node")
	}
}

func TestIngestSurfacesRebuildError(t *testing.T) {
	dir := t.TempDir()
	rebuildErr := errors.New("build exploded")
	h := IngestHandler(dir, "", func() error { return rebuildErr })
	srv := httptest.NewServer(h)
	defer srv.Close()

	err := Push(srv.URL, "", catalog.Manifest{Node: "node-a"})
	if err == nil {
		t.Fatal("want error when rebuild fails")
	}
}
