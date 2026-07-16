package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBuildProducesArtifacts(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "widgets")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, a := range [][]string{{"init", "-q"}, {"remote", "add", "origin", "git@github.com:acme/widgets.git"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "--allow-empty", "-q", "-m", "x"}} {
		c := exec.Command("git", a...)
		c.Dir = repo
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", a, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "mycelium.toml"), []byte("name=\"widgets\"\nsummary=\"cur\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	manDir := t.TempDir()
	outDir := t.TempDir()
	if err := runScan([]string{"--roots", root, "--node", "test", "--out", filepath.Join(manDir, "m.json")}); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if err := runBuild([]string{"--manifests", manDir, "--out", outDir}); err != nil {
		t.Fatalf("build: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "MAP.md")); err != nil {
		t.Fatalf("no MAP.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "graph.json")); err != nil {
		t.Fatalf("no graph.json: %v", err)
	}
}

// TestBuildThenAudit exercises the build -> audit round-trip: runAudit must
// be able to read whatever artifact runBuild actually writes. This is the
// regression gate for the artifact-name rename — it fails with a
// file-not-found error if runAudit reads a filename runBuild doesn't write.
func TestBuildThenAudit(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "widgets")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, a := range [][]string{{"init", "-q"}, {"remote", "add", "origin", "git@github.com:acme/widgets.git"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "--allow-empty", "-q", "-m", "x"}} {
		c := exec.Command("git", a...)
		c.Dir = repo
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", a, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "mycelium.toml"), []byte("name=\"widgets\"\nsummary=\"cur\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	manDir := t.TempDir()
	outDir := t.TempDir()
	if err := runScan([]string{"--roots", root, "--node", "test", "--out", filepath.Join(manDir, "m.json")}); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if err := runBuild([]string{"--manifests", manDir, "--out", outDir}); err != nil {
		t.Fatalf("build: %v", err)
	}

	err := runAudit([]string{"--catalog", outDir})
	if err != nil && errors.Is(err, os.ErrNotExist) {
		t.Fatalf("runAudit could not read the build artifact: %v", err)
	}
	// A non-nil, non-ErrNotExist error here just means the audit found
	// findings (e.g. an unreachable orphan) — that's expected output, not
	// a failure of this test, which only asserts the artifact was readable.
}
