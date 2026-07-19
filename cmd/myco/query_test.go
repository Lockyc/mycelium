package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeGraph drops a minimal graph.json into a temp dir and returns the dir.
func writeGraph(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	const g = `{
  "components": [
    {"id":"github.com/acme/warden","name":"warden","commit":"a","summary":"terminals","kind":"app","status":"active","stack":["rust"],
     "provides":[{"name":"sidebar","summary":"the sidebar"}]}
  ],
  "capabilities": {"sidebar":["warden"]},
  "edges": [], "dangling_edges": [], "orphans": []
}`
	if err := os.WriteFile(filepath.Join(dir, "graph.json"), []byte(g), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()
	if err := fn(); err != nil {
		w.Close()
		t.Fatalf("fn error: %v", err)
	}
	w.Close()
	data, _ := io.ReadAll(r)
	return string(data)
}

func TestRunQueryIndexNoArgs(t *testing.T) {
	// bare `query` prints the index and exits 0 (no graph needed).
	if err := runQuery(nil); err != nil {
		t.Fatalf("bare query should succeed: %v", err)
	}
}

func TestRunQueryComponentsJSON(t *testing.T) {
	dir := writeGraph(t)
	// --json output must be valid JSON and contain the flat provides path.
	out := captureStdout(t, func() error {
		return runQuery([]string{"components", "--dir", dir, "--kind", "app", "--json"})
	})
	var comps []map[string]any
	if err := json.Unmarshal([]byte(out), &comps); err != nil {
		t.Fatalf("--json not valid JSON: %v\n%s", err, out)
	}
	if len(comps) != 1 || comps[0]["name"] != "warden" {
		t.Fatalf("expected warden, got: %s", out)
	}
}

func TestRunQueryUnknownCapabilityErrors(t *testing.T) {
	dir := writeGraph(t)
	// --dir sits AFTER the positional: interspersed-flag parsing must honor it, so
	// the graph actually loads and the error is the real "no such capability" — not
	// a file-not-found masquerading as success-of-the-assertion because a dropped
	// --dir fell back to ./graph.json.
	err := runQuery([]string{"capability", "nope", "--dir", dir})
	if err == nil {
		t.Fatal("unknown capability must return a non-nil error, not empty success")
	}
	if !strings.Contains(err.Error(), "no such capability") {
		t.Fatalf("expected a 'no such capability' error (proving --dir after the positional was honored), got: %v", err)
	}
}

// Flags must be honored in any position — Go's flag package stops at the first
// non-flag, so runQuery loop-parses to accept flags placed after the positional
// arg (the residual footgun this fixes: `myco query used-by config-core --url X`
// used to silently drop --url).
func TestRunQueryFlagsAfterPositional(t *testing.T) {
	dir := writeGraph(t)
	out := captureStdout(t, func() error {
		return runQuery([]string{"capability", "sidebar", "--dir", dir, "--json"})
	})
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("--json after the positional not honored / invalid JSON: %v\n%s", err, out)
	}
	if v["name"] != "sidebar" {
		t.Fatalf("expected capability sidebar (--dir after the positional honored), got: %s", out)
	}
}
